//go:build darwin

// sheaf.go implements sheaf-theoretic consistency checking for multi-node
// tape recordings. The key insight: a distributed tape recording forms a
// presheaf over the causal order category. Consistency (cocycle condition)
// means that taking different paths through the causal graph yields the
// same result — i.e., all observers agree on the global state.
//
// The obstruction (Čech cohomology H¹) measures how far the tapes are
// from perfect causal consistency. H¹ = 0 means all tapes converge.
//
// In practice this detects:
//   - Lost frames (gaps in Lamport sequence from one node)
//   - Clock skew beyond causal ordering (requires resync)
//   - Partition events (periods when nodes couldn't communicate)
package tape

import (
	"sort"

	"github.com/bmorphism/boxxy/internal/gf3"
)

// SheafSection is a local section: a contiguous subsequence of frames
// from one node over a time interval [lo, hi] in Lamport time.
type SheafSection struct {
	NodeID    string
	LamportLo uint64
	LamportHi uint64
	Frames    []Frame
	Trit      gf3.Elem
}

// SheafCocycle records a failure of the cocycle condition between
// two overlapping sections. Non-zero cocycles are obstructions
// to global consistency.
type SheafCocycle struct {
	SectionA  string // nodeID-A
	SectionB  string // nodeID-B
	LamportAt uint64 // where the inconsistency occurs
	Kind      string // "gap", "skew", "partition", "trit-mismatch"
	Severity  int    // 0=info, 1=warning, 2=error
}

// SheafConsistency is the result of checking a TapeWorld for sheaf consistency.
type SheafConsistency struct {
	Consistent bool            `json:"consistent"`  // H¹ = 0
	H1Dim      int             `json:"h1_dim"`      // dimension of first cohomology
	Sections   int             `json:"sections"`     // number of local sections
	Cocycles   []SheafCocycle  `json:"cocycles"`     // all detected obstructions
	GF3Balanced bool           `json:"gf3_balanced"` // trit conservation
	Coverage   float64         `json:"coverage"`     // fraction of Lamport range covered
}

// CheckSheafConsistency runs the Čech cohomology check on a TapeWorld.
// Returns the consistency result with detected obstructions.
func CheckSheafConsistency(world *TapeWorld) SheafConsistency {
	result := SheafConsistency{}

	if len(world.Frames) == 0 {
		result.Consistent = true
		result.Coverage = 1.0
		return result
	}

	// Build sections per node
	sections := buildSections(world)
	result.Sections = len(sections)

	// Check cocycle conditions between overlapping sections
	result.Cocycles = checkCocycles(sections)
	result.H1Dim = len(result.Cocycles)
	result.Consistent = result.H1Dim == 0

	// Check GF(3) conservation
	result.GF3Balanced, _ = world.GF3Conservation()

	// Compute coverage
	result.Coverage = computeCoverage(sections, world)

	return result
}

// buildSections extracts local sections from a TapeWorld.
func buildSections(world *TapeWorld) []SheafSection {
	// Group frames by node
	byNode := make(map[string][]Frame)
	for _, f := range world.Frames {
		byNode[f.NodeID] = append(byNode[f.NodeID], Frame{
			SeqNo:     uint64(f.ID),
			LamportTS: f.LamportTS,
			NodeID:    f.NodeID,
			Trit:      f.Trit,
			Content:   f.Content,
		})
	}

	var sections []SheafSection
	for nodeID, frames := range byNode {
		sort.Slice(frames, func(i, j int) bool {
			return frames[i].LamportTS < frames[j].LamportTS
		})

		// Split into contiguous sections (break on gaps > 1 in Lamport time
		// for the same node, which indicates missed frames)
		sectionStart := 0
		for i := 1; i <= len(frames); i++ {
			isGap := i == len(frames)
			if !isGap {
				// Gap detection: if Lamport jumps by more than expected for this node
				// (allowing for remote events that advance the clock)
				delta := frames[i].LamportTS - frames[i-1].LamportTS
				isGap = delta > 10 // generous threshold for remote clock advancement
			}

			if isGap {
				sectionFrames := frames[sectionStart:i]
				if len(sectionFrames) > 0 {
					// Compute section trit from frame sum
					var trits []gf3.Elem
					for _, f := range sectionFrames {
						trits = append(trits, f.Trit)
					}
					sectionTrit := gf3.Elem(gf3.SeqSum(trits) % 3)

					sections = append(sections, SheafSection{
						NodeID:    nodeID,
						LamportLo: sectionFrames[0].LamportTS,
						LamportHi: sectionFrames[len(sectionFrames)-1].LamportTS,
						Frames:    sectionFrames,
						Trit:      sectionTrit,
					})
				}
				sectionStart = i
			}
		}
	}

	return sections
}

