//go:build darwin

// Narratives: sheaves on time categories for temporal scan data.
//
// Each scan produces a snapshot at time t. The narrative tracks
// how the network evolves over intervals [a,b], satisfying the
// Bumpus sheaf condition:
//
//   F([a,b]) = F([a,p]) ×_{F([p,p])} F([p,b])
//
// For GF(3)-valued narratives, this becomes:
//   F([a,b]) = F([a,p]) + F([p,b])  (in GF(3))
//
// Reference: Bumpus et al. "Unified Framework for Time-Varying Data"
package scan

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// Snapshot is a single point in the scan narrative: F([t,t]).
type Snapshot struct {
	Time    time.Time  `json:"time"`
	Index   int        `json:"index"`
	Devices []Device   `json:"devices"`
	GF3Sum  int        `json:"gf3_sum"`  // sum of all device trits mod 3
	GF9Sum  Nonet      `json:"gf9_sum"`  // sum of all device nonets
}

// Interval represents [a,b] in the time category I_N.
type Interval struct {
	Start int `json:"start"`
	End   int `json:"end"`
}

func (iv Interval) String() string { return fmt.Sprintf("[%d,%d]", iv.Start, iv.End) }

// IntervalValue is the sheaf value F([a,b]): the GF(3)/GF(9) summary over an interval.
type IntervalValue struct {
	Interval Interval `json:"interval"`
	GF3      int      `json:"gf3"`       // trit_sum_range of snapshots
	GF9      Nonet    `json:"gf9"`       // nonet_sum_range
	Devices  int      `json:"n_devices"` // max devices seen in interval
}

// Narrative is a sheaf F: I_N → (GF(3)-graded Device sets).
// Snapshots are the point values F([t,t]).
// Interval values are computed from the sheaf condition.
type Narrative struct {
	Snapshots []Snapshot               `json:"snapshots"`
	Values    map[string]IntervalValue `json:"values,omitempty"` // keyed by "[a,b]"
}

// NewNarrative creates an empty narrative.
func NewNarrative() *Narrative {
	return &Narrative{
		Values: make(map[string]IntervalValue),
	}
}

// AddSnapshot appends a scan result as a new point in the narrative.
func (n *Narrative) AddSnapshot(result *ScanResult) {
	gf3 := 0
	gf9 := Nonet{0, 0}
	for _, d := range result.Devices {
		gf3 = tritAdd(gf3, d.GF3Trit)
		if d.GF9Nonet != nil {
			gf9 = NonetAdd(gf9, *d.GF9Nonet)
		}
	}

	idx := len(n.Snapshots)
	snap := Snapshot{
		Time:    result.Timestamp,
		Index:   idx,
		Devices: result.Devices,
		GF3Sum:  gf3,
		GF9Sum:  gf9,
	}
	n.Snapshots = append(n.Snapshots, snap)

	// Update sheaf values for all intervals ending at this snapshot
	iv := Interval{idx, idx}
	n.Values[iv.String()] = IntervalValue{
		Interval: iv,
		GF3:      gf3,
		GF9:      gf9,
		Devices:  len(result.Devices),
	}

	// Extend all intervals [a, idx-1] to [a, idx] via sheaf condition
	for a := 0; a < idx; a++ {
		prevIv := Interval{a, idx - 1}
		prev, ok := n.Values[prevIv.String()]
		if !ok {
			continue
		}
		newIv := Interval{a, idx}
		n.Values[newIv.String()] = IntervalValue{
			Interval: newIv,
			GF3:      tritAdd(prev.GF3, gf3),
			GF9:      NonetAdd(prev.GF9, gf9),
			Devices:  max(prev.Devices, len(result.Devices)),
		}
	}
}

// SheafValue returns F([a,b]) — the narrative's value on an interval.
func (n *Narrative) SheafValue(a, b int) (IntervalValue, bool) {
	v, ok := n.Values[Interval{a, b}.String()]
	return v, ok
}

// VerifySheafCondition checks the Bumpus fibered product condition:
//
//	F([a,b]) = F([a,p]) ×_{F([p,p])} F([p,b])
//
// In GF(3) this becomes: F([a,b]) = F([a,p]) + F([p,b]) - F([p,p])
// because the point p is shared between both sub-intervals.
func (n *Narrative) VerifySheafCondition() []string {
	var violations []string
	for _, v := range n.Values {
		a, b := v.Interval.Start, v.Interval.End
		for p := a; p <= b; p++ {
			left, okL := n.SheafValue(a, p)
			right, okR := n.SheafValue(p, b)
			mid, okM := n.SheafValue(p, p)
			if !okL || !okR || !okM {
				continue
			}
			// Fibered product: left + right - overlap
			expected := tritAdd(left.GF3, tritAdd(right.GF3, tritNeg(mid.GF3)))
			if expected != v.GF3 {
				violations = append(violations,
					fmt.Sprintf("H⁰ obstruction: F(%s)=%d ≠ F([%d,%d])+F([%d,%d])-F([%d,%d])=%d+%d-%d=%d",
						v.Interval, v.GF3, a, p, p, b, p, p,
						left.GF3, right.GF3, mid.GF3, expected))
			}
		}
	}
	return violations
}

// FrobeniusFixed returns intervals whose GF(9) classification lies in GF(3).
// These are the Frobenius-fixed points of the narrative.
func (n *Narrative) FrobeniusFixed() []Interval {
	var fixed []Interval
	for _, v := range n.Values {
		frob := v.GF9.Frobenius()
		if frob == v.GF9 {
			fixed = append(fixed, v.Interval)
		}
	}
	sort.Slice(fixed, func(i, j int) bool {
		if fixed[i].Start != fixed[j].Start {
			return fixed[i].Start < fixed[j].Start
		}
		return fixed[i].End < fixed[j].End
	})
	return fixed
}

// IsBalanced checks if the full narrative is GF(3)-balanced (F([0,n]) = 0).
func (n *Narrative) IsBalanced() bool {
	if len(n.Snapshots) == 0 {
		return true
	}
	last := len(n.Snapshots) - 1
	v, ok := n.SheafValue(0, last)
	if !ok {
		return false
	}
	return v.GF3 == 0
}

// Save persists the narrative to disk.
func (n *Narrative) Save(dir string) error {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	path := filepath.Join(dir, "narrative.json")
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	return enc.Encode(n)
}

// Load reads a narrative from disk.
func LoadNarrative(dir string) (*Narrative, error) {
	path := filepath.Join(dir, "narrative.json")
	f, err := os.Open(path)
	if err != nil {
		return NewNarrative(), nil // fresh narrative if none exists
	}
	defer f.Close()
	var n Narrative
	if err := json.NewDecoder(f).Decode(&n); err != nil {
		return nil, err
	}
	if n.Values == nil {
		n.Values = make(map[string]IntervalValue)
	}
	return &n, nil
}
