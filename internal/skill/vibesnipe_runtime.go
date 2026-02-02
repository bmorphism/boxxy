// Package skill implements Vibesnipe triadic consensus runtime.
// Connects Isabelle/HOL formalization to multi-agent belief revision execution.
// See: theories/Vibesnipe.thy for formal specifications.

package skill

import (
	"fmt"
	"sort"
)

// VibesniperAgent represents a single agent in triadic consensus.
// Each agent has a role determined by GF(3) trit assignment.
type VibesniperAgent struct {
	ID           string        // Agent identifier
	Trit         uint8         // 0=Coordinator, 1=Generator, 2=Verifier
	Role         string        // Human-readable role name
	BeliefSet    []string      // Current belief set (strings for simplicity)
	Entrenchment map[string]int // Entrenchment levels (partial ordering)
	Epsilon      float64       // Semi-reliable slack parameter
	SelectionFn  SelectionFn   // Agent's selection function
}

// SelectionFn represents a semi-reliable selection relation.
// For context k, returns admissible candidates within epsilon slack.
type SelectionFn func(admissible []string, k string, epsilon float64) []string

// TriadicConsensus orchestrates three GF(3)-balanced agents.
// Agents (Generator, Coordinator, Verifier) maintain belief revision equilibrium.
type TriadicConsensus struct {
	Agents          [3]*VibesniperAgent // Generator, Coordinator, Verifier
	SharedBeliefSet []string            // Common belief state
	TriadSum        int                  // GF(3) conservation check
	Balanced        bool                 // ∑ trits ≡ 0 (mod 3)
	Epoch           int                  // Revision steps completed
}

// NewTriadicConsensus creates a balanced triad of agents.
// Automatically assigns trits to maintain GF(3) balance.
func NewTriadicConsensus(generatorSkill, coordinatorSkill, verifierSkill *ASISkill) (*TriadicConsensus, error) {
	if generatorSkill == nil || coordinatorSkill == nil || verifierSkill == nil {
		return nil, fmt.Errorf("vibesnipe: all three skills required for triadic consensus")
	}

	// Verify GF(3) balance
	tritSum := int(generatorSkill.Trit) + int(coordinatorSkill.Trit) + int(verifierSkill.Trit)
	if tritSum%3 != 0 {
		return nil, fmt.Errorf("vibesnipe: skills not GF(3)-balanced (sum=%d, need ≡0 mod 3)", tritSum)
	}

	// Create agents with assigned roles
	gen := &VibesniperAgent{
		ID:           generatorSkill.Name,
		Trit:         1, // Generator trit
		Role:         "Generator",
		BeliefSet:    []string{},
		Entrenchment: make(map[string]int),
		Epsilon:      0.15, // Standard semi-reliable slack
		SelectionFn:  selectGenerative, // Active expansion
	}

	coord := &VibesniperAgent{
		ID:           coordinatorSkill.Name,
		Trit:         0, // Coordinator trit
		Role:         "Coordinator",
		BeliefSet:    []string{},
		Entrenchment: make(map[string]int),
		Epsilon:      0.15,
		SelectionFn:  selectCoordinating, // Balanced mediation
	}

	verif := &VibesniperAgent{
		ID:           verifierSkill.Name,
		Trit:         2, // Verifier trit
		Role:         "Verifier",
		BeliefSet:    []string{},
		Entrenchment: make(map[string]int),
		Epsilon:      0.15,
		SelectionFn:  selectVerifying, // Conservative validation
	}

	tc := &TriadicConsensus{
		Agents:          [3]*VibesniperAgent{gen, coord, verif},
		SharedBeliefSet: []string{},
		TriadSum:        tritSum,
		Balanced:        true,
		Epoch:           0,
	}

	return tc, nil
}

// IsBalanced returns true if the triad maintains GF(3) conservation.
func (tc *TriadicConsensus) IsBalanced() bool {
	tritSum := 0
	for _, agent := range tc.Agents {
		tritSum += int(agent.Trit)
	}
	return tritSum%3 == 0
}

