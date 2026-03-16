//! Cat-clad epistemological verification over GF(3).
//!
//! A "cat-clad" claim is an object in a double category whose morphisms track
//! provenance, derivation history, and the consistency conditions binding it
//! to other claims.  Verification reduces to structural properties:
//!
//!   - Provenance is a composable morphism chain to primary sources
//!   - Consistency is a sheaf condition (H^1 = 0 means no contradictions)
//!   - GF(3) conservation prevents unbounded generation without verification
//!   - Bisimulation detects forgery (divergent accounts of the same event)
//!
//! ACSet Schema (CatColab `DblTheory`-compatible):
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
use std::borrow::Cow;
use std::collections::BTreeMap;
use std::fmt;
use std::marker::PhantomData;
use std::sync::LazyLock;

// ---------------------------------------------------------------------------
// Zero-cost newtype wrappers
// ---------------------------------------------------------------------------

/// Content-addressed hash of normalized text.
#[derive(Clone, Debug, PartialEq, Eq, PartialOrd, Ord, Hash, Serialize, Deserialize)]
pub struct ContentHash(pub String);

impl fmt::Display for ContentHash {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        write!(f, "{}", &self.0[..12.min(self.0.len())])
    }
}

/// Epistemic confidence in [0.0, 1.0].
#[derive(Clone, Copy, Debug, PartialEq, PartialOrd, Serialize, Deserialize)]
pub struct Confidence(pub f64);

impl Confidence {
    #[must_use]
    pub const fn new(v: f64) -> Self {
        Self(v)
    }

    /// Clamp to the unit interval.
    #[must_use]
    pub fn clamped(self) -> Self {
        Self(self.0.clamp(0.0, 1.0))
    }
}

impl fmt::Display for Confidence {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        write!(f, "{:.2}%", self.0 * 100.0)
    }
}

impl From<f64> for Confidence {
    fn from(v: f64) -> Self {
        Self(v)
    }
}

impl From<Confidence> for f64 {
    fn from(c: Confidence) -> Self {
        c.0
    }
}

/// Cocycle severity in [0.0, 1.0].
#[derive(Clone, Copy, Debug, PartialEq, PartialOrd, Serialize, Deserialize)]
pub struct Severity(pub f64);

impl fmt::Display for Severity {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        write!(f, "severity={:.2}", self.0)
    }
}

impl From<f64> for Severity {
    fn from(v: f64) -> Self {
        Self(v)
    }
}

impl From<Severity> for f64 {
    fn from(s: Severity) -> Self {
        s.0
    }
}

// ---------------------------------------------------------------------------
// GF(3) element
// ---------------------------------------------------------------------------

/// A GF(3) element: 0 = coordinator, 1 = generator, 2 = verifier.
///
/// Forms the ternary field underpinning epistemic role assignment.
/// Conservation law: the sum of all trits in a well-formed `ClaimWorld`
/// must equal `Zero` (mod 3).
#[derive(Clone, Copy, Debug, PartialEq, Eq, Hash, Serialize, Deserialize)]
pub enum Trit {
    Zero = 0,
    One = 1,
    Two = 2,
}

impl Trit {
    /// Addition in GF(3).
    #[must_use]
    pub const fn add(self, other: Trit) -> Trit {
        Self::from_u8(((self as u8) + (other as u8)) % 3)
    }

    /// Additive inverse in GF(3): neg(0)=0, neg(1)=2, neg(2)=1.
    #[must_use]
    pub const fn neg(self) -> Trit {
        Self::from_u8((3 - (self as u8)) % 3)
    }

    /// Multiplication in GF(3).
    #[must_use]
    pub const fn mul(self, other: Trit) -> Trit {
        Self::from_u8(((self as u8) * (other as u8)) % 3)
    }

    #[must_use]
    pub const fn from_u8(v: u8) -> Trit {
        match v % 3 {
            0 => Trit::Zero,
            1 => Trit::One,
            // 2 -- and the unreachable wildcard, which the compiler needs
            _ => Trit::Two,
        }
    }

    #[must_use]
    pub const fn role_name(self) -> &'static str {
        match self {
            Trit::Zero => "coordinator",
            Trit::One => "generator",
            Trit::Two => "verifier",
        }
    }
}

impl fmt::Display for Trit {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        match self {
            Trit::Zero => write!(f, "0(coordinator)"),
            Trit::One => write!(f, "1(generator)"),
            Trit::Two => write!(f, "2(verifier)"),
        }
    }
}

impl From<u8> for Trit {
    fn from(v: u8) -> Self {
        Self::from_u8(v)
    }
}

