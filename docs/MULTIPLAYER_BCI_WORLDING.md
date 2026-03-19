# Collaborative World Modeling & Debugging Multiplayer Neurofeedback

## The 4Es Framework for Multiplayer BCI

```
┌─────────────────────────────────────────────────────────────────────────────────┐
│                    4Es COGNITIVE ARCHITECTURE FOR BCI                            │
├─────────────────────────────────────────────────────────────────────────────────┤
│                                                                                  │
│  EMBODIED        EMBEDDED         ENACTIVE         EXTENDED                     │
│  ─────────       ────────         ────────         ────────                     │
│  Brain ↔ Body    Brain ↔ World    Brain ↔ Action   Brain ↔ Tools               │
│                                                                                  │
│  EEG captures    Environment      Closed-loop      BCI headset                  │
│  neural state    shapes signal    feedback         extends cognition            │
│                                                                                  │
│  ┌─────────┐     ┌─────────┐     ┌─────────┐     ┌─────────┐                   │
│  │   🧠    │ ←→  │   🌍    │ ←→  │   ⚡    │ ←→  │   🎧    │                   │
│  │ Neural  │     │ Context │     │ Action  │     │ Device  │                   │
│  │ State   │     │         │     │ Loop    │     │         │                   │
│  └────┬────┘     └────┬────┘     └────┬────┘     └────┬────┘                   │
│       │               │               │               │                         │
│       └───────────────┴───────────────┴───────────────┘                         │
│                           │                                                      │
│                           ▼                                                      │
│              ┌─────────────────────────┐                                        │
│              │  COMPOSITIONAL WORLD    │                                        │
│              │  MODEL (Active Inference)│                                        │
│              └─────────────────────────┘                                        │
│                                                                                  │
└─────────────────────────────────────────────────────────────────────────────────┘
```

## Categorical Structure of Multiplayer Worlding

### The Basic Morphism: Single Participant

```
                    Perception
            ┌──────────────────────┐
            │                      │
            ▼                      │
    ┌───────────────┐      ┌──────┴──────┐
    │   WORLD (W)   │      │  BRAIN (B)  │
    │               │      │             │
    │  Environment  │      │  Generative │
    │  + Stimuli    │      │  Model      │
    │  + Others     │      │             │
    └───────┬───────┘      └──────┬──────┘
            │                      │
            │                      │
            └──────────────────────┘
                    Action

    This forms a LENS (bidirectional morphism):
    
         get: B → W   (perception - sample world state)
         put: B × W → B   (action - update model given world)
```

### Multiplayer: Tensor Product of Lenses

For 3 participants, we form the tensor product:

```
    🧠₁ ⊗ 🧠₂ ⊗ 🧠₃  ←────────────────→  W₁₂₃
    
    Where W₁₂₃ is the SHARED WORLD that includes:
    - Physical environment
    - Each other's observable states
    - The experimental apparatus
    - The feedback loop itself
    
    ┌─────────────────────────────────────────────────────────────────────────┐
    │                                                                         │
    │     🧠₁ ─────────┐                                                     │
    │         get₁    │                                                     │
    │                 ▼                                                     │
    │              ┌──────────────────────────────────────┐                 │
    │              │                                      │                 │
    │     🧠₂ ────────→        W₁₂₃ (Shared World)       │                 │
    │         get₂ │                                      │                 │
    │              │   Contains: stimuli, feedback,       │                 │
    │              │   each other's actions/states        │                 │
    │     🧠₃ ────────→                                   │                 │
    │         get₃ │                                      │                 │
    │              └──────────────────────────────────────┘                 │
    │                              │                                         │
    │                              │ put₁₂₃ (joint action)                   │
    │                              ▼                                         │
    │                    ┌──────────────────┐                               │
    │                    │  Updated World   │                               │
    │                    │  W'₁₂₃           │                               │
    │                    └──────────────────┘                               │
    │                                                                         │
    └─────────────────────────────────────────────────────────────────────────┘
```

### The Synergy Emerges at the Interface

```
    INDIVIDUAL LEVEL:              COLLECTIVE LEVEL:
    
    I(🧠₁; W)                      I(🧠₁ ⊗ 🧠₂ ⊗ 🧠₃; W₁₂₃)
    I(🧠₂; W)                      
    I(🧠₃; W)                      This decomposes via PID/α-synergy:
                                   
    Sum of individuals:            ∂H₁ = TRUE SYNERGY (fragile to any 1 loss)
    Σ I(🧠ᵢ; W)                    ∂H₂ = PAIRWISE (robust to 1, fragile to 2)
                                   ∂H₃ = INDIVIDUAL (fully robust)
    
    KEY INSIGHT:
    ═══════════
    ∂H₁ > 0  ⟹  The group perceives something NO INDIVIDUAL can perceive alone
    
    This is the categorical signature of EMERGENCE in multiplayer worlding
```

## Debugging Multiplayer Neurofeedback: A Systematic Approach

### Debug Level 0: Signal Validity (Jo et al. Criterion)

```
┌─────────────────────────────────────────────────────────────────────────────────┐
│                    DEBUG LEVEL 0: IS THE SIGNAL REAL?                           │
├─────────────────────────────────────────────────────────────────────────────────┤
│                                                                                  │
│  For EACH participant i:                                                         │
│                                                                                  │
│      ┌────────────────┐        ┌────────────────┐                               │
│      │   EEG Signal   │        │ Matched Noise  │                               │
│      │      Xᵢ        │        │      Nᵢ        │                               │
│      └───────┬────────┘        └───────┬────────┘                               │
│              │                          │                                        │
│              ▼                          ▼                                        │
│      ┌────────────────┐        ┌────────────────┐                               │
│      │  Task Metric   │        │  Task Metric   │                               │
│      │   M(Xᵢ)        │        │   M(Nᵢ)        │                               │
│      └───────┬────────┘        └───────┬────────┘                               │
│              │                          │                                        │
│              └──────────┬───────────────┘                                        │
│                         │                                                        │
│                         ▼                                                        │
│              ┌────────────────────────┐                                         │
│              │  Signal Ratio:         │                                         │
│              │  R = M(Xᵢ) / M(Nᵢ)     │                                         │
│              └───────────┬────────────┘                                         │
│                          │                                                       │
│              ┌───────────┴───────────┐                                          │
│              │                       │                                          │
│              ▼                       ▼                                          │
│         R >> 1.0                R ≈ 1.0                                         │
│      ┌──────────────┐        ┌──────────────┐                                   │
│      │   ✓ VALID    │        │   ✗ INVALID  │                                   │
│      │   Proceed    │        │   Debug:     │                                   │
│      │   to Level 1 │        │   • Electrode│                                   │
│      └──────────────┘        │     contact? │                                   │
│                              │   • Artifact?│                                   │
│                              │   • Task fit?│                                   │
│                              └──────────────┘                                   │
│                                                                                  │
│  CHECKPOINT: ALL participants must pass before multiplayer analysis             │
│                                                                                  │
└─────────────────────────────────────────────────────────────────────────────────┘
```

### Debug Level 1: Individual World Models

```
┌─────────────────────────────────────────────────────────────────────────────────┐
│                    DEBUG LEVEL 1: INDIVIDUAL ACTIVE INFERENCE                   │
├─────────────────────────────────────────────────────────────────────────────────┤
│                                                                                  │
│  For each participant, verify the perception-action loop closes:                │
│                                                                                  │
│                    ┌─────────────────────────────────────┐                      │
│                    │        GENERATIVE MODEL              │                      │
│                    │                                      │                      │
│                    │   P(world | observations, actions)   │                      │
│                    │                                      │                      │
│                    └──────────────┬──────────────────────┘                      │
│                                   │                                              │
│           ┌───────────────────────┼───────────────────────┐                     │
│           │                       │                       │                     │
│           ▼                       ▼                       ▼                     │
│    ┌────────────┐          ┌────────────┐          ┌────────────┐              │
│    │ PREDICTION │          │ PREDICTION │          │  ACTION    │              │
│    │  ERROR     │          │   (what    │          │ SELECTION  │              │
│    │            │          │   expect)  │          │            │              │
│    └─────┬──────┘          └─────┬──────┘          └─────┬──────┘              │
│          │                       │                       │                      │
│          │                       │                       │                      │
│          ▼                       ▼                       ▼                      │
│    ┌────────────┐          ┌────────────┐          ┌────────────┐              │
│    │ SENSATION  │    vs    │ PERCEPTION │          │  MOTOR     │              │
│    │  (raw EEG) │          │ (decoded)  │          │  OUTPUT    │              │
│    └─────┬──────┘          └────────────┘          └─────┬──────┘              │
│          │                                               │                      │
│          │                                               │                      │
│          └───────────────────┬───────────────────────────┘                      │
│                              │                                                   │
│                              ▼                                                   │
│                    ┌─────────────────────┐                                      │
│                    │       WORLD         │                                      │
│                    │  (environment +     │                                      │
│                    │   feedback display) │                                      │
│                    └─────────────────────┘                                      │
│                                                                                  │
│  DEBUG CHECKS:                                                                  │
│  ─────────────                                                                  │
│  □ Prediction error decreases over trials? (learning)                          │
│  □ Actions affect world state? (agency)                                        │
│  □ Sensations correlate with world state? (grounding)                          │
│  □ Latency < 100ms? (real-time requirement)                                    │
│                                                                                  │
└─────────────────────────────────────────────────────────────────────────────────┘
```

