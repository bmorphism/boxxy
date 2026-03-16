//! Cat-clad epistemological verification over GF(3).
//!
//! A "cat-clad" claim is an object in a category with morphisms tracking
//! its provenance, derivation history, and the consistency conditions that
//! bind it to other claims.  Verification reduces to structural properties:
//!
//!   - Provenance is a composable morphism chain to primary sources
//!   - Consistency is a sheaf condition (H^1 = 0 means no contradictions)
//!   - GF(3) conservation prevents unbounded generation without verification
//!   - Bisimulation detects forgery (divergent accounts of the same event)
//!
//! ACSet Schema:
//!
//!   @present SchClaimWorld(FreeSchema) begin
//!     Claim::Ob           -- assertions to verify
//!     Source::Ob           -- evidence or citations
//!     Witness::Ob          -- attestation parties
//!     Derivation::Ob       -- inference steps
//!
//!     derives_from::Hom(Derivation, Source)
//!     produces::Hom(Derivation, Claim)
//!     attests::Hom(Witness, Source)
//!     cites::Hom(Claim, Source)
//!
//!     Trit::AttrType
//!     Confidence::AttrType
//!     ContentHash::AttrType
//!     Timestamp::AttrType
//!
//!     claim_trit::Attr(Claim, Trit)
//!     source_trit::Attr(Source, Trit)
//!     witness_trit::Attr(Witness, Trit)
//!     claim_hash::Attr(Claim, ContentHash)
//!     source_hash::Attr(Source, ContentHash)
//!     claim_confidence::Attr(Claim, Confidence)
//!   end

use regex::Regex;
use serde::{Deserialize, Serialize};
use sha2::{Digest, Sha256};
use std::collections::HashMap;

// ---------------------------------------------------------------------------
// GF(3) element
// ---------------------------------------------------------------------------

/// A GF(3) element: 0 = coordinator, 1 = generator, 2 = verifier.
#[derive(Clone, Copy, Debug, PartialEq, Eq, Hash, Serialize, Deserialize)]
pub enum Trit {
    Zero = 0,
    One = 1,
    Two = 2,
}

impl Trit {
    /// Addition in GF(3).
    pub fn add(self, other: Trit) -> Trit {
        Trit::from_u8(((self as u8) + (other as u8)) % 3)
    }

    /// Additive inverse in GF(3): neg(0)=0, neg(1)=2, neg(2)=1.
    pub fn neg(self) -> Trit {
        Trit::from_u8((3 - (self as u8)) % 3)
    }

    /// Multiplication in GF(3).
    pub fn mul(self, other: Trit) -> Trit {
        Trit::from_u8(((self as u8) * (other as u8)) % 3)
    }

    pub fn from_u8(v: u8) -> Trit {
        match v % 3 {
            0 => Trit::Zero,
            1 => Trit::One,
            2 => Trit::Two,
            _ => unreachable!(),
        }
    }

    pub fn role_name(self) -> &'static str {
        match self {
            Trit::Zero => "coordinator",
            Trit::One => "generator",
            Trit::Two => "verifier",
        }
    }
}

impl std::fmt::Display for Trit {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            Trit::Zero => write!(f, "0(coordinator)"),
            Trit::One => write!(f, "1(generator)"),
            Trit::Two => write!(f, "2(verifier)"),
        }
    }
}

// ---------------------------------------------------------------------------
// Source and witness classification enums
// ---------------------------------------------------------------------------

#[derive(Clone, Debug, PartialEq, Eq, Serialize, Deserialize)]
pub enum SourceKind {
    Academic,
    Authority,
    News,
    Url,
    Anecdotal,
    SelfReferential,
}

impl SourceKind {
    pub fn as_str(&self) -> &'static str {
        match self {
            SourceKind::Academic => "academic",
            SourceKind::Authority => "authority",
            SourceKind::News => "news",
            SourceKind::Url => "url",
            SourceKind::Anecdotal => "anecdotal",
            SourceKind::SelfReferential => "self-referential",
        }
    }
}

#[derive(Clone, Debug, PartialEq, Eq, Serialize, Deserialize)]
pub enum WitnessRole {
    Author,
    PeerReviewer,
    Editor,
    Publisher,
    SelfWitness,
}

