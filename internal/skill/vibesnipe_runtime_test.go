package skill

import (
	"fmt"
	"testing"
)

// TestTriadicConsensusCreation verifies GF(3)-balanced triad formation.
func TestTriadicConsensusCreation(t *testing.T) {
	// Create three balanced skills
	gen := &ASISkill{
		ID:          "gen-1",
		Name:        "generator-skill",
		Description: "Active belief expansion",
		Trit:        1,
		Role:        "Generator",
	}

	coord := &ASISkill{
		ID:          "coord-1",
		Name:        "coordinator-skill",
		Description: "Balanced mediation",
		Trit:        0,
		Role:        "Coordinator",
	}

	verif := &ASISkill{
		ID:          "verif-1",
		Name:        "verifier-skill",
		Description: "Conservative validation",
		Trit:        2,
		Role:        "Verifier",
	}

	tc, err := NewTriadicConsensus(gen, coord, verif)
	if err != nil {
		t.Fatalf("NewTriadicConsensus failed: %v", err)
	}

	// Verify GF(3) balance: 1 + 0 + 2 = 3 ≡ 0 (mod 3)
	if !tc.IsBalanced() {
		t.Error("Triad should be balanced")
	}

	if tc.TriadSum != 3 {
		t.Errorf("TriadSum = %d, want 3", tc.TriadSum)
	}

	// Verify agents assigned correctly
	if tc.Agents[0].Trit != 1 || tc.Agents[0].Role != "Generator" {
		t.Error("Generator not assigned correctly")
	}
	if tc.Agents[1].Trit != 0 || tc.Agents[1].Role != "Coordinator" {
		t.Error("Coordinator not assigned correctly")
	}
	if tc.Agents[2].Trit != 2 || tc.Agents[2].Role != "Verifier" {
		t.Error("Verifier not assigned correctly")
	}

	// Verify all agents initialized with epsilon
	for _, agent := range tc.Agents {
		if agent.Epsilon != 0.15 {
			t.Errorf("Agent %s epsilon = %f, want 0.15", agent.ID, agent.Epsilon)
		}
		if agent.SelectionFn == nil {
			t.Errorf("Agent %s has nil SelectionFn", agent.ID)
		}
	}
}

// TestTriadicConsensusImbalancedReject verifies imbalanced triads are rejected.
func TestTriadicConsensusImbalancedReject(t *testing.T) {
	// Create imbalanced skills (sum = 4, not ≡ 0 mod 3)
	s1 := &ASISkill{Name: "s1", Trit: 1}
	s2 := &ASISkill{Name: "s2", Trit: 1}
	s3 := &ASISkill{Name: "s3", Trit: 2}

	tc, err := NewTriadicConsensus(s1, s2, s3)
	if err == nil || tc != nil {
		t.Error("Should reject imbalanced triad")
	}
}

// TestReviseAndEquilibrium verifies consensus revision maintains equilibrium.
func TestReviseAndEquilibrium(t *testing.T) {
	tc := createBalancedTriad(t)

	// Revise with new belief
	err := tc.Revise("belief-1")
	if err != nil {
		t.Fatalf("Revise failed: %v", err)
	}

	// Verify invariants maintained
	if err := tc.VerifyConsensusInvariant(); err != nil {
		t.Fatalf("Invariant violation after revise: %v", err)
	}

	// Verify epoch incremented
	if tc.Epoch != 1 {
		t.Errorf("Epoch = %d, want 1", tc.Epoch)
	}

	// Verify shared belief set updated
	if len(tc.SharedBeliefSet) != 1 {
		t.Errorf("SharedBeliefSet len = %d, want 1", len(tc.SharedBeliefSet))
	}

	// Verify GF(3) still balanced
	if !tc.IsBalanced() {
		t.Error("Triad no longer balanced after revise")
	}
}

// TestSelectionFunctions verifies role-specific selection strategies.
func TestSelectionFunctions(t *testing.T) {
	admissible := []string{"belief-a", "belief-b", "belief-c", "belief-d", "belief-e"}

	// Generator: selects more (aggressive expansion)
	genSelected := selectGenerative(admissible, "ctx", 0.15)
	if len(genSelected) < len(admissible)/2 {
		t.Errorf("Generator selected %d, expected > %d", len(genSelected), len(admissible)/2)
	}

	// Coordinator: selects balanced middle
	coordSelected := selectCoordinating(admissible, "ctx", 0.15)
	if len(coordSelected) == 0 || len(coordSelected) > len(admissible)/2 {
		t.Errorf("Coordinator selected %d, expected balanced subset", len(coordSelected))
	}

	// Verifier: selects few (conservative)
	verifSelected := selectVerifying(admissible, "ctx", 0.15)
	if len(verifSelected) > len(admissible)/4 {
		t.Errorf("Verifier selected %d, expected conservative subset", len(verifSelected))
	}

	// Verify: verifier ≤ coordinator ≤ generator in selection size
	if len(verifSelected) > len(coordSelected) {
		t.Error("Verifier selected more than coordinator (expected: verifier ≤ coordinator)")
	}
	if len(coordSelected) > len(genSelected) {
		t.Error("Coordinator selected more than generator (expected: coordinator ≤ generator)")
	}
}