### Debug Level 2: Pairwise Interactions

```
┌─────────────────────────────────────────────────────────────────────────────────┐
│                    DEBUG LEVEL 2: PAIRWISE COUPLING                             │
├─────────────────────────────────────────────────────────────────────────────────┤
│                                                                                  │
│  For each pair (i,j) ∈ {(1,2), (1,3), (2,3)}:                                   │
│                                                                                  │
│                                                                                  │
│        🧠ᵢ ════════════════════════════════════ 🧠ⱼ                             │
│         │                                        │                              │
│         │         Inter-Brain Coupling           │                              │
│         │                                        │                              │
│         │    ┌────────────────────────────┐     │                              │
│         │    │                            │     │                              │
│         └────┤  IBS(i,j) = Phase Lock    ├─────┘                              │
│              │            Value           │                                     │
│              │                            │                                     │
│              │  TE(i→j) = Transfer       │                                     │
│              │           Entropy          │                                     │
│              │                            │                                     │
│              │  I(Xᵢ; Xⱼ) = Mutual       │                                     │
│              │              Information   │                                     │
│              │                            │                                     │
│              └────────────────────────────┘                                     │
│                                                                                  │
│                                                                                  │
│  COUPLING MATRIX (real-time display):                                           │
│  ─────────────────────────────────────                                          │
│                                                                                  │
│              🧠₁      🧠₂      🧠₃                                              │
│         ┌─────────┬─────────┬─────────┐                                        │
│    🧠₁  │    -    │ IBS₁₂   │ IBS₁₃   │                                        │
│         ├─────────┼─────────┼─────────┤                                        │
│    🧠₂  │ TE₂→₁   │    -    │ IBS₂₃   │                                        │
│         ├─────────┼─────────┼─────────┤                                        │
│    🧠₃  │ TE₃→₁   │ TE₃→₂   │    -    │                                        │
│         └─────────┴─────────┴─────────┘                                        │
│                                                                                  │
│  DEBUG CHECKS:                                                                  │
│  ─────────────                                                                  │
│  □ IBS increases during collaborative task? (synchronization)                  │
│  □ TE is asymmetric? (leader-follower dynamics)                                │
│  □ Coupling > baseline (solo) coupling? (genuine interaction)                  │
│  □ Coupling patterns match task structure? (validity)                          │
│                                                                                  │
└─────────────────────────────────────────────────────────────────────────────────┘
```

### Debug Level 3: Triadic Synergy (The Emergence Test)

```
┌─────────────────────────────────────────────────────────────────────────────────┐
│                    DEBUG LEVEL 3: TRIADIC SYNERGY                               │
├─────────────────────────────────────────────────────────────────────────────────┤
│                                                                                  │
│  This is where EMERGENCE happens (or doesn't):                                  │
│                                                                                  │
│                           ┌───────────────────┐                                 │
│                           │                   │                                 │
│                           │    SHARED TASK    │                                 │
│                           │        Y          │                                 │
│                           │                   │                                 │
│                           └─────────┬─────────┘                                 │
│                                     │                                           │
│                    Causal Collider Structure                                    │
│                                     │                                           │
│              ┌──────────────────────┼──────────────────────┐                   │
│              │                      │                      │                   │
│              ▼                      ▼                      ▼                   │
│           ┌─────┐               ┌─────┐               ┌─────┐                  │
│           │ 🧠₁ │               │ 🧠₂ │               │ 🧠₃ │                  │
│           └──┬──┘               └──┬──┘               └──┬──┘                  │
│              │                     │                     │                     │
│              │                     │                     │                     │
│              └─────────────────────┼─────────────────────┘                     │
│                                    │                                            │
│                                    ▼                                            │
│                    ┌───────────────────────────────┐                           │
│                    │  α-SYNERGY BACKBONE           │                           │
│                    │                               │                           │
│                    │  H(X₁,X₂,X₃) = ∂H₁ + ∂H₂ + ∂H₃│                           │
│                    │                               │                           │
│                    │  ∂H₁ = TRUE SYNERGY          │                           │
│                    │        (lost if ANY 1 fails) │                           │
│                    │                               │                           │
│                    │  ∂H₂ = PAIRWISE              │                           │
│                    │        (lost if ANY 2 fail)  │                           │
│                    │                               │                           │
│                    │  ∂H₃ = INDIVIDUAL            │                           │
│                    │        (lost if ALL 3 fail)  │                           │
│                    └───────────────────────────────┘                           │
│                                                                                  │
│                                                                                  │
│  SYNERGY RATIO OVER TIME:                                                       │
│  ─────────────────────────                                                      │
│                                                                                  │
│  ∂H₁/H(X)│                                    ╱╲                               │
│    1.0   │                               ╱╲  ╱  ╲   ← Phase transition!        │
│          │                              ╱  ╲╱    ╲                             │
│    0.5   │                         ╱╲╱╲                                        │
│          │                    ╱╲╱╲╱                                            │
│    0.0   │_______________╱╲╱╲_________________________________________         │
│          └──────────────────────────────────────────────────────── time        │
│              Solo        │    Collaborative task begins                        │
│              baseline    │                                                     │
│                                                                                  │
│  DEBUG CHECKS:                                                                  │
│  ─────────────                                                                  │
│  □ ∂H₁ > 0 during collaboration? (synergy exists)                              │
│  □ ∂H₁ increases over session? (learning to synchronize)                       │
│  □ ∂H₁ peaks during key task moments? (task-relevant)                          │
│  □ ∂H₁ > ∂H₁_noise_baseline? (not spurious)                                    │
│  □ GF(3) balance maintained? (conservation law)                                │
│                                                                                  │
└─────────────────────────────────────────────────────────────────────────────────┘
```

### Debug Level 4: Compositional World Model Consistency

```
┌─────────────────────────────────────────────────────────────────────────────────┐
│                    DEBUG LEVEL 4: COMPOSITIONAL CONSISTENCY                     │
├─────────────────────────────────────────────────────────────────────────────────┤
│                                                                                  │
│  The categorical perspective: Does the diagram commute?                         │
│                                                                                  │
│                                                                                  │
│                    Individual Worlding                                          │
│         🧠₁ ──────────────────────────────────→ W₁                              │
│          │                                       │                              │
│          │                                       │                              │
│          │ ι₁ (inclusion)                        │ π₁ (projection)              │
│          │                                       │                              │
│          ▼                                       ▼                              │
│    🧠₁ ⊗ 🧠₂ ⊗ 🧠₃ ═══════════════════════════→ W₁₂₃                           │
│          │               Joint Worlding          │                              │
│          │                                       │                              │
│          │ π₂₃ (marginalize out 🧠₁)             │                              │
│          │                                       │                              │
│          ▼                                       ▼                              │
│      🧠₂ ⊗ 🧠₃ ────────────────────────────→ W₂₃                               │
│                    Pairwise Worlding                                            │
│                                                                                  │
│                                                                                  │
│  COMMUTATIVITY CHECK:                                                           │
│  ════════════════════                                                           │
│                                                                                  │
│  Does:  π₁ ∘ (Joint Worlding) = (Individual Worlding) ∘ ι₁  ?                   │
│                                                                                  │
│  In information terms:                                                          │
│  I(🧠₁; W₁) ≤ I(🧠₁; W₁₂₃ | 🧠₂, 🧠₃)                                          │
│                                                                                  │
│  If equality: 🧠₁'s world model is INDEPENDENT of others                       │
│  If strict <: 🧠₁ perceives MORE in the collective context (EMERGENCE!)        │
│                                                                                  │
│                                                                                  │
│  FUNCTORIALITY CHECK:                                                           │
│  ════════════════════                                                           │
│                                                                                  │
│  The worlding functor F: Brain^⊗ → World should preserve composition:          │
│                                                                                  │
│      F(🧠₁ ⊗ 🧠₂) = F(🧠₁) ⊗_W F(🧠₂)                                          │
│                                                                                  │
│  where ⊗_W is the "fiber product" over shared world structure                   │
│                                                                                  │
│                                                                                  │
│  DEBUG CHECKS:                                                                  │
│  ─────────────                                                                  │
│  □ Individual predictions consistent with group predictions?                    │
│  □ Removing one participant preserves others' world models?                     │
│  □ Adding participant enriches (not corrupts) world model?                     │
│  □ Temporal consistency: World model evolves smoothly?                         │
│                                                                                  │
└─────────────────────────────────────────────────────────────────────────────────┘
```

## The Full Debugging Pipeline