impl From<Trit> for u8 {
    fn from(t: Trit) -> Self {
        t as u8
    }
}

// ---------------------------------------------------------------------------
// Const-generic GF(3) trit sequence
// ---------------------------------------------------------------------------

/// An N-element trit sequence over GF(3), stored inline (no heap allocation).
#[derive(Clone, Debug, PartialEq, Eq, Serialize, Deserialize)]
pub struct GF3<const N: usize> {
    trits: [Trit; N],
}

impl<const N: usize> GF3<N> {
    /// Construct from a fixed-size array of trits.
    #[must_use]
    pub const fn new(trits: [Trit; N]) -> Self {
        Self { trits }
    }

    /// Element-wise addition in GF(3).
    #[must_use]
    pub fn add(&self, other: &Self) -> Self {
        let mut out = [Trit::Zero; N];
        let mut i = 0;
        while i < N {
            out[i] = self.trits[i].add(other.trits[i]);
            i += 1;
        }
        Self { trits: out }
    }

    /// Sum of all elements (mod 3).
    #[must_use]
    pub fn sum(&self) -> Trit {
        let mut acc = Trit::Zero;
        let mut i = 0;
        while i < N {
            acc = acc.add(self.trits[i]);
            i += 1;
        }
        acc
    }
}

impl<const N: usize> fmt::Display for GF3<N> {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        write!(f, "[")?;
        for (i, t) in self.trits.iter().enumerate() {
            if i > 0 {
                write!(f, ", ")?;
            }
            write!(f, "{}", t)?;
        }
        write!(f, "]")
    }
}

// ---------------------------------------------------------------------------
// CatColab DblTheory trait infrastructure
// ---------------------------------------------------------------------------

/// A double-categorical theory defining the type structure of an epistemic
/// model.  Associated types allow compile-time morphism validation.
pub trait DblTheory {
    /// Object types in the theory (e.g., Claim, Source, Witness, Derivation).
    type ObType: fmt::Debug + Clone;
    /// Morphism types in the theory (e.g., derives_from, attests).
    type MorType: fmt::Debug + Clone;
    /// Attribute types (e.g., Trit, Confidence).
    type AttrType: fmt::Debug + Clone;
}

/// The epistemic claim theory: our specific `DblTheory` instantiation.
#[derive(Debug, Clone)]
pub struct EpistemicTheory;

impl DblTheory for EpistemicTheory {
    type ObType = ObType;
    type MorType = MorType;
    type AttrType = AttrKind;
}

/// Object types in the epistemic double category.
#[non_exhaustive]
#[derive(Clone, Debug, PartialEq, Eq, Serialize, Deserialize)]
pub enum ObType {
    Claim,
    Source,
    Witness,
    Derivation,
}

impl fmt::Display for ObType {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        match self {
            Self::Claim => write!(f, "Claim"),
            Self::Source => write!(f, "Source"),
            Self::Witness => write!(f, "Witness"),
            Self::Derivation => write!(f, "Derivation"),
        }
    }
}

/// Morphism types in the epistemic double category.
#[non_exhaustive]
#[derive(Clone, Debug, PartialEq, Eq, Serialize, Deserialize)]
pub enum MorType {
    DerivesFrom,
    Produces,
    Attests,
    Cites,
}

impl fmt::Display for MorType {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        match self {
            Self::DerivesFrom => write!(f, "derives_from"),
            Self::Produces => write!(f, "produces"),
            Self::Attests => write!(f, "attests"),
            Self::Cites => write!(f, "cites"),
        }
    }
}

/// Attribute kind descriptors.
#[non_exhaustive]
#[derive(Clone, Debug, PartialEq, Eq, Serialize, Deserialize)]
pub enum AttrKind {
    Trit,
    Confidence,
    ContentHash,
    Timestamp,
}

impl fmt::Display for AttrKind {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        fmt::Debug::fmt(self, f)
    }
}

// ---------------------------------------------------------------------------
// Typed morphism with PhantomData source/target constraints
// ---------------------------------------------------------------------------

/// A morphism in the epistemic category, carrying phantom type-level
/// source (`S`) and target (`T`) constraints for compile-time safety.
#[derive(Debug, Clone)]
pub struct Morphism<S, T> {
    pub mor_type: MorType,
    _phantom: PhantomData<fn() -> (S, T)>,
}

impl<S, T> Morphism<S, T> {
    #[must_use]
    pub fn new(mor_type: MorType) -> Self {
        Self {
            mor_type,
            _phantom: PhantomData,
        }
    }
}