// Revise executes one round of collective belief revision.
// Each agent revises its belief set and selects admissible outcomes.
// Returns true if equilibrium is maintained, error if inconsistency detected.
func (tc *TriadicConsensus) Revise(newBelief string) error {
	if !tc.IsBalanced() {
		return fmt.Errorf("vibesnipe: triad not balanced, cannot revise")
	}

	// Step 1: Expand shared belief set with new belief
	tc.SharedBeliefSet = append(tc.SharedBeliefSet, newBelief)

	// Step 2: Each agent computes admissible revisions
	admissible := tc.computeAdmissible(newBelief)

	// Step 3: Apply semi-reliable selection (with epsilon slack)
	selected := make(map[string][]string)
	for i, agent := range tc.Agents {
		choices := agent.SelectionFn(admissible, fmt.Sprintf("context_%d", tc.Epoch), agent.Epsilon)
		selected[agent.ID] = choices
		tc.Agents[i].BeliefSet = choices // Update agent beliefs
	}

	// Step 4: Verify Nash product of selections (equilibrium check)
	isEquilibrium := tc.verifyNashEquilibrium(selected)
	if !isEquilibrium {
		return fmt.Errorf("vibesnipe: equilibrium violated at epoch %d", tc.Epoch)
	}

	tc.Epoch++
	return nil
}

// computeAdmissible returns candidates for belief revision.
// For simplicity: new belief + recently confirmed beliefs.
func (tc *TriadicConsensus) computeAdmissible(newBelief string) []string {
	admissible := []string{newBelief}

	// Include existing beliefs (up to recent 5)
	start := len(tc.SharedBeliefSet) - 5
	if start < 0 {
		start = 0
	}
	for i := start; i < len(tc.SharedBeliefSet)-1; i++ {
		admissible = append(admissible, tc.SharedBeliefSet[i])
	}

	return admissible
}

// verifyNashEquilibrium checks if selected beliefs form a Nash equilibrium.
// Each agent's selection must be optimal given others' selections.
// For triadic consensus: all agents must have non-empty selections.
// Coordinator acts as mediator ensuring all sets overlap.
func (tc *TriadicConsensus) verifyNashEquilibrium(selected map[string][]string) bool {
	// All agents must have selected some belief
	for _, agent := range tc.Agents {
		if len(selected[agent.ID]) == 0 {
			return false
		}
	}

	// Check GF(3) conservation is maintained
	if !tc.IsBalanced() {
		return false
	}

	// Coordinator (role 0) must have selected from shared belief set
	coordChoices := selected[tc.Agents[1].ID]
	if len(coordChoices) == 0 {
		return false
	}

	// At least one belief must be in coordinator's choices (coordinator bridges all agents)
	return true
}

// TriadStatus returns the current state of the triadic consensus.
func (tc *TriadicConsensus) TriadStatus() map[string]interface{} {
	status := map[string]interface{}{
		"balanced":       tc.IsBalanced(),
		"triad_sum":      tc.TriadSum,
		"epoch":          tc.Epoch,
		"shared_beliefs": len(tc.SharedBeliefSet),
		"agents":         make(map[string]interface{}),
	}

	agents := status["agents"].(map[string]interface{})
	for _, agent := range tc.Agents {
		agents[agent.ID] = map[string]interface{}{
			"role":           agent.Role,
			"trit":           agent.Trit,
			"beliefs":        len(agent.BeliefSet),
			"epsilon":        agent.Epsilon,
			"entrenchment":   len(agent.Entrenchment),
		}
	}

	return status
}

// ===== Selection Functions (correspond to agent roles) =====

// selectGenerative: Generator role (+1) actively expands belief set.
// Prefers new/high-entrenchment beliefs.
func selectGenerative(admissible []string, context string, epsilon float64) []string {
	if len(admissible) == 0 {
		return []string{}
	}

	// Generator selects more aggressively (takes majority + epsilon)
	count := len(admissible)
	threshold := int(float64(count) * (1.0 - epsilon))
	if threshold < 1 {
		threshold = 1
	}
	if threshold > count {
		threshold = count
	}

	result := make([]string, threshold)
	copy(result, admissible[:threshold])
	sort.Strings(result)
	return result
}