```
┌─────────────────────────────────────────────────────────────────────────────────┐
│                    COMPLETE DEBUGGING FLOWCHART                                 │
├─────────────────────────────────────────────────────────────────────────────────┤
│                                                                                  │
│  START                                                                          │
│    │                                                                            │
│    ▼                                                                            │
│  ┌─────────────────────────────────────────────────┐                           │
│  │  LEVEL 0: Signal Validity (Jo et al.)           │                           │
│  │  For each participant: EEG vs Noise baseline    │                           │
│  └─────────────────────┬───────────────────────────┘                           │
│                        │                                                        │
│            ┌───────────┴───────────┐                                           │
│            │                       │                                           │
│       ALL PASS                SOME FAIL                                        │
│            │                       │                                           │
│            ▼                       ▼                                           │
│  ┌──────────────────┐    ┌────────────────────────┐                           │
│  │ Proceed to L1    │    │ DEBUG: Hardware?       │                           │
│  └────────┬─────────┘    │        Artifacts?      │                           │
│           │              │        Task design?    │                           │
│           │              └────────────────────────┘                           │
│           ▼                                                                     │
│  ┌─────────────────────────────────────────────────┐                           │
│  │  LEVEL 1: Individual Active Inference           │                           │
│  │  Does each participant's loop close?            │                           │
│  └─────────────────────┬───────────────────────────┘                           │
│                        │                                                        │
│            ┌───────────┴───────────┐                                           │
│            │                       │                                           │
│       ALL CLOSE               SOME BROKEN                                      │
│            │                       │                                           │
│            ▼                       ▼                                           │
│  ┌──────────────────┐    ┌────────────────────────┐                           │
│  │ Proceed to L2    │    │ DEBUG: Latency?        │                           │
│  └────────┬─────────┘    │        Feedback fidelity│                          │
│           │              │        Training needed? │                           │
│           │              └────────────────────────┘                           │
│           ▼                                                                     │
│  ┌─────────────────────────────────────────────────┐                           │
│  │  LEVEL 2: Pairwise Coupling                     │                           │
│  │  IBS, TE, MI between all pairs                  │                           │
│  └─────────────────────┬───────────────────────────┘                           │
│                        │                                                        │
│            ┌───────────┴───────────┐                                           │
│            │                       │                                           │
│      COUPLING > 0             NO COUPLING                                      │
│            │                       │                                           │
│            ▼                       ▼                                           │
│  ┌──────────────────┐    ┌────────────────────────┐                           │
│  │ Proceed to L3    │    │ DEBUG: Task too hard?  │                           │
│  └────────┬─────────┘    │        Participants    │                           │
│           │              │        incompatible?   │                           │
│           │              │        Insufficient    │                           │
│           │              │        shared context? │                           │
│           │              └────────────────────────┘                           │
│           ▼                                                                     │
│  ┌─────────────────────────────────────────────────┐                           │
│  │  LEVEL 3: Triadic Synergy                       │                           │
│  │  α-backbone: ∂H₁ > 0?                           │                           │
│  └─────────────────────┬───────────────────────────┘                           │
│                        │                                                        │
│            ┌───────────┴───────────┐                                           │
│            │                       │                                           │
│        ∂H₁ > 0               ∂H₁ ≈ 0                                          │
│            │                       │                                           │
│            ▼                       ▼                                           │
│  ┌──────────────────┐    ┌────────────────────────┐                           │
│  │ Proceed to L4    │    │ DEBUG: Synergy exists  │                           │
│  │ EMERGENCE        │    │ at pairwise level?     │                           │
│  │ DETECTED! 🎉     │    │ Task not triadic?      │                           │
│  └────────┬─────────┘    │ Participants not       │                           │
│           │              │ truly collaborative?   │                           │
│           │              └────────────────────────┘                           │
│           ▼                                                                     │
│  ┌─────────────────────────────────────────────────┐                           │
│  │  LEVEL 4: Compositional Consistency             │                           │
│  │  Does the categorical diagram commute?          │                           │
│  └─────────────────────┬───────────────────────────┘                           │
│                        │                                                        │
│            ┌───────────┴───────────┐                                           │
│            │                       │                                           │
│        COMMUTES              DOESN'T COMMUTE                                   │
│            │                       │                                           │
│            ▼                       ▼                                           │
│  ┌──────────────────┐    ┌────────────────────────┐                           │
│  │ SUCCESS!         │    │ DEBUG: World model     │                           │
│  │                  │    │ inconsistent across    │                           │
│  │ Valid multiplayer│    │ scales? Temporal       │                           │
│  │ BCI experiment   │    │ coherence broken?      │                           │
│  │ with measurable  │    │ Need better shared     │                           │
│  │ emergence        │    │ representation?        │                           │
│  └──────────────────┘    └────────────────────────┘                           │
│                                                                                  │
└─────────────────────────────────────────────────────────────────────────────────┘
```

## Implementation in boxxy

```go
// internal/bci/worlding_debug.go

type WorldingDebugger struct {
    Participants []Participant
    SharedWorld  *SharedWorld
    Levels       [5]DebugLevel
}

type DebugLevel struct {
    Name        string
    Passed      bool
    Metrics     map[string]float64
    DebugNotes  []string
}

func (wd *WorldingDebugger) RunFullPipeline() DebugReport {
    report := DebugReport{}
    
    // Level 0: Signal Validity
    report.Level0 = wd.checkSignalValidity()
    if !report.Level0.Passed {
        return report
    }
    
    // Level 1: Individual Active Inference
    report.Level1 = wd.checkIndividualLoops()
    if !report.Level1.Passed {
        return report
    }
    
    // Level 2: Pairwise Coupling
    report.Level2 = wd.checkPairwiseCoupling()
    if !report.Level2.Passed {
        return report
    }
    
    // Level 3: Triadic Synergy
    report.Level3 = wd.checkTriadicSynergy()
    if !report.Level3.Passed {
        return report
    }
    
    // Level 4: Compositional Consistency
    report.Level4 = wd.checkCompositionalConsistency()
    
    return report
}
```

## The Grid Control Paradigm: fNIRS, tFUS, and Neuromodulation

### Power Grid as BCI Metaphor

The problem of **when to switch on/off power-generating stations and when to charge/discharge grid-scale storage** maps precisely to neuromodulation control:

```
┌─────────────────────────────────────────────────────────────────────────────────┐
│              POWER GRID  ←──────────────────────→  NEURAL GRID                  │
├─────────────────────────────────────────────────────────────────────────────────┤
│                                                                                  │
│  OBSERVATIONS                           OBSERVATIONS                            │
│  ════════════                           ════════════                            │
│  • Voltages (magnitude & phase)         • fNIRS: HbO/HbR concentrations        │
│  • Currents (real & reactive)           • EEG: voltage potentials (μV)         │
│  • Frequency deviations                 • Phase synchrony (PLV, coherence)     │
│  • Power factor                         • Hemodynamic response function        │
│                                                                                  │
│  COMPLEX ENVIRONMENT                    COMPLEX ENVIRONMENT                     │
│  ═══════════════════                    ═══════════════════                     │
│  • Transmission lines (impedance)       • White matter tracts (connectivity)   │
│  • Predictable demand (industrial)      • Task-evoked responses (predictable)  │
│  • Unpredictable demand (residential)   • Spontaneous fluctuations (resting)   │
│  • Weather → renewable capacity         • Arousal/fatigue → neural capacity    │
│  • Price levels (economic dispatch)     • Cognitive load (resource allocation) │
│                                                                                  │
│  CONTROL ACTIONS                        CONTROL ACTIONS                         │
│  ═══════════════                        ═══════════════                         │
│  • Switch generators on/off             • tFUS: focal ultrasound stimulation   │
│  • Charge/discharge storage             • tDCS: excitation/inhibition          │
│  • Adjust transformer taps              • TMS: pulse timing & location         │
│  • Reactive power compensation          • Neurofeedback: reward/penalty        │
│                                                                                  │
└─────────────────────────────────────────────────────────────────────────────────┘
```

### The Neuromodulation Dispatch Problem

```
┌─────────────────────────────────────────────────────────────────────────────────┐
│                    OPTIMAL NEUROMODULATION DISPATCH                             │
├─────────────────────────────────────────────────────────────────────────────────┤
│                                                                                  │
│  Given:                                                                         │
│  ──────                                                                         │
│    • Current neural state S(t) from fNIRS/EEG observations                     │
│    • Target state S* (task goal, therapeutic target)                           │
│    • Available modulators: {tFUS, tDCS, TMS, neurofeedback, pharmacological}   │
│    • Safety constraints: max intensity, duty cycle, spatial bounds             │
│    • "Price" = metabolic cost + fatigue + side effects                         │
│                                                                                  │
│  Find:                                                                          │
│  ─────                                                                          │
│    Optimal control sequence u(t) that minimizes:                               │
│                                                                                  │
│         J = ∫ [ ||S(t) - S*||² + λ·Cost(u(t)) ] dt                            │
│                                                                                  │
│    Subject to:                                                                  │
│         dS/dt = f(S, u, environment)     (neural dynamics)                     │
│         u ∈ U_safe                        (safety constraints)                  │
│         Observations = g(S) + noise       (measurement model)                  │
│                                                                                  │
│                                                                                  │
│  DISPATCH HIERARCHY (like grid economic dispatch):                             │
│  ─────────────────────────────────────────────────                             │
│                                                                                  │
│  ┌─────────────────────────────────────────────────────────────┐               │
│  │  LEVEL 3: Strategic Planning (hours-days)                   │               │
│  │  • Session scheduling                                        │               │
│  │  • Protocol selection                                        │               │
│  │  • Participant matching (multiplayer)                        │               │
│  └─────────────────────────┬───────────────────────────────────┘               │
│                            │                                                    │
│                            ▼                                                    │
│  ┌─────────────────────────────────────────────────────────────┐               │
│  │  LEVEL 2: Economic Dispatch (minutes)                       │               │
│  │  • Which modalities to activate                             │               │
│  │  • Intensity setpoints                                       │               │
│  │  • Target region selection                                   │               │
│  └─────────────────────────┬───────────────────────────────────┘               │
│                            │                                                    │
│                            ▼                                                    │
│  ┌─────────────────────────────────────────────────────────────┐               │
│  │  LEVEL 1: Automatic Generation Control (seconds)            │               │
│  │  • Real-time feedback loops                                  │               │
│  │  • Closed-loop tFUS targeting                               │               │
│  │  • Adaptive neurofeedback thresholds                        │               │
│  └─────────────────────────┬───────────────────────────────────┘               │
│                            │                                                    │
│                            ▼                                                    │
│  ┌─────────────────────────────────────────────────────────────┐               │
│  │  LEVEL 0: Safety Interlocks (milliseconds)                  │               │
│  │  • Hardware safety limits                                    │               │
│  │  • Emergency shutoff                                         │               │
│  │  • Thermal monitoring                                        │               │
│  └─────────────────────────────────────────────────────────────┘               │
│                                                                                  │
└─────────────────────────────────────────────────────────────────────────────────┘
```

