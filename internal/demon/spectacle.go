package demon

import (
	"encoding/json"
	"math"
	"sync"
	"time"
)

// Spectacle renders the demon's decisions into a format suitable for
// in-game visualization in Hyperion's Minecraft world.
//
// The demon arena is a region of blocks where:
//   - Each column represents a network path
//   - Block height = inverse RTT (tall = fast path)
//   - Block color = path assignment (deterministic via Gay.jl palette)
//   - Particle effects = probe packets (PATH_CHALLENGE / PATH_RESPONSE)
//   - Redstone signal = entropy gradient (high entropy = more redstone)
//
// This makes the demon's entropy-sorting *visible* as a physical phenomenon.

// ArenaConfig defines the demon arena's geometry in the Minecraft world.
type ArenaConfig struct {
	// Origin is the world position of the arena's southwest corner.
	OriginX, OriginY, OriginZ int
	// Columns is the number of path columns (= number of paths).
	Columns int
	// ColumnWidth is the width of each column in blocks.
	ColumnWidth int
	// MaxHeight is the maximum column height (corresponds to min RTT).
	MaxHeight int
	// TickRate is how often the spectacle updates (Minecraft ticks).
	TickRate int
}

// DefaultArenaConfig returns arena config for N paths.
func DefaultArenaConfig(numPaths int) ArenaConfig {
	return ArenaConfig{
		OriginX:     0,
		OriginY:     64,
		OriginZ:     0,
		Columns:     numPaths,
		ColumnWidth:  4,
		MaxHeight:   32,
		TickRate:    1,
	}
}

// BlockUpdate represents a single block change to send to Hyperion.
// Maps to Hyperion's chunk delta broadcast system.
type BlockUpdate struct {
	X, Y, Z int    `json:"x,y,z"`
	BlockID  int    `json:"block_id"`
	Metadata int    `json:"metadata"`
}

// ParticleSpawn represents a particle effect for probe visualization.
type ParticleSpawn struct {
	X, Y, Z    float64 `json:"x,y,z"`
	ParticleID string  `json:"particle_id"`
	Count      int     `json:"count"`
	SpeedX     float64 `json:"speed_x"`
	SpeedY     float64 `json:"speed_y"`
	SpeedZ     float64 `json:"speed_z"`
}

// SpectacleFrame is one tick's worth of visual updates.
type SpectacleFrame struct {
	Tick       int64           `json:"tick"`
	Timestamp  time.Time       `json:"timestamp"`
	Blocks     []BlockUpdate   `json:"blocks"`
	Particles  []ParticleSpawn `json:"particles"`
	Entropy    float64         `json:"entropy"`
	EntropyMax float64         `json:"entropy_max"`
	PathStats  []PathStats     `json:"path_stats"`
}

// Spectacle generates visual frames from demon state.
type Spectacle struct {
	config ArenaConfig
	demon  *Demon

	mu       sync.Mutex
	tick     int64
	prevHeights []int
}

// NewSpectacle creates a spectacle renderer.
func NewSpectacle(arena ArenaConfig, demon *Demon) *Spectacle {
	return &Spectacle{
		config:      arena,
		demon:       demon,
		prevHeights: make([]int, arena.Columns),
	}
}

// RenderFrame generates the visual update for the current tick.
func (s *Spectacle) RenderFrame() *SpectacleFrame {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tick++

	stats := s.demon.Stats()
	frame := &SpectacleFrame{
		Tick:       s.tick,
		Timestamp:  time.Now(),
		Entropy:    stats.EntropyAfter,
		EntropyMax: stats.EntropyBefore,
		PathStats:  stats.Paths,
	}

	// Generate block updates: one column per path
	for i, ps := range stats.Paths {
		height := s.rttToHeight(ps.SmoothedRTT)

		// Only update if height changed
		if height != s.prevHeights[i] {
			blocks := s.renderColumn(i, height, ps)
			frame.Blocks = append(frame.Blocks, blocks...)
			s.prevHeights[i] = height
		}

		// Spawn probe particles
		if ps.Alive && time.Since(ps.LastProbe) < 100*time.Millisecond {
			particle := s.probeParticle(i, ps)
			frame.Particles = append(frame.Particles, particle)
		}
	}

	return frame
}