// TestMultipleRevisions verifies equilibrium over multiple epochs.
func TestMultipleRevisions(t *testing.T) {
	tc := createBalancedTriad(t)

	beliefs := []string{"belief-1", "belief-2", "belief-3", "belief-4"}
	for i, belief := range beliefs {
		err := tc.Revise(belief)
		if err != nil {
			t.Fatalf("Revise %d failed: %v", i, err)
		}

		if tc.Epoch != i+1 {
			t.Errorf("Epoch = %d, want %d", tc.Epoch, i+1)
		}

		if !tc.IsBalanced() {
			t.Errorf("Triad unbalanced at epoch %d", tc.Epoch)
		}

		if err := tc.VerifyConsensusInvariant(); err != nil {
			t.Fatalf("Invariant violation at epoch %d: %v", tc.Epoch, err)
		}
	}
}

// TestConvexCombination verifies consensus belief computation.
func TestConvexCombination(t *testing.T) {
	tc := createBalancedTriad(t)

	err := tc.Revise("consensus-belief")
	if err != nil {
		t.Fatalf("Revise failed: %v", err)
	}

	consensus := tc.ComputeConvexCombination()
	if consensus == "" {
		t.Error("ComputeConvexCombination returned empty string")
	}

	// Consensus should be one of the beliefs in the system
	found := false
	for _, agent := range tc.Agents {
		for _, belief := range agent.BeliefSet {
			if belief == consensus {
				found = true
				break
			}
		}
	}
	if !found {
		t.Errorf("Consensus belief %q not in any agent's belief set", consensus)
	}
}

// TestVibesniperTriadStatus verifies status export format.
func TestVibesniperTriadStatus(t *testing.T) {
	tc := createBalancedTriad(t)

	status := tc.TriadStatus()

	// Verify required fields
	if balanced, ok := status["balanced"].(bool); !ok || !balanced {
		t.Error("status[balanced] missing or not balanced")
	}

	if triSum, ok := status["triad_sum"].(int); !ok || triSum == 0 {
		t.Error("status[triad_sum] missing or zero")
	}

	if _, ok := status["epoch"].(int); !ok {
		t.Error("status[epoch] missing")
	}

	if agents, ok := status["agents"].(map[string]interface{}); !ok || len(agents) != 3 {
		t.Error("status[agents] missing or not 3 agents")
	}
}

// TestExportTriadAsJSON verifies JSON export format.
func TestExportTriadAsJSON(t *testing.T) {
	tc := createBalancedTriad(t)

	err := tc.Revise("belief-1")
	if err != nil {
		t.Fatalf("Revise failed: %v", err)
	}

	exported := tc.ExportTriadAsJSON()

	// Verify structure
	if balanced, ok := exported["balanced"].(bool); !ok || !balanced {
		t.Error("exported[balanced] missing")
	}

	if agents, ok := exported["agents"].([]map[string]interface{}); !ok || len(agents) != 3 {
		t.Error("exported[agents] not array of 3")
	}

	if _, ok := exported["consensus"].(string); !ok {
		t.Error("exported[consensus] missing or not string")
	}
}

// TestInvariantViolation verifies error on invariant failure.
func TestInvariantViolation(t *testing.T) {
	tc := createBalancedTriad(t)

	// Manually corrupt the triad
	tc.Agents[0].Trit = 99 // Invalid trit value

	// Verify invariant catches this
	err := tc.VerifyConsensusInvariant()
	if err == nil {
		t.Error("VerifyConsensusInvariant should catch imbalanced trit")
	}
}

// Benchmark: RevisePerformance tests equilibrium computation speed.
func BenchmarkRevisePerformance(b *testing.B) {
	tc := createBalancedTriad(&testing.T{})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tc.Revise("benchmark-belief")
	}
}

// Benchmark: SelectionFunctionPerformance tests selection function speed.
func BenchmarkSelectionFunctionPerformance(b *testing.B) {
	admissible := make([]string, 100)
	for i := 0; i < 100; i++ {
		admissible[i] = fmt.Sprintf("belief-%d", i)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		selectGenerative(admissible, "ctx", 0.15)
		selectCoordinating(admissible, "ctx", 0.15)
		selectVerifying(admissible, "ctx", 0.15)
	}
}

// ===== Test Helpers =====

// createBalancedTriad creates a test triad with balanced GF(3) trits.
func createBalancedTriad(t *testing.T) *TriadicConsensus {
	gen := &ASISkill{ID: "g1", Name: "gen", Trit: 1}
	coord := &ASISkill{ID: "c1", Name: "coord", Trit: 0}
	verif := &ASISkill{ID: "v1", Name: "verif", Trit: 2}

	tc, err := NewTriadicConsensus(gen, coord, verif)
	if err != nil {
		t.Fatalf("createBalancedTriad failed: %v", err)
	}

	return tc
}