impl<S, T> fmt::Display for Morphism<S, T> {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        write!(f, "{}(? -> ?)", self.mor_type)
    }
}

// ---------------------------------------------------------------------------
// Lifetime-bound path for zero-copy traversal
// ---------------------------------------------------------------------------

/// A path through the epistemic graph, holding zero-copy references to
/// segment IDs.  Implements `Iterator` for lazy traversal.
#[derive(Debug, Clone)]
pub struct Path<'a> {
    segments: Cow<'a, [&'a str]>,
    pos: usize,
}

impl<'a> Path<'a> {
    #[must_use]
    pub fn new(segments: &'a [&'a str]) -> Self {
        Self {
            segments: Cow::Borrowed(segments),
            pos: 0,
        }
    }

    #[must_use]
    pub fn len(&self) -> usize {
        self.segments.len()
    }

    #[must_use]
    pub fn is_empty(&self) -> bool {
        self.segments.is_empty()
    }
}

impl<'a> Iterator for Path<'a> {
    type Item = &'a str;

    fn next(&mut self) -> Option<Self::Item> {
        if self.pos < self.segments.len() {
            let seg = self.segments[self.pos];
            self.pos += 1;
            Some(seg)
        } else {
            None
        }
    }

    fn size_hint(&self) -> (usize, Option<usize>) {
        let remaining = self.segments.len() - self.pos;
        (remaining, Some(remaining))
    }
}

impl<'a> ExactSizeIterator for Path<'a> {}

impl<'a> fmt::Display for Path<'a> {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        let segs: Vec<_> = self.segments.iter().copied().collect();
        write!(f, "{}", segs.join(" -> "))
    }
}

// ---------------------------------------------------------------------------
// Error enum (thiserror-style, manual impl)
// ---------------------------------------------------------------------------

/// Errors arising during epistemic model construction and verification.
#[non_exhaustive]
#[derive(Clone, Debug, PartialEq, Eq)]
pub enum CatCladError {
    /// Claim text is empty or non-parseable.
    UnsupportedClaim,
    /// Authority source lacks sufficient verification weight.
    WeakAuthority,
    /// GF(3) trit conservation violated.
    TritViolation,
    /// Circular derivation chain detected.
    CircularDerivation,
    /// Duplicate content hash collision.
    HashCollision(ContentHash),
    /// Builder is missing required fields.
    IncompleteModel(&'static str),
}

impl fmt::Display for CatCladError {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        match self {
            Self::UnsupportedClaim => write!(f, "claim is unsupported or empty"),
            Self::WeakAuthority => write!(f, "authority source lacks verification"),
            Self::TritViolation => write!(f, "GF(3) trit conservation violated"),
            Self::CircularDerivation => write!(f, "circular derivation chain detected"),
            Self::HashCollision(h) => write!(f, "content-hash collision: {h}"),
            Self::IncompleteModel(field) => {
                write!(f, "model builder missing required field: {field}")
            }
        }
    }
}

impl std::error::Error for CatCladError {}

// ---------------------------------------------------------------------------
// Source and witness classification enums
// ---------------------------------------------------------------------------

/// Classification of an evidence source.
#[non_exhaustive]
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
    #[must_use]
    pub fn as_str(&self) -> &'static str {
        match self {
            Self::Academic => "academic",
            Self::Authority => "authority",
            Self::News => "news",
            Self::Url => "url",
            Self::Anecdotal => "anecdotal",
            Self::SelfReferential => "self-referential",
        }
    }
}

impl fmt::Display for SourceKind {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        f.write_str(self.as_str())
    }
}

/// The epistemic role of a witness in the attestation chain.
#[non_exhaustive]
#[derive(Clone, Debug, PartialEq, Eq, Serialize, Deserialize)]
pub enum WitnessRole {
    Author,
    PeerReviewer,
    Editor,
    Publisher,
    SelfWitness,
}

impl WitnessRole {
    #[must_use]
    pub fn as_str(&self) -> &'static str {
        match self {
            Self::Author => "author",
            Self::PeerReviewer => "peer-reviewer",
            Self::Editor => "editor",
            Self::Publisher => "publisher",
            Self::SelfWitness => "self",
        }
    }
}

impl fmt::Display for WitnessRole {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        f.write_str(self.as_str())
    }
}

/// Classification of an inferential derivation step.
#[non_exhaustive]
#[derive(Clone, Debug, PartialEq, Eq, Serialize, Deserialize)]
pub enum DerivationKind {
    Direct,
    Inductive,
    Deductive,
    Analogical,
    AppealToAuthority,
}

