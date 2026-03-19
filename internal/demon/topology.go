package demon

import (
	"encoding/json"
	"math"
	"sort"
	"time"
)

// TopologyEmbedding computes a simplicial complex from pcap-style flow records.
// This is the "topological view" of the demon's sorting decisions:
//
//   - Vertices (0-simplices): endpoints (src, dst IPs)
//   - Edges (1-simplices): active paths between endpoints
//   - Triangles (2-simplices): concurrent multipath flows (3+ paths active simultaneously)
//
// Persistent homology on the latency filtration reveals:
//   - H0: connected components (path diversity)
//   - H1: loops (redundant paths the demon exploits)
//   - H2: voids (path combinations that never co-occur)

// FlowRecord is a single observed packet flow — the demon's audit trail.
type FlowRecord struct {
	Timestamp time.Time     `json:"timestamp"`
	SrcAddr   string        `json:"src_addr"`
	DstAddr   string        `json:"dst_addr"`
	PathID    PathID        `json:"path_id"`
	RTT       time.Duration `json:"rtt"`
	Size      int           `json:"size"`
	Lost      bool          `json:"lost"`
}

// Vertex is a 0-simplex in the topological embedding.
type Vertex struct {
	ID      int    `json:"id"`
	Address string `json:"address"`
	Degree  int    `json:"degree"`
}

// Edge is a 1-simplex — an active path between two endpoints.
type Edge struct {
	ID          int           `json:"id"`
	Src         int           `json:"src"`
	Dst         int           `json:"dst"`
	PathID      PathID        `json:"path_id"`
	MeanRTT     time.Duration `json:"mean_rtt"`
	FlowCount   int           `json:"flow_count"`
	TotalBytes  int64         `json:"total_bytes"`
	Filtration  float64       `json:"filtration"` // RTT-based filtration value
}

// Triangle is a 2-simplex — three endpoints with concurrent multipath.
type Triangle struct {
	Vertices   [3]int  `json:"vertices"`
	Edges      [3]int  `json:"edges"`
	Filtration float64 `json:"filtration"`
}

// PersistencePair represents a birth-death pair in persistent homology.
type PersistencePair struct {
	Dimension  int     `json:"dimension"`
	Birth      float64 `json:"birth"`
	Death      float64 `json:"death"`
	Persistence float64 `json:"persistence"`
}

// SimplicialComplex is the topological embedding of network flows.
type SimplicialComplex struct {
	Vertices  []Vertex  `json:"vertices"`
	Edges     []Edge    `json:"edges"`
	Triangles []Triangle `json:"triangles"`

	// Persistent homology
	H0 []PersistencePair `json:"h0"` // Connected components
	H1 []PersistencePair `json:"h1"` // Loops / redundant paths
	H2 []PersistencePair `json:"h2"` // Voids

	// Summary
	BettiNumbers [3]int  `json:"betti_numbers"` // [β0, β1, β2]
	EulerChar    int     `json:"euler_characteristic"`
	MaxFiltration float64 `json:"max_filtration"`
}

// TopologyBuilder incrementally builds the simplicial complex from flow records.
type TopologyBuilder struct {
	addrToID  map[string]int
	edgeKey   map[[3]int]int // [src, dst, pathID] -> edge index
	vertices  []Vertex
	edges     []Edge
	triangles []Triangle
	flows     []FlowRecord
}

// NewTopologyBuilder creates a builder.
func NewTopologyBuilder() *TopologyBuilder {
	return &TopologyBuilder{
		addrToID: make(map[string]int),
		edgeKey:  make(map[[3]int]int),
	}
}

// AddFlow adds a flow record to the topology.
func (b *TopologyBuilder) AddFlow(f FlowRecord) {
	b.flows = append(b.flows, f)

	srcID := b.getOrCreateVertex(f.SrcAddr)
	dstID := b.getOrCreateVertex(f.DstAddr)

	b.getOrCreateEdge(srcID, dstID, f.PathID, f.RTT, f.Size)
}