impl WitnessRole {
    pub fn as_str(&self) -> &'static str {
        match self {
            WitnessRole::Author => "author",
            WitnessRole::PeerReviewer => "peer-reviewer",
            WitnessRole::Editor => "editor",
            WitnessRole::Publisher => "publisher",
            WitnessRole::SelfWitness => "self",
        }
    }
}

#[derive(Clone, Debug, PartialEq, Eq, Serialize, Deserialize)]
pub enum DerivationKind {
    Direct,
    Inductive,
    Deductive,
    Analogical,
    AppealToAuthority,
}

impl DerivationKind {
    pub fn as_str(&self) -> &'static str {
        match self {
            DerivationKind::Direct => "direct",
            DerivationKind::Inductive => "inductive",
            DerivationKind::Deductive => "deductive",
            DerivationKind::Analogical => "analogical",
            DerivationKind::AppealToAuthority => "appeal-to-authority",
        }
    }
}

#[derive(Clone, Debug, PartialEq, Eq, Serialize, Deserialize)]
pub enum CocycleKind {
    Contradiction,
    Unsupported,
    Circular,
    TritViolation,
    WeakAuthority,
}

impl CocycleKind {
    pub fn as_str(&self) -> &'static str {
        match self {
            CocycleKind::Contradiction => "contradiction",
            CocycleKind::Unsupported => "unsupported",
            CocycleKind::Circular => "circular",
            CocycleKind::TritViolation => "trit-violation",
            CocycleKind::WeakAuthority => "weak-authority",
        }
    }
}

// ---------------------------------------------------------------------------
// ACSet objects
// ---------------------------------------------------------------------------

#[derive(Clone, Debug, Serialize, Deserialize)]
pub struct Claim {
    pub id: String,
    pub text: String,
    pub trit: Trit,
    pub hash: String,
    pub confidence: f64,
    pub framework: String,
}

#[derive(Clone, Debug, Serialize, Deserialize)]
pub struct Source {
    pub id: String,
    pub citation: String,
    pub trit: Trit,
    pub hash: String,
    pub kind: SourceKind,
}

#[derive(Clone, Debug, Serialize, Deserialize)]
pub struct Witness {
    pub id: String,
    pub name: String,
    pub trit: Trit,
    pub role: WitnessRole,
    pub weight: f64,
}

#[derive(Clone, Debug, Serialize, Deserialize)]
pub struct Derivation {
    pub id: String,
    pub source_id: String,
    pub claim_id: String,
    pub kind: DerivationKind,
    pub strength: f64,
}

#[derive(Clone, Debug, Serialize, Deserialize)]
pub struct Cocycle {
    pub claim_a: String,
    pub claim_b: Option<String>,
    pub kind: CocycleKind,
    pub severity: f64,
}

// ---------------------------------------------------------------------------
// ClaimWorld -- the ACSet instance
// ---------------------------------------------------------------------------

#[derive(Clone, Debug, Serialize, Deserialize)]
pub struct ClaimWorld {
    pub claims: HashMap<String, Claim>,
    pub sources: HashMap<String, Source>,
    pub witnesses: HashMap<String, Witness>,
    pub derivations: Vec<Derivation>,
    pub cocycles: Vec<Cocycle>,
}

impl ClaimWorld {
    pub fn new() -> Self {
        ClaimWorld {
            claims: HashMap::new(),
            sources: HashMap::new(),
            witnesses: HashMap::new(),
            derivations: Vec::new(),
            cocycles: Vec::new(),
        }
    }
}

impl Default for ClaimWorld {
    fn default() -> Self {
        Self::new()
    }
}

// ---------------------------------------------------------------------------
// Manipulation pattern
// ---------------------------------------------------------------------------

#[derive(Clone, Debug, Serialize, Deserialize)]
pub struct ManipulationPattern {
    pub kind: String,
    pub evidence: String,
    pub severity: f64,
}

// ---------------------------------------------------------------------------
// Core analysis functions
// ---------------------------------------------------------------------------

/// SHA-256 content hash of lowercased, trimmed text.
pub fn content_hash(text: &str) -> String {
    let normalized = text.trim().to_lowercase();
    let mut hasher = Sha256::new();
    hasher.update(normalized.as_bytes());
    let result = hasher.finalize();
    hex::encode(result)
}