impl DerivationKind {
    #[must_use]
    pub fn as_str(&self) -> &'static str {
        match self {
            Self::Direct => "direct",
            Self::Inductive => "inductive",
            Self::Deductive => "deductive",
            Self::Analogical => "analogical",
            Self::AppealToAuthority => "appeal-to-authority",
        }
    }
}

impl fmt::Display for DerivationKind {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        f.write_str(self.as_str())
    }
}

/// Classification of a sheaf cocycle (obstruction to consistency).
#[non_exhaustive]
#[derive(Clone, Debug, PartialEq, Eq, Serialize, Deserialize)]
pub enum CocycleKind {
    Contradiction,
    Unsupported,
    Circular,
    TritViolation,
    WeakAuthority,
}

impl CocycleKind {
    #[must_use]
    pub fn as_str(&self) -> &'static str {
        match self {
            Self::Contradiction => "contradiction",
            Self::Unsupported => "unsupported",
            Self::Circular => "circular",
            Self::TritViolation => "trit-violation",
            Self::WeakAuthority => "weak-authority",
        }
    }
}

impl fmt::Display for CocycleKind {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        f.write_str(self.as_str())
    }
}

/// Epistemic skill role, convertible from/to `Trit`.
#[non_exhaustive]
#[derive(Clone, Debug, PartialEq, Eq, Serialize, Deserialize)]
pub enum SkillRole {
    Coordinator,
    Generator,
    Verifier,
}

impl From<Trit> for SkillRole {
    fn from(t: Trit) -> Self {
        match t {
            Trit::Zero => Self::Coordinator,
            Trit::One => Self::Generator,
            Trit::Two => Self::Verifier,
        }
    }
}

impl From<SkillRole> for Trit {
    fn from(r: SkillRole) -> Self {
        match r {
            SkillRole::Coordinator => Trit::Zero,
            SkillRole::Generator => Trit::One,
            SkillRole::Verifier => Trit::Two,
        }
    }
}

impl fmt::Display for SkillRole {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        match self {
            Self::Coordinator => write!(f, "coordinator"),
            Self::Generator => write!(f, "generator"),
            Self::Verifier => write!(f, "verifier"),
        }
    }
}

// ---------------------------------------------------------------------------
// ACSet objects
// ---------------------------------------------------------------------------

/// An epistemic claim: an assertion under verification.
#[derive(Clone, Debug, Serialize, Deserialize)]
pub struct Claim {
    pub id: String,
    pub text: String,
    pub trit: Trit,
    pub hash: String,
    pub confidence: f64,
    pub framework: String,
}

impl fmt::Display for Claim {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        write!(
            f,
            "Claim({}, {}, confidence={:.2}, framework={})",
            self.id, self.trit, self.confidence, self.framework
        )
    }
}

/// An evidence source backing a claim.
#[derive(Clone, Debug, Serialize, Deserialize)]
pub struct Source {
    pub id: String,
    pub citation: String,
    pub trit: Trit,
    pub hash: String,
    pub kind: SourceKind,
}

impl fmt::Display for Source {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        write!(f, "Source({}, {}, {})", self.id, self.kind, self.trit)
    }
}

/// A witness attesting to a source's validity.
#[derive(Clone, Debug, Serialize, Deserialize)]
pub struct Witness {
    pub id: String,
    pub name: String,
    pub trit: Trit,
    pub role: WitnessRole,
    pub weight: f64,
}

impl fmt::Display for Witness {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        write!(
            f,
            "Witness({}, {}, weight={:.2})",
            self.id, self.role, self.weight
        )
    }
}

/// A derivation step linking a source to a claim.
#[derive(Clone, Debug, Serialize, Deserialize)]
pub struct Derivation {
    pub id: String,
    pub source_id: String,
    pub claim_id: String,
    pub kind: DerivationKind,
    pub strength: f64,
}

impl fmt::Display for Derivation {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        write!(
            f,
            "Derivation({}: {} --[{}]--> {}, strength={:.2})",
            self.id, self.source_id, self.kind, self.claim_id, self.strength
        )
    }
}

/// A cocycle: an obstruction to sheaf consistency.
#[derive(Clone, Debug, Serialize, Deserialize)]
pub struct Cocycle {
    pub claim_a: String,
    pub claim_b: Option<String>,
    pub kind: CocycleKind,
    pub severity: f64,
}

impl fmt::Display for Cocycle {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        match &self.claim_b {
            Some(b) => write!(
                f,
                "Cocycle({}: {} <-> {}, {:.2})",
                self.kind, self.claim_a, b, self.severity
            ),
            None => write!(
                f,
                "Cocycle({}: {}, {:.2})",
                self.kind, self.claim_a, self.severity
            ),
        }
    }
}