### fNIRS + tFUS: The Read-Write Pair

```
┌─────────────────────────────────────────────────────────────────────────────────┐
│                    fNIRS (READ) + tFUS (WRITE) CLOSED LOOP                      │
├─────────────────────────────────────────────────────────────────────────────────┤
│                                                                                  │
│                         ┌─────────────────────┐                                 │
│                         │    NEURAL STATE     │                                 │
│                         │    (cortical ROI)   │                                 │
│                         └──────────┬──────────┘                                 │
│                                    │                                            │
│              ┌─────────────────────┼─────────────────────┐                     │
│              │                     │                     │                     │
│              ▼                     │                     ▼                     │
│     ┌────────────────┐             │            ┌────────────────┐             │
│     │    fNIRS       │             │            │     tFUS       │             │
│     │    (READ)      │             │            │    (WRITE)     │             │
│     │                │             │            │                │             │
│     │  Near-infrared │             │            │  Focused       │             │
│     │  spectroscopy  │             │            │  ultrasound    │             │
│     │                │             │            │                │             │
│     │  Measures:     │             │            │  Modulates:    │             │
│     │  • HbO (oxy)   │             │            │  • Excitation  │             │
│     │  • HbR (deoxy) │             │            │  • Inhibition  │             │
│     │  • Total Hb    │             │            │  • Plasticity  │             │
│     └───────┬────────┘             │            └───────┬────────┘             │
│             │                      │                    │                      │
│             │    OBSERVATIONS      │     CONTROL        │                      │
│             │                      │                    │                      │
│             ▼                      │                    ▼                      │
│     ┌────────────────────────────────────────────────────────────┐             │
│     │                                                            │             │
│     │                    CONTROL ALGORITHM                       │             │
│     │                                                            │             │
│     │   1. Observe: O(t) = [HbO, HbR, phase_sync, ...]          │             │
│     │                                                            │             │
│     │   2. Estimate: Ŝ(t) = Kalman_filter(O(t), Ŝ(t-1))         │             │
│     │                                                            │             │
│     │   3. Predict: Ŝ(t+1) = f(Ŝ(t), u(t))                      │             │
│     │                                                            │             │
│     │   4. Optimize: u*(t) = argmin ||Ŝ(t+1) - S*||² + λC(u)    │             │
│     │                                                            │             │
│     │   5. Apply: tFUS_intensity, tFUS_location, tFUS_timing    │             │
│     │                                                            │             │
│     │   6. Safety check: u*(t) ∈ U_safe ? apply : clip          │             │
│     │                                                            │             │
│     └────────────────────────────────────────────────────────────┘             │
│                                                                                  │
│  GRID ANALOGY:                                                                  │
│  ═════════════                                                                  │
│  fNIRS = PMU (Phasor Measurement Unit) - high-rate state observation           │
│  tFUS  = FACTS device (Flexible AC Transmission) - targeted power injection    │
│  Loop  = Automatic Generation Control - frequency/voltage regulation           │
│                                                                                  │
└─────────────────────────────────────────────────────────────────────────────────┘
```

### Multiplayer Grid: Interconnected Control Areas

```
┌─────────────────────────────────────────────────────────────────────────────────┐
│              MULTIPLAYER BCI AS INTERCONNECTED CONTROL AREAS                    │
├─────────────────────────────────────────────────────────────────────────────────┤
│                                                                                  │
│  Power grids are divided into CONTROL AREAS connected by TIE LINES.            │
│  Each area maintains its own frequency while exchanging power with neighbors.   │
│                                                                                  │
│  In multiplayer BCI:                                                            │
│  • Each participant = Control Area (autonomous neural regulation)              │
│  • Shared task/feedback = Tie Lines (coupling between areas)                   │
│  • Synergy = Net Interchange (power flow that benefits the whole grid)         │
│                                                                                  │
│                                                                                  │
│         CONTROL AREA 1 (🧠₁)              CONTROL AREA 2 (🧠₂)                 │
│        ┌─────────────────────┐           ┌─────────────────────┐               │
│        │                     │           │                     │               │
│        │   ┌───┐    ┌───┐   │           │   ┌───┐    ┌───┐   │               │
│        │   │fNI│    │tFU│   │  TIE LINE │   │fNI│    │tFU│   │               │
│        │   │RS │    │S  │   │◄─────────►│   │RS │    │S  │   │               │
│        │   └─┬─┘    └─┬─┘   │  (shared  │   └─┬─┘    └─┬─┘   │               │
│        │     │        │     │  feedback)│     │        │     │               │
│        │     └────┬───┘     │           │     └────┬───┘     │               │
│        │          │         │           │          │         │               │
│        │    ┌─────┴─────┐   │           │    ┌─────┴─────┐   │               │
│        │    │  AGC 1    │   │           │    │  AGC 2    │   │               │
│        │    │ (control) │   │           │    │ (control) │   │               │
│        │    └───────────┘   │           │    └───────────┘   │               │
│        │                     │           │                     │               │
│        └──────────┬──────────┘           └──────────┬──────────┘               │
│                   │                                 │                          │
│                   │         TIE LINE                │                          │
│                   └────────────┬────────────────────┘                          │
│                                │                                               │
│                                ▼                                               │
│                   ┌─────────────────────────┐                                  │
│                   │   CONTROL AREA 3 (🧠₃)  │                                  │
│                   │                         │                                  │
│                   │   ┌───┐    ┌───┐       │                                  │
│                   │   │fNI│    │tFU│       │                                  │
│                   │   │RS │    │S  │       │                                  │
│                   │   └─┬─┘    └─┬─┘       │                                  │
│                   │     └────┬───┘         │                                  │
│                   │          │             │                                  │
│                   │    ┌─────┴─────┐       │                                  │
│                   │    │  AGC 3    │       │                                  │
│                   │    └───────────┘       │                                  │
│                   │                         │                                  │
│                   └─────────────────────────┘                                  │
│                                                                                  │
│                                                                                  │
│  KEY METRICS (Grid ↔ BCI):                                                      │
│  ═════════════════════════                                                      │
│                                                                                  │
│  ┌──────────────────────┬────────────────────────────────────────────┐         │
│  │  GRID METRIC         │  BCI EQUIVALENT                            │         │
│  ├──────────────────────┼────────────────────────────────────────────┤         │
│  │  Frequency (60Hz)    │  Dominant oscillation (alpha/theta/gamma)  │         │
│  │  Voltage magnitude   │  Signal amplitude (μV for EEG, ΔHb for fNIRS)│       │
│  │  Phase angle         │  Phase synchrony index                     │         │
│  │  Real power (P)      │  Task-related activation                   │         │
│  │  Reactive power (Q)  │  Connectivity/coupling strength            │         │
│  │  Power factor        │  Efficiency of neural coding               │         │
│  │  Tie-line flow       │  Inter-brain synchrony (IBS)              │         │
│  │  Area Control Error  │  Prediction error (active inference)       │         │
│  │  Reserves (spinning) │  Cognitive capacity / attention            │         │
│  └──────────────────────┴────────────────────────────────────────────┘         │
│                                                                                  │
└─────────────────────────────────────────────────────────────────────────────────┘
```

### Safety: The N-1 Criterion for Neuromodulation