func (b *TopologyBuilder) getOrCreateVertex(addr string) int {
	if id, ok := b.addrToID[addr]; ok {
		return id
	}
	id := len(b.vertices)
	b.vertices = append(b.vertices, Vertex{ID: id, Address: addr})
	b.addrToID[addr] = id
	return id
}

func (b *TopologyBuilder) getOrCreateEdge(src, dst int, pathID PathID, rtt time.Duration, size int) int {
	key := [3]int{src, dst, int(pathID)}
	if idx, ok := b.edgeKey[key]; ok {
		b.edges[idx].FlowCount++
		b.edges[idx].TotalBytes += int64(size)
		// Running average RTT
		n := float64(b.edges[idx].FlowCount)
		b.edges[idx].MeanRTT = time.Duration(
			(float64(b.edges[idx].MeanRTT)*(n-1) + float64(rtt)) / n,
		)
		b.edges[idx].Filtration = b.edges[idx].MeanRTT.Seconds() * 1000 // ms
		return idx
	}

	idx := len(b.edges)
	b.edges = append(b.edges, Edge{
		ID:         idx,
		Src:        src,
		Dst:        dst,
		PathID:     pathID,
		MeanRTT:    rtt,
		FlowCount:  1,
		TotalBytes: int64(size),
		Filtration: rtt.Seconds() * 1000,
	})
	b.edgeKey[key] = idx

	b.vertices[src].Degree++
	b.vertices[dst].Degree++

	return idx
}

// Build constructs the simplicial complex with persistent homology.
func (b *TopologyBuilder) Build() *SimplicialComplex {
	sc := &SimplicialComplex{
		Vertices: b.vertices,
		Edges:    b.edges,
	}

	// Find triangles: for each pair of edges sharing a vertex,
	// check if the third edge exists (concurrent multipath)
	b.findTriangles(sc)

	// Compute persistent homology via the filtration
	b.computePersistence(sc)

	// Betti numbers (at max filtration)
	sc.BettiNumbers = [3]int{
		b.countInfinite(sc.H0),
		b.countInfinite(sc.H1),
		b.countInfinite(sc.H2),
	}
	sc.EulerChar = sc.BettiNumbers[0] - sc.BettiNumbers[1] + sc.BettiNumbers[2]

	if len(sc.Edges) > 0 {
		sc.MaxFiltration = sc.Edges[len(sc.Edges)-1].Filtration
	}

	return sc
}

func (b *TopologyBuilder) findTriangles(sc *SimplicialComplex) {
	// Build adjacency: vertex -> list of (other_vertex, edge_idx)
	adj := make(map[int][]struct{ v, e int })
	for i, e := range sc.Edges {
		adj[e.Src] = append(adj[e.Src], struct{ v, e int }{e.Dst, i})
		adj[e.Dst] = append(adj[e.Dst], struct{ v, e int }{e.Src, i})
	}

	seen := make(map[[3]int]bool)
	for v := range adj {
		neighbors := adj[v]
		for i := 0; i < len(neighbors); i++ {
			for j := i + 1; j < len(neighbors); j++ {
				u := neighbors[i].v
				w := neighbors[j].v
				if u == w {
					continue
				}
				// Check if edge (u, w) exists
				for _, nb := range adj[u] {
					if nb.v == w {
						tri := [3]int{v, u, w}
						sort.Ints(tri[:])
						if !seen[tri] {
							seen[tri] = true
							// Filtration = max of the three edge filtrations
							f1 := sc.Edges[neighbors[i].e].Filtration
							f2 := sc.Edges[neighbors[j].e].Filtration
							f3 := sc.Edges[nb.e].Filtration
							maxF := math.Max(f1, math.Max(f2, f3))
							sc.Triangles = append(sc.Triangles, Triangle{
								Vertices:   tri,
								Edges:      [3]int{neighbors[i].e, neighbors[j].e, nb.e},
								Filtration: maxF,
							})
						}
						break
					}
				}
			}
		}
	}
}