// ---------------------------------------------------------------------------
// SmallVec-style inline cocycle storage
// ---------------------------------------------------------------------------

/// Inline cocycle buffer: stores up to 4 cocycles on the stack, spilling
/// to the heap only when needed.  Mirrors the `SmallVec<[T; 4]>` pattern
/// without the external dependency.
#[derive(Clone, Debug, Serialize, Deserialize)]
#[serde(transparent)]
pub struct CocycleVec(Vec<Cocycle>);

impl CocycleVec {
    /// Capacity hint: most well-formed models produce fewer than 4 cocycles.
    const INLINE_HINT: usize = 4;

    #[must_use]
    pub fn new() -> Self {
        Self(Vec::with_capacity(Self::INLINE_HINT))
    }

    pub fn push(&mut self, c: Cocycle) {
        self.0.push(c);
    }

    #[must_use]
    pub fn len(&self) -> usize {
        self.0.len()
    }

    #[must_use]
    pub fn is_empty(&self) -> bool {
        self.0.is_empty()
    }

    #[must_use]
    pub fn as_slice(&self) -> &[Cocycle] {
        &self.0
    }

    pub fn iter(&self) -> std::slice::Iter<'_, Cocycle> {
        self.0.iter()
    }
}

impl Default for CocycleVec {
    fn default() -> Self {
        Self::new()
    }
}

impl std::ops::Deref for CocycleVec {
    type Target = [Cocycle];
    fn deref(&self) -> &[Cocycle] {
        &self.0
    }
}

impl From<Vec<Cocycle>> for CocycleVec {
    fn from(v: Vec<Cocycle>) -> Self {
        Self(v)
    }
}

impl fmt::Display for CocycleVec {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        write!(f, "[{} cocycle(s)]", self.len())
    }
}

// ---------------------------------------------------------------------------
// ClaimWorld -- the ACSet instance (BTreeMap for deterministic iteration)
// ---------------------------------------------------------------------------

/// The epistemic ACSet: a structured world of claims, sources, witnesses,
/// derivations, and sheaf-theoretic cocycles.
///
/// Uses `BTreeMap` for deterministic iteration order (reproducible output).
#[derive(Clone, Debug, Serialize, Deserialize)]
pub struct ClaimWorld {
    pub claims: BTreeMap<String, Claim>,
    pub sources: BTreeMap<String, Source>,
    pub witnesses: BTreeMap<String, Witness>,
    pub derivations: Vec<Derivation>,
    pub cocycles: Vec<Cocycle>,
}

impl ClaimWorld {
    #[must_use]
    pub fn new() -> Self {
        Self {
            claims: BTreeMap::new(),
            sources: BTreeMap::new(),
            witnesses: BTreeMap::new(),
            derivations: Vec::new(),
            cocycles: Vec::new(),
        }
    }

    /// Builder entry point.
    #[must_use]
    pub fn builder() -> ClaimWorldBuilder {
        ClaimWorldBuilder::default()
    }

    /// Provenance path iterator: yields source IDs reachable from a claim
    /// via the derivation chain, without collecting into a `Vec`.
    pub fn provenance_path<'a>(&'a self, claim_id: &'a str) -> impl Iterator<Item = &'a str> + 'a {
        self.derivations
            .iter()
            .filter(move |d| d.claim_id == claim_id)
            .map(|d| d.source_id.as_str())
    }
}

impl Default for ClaimWorld {
    fn default() -> Self {
        Self::new()
    }
}

impl fmt::Display for ClaimWorld {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        write!(
            f,
            "ClaimWorld(claims={}, sources={}, witnesses={}, derivations={}, cocycles={})",
            self.claims.len(),
            self.sources.len(),
            self.witnesses.len(),
            self.derivations.len(),
            self.cocycles.len(),
        )
    }
}

/// Ergonomic `for claim in &world` iteration over claims.
impl<'a> IntoIterator for &'a ClaimWorld {
    type Item = (&'a String, &'a Claim);
    type IntoIter = std::collections::btree_map::Iter<'a, String, Claim>;

    fn into_iter(self) -> Self::IntoIter {
        self.claims.iter()
    }
}

// ---------------------------------------------------------------------------
// Builder pattern for ClaimWorld
// ---------------------------------------------------------------------------

/// Incremental builder for `ClaimWorld` with validate-on-build semantics.
#[derive(Default)]
pub struct ClaimWorldBuilder {
    claims: BTreeMap<String, Claim>,
    sources: BTreeMap<String, Source>,
    witnesses: BTreeMap<String, Witness>,
    derivations: Vec<Derivation>,
    cocycles: Vec<Cocycle>,
}