```
┌─────────────────────────────────────────────────────────────────────────────────┐
│                    N-1 SAFETY CRITERION FOR BCI                                 │
├─────────────────────────────────────────────────────────────────────────────────┤
│                                                                                  │
│  Power grids are designed to survive ANY SINGLE contingency (N-1 criterion).   │
│  The system must remain stable if any one component fails.                      │
│                                                                                  │
│  FOR MULTIPLAYER BCI + NEUROMODULATION:                                         │
│                                                                                  │
│  ┌─────────────────────────────────────────────────────────────────────────┐   │
│  │  N-1 SAFETY CHECKLIST                                                   │   │
│  ├─────────────────────────────────────────────────────────────────────────┤   │
│  │                                                                         │   │
│  │  □ If tFUS fails → System degrades gracefully to observation-only      │   │
│  │  □ If fNIRS fails → tFUS enters safe standby (no blind stimulation)    │   │
│  │  □ If one participant disconnects → Others continue safely             │   │
│  │  □ If network fails → Local safety interlocks take over                │   │
│  │  □ If control algorithm diverges → Hardware limits prevent harm        │   │
│  │                                                                         │   │
│  │  NEVER ALLOWED (N-2 scenarios that must be prevented):                 │   │
│  │  ═══════════════════════════════════════════════════                   │   │
│  │  ✗ Stimulation without observation (blind write)                       │   │
│  │  ✗ Multiple simultaneous modality failures                             │   │
│  │  ✗ Cascading failures across participants                              │   │
│  │  ✗ Control loop without safety bounds                                  │   │
│  │                                                                         │   │
│  └─────────────────────────────────────────────────────────────────────────┘   │
│                                                                                  │
│                                                                                  │
│  DEVICE COMBINATION SAFETY MATRIX:                                              │
│  ═════════════════════════════════                                              │
│                                                                                  │
│              │  EEG   │ fNIRS  │  tFUS  │  tDCS  │  TMS   │  fMRI  │           │
│  ────────────┼────────┼────────┼────────┼────────┼────────┼────────┤           │
│  EEG         │   ✓    │   ✓    │   ⚠    │   ⚠    │   ✗    │   ⚠    │           │
│  fNIRS       │   ✓    │   ✓    │   ✓    │   ✓    │   ⚠    │   ✗    │           │
│  tFUS        │   ⚠    │   ✓    │   ✗    │   ⚠    │   ✗    │   ✗    │           │
│  tDCS        │   ⚠    │   ✓    │   ⚠    │   ✗    │   ✗    │   ✗    │           │
│  TMS         │   ✗    │   ⚠    │   ✗    │   ✗    │   ✗    │   ✗    │           │
│  fMRI        │   ⚠    │   ✗    │   ✗    │   ✗    │   ✗    │   ✓    │           │
│                                                                                  │
│  Legend: ✓ = Safe concurrent use                                               │
│          ⚠ = Requires careful protocol / spatial separation                    │
│          ✗ = Contraindicated / requires expert review                          │
│                                                                                  │
│                                                                                  │
│  THERMAL BUDGET (like transmission line thermal limits):                        │
│  ═══════════════════════════════════════════════════════                        │
│                                                                                  │
│  Each modality contributes to cumulative thermal load:                          │
│                                                                                  │
│      T_total = Σᵢ T_modality_i + T_metabolic + T_ambient                       │
│                                                                                  │
│      CONSTRAINT: T_total < T_safe (typically ΔT < 1°C in tissue)               │
│                                                                                  │
│      tFUS duty cycle must respect:                                              │
│      • ISPTA < 720 mW/cm² (FDA diagnostic limit)                               │
│      • ISPPA < 190 W/cm² (mechanical index limit)                              │
│      • Cumulative exposure tracking across session                             │
│                                                                                  │
└─────────────────────────────────────────────────────────────────────────────────┘
```

### State Estimation: The Neural Kalman Filter

```
┌─────────────────────────────────────────────────────────────────────────────────┐
│                    STATE ESTIMATION FOR NEUROMODULATION                         │
├─────────────────────────────────────────────────────────────────────────────────┤
│                                                                                  │
│  Like grid state estimation, we never observe the true neural state directly.  │
│  We must ESTIMATE it from noisy, partial observations.                          │
│                                                                                  │
│                                                                                  │
│      TRUE STATE                    OBSERVATIONS                                 │
│      ══════════                    ════════════                                 │
│                                                                                  │
│      x(t) = [                      y(t) = [                                    │
│        neural_activity,              HbO_channel_1,                            │
│        connectivity,                 HbO_channel_2,                            │
│        arousal,                      ...,                                      │
│        fatigue,                      HbR_channel_1,                            │
│        attention,                    ...,                                      │
│        ...                           EEG_electrode_1,                          │
│      ]                               ...                                       │
│                                    ]                                           │
│                                                                                  │
│      dim(x) >> dim(y)   (state is higher-dimensional than observations)        │
│                                                                                  │
│                                                                                  │
│  ESTIMATION PIPELINE:                                                           │
│  ════════════════════                                                           │
│                                                                                  │
│      ┌─────────────────────────────────────────────────────────────────┐       │
│      │                                                                 │       │
│      │   1. PREDICT (time update):                                    │       │
│      │      x̂(t|t-1) = A·x̂(t-1|t-1) + B·u(t-1)                       │       │
│      │      P(t|t-1) = A·P(t-1|t-1)·Aᵀ + Q                            │       │
│      │                                                                 │       │
│      │   2. UPDATE (measurement update):                              │       │
│      │      K(t) = P(t|t-1)·Hᵀ·(H·P(t|t-1)·Hᵀ + R)⁻¹                 │       │
│      │      x̂(t|t) = x̂(t|t-1) + K(t)·(y(t) - H·x̂(t|t-1))            │       │
│      │      P(t|t) = (I - K(t)·H)·P(t|t-1)                            │       │
│      │                                                                 │       │
│      │   Where:                                                        │       │
│      │      A = State transition (neural dynamics model)              │       │
│      │      B = Control input (neuromodulation effect model)          │       │
│      │      H = Observation model (fNIRS forward model)               │       │
│      │      Q = Process noise (neural variability)                    │       │
│      │      R = Measurement noise (sensor noise)                      │       │
│      │      K = Kalman gain (optimal weighting)                       │       │
│      │                                                                 │       │
│      └─────────────────────────────────────────────────────────────────┘       │
│                                                                                  │
│                                                                                  │
│  MULTIPLAYER EXTENSION:                                                         │
│  ══════════════════════                                                         │
│                                                                                  │
│  For 3 participants, the joint state includes coupling terms:                   │
│                                                                                  │
│      x_joint(t) = [                                                            │
│        x₁(t),           # Participant 1 state                                  │
│        x₂(t),           # Participant 2 state                                  │
│        x₃(t),           # Participant 3 state                                  │
│        c₁₂(t),          # 1↔2 coupling state                                   │
│        c₁₃(t),          # 1↔3 coupling state                                   │
│        c₂₃(t),          # 2↔3 coupling state                                   │
│        c₁₂₃(t),         # TRIADIC synergy state ← THIS IS THE EMERGENCE       │
│      ]                                                                         │
│                                                                                  │
│  The triadic term c₁₂₃(t) captures information that exists ONLY in the        │
│  three-way interaction — the α-synergy backbone's ∂H₁ component.              │
│                                                                                  │
└─────────────────────────────────────────────────────────────────────────────────┘
```

### Economic Dispatch: Optimizing Neuromodulation Resources

```
┌─────────────────────────────────────────────────────────────────────────────────┐
│                    ECONOMIC DISPATCH FOR NEUROMODULATION                        │
├─────────────────────────────────────────────────────────────────────────────────┤
│                                                                                  │
│  Grid operators solve ECONOMIC DISPATCH to minimize generation cost while       │
│  meeting demand and respecting constraints.                                      │
│                                                                                  │
│  For BCI, we solve NEUROMODULATION DISPATCH:                                    │
│                                                                                  │
│                                                                                  │
│  OBJECTIVE:                                                                     │
│  ══════════                                                                     │
│                                                                                  │
│      min  Σᵢ Costᵢ(uᵢ)  +  λ·||S(t+Δt) - S*||²                                │
│       u                                                                         │
│           ↑                      ↑                                              │
│    modulation cost         tracking error                                       │
│    (fatigue, side          (deviation from                                      │
│     effects, $)             target state)                                       │
│                                                                                  │
│                                                                                  │
│  CONSTRAINTS:                                                                   │
│  ════════════                                                                   │
│                                                                                  │
│  • Power balance:    Σⱼ Stimulation_j = Required_activation                    │
│  • Safety limits:    u_min ≤ uᵢ ≤ u_max (intensity bounds)                     │
│  • Thermal limits:   T_total < T_safe                                          │
│  • Duty cycle:       ON_time / (ON_time + OFF_time) ≤ max_duty                 │
│  • Ramp rates:       |duᵢ/dt| ≤ ramp_max (no sudden changes)                   │
│  • Spatial:          Modalities don't overlap unsafely                         │
│                                                                                  │
│                                                                                  │
│  COST FUNCTIONS BY MODALITY:                                                    │
│  ════════════════════════════                                                   │
│                                                                                  │
│  ┌────────────┬─────────────────────────────────────────────────────────┐      │
│  │  MODALITY  │  COST FUNCTION C(u)                                     │      │
│  ├────────────┼─────────────────────────────────────────────────────────┤      │
│  │  tFUS      │  C = α·u² + β·∫u dt  (quadratic + cumulative)          │      │
│  │            │  Low marginal cost, good for sustained modulation      │      │
│  ├────────────┼─────────────────────────────────────────────────────────┤      │
│  │  tDCS      │  C = α·u + β·duration  (linear + time)                 │      │
│  │            │  Skin sensation limits intensity                        │      │
│  ├────────────┼─────────────────────────────────────────────────────────┤      │
│  │  TMS       │  C = α·n_pulses + β·intensity²                         │      │
│  │            │  High startup cost, good for brief interventions        │      │
│  ├────────────┼─────────────────────────────────────────────────────────┤      │
│  │  Neuro-    │  C = γ·cognitive_load + δ·time                         │      │
│  │  feedback  │  Zero hardware cost, attention is the currency         │      │
│  └────────────┴─────────────────────────────────────────────────────────┘      │
│                                                                                  │
│                                                                                  │
│  MERIT ORDER (like power plant dispatch order):                                 │
│  ══════════════════════════════════════════════                                 │
│                                                                                  │
│  Cost/effect │                                                                  │
│              │                                    ┌────────────┐                │
│              │                          ┌────────┤    TMS     │                │
│              │                ┌─────────┤        │ (peaking)  │                │
│              │      ┌─────────┤  tDCS   │        └────────────┘                │
│              │      │         │         │                                       │
│              │      │  tFUS   └─────────┘                                       │
│              │      │ (base                                                     │
│              │      │  load)                                                    │
│              │──────┴──────────────────────────────────────────► Effect        │
│              │                                                                  │
│              │ Neurofeedback (zero marginal cost, always "on")                 │
│              └─────────────────────────────────────────────────────────        │
│                                                                                  │
│  Dispatch order: Neurofeedback → tFUS → tDCS → TMS                             │
│  (like: renewables → nuclear → gas → peakers)                                  │
│                                                                                  │
└─────────────────────────────────────────────────────────────────────────────────┘
```