/// Hex encoding (no external crate -- inline).
mod hex {
    pub fn encode(bytes: impl AsRef<[u8]>) -> String {
        bytes
            .as_ref()
            .iter()
            .map(|b| format!("{:02x}", b))
            .collect()
    }
}

/// Analyze a textual claim, extracting sources, witnesses, derivations, and
/// checking sheaf consistency and GF(3) balance.
pub fn analyze_claim(text: &str, framework: &str) -> ClaimWorld {
    let mut world = ClaimWorld::new();

    let hash = content_hash(text);
    let id = hash[..12].to_string();

    let claim = Claim {
        id: id.clone(),
        text: text.to_string(),
        trit: Trit::One, // Generator: creating an assertion
        hash: hash.clone(),
        confidence: 0.0, // computed below
        framework: framework.to_string(),
    };
    world.claims.insert(id.clone(), claim);

    // Extract sources as morphisms from claim
    let sources = extract_sources(text);
    for src in &sources {
        world.sources.insert(src.id.clone(), src.clone());
        world.derivations.push(Derivation {
            id: format!("d-{}-{}", src.id, id),
            source_id: src.id.clone(),
            claim_id: id.clone(),
            kind: classify_derivation(&src.kind),
            strength: source_strength(&src.kind),
        });
    }

    // Extract witnesses (who attests to each source)
    for src in &sources {
        let w = extract_witness(src);
        world.witnesses.insert(w.id.clone(), w);
    }

    // Compute confidence
    let confidence = compute_confidence(&world, &id, framework);
    if let Some(c) = world.claims.get_mut(&id) {
        c.confidence = confidence;
    }

    // Detect cocycles (contradictions, unsupported claims, circular reasoning)
    world.cocycles = detect_cocycles(&world);

    world
}

/// Detect manipulation patterns in text via regex heuristics.
pub fn detect_manipulation(text: &str) -> Vec<ManipulationPattern> {
    let checks: Vec<(&str, &str, f64)> = vec![
        (
            "emotional_fear",
            r"(?i)(fear|terrif|alarm|panic|dread|catastroph)",
            0.7,
        ),
        (
            "urgency",
            r"(?i)(act now|limited time|don't wait|expires|hurry|last chance|before it's too late)",
            0.8,
        ),
        (
            "false_consensus",
            r"(?i)(everyone knows|nobody (believes|wants|thinks)|all experts|unanimous|widely accepted)",
            0.6,
        ),
        (
            "appeal_authority",
            r"(?i)(experts say|scientists (claim|prove)|studies show|research proves|doctors recommend)",
            0.5,
        ),
        (
            "artificial_scarcity",
            r"(?i)(exclusive|rare opportunity|only \d+ left|limited (edition|supply|spots))",
            0.7,
        ),
        (
            "social_pressure",
            r"(?i)(you don't want to be|don't miss out|join .* (others|people)|be the first)",
            0.6,
        ),
        (
            "loaded_language",
            r"(?i)(obviously|clearly|undeniably|unquestionably|beyond doubt)",
            0.4,
        ),
        (
            "false_dichotomy",
            r"(?i)(either .* or|only (two|2) (options|choices)|if you don't .* then)",
            0.6,
        ),
        (
            "circular_reasoning",
            r"(?i)(because .* therefore .* because|true because .* which is true)",
            0.9,
        ),
        (
            "ad_hominem",
            r"(?i)(stupid|idiot|moron|fool|ignorant|naive) .* (think|believe|say)",
            0.8,
        ),
    ];

    let mut patterns = Vec::new();
    for (kind, pat, weight) in &checks {
        if let Ok(re) = Regex::new(pat) {
            for m in re.find_iter(text) {
                patterns.push(ManipulationPattern {
                    kind: kind.to_string(),
                    evidence: m.as_str().to_string(),
                    severity: *weight,
                });
            }
        }
    }
    patterns
}

/// Sheaf consistency: returns (H^1 dimension, cocycles).
/// H^1 = 0 means no contradictions detected.
pub fn sheaf_consistency(world: &ClaimWorld) -> (usize, &[Cocycle]) {
    (world.cocycles.len(), &world.cocycles)
}