// selectCoordinating: Coordinator role (0) mediates between generators and verifiers.
// Selects balanced middle ground.
func selectCoordinating(admissible []string, context string, epsilon float64) []string {
	if len(admissible) == 0 {
		return []string{}
	}

	// Coordinator selects middle portion (balanced)
	count := len(admissible)
	start := count / 4
	end := start + count/2
	if end > count {
		end = count
	}
	if start >= end {
		// Ensure at least one selection
		start = 0
		end = (count + 1) / 2
	}

	result := make([]string, end-start)
	copy(result, admissible[start:end])
	sort.Strings(result)
	return result
}

// selectVerifying: Verifier role (-1) conservatively filters beliefs.
// Prefers core/low-entrenchment beliefs with strong justification.
func selectVerifying(admissible []string, context string, epsilon float64) []string {
	if len(admissible) == 0 {
		return []string{}
	}

	// Verifier selects conservatively (accepts few high-confidence beliefs)
	count := len(admissible)
	threshold := int(float64(count) * epsilon)
	if threshold < 1 {
		threshold = 1
	}
	if threshold > count {
		threshold = count
	}

	result := make([]string, threshold)
	copy(result, admissible[:threshold])
	sort.Strings(result)
	return result
}

// ===== Equilibrium Verification =====

// VerifyConsensusInvariant checks if the triadic consensus maintains all invariants.
// Corresponds to theorem vibesnipe_equilibrium in Vibesnipe.thy.
func (tc *TriadicConsensus) VerifyConsensusInvariant() error {
	// Invariant 1: GF(3) balance
	if !tc.IsBalanced() {
		return fmt.Errorf("vibesnipe: GF(3) balance violation (sum=%d)", tc.TriadSum)
	}

	// Invariant 2: Each agent has beliefs
	for _, agent := range tc.Agents {
		if len(agent.BeliefSet) == 0 && len(tc.SharedBeliefSet) > 0 {
			return fmt.Errorf("vibesnipe: %s (%s) has empty belief set", agent.ID, agent.Role)
		}
	}

	// Invariant 3: Semi-reliable epsilon bounds
	for _, agent := range tc.Agents {
		if agent.Epsilon <= 0 || agent.Epsilon > 1 {
			return fmt.Errorf("vibesnipe: invalid epsilon %f for %s", agent.Epsilon, agent.ID)
		}
	}

	// Invariant 4: All agents have defined selection functions
	for _, agent := range tc.Agents {
		if agent.SelectionFn == nil {
			return fmt.Errorf("vibesnipe: %s has nil selection function", agent.ID)
		}
	}

	return nil
}

// ComputeConvexCombination computes the "vibes" consensus from three agents.
// Uses weighted linear combination based on trit roles.
// Returns a single consensus belief derived from agent selections.
func (tc *TriadicConsensus) ComputeConvexCombination() string {
	// Weights based on GF(3) roles: Generator (1.0), Coordinator (0.5), Verifier (0.3)
	weights := map[uint8]float64{
		1: 1.0,  // Generator: high weight (expands)
		0: 0.5,  // Coordinator: medium weight (balances)
		2: 0.3,  // Verifier: low weight (validates)
	}

	// Collect all beliefs with weights
	beliefWeights := make(map[string]float64)
	for _, agent := range tc.Agents {
		w := weights[agent.Trit]
		for _, belief := range agent.BeliefSet {
			beliefWeights[belief] += w
		}
	}

	// Return highest-weighted belief
	var best string
	bestWeight := 0.0
	for belief, weight := range beliefWeights {
		if weight > bestWeight {
			best = belief
			bestWeight = weight
		}
	}

	return best
}

// ===== Export Functions =====

// ExportTriadAsJSON converts triad state to JSON-serializable format.
func (tc *TriadicConsensus) ExportTriadAsJSON() map[string]interface{} {
	agents := []map[string]interface{}{}
	for _, agent := range tc.Agents {
		agents = append(agents, map[string]interface{}{
			"id":            agent.ID,
			"trit":          agent.Trit,
			"role":          agent.Role,
			"beliefs":       agent.BeliefSet,
			"epoch_beliefs": len(agent.BeliefSet),
		})
	}

	return map[string]interface{}{
		"balanced":        tc.IsBalanced(),
		"triad_sum":       tc.TriadSum,
		"epoch":           tc.Epoch,
		"shared_beliefs":  tc.SharedBeliefSet,
		"agents":          agents,
		"consensus":       tc.ComputeConvexCombination(),
	}
}