## Counterfactual Interventions: Causal Inference in Neuromodulation

### The Fundamental Problem: Correlation ≠ Causation

```
┌─────────────────────────────────────────────────────────────────────────────────┐
│                    THE COUNTERFACTUAL QUESTION                                  │
├─────────────────────────────────────────────────────────────────────────────────┤
│                                                                                  │
│  OBSERVATION:   When we apply tFUS to region R, activity increases.            │
│                                                                                  │
│  QUESTION:      Did tFUS CAUSE the increase?                                   │
│                 Or would it have happened anyway?                               │
│                                                                                  │
│  COUNTERFACTUAL: What WOULD have happened if we HADN'T applied tFUS?           │
│                                                                                  │
│                                                                                  │
│       ACTUAL WORLD                     COUNTERFACTUAL WORLD                    │
│       ════════════                     ════════════════════                    │
│                                                                                  │
│       t=0: State S₀                    t=0: State S₀  (same)                   │
│             │                                │                                  │
│             ▼                                ▼                                  │
│       t=1: Apply tFUS                  t=1: No intervention                    │
│             │                                │                                  │
│             ▼                                ▼                                  │
│       t=2: State S₂                    t=2: State S₂'                          │
│             │                                │                                  │
│             ▼                                ▼                                  │
│       OBSERVED                         UNOBSERVED                              │
│       (factual)                        (counterfactual)                        │
│                                                                                  │
│                                                                                  │
│   CAUSAL EFFECT = S₂ - S₂'  (difference between worlds)                        │
│                                                                                  │
│   Problem: We can NEVER observe S₂' directly.                                  │
│   Solution: Structural causal models + do-calculus                             │
│                                                                                  │
└─────────────────────────────────────────────────────────────────────────────────┘
```

### Pearl's Ladder of Causation for BCI

```
┌─────────────────────────────────────────────────────────────────────────────────┐
│                    LADDER OF CAUSATION IN BCI                                   │
├─────────────────────────────────────────────────────────────────────────────────┤
│                                                                                  │
│  RUNG 3: COUNTERFACTUALS (Imagination)                                         │
│  ══════════════════════════════════════                                         │
│  "What if I had NOT stimulated?"                                               │
│  "Would synchrony have emerged without tFUS?"                                  │
│                                                                                  │
│       Query:  P(S₂' | S₂, do(tFUS))                                            │
│               "Given what I observed after stimulation,                        │
│                what would have happened without it?"                           │
│                                                                                  │
│       Requires: Full structural model + noise terms                            │
│                                                                                  │
│                          ▲                                                      │
│                          │                                                      │
│  ────────────────────────┼──────────────────────────────────────────────────   │
│                          │                                                      │
│  RUNG 2: INTERVENTIONS (Doing)                                                 │
│  ═════════════════════════════                                                  │
│  "What happens if I stimulate region R?"                                       │
│  "What if I turn OFF participant 2's feedback?"                                │
│                                                                                  │
│       Query:  P(S₂ | do(tFUS = on))                                            │
│               "What is the distribution of outcomes                            │
│                when I force tFUS to be on?"                                    │
│                                                                                  │
│       Requires: Causal graph + intervention semantics                          │
│                                                                                  │
│       do(X=x) ≠ observe(X=x)                                                   │
│       ─────────────────────                                                    │
│       Intervention CUTS incoming edges to X                                    │
│       Observation does NOT                                                     │
│                                                                                  │
│                          ▲                                                      │
│                          │                                                      │
│  ────────────────────────┼──────────────────────────────────────────────────   │
│                          │                                                      │
│  RUNG 1: ASSOCIATION (Seeing)                                                  │
│  ═════════════════════════════                                                  │
│  "tFUS and increased activity co-occur"                                        │
│  "Participants who synchronize perform better"                                 │
│                                                                                  │
│       Query:  P(S₂ | tFUS = on)                                                │
│               "What is the probability of high activity                        │
│                given that tFUS was on?"                                        │
│                                                                                  │
│       Requires: Only joint distribution P(S, tFUS)                             │
│                                                                                  │
│       ⚠️ CONFOUNDING: Maybe high-activity states                               │
│          trigger tFUS AND cause S₂ independently                               │
│                                                                                  │
└─────────────────────────────────────────────────────────────────────────────────┘
```

### Structural Causal Model for Multiplayer BCI

```
┌─────────────────────────────────────────────────────────────────────────────────┐
│                    SCM FOR 3-PARTICIPANT NEUROMODULATION                        │
├─────────────────────────────────────────────────────────────────────────────────┤
│                                                                                  │
│  STRUCTURAL EQUATIONS:                                                          │
│  ═════════════════════                                                          │
│                                                                                  │
│    X₁ := f₁(U₁, tFUS₁, Feedback)           # Participant 1 neural state        │
│    X₂ := f₂(U₂, tFUS₂, Feedback)           # Participant 2 neural state        │
│    X₃ := f₃(U₃, tFUS₃, Feedback)           # Participant 3 neural state        │
│                                                                                  │
│    Sync₁₂ := g₁₂(X₁, X₂, U₁₂)              # Pairwise synchrony                │
│    Sync₁₃ := g₁₃(X₁, X₃, U₁₃)                                                  │
│    Sync₂₃ := g₂₃(X₂, X₃, U₂₃)                                                  │
│                                                                                  │
│    Synergy := h(X₁, X₂, X₃, U_syn)         # Triadic emergence                 │
│                                                                                  │
│    Task := τ(Synergy, U_task)              # Task performance                  │
│                                                                                  │
│    Feedback := φ(Task)                      # Closed-loop feedback             │
│                                                                                  │
│                                                                                  │
│  CAUSAL GRAPH:                                                                  │
│  ═════════════                                                                  │
│                                                                                  │
│                    ┌─────────┐                                                  │
│                    │ U_task  │                                                  │
│                    └────┬────┘                                                  │
│                         │                                                       │
│                         ▼                                                       │
│    ┌──────┐       ┌──────────┐       ┌──────────┐                              │
│    │tFUS₁ │──────►│          │◄──────│  tFUS₂   │                              │
│    └──────┘       │   TASK   │       └──────────┘                              │
│         │         │PERFORMANCE│            │                                    │
│         │         └─────▲────┘            │                                    │
│         │               │                 │                                    │
│         │         ┌─────┴────┐            │                                    │
│         │         │ SYNERGY  │            │                                    │
│         │         │ (∂H₁)    │            │                                    │
│         │         └─────▲────┘            │                                    │
│         │               │                 │                                    │
│         │    ┌──────────┼──────────┐      │                                    │
│         │    │          │          │      │                                    │
│         ▼    ▼          │          ▼      ▼                                    │
│       ┌────────┐   ┌────┴───┐   ┌────────┐                                     │
│       │   X₁   │───│  X₂    │───│   X₃   │                                     │
│       │   🧠₁  │   │  🧠₂   │   │   🧠₃  │                                     │
│       └───▲────┘   └───▲────┘   └───▲────┘                                     │
│           │            │            │                                          │
│       ┌───┴───┐    ┌───┴───┐    ┌───┴───┐                                      │
│       │  U₁   │    │  U₂   │    │  U₃   │                                      │
│       │(noise)│    │(noise)│    │(noise)│                                      │
│       └───────┘    └───────┘    └───────┘                                      │
│                                                                                  │
│           ▲                                         │                          │
│           │         ┌──────────────┐                │                          │
│           └─────────│   FEEDBACK   │◄───────────────┘                          │
│                     │   (closed    │                                           │
│                     │    loop)     │                                           │
│                     └──────────────┘                                           │
│                                                                                  │
└─────────────────────────────────────────────────────────────────────────────────┘
```

### Intervention Types and Their Effects