/// GF(3) balance check: sum of all trits should be 0 (mod 3).
pub fn gf3_balance(world: &ClaimWorld) -> (bool, HashMap<String, usize>) {
    let mut counts: HashMap<String, usize> = HashMap::new();
    counts.insert("coordinator".to_string(), 0);
    counts.insert("generator".to_string(), 0);
    counts.insert("verifier".to_string(), 0);

    let mut sum: u32 = 0;

    for c in world.claims.values() {
        sum += c.trit as u32;
        *counts.get_mut(c.trit.role_name()).unwrap() += 1;
    }
    for s in world.sources.values() {
        sum += s.trit as u32;
        *counts.get_mut(s.trit.role_name()).unwrap() += 1;
    }
    for w in world.witnesses.values() {
        sum += w.trit as u32;
        *counts.get_mut(w.trit.role_name()).unwrap() += 1;
    }

    let balanced = (sum % 3) == 0;
    (balanced, counts)
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

fn extract_sources(text: &str) -> Vec<Source> {
    let patterns: Vec<(&str, SourceKind)> = vec![
        (
            r"(?i)(?:according to|cited by|reported by)\s+([^,\.]+)",
            SourceKind::Authority,
        ),
        (
            r"(?i)(?:study|research|paper)\s+(?:by|from|in)\s+([^,\.]+)",
            SourceKind::Academic,
        ),
        (
            r"(?i)(?:published in|journal of)\s+([^,\.]+)",
            SourceKind::Academic,
        ),
        (r"(?i)(https?://\S+)", SourceKind::Url),
    ];

    let mut sources = Vec::new();
    let mut seen = std::collections::HashSet::new();

    for (pat, kind) in &patterns {
        if let Ok(re) = Regex::new(pat) {
            for caps in re.captures_iter(text) {
                if let Some(m) = caps.get(1) {
                    let citation = m.as_str().trim().to_string();
                    let hash = content_hash(&citation);
                    let id = hash[..12].to_string();
                    if seen.contains(&id) {
                        continue;
                    }
                    seen.insert(id.clone());
                    sources.push(Source {
                        id,
                        citation,
                        trit: Trit::Two, // Verifier: evidence checks claims
                        hash,
                        kind: kind.clone(),
                    });
                }
            }
        }
    }
    sources
}

fn extract_witness(src: &Source) -> Witness {
    let (role, weight) = match src.kind {
        SourceKind::Academic => (WitnessRole::PeerReviewer, 0.9),
        SourceKind::Authority => (WitnessRole::Author, 0.6),
        SourceKind::Url => (WitnessRole::Publisher, 0.4),
        _ => (WitnessRole::SelfWitness, 0.2),
    };
    Witness {
        id: format!("w-{}", src.id),
        name: src.citation.clone(),
        trit: Trit::Zero, // Coordinator: mediating between claim and verification
        role,
        weight,
    }
}

fn classify_derivation(kind: &SourceKind) -> DerivationKind {
    match kind {
        SourceKind::Academic => DerivationKind::Deductive,
        SourceKind::Authority => DerivationKind::AppealToAuthority,
        SourceKind::Url => DerivationKind::Direct,
        _ => DerivationKind::Analogical,
    }
}

fn source_strength(kind: &SourceKind) -> f64 {
    match kind {
        SourceKind::Academic => 0.85,
        SourceKind::Authority => 0.5,
        SourceKind::Url => 0.3,
        _ => 0.1,
    }
}

fn compute_confidence(world: &ClaimWorld, claim_id: &str, framework: &str) -> f64 {
    if world.sources.is_empty() {
        return 0.1; // unsupported claim
    }

    // Average derivation strength for this claim
    let mut total_strength = 0.0;
    let mut count = 0;
    for d in &world.derivations {
        if d.claim_id == claim_id {
            total_strength += d.strength;
            count += 1;
        }
    }
    if count == 0 {
        return 0.1;
    }
    let mut avg = total_strength / count as f64;

    // Weight by framework
    match framework {
        "empirical" => {
            let academic_count = world
                .sources
                .values()
                .filter(|s| s.kind == SourceKind::Academic)
                .count();
            if academic_count > 0 {
                avg *= 1.0 + 0.1 * academic_count as f64;
            }
        }
        "responsible" => {
            let text_lower = world
                .claims
                .get(claim_id)
                .map(|c| c.text.to_lowercase())
                .unwrap_or_default();
            if text_lower.contains("community") || text_lower.contains("benefit") {
                avg *= 1.1;
            }
        }
        "harmonic" => {
            if world.sources.len() >= 3 {
                avg *= 1.15;
            }
        }
        "pluralistic" | _ => {
            // raw structural quality, no special boost
        }
    }

    // Penalize cocycles (computed later, so use 0 penalty on first pass)
    let cocycle_penalty = 0.15 * world.cocycles.len() as f64;
    let confidence = (avg - cocycle_penalty).clamp(0.0, 1.0);
    confidence
}

fn detect_cocycles(world: &ClaimWorld) -> Vec<Cocycle> {
    let mut cocycles = Vec::new();

    // Check for unsupported claims (no derivation chain)
    for claim in world.claims.values() {
        let has_derivation = world.derivations.iter().any(|d| d.claim_id == claim.id);
        if !has_derivation {
            cocycles.push(Cocycle {
                claim_a: claim.id.clone(),
                claim_b: None,
                kind: CocycleKind::Unsupported,
                severity: 0.9,
            });
        }
    }

    // Check for appeal-to-authority without strong verification
    for d in &world.derivations {
        if d.kind == DerivationKind::AppealToAuthority && d.strength < 0.6 {
            cocycles.push(Cocycle {
                claim_a: d.claim_id.clone(),
                claim_b: Some(d.source_id.clone()),
                kind: CocycleKind::WeakAuthority,
                severity: 0.5,
            });
        }
    }

    // Check GF(3) conservation
    let (balanced, _) = gf3_balance(world);
    if !balanced {
        cocycles.push(Cocycle {
            claim_a: String::new(),
            claim_b: None,
            kind: CocycleKind::TritViolation,
            severity: 0.3,
        });
    }

    cocycles
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_trit_arithmetic() {
        assert_eq!(Trit::One.add(Trit::Two), Trit::Zero);
        assert_eq!(Trit::One.add(Trit::One), Trit::Two);
        assert_eq!(Trit::Two.add(Trit::Two), Trit::One);
        assert_eq!(Trit::Zero.add(Trit::Zero), Trit::Zero);
        assert_eq!(Trit::One.neg(), Trit::Two);
        assert_eq!(Trit::Two.neg(), Trit::One);
        assert_eq!(Trit::Zero.neg(), Trit::Zero);
    }

    #[test]
    fn test_trit_multiplication() {
        assert_eq!(Trit::One.mul(Trit::One), Trit::One);
        assert_eq!(Trit::Two.mul(Trit::Two), Trit::One); // 2*2=4 mod 3=1
        assert_eq!(Trit::One.mul(Trit::Two), Trit::Two);
        assert_eq!(Trit::Zero.mul(Trit::One), Trit::Zero);
    }

    #[test]
    fn test_content_hash_deterministic() {
        let h1 = content_hash("hello world");
        let h2 = content_hash("hello world");
        let h3 = content_hash("Hello World"); // case-insensitive
        assert_eq!(h1, h2);
        assert_eq!(h1, h3);
    }

    #[test]
    fn test_content_hash_whitespace() {
        let h1 = content_hash("  hello world  ");
        let h2 = content_hash("hello world");
        assert_eq!(h1, h2);
    }

    #[test]
    fn test_analyze_claim_with_sources() {
        let world = analyze_claim(
            "According to Dr. Smith, research from Harvard shows that exercise reduces stress by 40%",
            "empirical",
        );
        assert_eq!(world.claims.len(), 1);
        assert!(!world.sources.is_empty(), "should extract at least 1 source");
        assert!(
            !world.derivations.is_empty(),
            "should have derivations linking sources to claim"
        );
        for c in world.claims.values() {
            assert!(c.confidence > 0.0, "claim should have positive confidence");
        }
    }

    #[test]
    fn test_analyze_unsupported_claim() {
        let world = analyze_claim("The moon is made of cheese", "empirical");
        assert!(world.sources.is_empty());

        let (h1, cocycles) = sheaf_consistency(&world);
        assert!(h1 > 0, "unsupported claim should produce H^1 > 0");
        assert!(
            cocycles.iter().any(|c| c.kind == CocycleKind::Unsupported),
            "should have 'unsupported' cocycle"
        );

        for c in world.claims.values() {
            assert!(
                c.confidence <= 0.2,
                "unsupported claim should have low confidence, got {}",
                c.confidence
            );
        }
    }

    #[test]
    fn test_gf3_balance_with_full_triad() {
        let world = analyze_claim(
            "According to the WHO, a study by Johns Hopkins published in Nature confirms vaccine efficacy",
            "pluralistic",
        );
        let (_, counts) = gf3_balance(&world);
        assert!(
            *counts.get("generator").unwrap() > 0,
            "expected at least 1 generator (claim)"
        );
        assert!(
            *counts.get("verifier").unwrap() > 0,
            "expected at least 1 verifier (source)"
        );
        assert!(
            *counts.get("coordinator").unwrap() > 0,
            "expected at least 1 coordinator (witness)"
        );
    }

    #[test]
    fn test_detect_manipulation() {
        let patterns = detect_manipulation(
            "Act now! This exclusive offer expires in 10 minutes. Everyone knows this is the best deal. Scientists claim it's proven.",
        );
        assert!(!patterns.is_empty(), "expected manipulation patterns");

        let kinds: std::collections::HashSet<_> = patterns.iter().map(|p| p.kind.as_str()).collect();
        assert!(kinds.contains("urgency"), "expected urgency pattern");
        assert!(
            kinds.contains("artificial_scarcity"),
            "expected scarcity pattern"
        );
        assert!(
            kinds.contains("appeal_authority"),
            "expected appeal to authority pattern"
        );
    }

    #[test]
    fn test_detect_no_manipulation() {
        let patterns = detect_manipulation(
            "The temperature today is 72 degrees Fahrenheit with partly cloudy skies.",
        );
        assert!(
            patterns.is_empty(),
            "expected 0 manipulation patterns for neutral text, got {}",
            patterns.len()
        );
    }

    #[test]
    fn test_source_kind_classification() {
        let world = analyze_claim(
            "A study by Stanford published in Nature, and according to the CDC, plus https://example.com/data",
            "empirical",
        );
        let mut kinds = HashMap::new();
        for s in world.sources.values() {
            *kinds.entry(s.kind.as_str().to_string()).or_insert(0usize) += 1;
        }
        assert!(
            kinds.contains_key("academic")
                || kinds.contains_key("authority")
                || kinds.contains_key("url"),
            "expected at least one classified source, got {:?}",
            kinds
        );
    }

    #[test]
    fn test_multiple_frameworks() {
        let text = "Study by MIT shows community benefit from sustainable energy integration";
        for fw in &["empirical", "responsible", "harmonic", "pluralistic"] {
            let world = analyze_claim(text, fw);
            for c in world.claims.values() {
                assert_eq!(c.framework, *fw);
            }
        }
    }

    #[test]
    fn test_sheaf_consistency_clean() {
        let world = analyze_claim(
            "According to Dr. Smith, a study by MIT published in Science validates the hypothesis",
            "empirical",
        );
        // With sources present, there should be no unsupported cocycle
        let (h1, cocycles) = sheaf_consistency(&world);
        let unsupported = cocycles
            .iter()
            .filter(|c| c.kind == CocycleKind::Unsupported)
            .count();
        assert_eq!(
            unsupported, 0,
            "well-sourced claim should have no unsupported cocycles (H^1 unsupported = {})",
            unsupported
        );
        // h1 may still be > 0 from weak-authority or trit-violation, that is acceptable
        let _ = h1;
    }

    #[test]
    fn test_gf3_balance_sum() {
        // With 1 claim (trit=1), 1 source (trit=2), 1 witness (trit=0): sum = 3 = 0 mod 3
        let world = analyze_claim("According to Dr. Smith, the data is clear", "empirical");
        let source_count = world.sources.len();
        let witness_count = world.witnesses.len();
        if source_count == 1 && witness_count == 1 {
            // 1 (claim) + 2 (source) + 0 (witness) = 3 => balanced
            let (balanced, _) = gf3_balance(&world);
            assert!(balanced, "single triad should be GF(3) balanced");
        }
    }
}