// computePersistence computes a simplified persistent homology.
// Full computation would use a boundary matrix reduction algorithm;
// here we compute the essential features directly from the filtration.
func (b *TopologyBuilder) computePersistence(sc *SimplicialComplex) {
	// H0: each vertex is born at filtration 0.
	// Components merge when edges appear. The oldest component survives.
	// Uses union-find on edges sorted by filtration.
	parent := make([]int, len(sc.Vertices))
	rank := make([]int, len(sc.Vertices))
	for i := range parent {
		parent[i] = i
	}

	var find func(int) int
	find = func(x int) int {
		if parent[x] != x {
			parent[x] = find(parent[x])
		}
		return parent[x]
	}

	union := func(a, b int) bool {
		ra, rb := find(a), find(b)
		if ra == rb {
			return false // Already connected
		}
		if rank[ra] < rank[rb] {
			ra, rb = rb, ra
		}
		parent[rb] = ra
		if rank[ra] == rank[rb] {
			rank[ra]++
		}
		return true
	}

	// Sort edges by filtration
	sortedEdges := make([]int, len(sc.Edges))
	for i := range sortedEdges {
		sortedEdges[i] = i
	}
	sort.Slice(sortedEdges, func(i, j int) bool {
		return sc.Edges[sortedEdges[i]].Filtration < sc.Edges[sortedEdges[j]].Filtration
	})

	// Process edges in filtration order
	for _, ei := range sortedEdges {
		e := sc.Edges[ei]
		if union(e.Src, e.Dst) {
			// Merging two components: the younger one dies
			sc.H0 = append(sc.H0, PersistencePair{
				Dimension:  0,
				Birth:      0,
				Death:      e.Filtration,
				Persistence: e.Filtration,
			})
		} else {
			// Already connected: this edge creates a 1-cycle (loop)
			sc.H1 = append(sc.H1, PersistencePair{
				Dimension:  1,
				Birth:      e.Filtration,
				Death:      math.Inf(1), // Dies when filled by a triangle
				Persistence: math.Inf(1),
			})
		}
	}

	// Add the surviving H0 component (infinite persistence)
	components := make(map[int]bool)
	for i := range sc.Vertices {
		components[find(i)] = true
	}
	for range components {
		sc.H0 = append(sc.H0, PersistencePair{
			Dimension:  0,
			Birth:      0,
			Death:      math.Inf(1),
			Persistence: math.Inf(1),
		})
	}

	// H1 deaths from triangles: when a triangle appears, it kills a 1-cycle
	sort.Slice(sc.Triangles, func(i, j int) bool {
		return sc.Triangles[i].Filtration < sc.Triangles[j].Filtration
	})
	h1Killed := 0
	for _, tri := range sc.Triangles {
		if h1Killed < len(sc.H1) && !math.IsInf(sc.H1[h1Killed].Death, 1) {
			h1Killed++
			continue
		}
		if h1Killed < len(sc.H1) {
			sc.H1[h1Killed].Death = tri.Filtration
			sc.H1[h1Killed].Persistence = tri.Filtration - sc.H1[h1Killed].Birth
			h1Killed++
		}
	}
}

func (b *TopologyBuilder) countInfinite(pairs []PersistencePair) int {
	count := 0
	for _, p := range pairs {
		if math.IsInf(p.Death, 1) {
			count++
		}
	}
	return count
}

// JSON returns the simplicial complex as formatted JSON.
func (sc *SimplicialComplex) JSON() (string, error) {
	data, err := json.MarshalIndent(sc, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// PersistenceDiagram returns (birth, death) pairs for plotting.
func (sc *SimplicialComplex) PersistenceDiagram() [][]float64 {
	var pairs [][]float64
	for _, h := range [][]PersistencePair{sc.H0, sc.H1, sc.H2} {
		for _, p := range h {
			death := p.Death
			if math.IsInf(death, 1) {
				death = sc.MaxFiltration * 1.5
			}
			pairs = append(pairs, []float64{p.Birth, death, float64(p.Dimension)})
		}
	}
	return pairs
}