```
┌─────────────────────────────────────────────────────────────────────────────────┐
│                    INTERVENTION CALCULUS FOR BCI                                │
├─────────────────────────────────────────────────────────────────────────────────┤
│                                                                                  │
│  TYPE 1: HARD INTERVENTION do(X := x)                                          │
│  ════════════════════════════════════                                          │
│  Force variable to specific value, cutting all incoming edges.                  │
│                                                                                  │
│       Example: do(tFUS₁ := 0.5 W/cm²)                                          │
│                                                                                  │
│       Before intervention:        After do(tFUS₁ := 0.5):                      │
│                                                                                  │
│       Controller ──► tFUS₁        Controller ──╳  tFUS₁ := 0.5                 │
│            │              │                              │                      │
│            │              ▼                              ▼                      │
│            └────► X₁ ◄────┘                       X₁ ◄───┘                      │
│                                                                                  │
│       The controller no longer affects tFUS₁;                                  │
│       we've surgically set it to 0.5 W/cm².                                    │
│                                                                                  │
│                                                                                  │
│  TYPE 2: SOFT INTERVENTION do(X := f(PA_X, ε))                                 │
│  ═════════════════════════════════════════════                                  │
│  Modify the mechanism, not the value.                                           │
│                                                                                  │
│       Example: do(tFUS₁ := 2 × original_policy)                                │
│       "Double the stimulation intensity the controller would have chosen"       │
│                                                                                  │
│       Useful for: Sensitivity analysis, dose-response curves                   │
│                                                                                  │
│                                                                                  │
│  TYPE 3: CONDITIONAL INTERVENTION do(X := x | Z = z)                           │
│  ═══════════════════════════════════════════════════                           │
│  Intervene only when condition is met.                                          │
│                                                                                  │
│       Example: do(tFUS₁ := ON | Sync₁₂ < threshold)                            │
│       "Turn on stimulation only when synchrony drops"                          │
│                                                                                  │
│       This is the ADAPTIVE PROTOCOL — intervene based on state.                │
│                                                                                  │
│                                                                                  │
│  TYPE 4: STOCHASTIC INTERVENTION do(X ~ P'(X))                                 │
│  ═════════════════════════════════════════════                                  │
│  Replace mechanism with new distribution.                                       │
│                                                                                  │
│       Example: do(tFUS₁ ~ Uniform(0, 1))                                       │
│       "Randomize stimulation" — gold standard for causal identification        │
│                                                                                  │
│       But: Ethical constraints limit randomization in human BCI                │
│                                                                                  │
└─────────────────────────────────────────────────────────────────────────────────┘
```

### Counterfactual Queries for Debugging

```
┌─────────────────────────────────────────────────────────────────────────────────┐
│                    COUNTERFACTUAL DEBUGGING QUERIES                             │
├─────────────────────────────────────────────────────────────────────────────────┤
│                                                                                  │
│  When an experiment fails or succeeds, we need to understand WHY.              │
│  Counterfactuals let us reason about alternative histories.                     │
│                                                                                  │
│                                                                                  │
│  QUERY 1: Attribution                                                           │
│  ═══════════════════                                                            │
│  "The task succeeded. Was it BECAUSE of tFUS?"                                 │
│                                                                                  │
│       P(Task_success = 0 | Task_success = 1, do(tFUS = 0))                     │
│       "Given success occurred, would it have failed without tFUS?"             │
│                                                                                  │
│       If P > 0.5: tFUS was likely necessary for success                        │
│       If P ≈ 0:   Success would have happened anyway                           │
│                                                                                  │
│                                                                                  │
│  QUERY 2: Synergy Counterfactual                                               │
│  ═══════════════════════════════                                                │
│  "Synergy emerged. Was participant 3 essential?"                               │
│                                                                                  │
│       P(∂H₁ = 0 | ∂H₁ > 0, do(X₃ = noise))                                     │
│       "Given synergy occurred, would it vanish if P3 contributed noise?"       │
│                                                                                  │
│       By definition of ∂H₁ (true synergy): should be 1.0                       │
│       If < 1.0: What we measured wasn't true 3-way synergy                     │
│                                                                                  │
│                                                                                  │
│  QUERY 3: Feedback Necessity                                                   │
│  ═════════════════════════                                                      │
│  "Would the system have synchronized without feedback?"                        │
│                                                                                  │
│       P(Sync > θ at t=T | Sync > θ at t=T, do(Feedback = 0 for t>0))          │
│       "Given sync achieved, would open-loop have worked?"                      │
│                                                                                  │
│       If P ≈ 1: Feedback wasn't necessary (intrinsic sync)                     │
│       If P ≈ 0: Feedback was essential (learned sync)                          │
│                                                                                  │
│                                                                                  │
│  QUERY 4: Modality Substitution                                                │
│  ═════════════════════════════                                                  │
│  "Would tDCS have worked as well as tFUS?"                                     │
│                                                                                  │
│       P(Task_success | Task_success, do(tFUS → tDCS))                          │
│       "Given tFUS succeeded, would tDCS have also?"                            │
│                                                                                  │
│       Requires: Structural equivalence assumptions                             │
│       Useful for: Protocol optimization, resource allocation                   │
│                                                                                  │
│                                                                                  │
│  QUERY 5: Participant Counterfactual                                           │
│  ═══════════════════════════════════                                            │
│  "What if participant 2 had been more experienced?"                            │
│                                                                                  │
│       P(Sync | Sync_observed, do(U₂ → U₂_expert))                              │
│       "Would sync have been higher with an expert?"                            │
│                                                                                  │
│       Requires: Model of expertise as noise reduction                          │
│       Useful for: Participant matching, training design                        │
│                                                                                  │
└─────────────────────────────────────────────────────────────────────────────────┘
```

### The Twin Network Method for Computing Counterfactuals

```
┌─────────────────────────────────────────────────────────────────────────────────┐
│                    TWIN NETWORK FOR BCI COUNTERFACTUALS                         │
├─────────────────────────────────────────────────────────────────────────────────┤
│                                                                                  │
│  To compute counterfactuals, we run TWO copies of the causal model:            │
│  one factual (what happened) and one counterfactual (what would have).         │
│  They SHARE the same noise terms (U), ensuring consistency.                    │
│                                                                                  │
│                                                                                  │
│       FACTUAL WORLD                    COUNTERFACTUAL WORLD                    │
│       (what happened)                  (what would have happened)              │
│                                                                                  │
│       tFUS₁ = ON                       tFUS₁' = OFF (intervention)             │
│           │                                │                                    │
│           ▼                                ▼                                    │
│       X₁ = f₁(U₁, ON, FB)              X₁' = f₁(U₁, OFF, FB')                 │
│           │                                │                                    │
│           │     ┌──────────────────────────┤                                    │
│           │     │     SHARED NOISE         │                                    │
│           │     │                          │                                    │
│           │     │    U₁ ════════════ U₁    │                                    │
│           │     │    U₂ ════════════ U₂    │                                    │
│           │     │    U₃ ════════════ U₃    │                                    │
│           │     │                          │                                    │
│           │     └──────────────────────────┤                                    │
│           │                                │                                    │
│           ▼                                ▼                                    │
│       Sync = g(X₁,X₂,X₃)               Sync' = g(X₁',X₂',X₃')                 │
│           │                                │                                    │
│           ▼                                ▼                                    │
│       Task = τ(Sync)                   Task' = τ(Sync')                        │
│                                                                                  │
│                                                                                  │
│  CAUSAL EFFECT = Task - Task'                                                  │
│                                                                                  │
│                                                                                  │
│  ALGORITHM:                                                                     │
│  ══════════                                                                     │
│                                                                                  │
│  1. ABDUCTION: Given observations, infer noise terms                           │
│     U₁, U₂, U₃ ← argmax P(U | observed X₁, X₂, X₃, tFUS, Feedback)            │
│                                                                                  │
│  2. ACTION: Modify the structural equation for intervention                    │
│     Replace: X₁ := f₁(U₁, tFUS₁, FB)                                          │
│     With:    X₁' := f₁(U₁, 0, FB')   ← tFUS forced to 0                       │
│                                                                                  │
│  3. PREDICTION: Propagate through counterfactual world                         │
│     Compute X₁', X₂', X₃', Sync', Task' using inferred U                       │
│                                                                                  │
│  4. COMPARE: Causal effect = Factual - Counterfactual                          │
│     ΔTask = Task - Task'                                                       │
│     ΔSync = Sync - Sync'                                                       │
│     Δ∂H₁ = ∂H₁ - ∂H₁'                                                         │
│                                                                                  │
└─────────────────────────────────────────────────────────────────────────────────┘
```

### Identifiability: When Can We Compute Counterfactuals?