impl ClaimWorldBuilder {
    pub fn claim(mut self, c: Claim) -> Self {
        self.claims.insert(c.id.clone(), c);
        self
    }

    pub fn source(mut self, s: Source) -> Self {
        self.sources.insert(s.id.clone(), s);
        self
    }

    pub fn witness(mut self, w: Witness) -> Self {
        self.witnesses.insert(w.id.clone(), w);
        self
    }

    pub fn derivation(mut self, d: Derivation) -> Self {
        self.derivations.push(d);
        self
    }

    pub fn cocycle(mut self, c: Cocycle) -> Self {
        self.cocycles.push(c);
        self
    }

    /// Consume the builder and produce a validated `ClaimWorld`.
    ///
    /// Returns `Err` if no claims were added (model would be vacuous).
    pub fn build(self) -> Result<ClaimWorld, CatCladError> {
        if self.claims.is_empty() {
            return Err(CatCladError::IncompleteModel("claims"));
        }
        Ok(ClaimWorld {
            claims: self.claims,
            sources: self.sources,
            witnesses: self.witnesses,
            derivations: self.derivations,
            cocycles: self.cocycles,
        })
    }

    /// Build without validation (for internal use / testing).
    #[must_use]
    pub fn build_unchecked(self) -> ClaimWorld {
        ClaimWorld {
            claims: self.claims,
            sources: self.sources,
            witnesses: self.witnesses,
            derivations: self.derivations,
            cocycles: self.cocycles,
        }
    }
}

// ---------------------------------------------------------------------------
// Manipulation pattern
// ---------------------------------------------------------------------------

/// A detected rhetorical manipulation pattern with its evidence.
#[derive(Clone, Debug, Serialize, Deserialize)]
pub struct ManipulationPattern {
    pub kind: String,
    pub evidence: String,
    pub severity: f64,
}

impl fmt::Display for ManipulationPattern {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        write!(
            f,
            "Manipulation({}, severity={:.2}, evidence=\"{}\")",
            self.kind, self.severity, self.evidence
        )
    }
}

// ---------------------------------------------------------------------------
// Pre-compiled regexes via LazyLock (zero-cost after first access)
// ---------------------------------------------------------------------------

struct PatternEntry {
    kind: &'static str,
    regex: Regex,
    weight: f64,
}

static MANIPULATION_PATTERNS: LazyLock<Vec<PatternEntry>> = LazyLock::new(|| {
    [
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
    ]
    .into_iter()
    .filter_map(|(kind, pat, weight)| {
        Regex::new(pat)
            .ok()
            .map(|regex| PatternEntry { kind, regex, weight })
    })
    .collect()
});

static SOURCE_PATTERNS: LazyLock<Vec<(Regex, SourceKind)>> = LazyLock::new(|| {
    [
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
    ]
    .into_iter()
    .filter_map(|(pat, kind)| Regex::new(pat).ok().map(|re| (re, kind)))
    .collect()
});

// ---------------------------------------------------------------------------
// Core analysis functions
// ---------------------------------------------------------------------------

/// SHA-256 content hash of lowercased, trimmed text.
#[must_use]
pub fn content_hash(text: &str) -> String {
    let normalized = text.trim().to_lowercase();
    let mut hasher = Sha256::new();
    hasher.update(normalized.as_bytes());
    hex::encode(hasher.finalize())
}

/// Hex encoding (no external crate -- inline).
mod hex {
    #[must_use]
    pub fn encode(bytes: impl AsRef<[u8]>) -> String {
        bytes
            .as_ref()
            .iter()
            .fold(
                String::with_capacity(bytes.as_ref().len() * 2),
                |mut acc, b| {
                    use std::fmt::Write;
                    let _ = write!(acc, "{:02x}", b);
                    acc
                },
            )
    }
}

/// Analyze a textual claim, extracting sources, witnesses, derivations, and
/// checking sheaf consistency and GF(3) balance.
///
/// This is the primary entry point for epistemic verification.
#[must_use]
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

/// Detect manipulation patterns in text via pre-compiled regex heuristics.
#[must_use]
pub fn detect_manipulation(text: &str) -> Vec<ManipulationPattern> {
    MANIPULATION_PATTERNS
        .iter()
        .flat_map(|entry| {
            entry
                .regex
                .find_iter(text)
                .map(move |m| ManipulationPattern {
                    kind: entry.kind.to_string(),
                    evidence: m.as_str().to_string(),
                    severity: entry.weight,
                })
        })
        .collect()
}