// rttToHeight maps RTT to column height. Lower RTT = taller column.
func (s *Spectacle) rttToHeight(rtt time.Duration) int {
	if rtt <= 0 {
		return 1 // Unprobed paths get minimum height
	}
	// Map 1ms -> maxHeight, 200ms -> 1
	ms := float64(rtt) / float64(time.Millisecond)
	if ms < 1 {
		ms = 1
	}
	height := int(float64(s.config.MaxHeight) * (1.0 - math.Log10(ms)/math.Log10(200)))
	if height < 1 {
		height = 1
	}
	if height > s.config.MaxHeight {
		height = s.config.MaxHeight
	}
	return height
}

// renderColumn generates block updates for a single path column.
func (s *Spectacle) renderColumn(pathIdx, height int, ps PathStats) []BlockUpdate {
	var blocks []BlockUpdate
	ox := s.config.OriginX + pathIdx*s.config.ColumnWidth
	oy := s.config.OriginY
	oz := s.config.OriginZ

	// Block type based on path quality:
	// Fastest path (lowest RTT): glowstone (warm)
	// Medium: sea lantern
	// Slow: packed ice (cool)
	// Dead: obsidian
	blockID := pathQualityBlock(ps)

	for dy := 0; dy < s.config.MaxHeight; dy++ {
		for dx := 0; dx < s.config.ColumnWidth; dx++ {
			for dz := 0; dz < s.config.ColumnWidth; dz++ {
				if dy < height {
					blocks = append(blocks, BlockUpdate{
						X: ox + dx, Y: oy + dy, Z: oz + dz,
						BlockID: blockID,
					})
				} else {
					// Air above the column
					blocks = append(blocks, BlockUpdate{
						X: ox + dx, Y: oy + dy, Z: oz + dz,
						BlockID: 0, // Air
					})
				}
			}
		}
	}

	return blocks
}

// pathQualityBlock maps path quality to a Minecraft block type.
// This is the demon's color coding — visible entropy sorting.
func pathQualityBlock(ps PathStats) int {
	if !ps.Alive {
		return 49 // Obsidian — dead path
	}

	ms := float64(ps.SmoothedRTT) / float64(time.Millisecond)
	switch {
	case ms < 10:
		return 89 // Glowstone — excellent (fast chamber)
	case ms < 25:
		return 169 // Sea lantern — good
	case ms < 50:
		return 22 // Lapis block — moderate
	case ms < 100:
		return 174 // Packed ice — slow
	default:
		return 173 // Blue ice — very slow (cold chamber)
	}
}

// probeParticle creates a particle effect for a path probe.
func (s *Spectacle) probeParticle(pathIdx int, ps PathStats) ParticleSpawn {
	cx := float64(s.config.OriginX+pathIdx*s.config.ColumnWidth) + float64(s.config.ColumnWidth)/2
	cy := float64(s.config.OriginY + s.rttToHeight(ps.SmoothedRTT))
	cz := float64(s.config.OriginZ) + float64(s.config.ColumnWidth)/2

	// Fast paths get warm particles, slow paths get cool particles
	particleID := "minecraft:flame"
	ms := float64(ps.SmoothedRTT) / float64(time.Millisecond)
	if ms > 50 {
		particleID = "minecraft:soul_fire_flame"
	}
	if ms > 100 {
		particleID = "minecraft:snowflake"
	}

	return ParticleSpawn{
		X: cx, Y: cy, Z: cz,
		ParticleID: particleID,
		Count:      3,
		SpeedY:     0.1,
	}
}

// JSON returns the frame as JSON for sending to Hyperion.
func (f *SpectacleFrame) JSON() string {
	data, _ := json.Marshal(f)
	return string(data)
}