```
┌─────────────────────────────────────────────────────────────────────────────────┐
│                    IDENTIFIABILITY CONDITIONS                                   │
├─────────────────────────────────────────────────────────────────────────────────┤
│                                                                                  │
│  Not all counterfactuals can be computed from data. We need:                   │
│                                                                                  │
│                                                                                  │
│  CONDITION 1: No Unmeasured Confounders                                        │
│  ═══════════════════════════════════════                                        │
│                                                                                  │
│       BAD (unidentifiable):           GOOD (identifiable):                     │
│                                                                                  │
│            ┌───┐                           tFUS ──────► X                       │
│            │ U │ (unmeasured)                   │                               │
│            └─┬─┘                                │                               │
│         ┌───┴───┐                              ▼                               │
│         │       │                            Task                              │
│         ▼       ▼                                                              │
│       tFUS ──► X ──► Task              (or U is measured/randomized)           │
│                                                                                  │
│       U confounds tFUS→Task                                                    │
│       We can't distinguish:                                                    │
│       "tFUS helps" vs "U causes both"                                          │
│                                                                                  │
│                                                                                  │
│  CONDITION 2: Invertible Mechanisms                                            │
│  ═══════════════════════════════════                                            │
│                                                                                  │
│       To infer U from X, we need:                                              │
│       X = f(U, Parents) to be INVERTIBLE in U                                  │
│                                                                                  │
│       Linear case:  X = AX_pa + U  →  U = X - AX_pa  ✓                        │
│       Nonlinear:    X = σ(WX_pa + U)  →  U = σ⁻¹(X) - WX_pa  ✓               │
│       Non-invertible: X = max(U₁, U₂)  →  Can't recover U₁, U₂  ✗            │
│                                                                                  │
│                                                                                  │
│  CONDITION 3: Sufficient Observations                                          │
│  ════════════════════════════════════                                           │
│                                                                                  │
│       Need enough observations to constrain counterfactual:                    │
│                                                                                  │
│       • Single-trial: High uncertainty in U, wide counterfactual bounds       │
│       • Many trials: Can estimate P(U), sharper counterfactual inference      │
│       • Repeated measures: Can separate within-subject U from between         │
│                                                                                  │
│                                                                                  │
│  FOR MULTIPLAYER BCI:                                                          │
│  ════════════════════                                                           │
│                                                                                  │
│  ┌─────────────────────────────────────────────────────────────────────────┐   │
│  │  IDENTIFIABLE (can compute counterfactual)                              │   │
│  │  • "Would sync have occurred without tFUS?" (if tFUS randomized)       │   │
│  │  • "What if feedback latency was 50ms lower?" (latency measured)       │   │
│  │  • "Would P3 alone have solved the task?" (marginal computable)        │   │
│  ├─────────────────────────────────────────────────────────────────────────┤   │
│  │  PARTIALLY IDENTIFIABLE (bounds only)                                  │   │
│  │  • "Would sync have occurred with different participants?"             │   │
│  │  • "What if participants had met before?" (social U unmeasured)        │   │
│  ├─────────────────────────────────────────────────────────────────────────┤   │
│  │  NOT IDENTIFIABLE (need assumptions)                                   │   │
│  │  • "Would this work with healthy controls?" (population mismatch)      │   │
│  │  • "What if we used EEG instead of fNIRS?" (mechanism changes)         │   │
│  └─────────────────────────────────────────────────────────────────────────┘   │
│                                                                                  │
└─────────────────────────────────────────────────────────────────────────────────┘
```

### Counterfactual-Guided Protocol Design

```
┌─────────────────────────────────────────────────────────────────────────────────┐
│                    COUNTERFACTUAL PROTOCOL OPTIMIZATION                         │
├─────────────────────────────────────────────────────────────────────────────────┤
│                                                                                  │
│  Use counterfactual reasoning to design BETTER protocols:                       │
│                                                                                  │
│                                                                                  │
│  STEP 1: Run pilot with current protocol                                       │
│  ════════════════════════════════════════                                       │
│                                                                                  │
│       Protocol P₀: tFUS at 0.5 W/cm², feedback delay 100ms                     │
│       Result: 60% of sessions achieve sync                                     │
│                                                                                  │
│                                                                                  │
│  STEP 2: Fit structural causal model                                           │
│  ═══════════════════════════════════                                            │
│                                                                                  │
│       From pilot data, estimate:                                               │
│       • f₁, f₂, f₃ (neural dynamics)                                          │
│       • g (synchrony mechanism)                                                │
│       • P(U) (noise distributions)                                             │
│                                                                                  │
│                                                                                  │
│  STEP 3: Ask counterfactual questions                                          │
│  ════════════════════════════════════                                           │
│                                                                                  │
│       For each FAILED session:                                                 │
│                                                                                  │
│       Q1: P(Sync > θ | Sync < θ, do(tFUS = 0.7))                              │
│           "Would higher intensity have helped?"                                │
│                                                                                  │
│       Q2: P(Sync > θ | Sync < θ, do(delay = 50ms))                            │
│           "Would faster feedback have helped?"                                 │
│                                                                                  │
│       Q3: P(Sync > θ | Sync < θ, do(Participant₃ = expert))                   │
│           "Would an expert P3 have helped?"                                    │
│                                                                                  │
│                                                                                  │
│  STEP 4: Identify intervention with highest counterfactual lift                │
│  ══════════════════════════════════════════════════════════════                │
│                                                                                  │
│       Rank by: E[Sync | do(intervention)] - E[Sync | status quo]              │
│                                                                                  │
│       ┌──────────────────────────────┬─────────────────┐                       │
│       │  INTERVENTION                │  EXPECTED LIFT  │                       │
│       ├──────────────────────────────┼─────────────────┤                       │
│       │  Reduce delay to 50ms        │     +0.23       │ ← Best                │
│       │  Increase tFUS to 0.7 W/cm²  │     +0.15       │                       │
│       │  Add training session        │     +0.12       │                       │
│       │  Match participants by trait │     +0.08       │                       │
│       └──────────────────────────────┴─────────────────┘                       │
│                                                                                  │
│                                                                                  │
│  STEP 5: Implement top intervention, iterate                                   │
│  ═══════════════════════════════════════════                                    │
│                                                                                  │
│       Protocol P₁: tFUS at 0.5 W/cm², feedback delay 50ms                      │
│       Re-run pilot, update model, repeat...                                    │
│                                                                                  │
│       This is CAUSAL BANDIT OPTIMIZATION:                                      │
│       Explore interventions, exploit counterfactual predictions                │
│                                                                                  │
└─────────────────────────────────────────────────────────────────────────────────┘
```

### Grid Analogy: Contingency Analysis

```
┌─────────────────────────────────────────────────────────────────────────────────┐
│                    CONTINGENCY ANALYSIS FOR BCI                                 │
├─────────────────────────────────────────────────────────────────────────────────┤
│                                                                                  │
│  Power grid operators run CONTINGENCY ANALYSIS: "What if line X fails?"        │
│  This is exactly counterfactual reasoning applied to infrastructure.            │
│                                                                                  │
│                                                                                  │
│  GRID CONTINGENCY                      BCI CONTINGENCY                          │
│  ════════════════                      ═══════════════                          │
│                                                                                  │
│  "What if transmission                 "What if the tFUS                        │
│   line A-B trips?"                      transducer fails?"                      │
│                                                                                  │
│  → Simulate N-1 state                  → Simulate fallback mode                 │
│  → Check voltage limits                → Check sync maintained?                 │
│  → Check thermal limits                → Check safety limits                    │
│  → Verify no cascade                   → Verify graceful degradation           │
│                                                                                  │
│                                                                                  │
│  CONTINGENCY TABLE FOR 3-PARTICIPANT BCI:                                       │
│  ═════════════════════════════════════════                                      │
│                                                                                  │
│  ┌─────────────────────────┬────────────────────┬───────────────────────┐      │
│  │  CONTINGENCY            │  COUNTERFACTUAL Q  │  REQUIRED ACTION      │      │
│  ├─────────────────────────┼────────────────────┼───────────────────────┤      │
│  │  P1 tFUS fails          │  Sync without P1   │  Reduce to 2-player   │      │
│  │                         │  stimulation?      │  or pause experiment  │      │
│  ├─────────────────────────┼────────────────────┼───────────────────────┤      │
│  │  P2 disconnects         │  Can P1+P3         │  Graceful exit or     │      │
│  │                         │  continue?         │  wait for reconnect   │      │
│  ├─────────────────────────┼────────────────────┼───────────────────────┤      │
│  │  Feedback server lags   │  Open-loop sync    │  Local fallback       │      │
│  │                         │  possible?         │  feedback             │      │
│  ├─────────────────────────┼────────────────────┼───────────────────────┤      │
│  │  fNIRS signal lost      │  Safe to continue  │  STOP all stimulation │      │
│  │                         │  blind?            │  (N-1 violated)       │      │
│  ├─────────────────────────┼────────────────────┼───────────────────────┤      │
│  │  Thermal limit reached  │  Reduce intensity  │  Derate tFUS, notify  │      │
│  │                         │  effect?           │  participant          │      │
│  └─────────────────────────┴────────────────────┴───────────────────────┘      │
│                                                                                  │
│                                                                                  │
│  PRE-SESSION CHECKLIST:                                                         │
│  ══════════════════════                                                         │
│                                                                                  │
│  □ All N-1 contingencies analyzed                                              │
│  □ Counterfactual bounds acceptable for each contingency                       │
│  □ Automatic fallback protocols configured                                     │
│  □ Participants informed of contingency procedures                             │
│  □ Session can be safely terminated at any point                               │
│                                                                                  │
└─────────────────────────────────────────────────────────────────────────────────┘
```

## References & Inspirations

- **Topos Institute**: Polynomial functors, lenses, and compositional systems
- **CyberCat Institute**: Categorical cybernetics, bidirectional processes
- **Wolfram Institute**: Ruliology, multicomputational paradigm
- **Santa Fe Institute**: Complex adaptive systems, emergence
- **Simons Institute**: Theoretical foundations, information theory

The key insight from categorical thinking: **Higher-order interactions (synergies) are not just
statistical artifacts — they are the morphisms that connect individual world models into a shared,
emergent worlding process.**