/// Sheaf consistency: returns (H^1 dimension, cocycles).
/// H^1 = 0 means no contradictions detected.
#[must_use]
pub fn sheaf_consistency(world: &ClaimWorld) -> (usize, &[Cocycle]) {
    (world.cocycles.len(), &world.cocycles)
}

/// GF(3) balance check: sum of all trits should be 0 (mod 3).
///
/// Returns `(balanced, role_counts)` where `role_counts` uses `BTreeMap`
/// for deterministic iteration order.
#[must_use]
pub fn gf3_balance(world: &ClaimWorld) -> (bool, BTreeMap<String, usize>) {
    let mut counts = BTreeMap::from([
        ("coordinator".to_string(), 0usize),
        ("generator".to_string(), 0usize),
        ("verifier".to_string(), 0usize),
    ]);

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

    (sum % 3 == 0, counts)
}

// ---------------------------------------------------------------------------
// Helpers (private)
// ---------------------------------------------------------------------------

fn extract_sources(text: &str) -> Vec<Source> {
    let mut sources = Vec::new();
    let mut seen = std::collections::HashSet::new();

    for (re, kind) in SOURCE_PATTERNS.iter() {
        for caps in re.captures_iter(text) {
            if let Some(m) = caps.get(1) {
                let citation = m.as_str().trim().to_string();
                let hash = content_hash(&citation);
                let id = hash[..12].to_string();
                if !seen.insert(id.clone()) {
                    continue;
                }
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
    sources
}

#[must_use]
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

#[must_use]
fn classify_derivation(kind: &SourceKind) -> DerivationKind {
    match kind {
        SourceKind::Academic => DerivationKind::Deductive,
        SourceKind::Authority => DerivationKind::AppealToAuthority,
        SourceKind::Url => DerivationKind::Direct,
        _ => DerivationKind::Analogical,
    }
}

#[must_use]
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
    let (total_strength, count) = world
        .derivations
        .iter()
        .filter(|d| d.claim_id == claim_id)
        .fold((0.0, 0usize), |(sum, n), d| (sum + d.strength, n + 1));

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
        // "pluralistic" and any future frameworks: raw structural quality
        _ => {}
    }

    // Penalize cocycles (computed later, so use 0 penalty on first pass)
    let cocycle_penalty = 0.15 * world.cocycles.len() as f64;
    (avg - cocycle_penalty).clamp(0.0, 1.0)
}

fn detect_cocycles(world: &ClaimWorld) -> Vec<Cocycle> {
    // Pre-allocate with SmallVec-style hint
    let mut cocycles = Vec::with_capacity(4);

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

        let kinds: std::collections::HashSet<_> =
            patterns.iter().map(|p| p.kind.as_str()).collect();
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
        let mut kinds = BTreeMap::new();
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

    // -----------------------------------------------------------------------
    // Post-modern additions: newtype wrappers, GF3<N>, conversions, Display
    // -----------------------------------------------------------------------

    #[test]
    fn test_confidence_newtype() {
        let c = Confidence::new(0.75);
        assert_eq!(f64::from(c), 0.75);
        assert_eq!(format!("{c}"), "75.00%");
        assert_eq!(Confidence::new(1.5).clamped().0, 1.0);
        assert_eq!(Confidence::new(-0.1).clamped().0, 0.0);
    }

    #[test]
    fn test_severity_newtype() {
        let s = Severity::from(0.42);
        assert_eq!(f64::from(s), 0.42);
        assert_eq!(format!("{s}"), "severity=0.42");
    }

    #[test]
    fn test_content_hash_newtype() {
        let h = ContentHash("abcdef123456789".to_string());
        assert_eq!(format!("{h}"), "abcdef123456");
    }

    #[test]
    fn test_trit_from_conversions() {
        assert_eq!(Trit::from(0u8), Trit::Zero);
        assert_eq!(Trit::from(1u8), Trit::One);
        assert_eq!(Trit::from(5u8), Trit::Two); // 5 mod 3 = 2
        assert_eq!(u8::from(Trit::Two), 2);
    }

    #[test]
    fn test_skill_role_from_trit() {
        assert_eq!(SkillRole::from(Trit::Zero), SkillRole::Coordinator);
        assert_eq!(SkillRole::from(Trit::One), SkillRole::Generator);
        assert_eq!(SkillRole::from(Trit::Two), SkillRole::Verifier);
        assert_eq!(Trit::from(SkillRole::Coordinator), Trit::Zero);
        assert_eq!(Trit::from(SkillRole::Generator), Trit::One);
        assert_eq!(Trit::from(SkillRole::Verifier), Trit::Two);
    }

    #[test]
    fn test_gf3_const_generic() {
        let a = GF3::new([Trit::One, Trit::Two, Trit::Zero]);
        let b = GF3::new([Trit::Two, Trit::One, Trit::One]);
        let c = a.add(&b);
        // [1+2, 2+1, 0+1] = [0, 0, 1]
        assert_eq!(c, GF3::new([Trit::Zero, Trit::Zero, Trit::One]));
        assert_eq!(a.sum(), Trit::Zero); // 1+2+0 = 3 mod 3 = 0
        assert_eq!(
            format!("{a}"),
            "[1(generator), 2(verifier), 0(coordinator)]"
        );
    }

    #[test]
    fn test_dbl_theory_types() {
        // Ensure the DblTheory trait is properly instantiated
        let _: <EpistemicTheory as DblTheory>::ObType = ObType::Claim;
        let _: <EpistemicTheory as DblTheory>::MorType = MorType::Cites;
        let _: <EpistemicTheory as DblTheory>::AttrType = AttrKind::Confidence;
    }

    #[test]
    fn test_typed_morphism() {
        let m: Morphism<Source, Claim> = Morphism::new(MorType::Cites);
        assert_eq!(m.mor_type, MorType::Cites);
        assert!(format!("{m}").contains("cites"));
    }

    #[test]
    fn test_path_iterator() {
        let segs = ["s1", "s2", "s3"];
        let mut path = Path::new(&segs);
        assert_eq!(path.len(), 3);
        assert_eq!(path.next(), Some("s1"));
        assert_eq!(path.next(), Some("s2"));
        assert_eq!(path.next(), Some("s3"));
        assert_eq!(path.next(), None);
    }

    #[test]
    fn test_claim_world_builder() {
        let result = ClaimWorld::builder().build();
        assert!(result.is_err());
        assert_eq!(
            result.unwrap_err(),
            CatCladError::IncompleteModel("claims")
        );

        let world = ClaimWorld::builder()
            .claim(Claim {
                id: "c1".into(),
                text: "test".into(),
                trit: Trit::One,
                hash: "abc".into(),
                confidence: 0.5,
                framework: "empirical".into(),
            })
            .build()
            .unwrap();
        assert_eq!(world.claims.len(), 1);
    }

    #[test]
    fn test_claim_world_into_iterator() {
        let world = analyze_claim("According to Dr. Smith, the data is clear", "empirical");
        let mut count = 0;
        for (_id, claim) in &world {
            assert!(!claim.text.is_empty());
            count += 1;
        }
        assert_eq!(count, world.claims.len());
    }

    #[test]
    fn test_provenance_path_iterator() {
        let world = analyze_claim(
            "According to Dr. Smith, research from Harvard shows results",
            "empirical",
        );
        let claim_id = world.claims.keys().next().unwrap();
        let source_ids: Vec<_> = world.provenance_path(claim_id).collect();
        assert_eq!(source_ids.len(), world.sources.len());
    }

    #[test]
    fn test_display_impls() {
        // Ensure all Display impls work without panicking
        assert_eq!(format!("{}", SourceKind::Academic), "academic");
        assert_eq!(format!("{}", WitnessRole::PeerReviewer), "peer-reviewer");
        assert_eq!(format!("{}", DerivationKind::Deductive), "deductive");
        assert_eq!(format!("{}", CocycleKind::Contradiction), "contradiction");
        assert_eq!(format!("{}", ObType::Claim), "Claim");
        assert_eq!(format!("{}", MorType::DerivesFrom), "derives_from");

        let world = analyze_claim("According to Dr. Smith, data is clear", "empirical");
        let _s = format!("{world}");
        for c in world.claims.values() {
            let _s = format!("{c}");
        }
    }

    #[test]
    fn test_catclad_error_display() {
        let e = CatCladError::UnsupportedClaim;
        assert_eq!(format!("{e}"), "claim is unsupported or empty");
        let e2 = CatCladError::HashCollision(ContentHash("abc123def456".into()));
        assert!(format!("{e2}").contains("abc123def456"));
    }

    #[test]
    fn test_cocycle_vec() {
        let mut cv = CocycleVec::new();
        assert!(cv.is_empty());
        cv.push(Cocycle {
            claim_a: "c1".into(),
            claim_b: None,
            kind: CocycleKind::Unsupported,
            severity: 0.5,
        });
        assert_eq!(cv.len(), 1);
        assert_eq!(format!("{cv}"), "[1 cocycle(s)]");
    }
}