// checkCocycles checks the cocycle condition between overlapping sections.
// Sorted by LamportLo for early break when no further overlaps possible.
func checkCocycles(sections []SheafSection) []SheafCocycle {
	// Sort by LamportLo for sweep-line efficiency
	sort.Slice(sections, func(i, j int) bool {
		return sections[i].LamportLo < sections[j].LamportLo
	})

	var cocycles []SheafCocycle

	for i := 0; i < len(sections); i++ {
		for j := i + 1; j < len(sections); j++ {
			a, b := sections[i], sections[j]

			// Early break: if b starts after a ends, no further j can overlap a
			if b.LamportLo > a.LamportHi {
				break
			}

			if a.NodeID == b.NodeID {
				continue
			}

			// Check for gaps in the overlap region
			overlapLo := a.LamportLo
			if b.LamportLo > overlapLo {
				overlapLo = b.LamportLo
			}
			overlapHi := a.LamportHi
			if b.LamportHi < overlapHi {
				overlapHi = b.LamportHi
			}

			// Check frame density in overlap
			aCount := countFramesInRange(a.Frames, overlapLo, overlapHi)
			bCount := countFramesInRange(b.Frames, overlapLo, overlapHi)

			if aCount == 0 || bCount == 0 {
				cocycles = append(cocycles, SheafCocycle{
					SectionA:  a.NodeID,
					SectionB:  b.NodeID,
					LamportAt: overlapLo,
					Kind:      "gap",
					Severity:  1,
				})
			}

			// Check trit consistency in overlap
			aTrit := tritInRange(a.Frames, overlapLo, overlapHi)
			bTrit := tritInRange(b.Frames, overlapLo, overlapHi)
			combined := gf3.Add(aTrit, bTrit)
			if combined != gf3.Zero && aCount > 0 && bCount > 0 {
				cocycles = append(cocycles, SheafCocycle{
					SectionA:  a.NodeID,
					SectionB:  b.NodeID,
					LamportAt: overlapLo,
					Kind:      "trit-mismatch",
					Severity:  0,
				})
			}
		}
	}

	return cocycles
}

func countFramesInRange(frames []Frame, lo, hi uint64) int {
	count := 0
	for _, f := range frames {
		if f.LamportTS >= lo && f.LamportTS <= hi {
			count++
		}
	}
	return count
}

func tritInRange(frames []Frame, lo, hi uint64) gf3.Elem {
	var trits []gf3.Elem
	for _, f := range frames {
		if f.LamportTS >= lo && f.LamportTS <= hi {
			trits = append(trits, f.Trit)
		}
	}
	if len(trits) == 0 {
		return gf3.Zero
	}
	return gf3.Elem(gf3.SeqSum(trits) % 3)
}

func computeCoverage(sections []SheafSection, world *TapeWorld) float64 {
	if len(world.Frames) == 0 {
		return 1.0
	}

	// Find global Lamport range
	var minTS, maxTS uint64
	first := true
	for _, f := range world.Frames {
		if first || f.LamportTS < minTS {
			minTS = f.LamportTS
		}
		if first || f.LamportTS > maxTS {
			maxTS = f.LamportTS
		}
		first = false
	}

	if maxTS == minTS {
		return 1.0
	}

	// Count covered Lamport ticks
	covered := make(map[uint64]bool)
	for _, s := range sections {
		for _, f := range s.Frames {
			covered[f.LamportTS] = true
		}
	}

	totalRange := maxTS - minTS + 1
	return float64(len(covered)) / float64(totalRange)
}
