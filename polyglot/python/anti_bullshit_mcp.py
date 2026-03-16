#!/usr/bin/env python3
"""Cat-clad anti-bullshit epistemological verification MCP server.

A "cat-clad" claim is an object in a double category with morphisms tracking
its provenance, derivation history, and the consistency conditions that
bind it to other claims.  Verification reduces to structural properties:

  - Provenance is a composable morphism chain to primary sources
  - Consistency is a sheaf condition (H^1 = 0 means no contradictions)
  - GF(3) conservation prevents unbounded generation without verification
  - Cocycles detect structural inconsistencies

DblTheory Schema (EpistemicTheory):

    ObTypes:  Claim, Source, Witness
    MorTypes: Derivation (Source -> Claim), Attestation (Witness -> Source)
    Cocycle:  sheaf obstructions (H^1)

    Paths compose via PathSegment chains (immutable tuples).

GF(3) roles:
    0 = coordinator (Witnesses)
    1 = generator   (Claims)
    2 = verifier    (Sources)

Conservation law: sum of all trits == 0 (mod 3).

Requirements:
    pip install mcp
    # or: pip install "mcp[cli]"
"""

from __future__ import annotations

import asyncio
import hashlib
import json
import operator
import re
from collections import Counter
from dataclasses import asdict, dataclass, field
from enum import IntEnum, StrEnum
from functools import reduce
from itertools import chain
from typing import Any, Literal, NamedTuple, TypedDict

from mcp.server import Server
from mcp.server.stdio import stdio_server
from mcp.types import TextContent, Tool

# ---------------------------------------------------------------------------
# GF(3) arithmetic — the Galois field with three elements
# ---------------------------------------------------------------------------

type Trit = Literal[0, 1, 2]


class Trit(IntEnum):
    """Element of GF(3): the Galois field with three elements."""
    ZERO = 0   # coordinator
    ONE  = 1   # generator
    TWO  = 2   # verifier

    def __add__(self, other: Trit) -> Trit:  # type: ignore[override]
        return Trit((int(self) + int(other)) % 3)

    def __mul__(self, other: Trit) -> Trit:  # type: ignore[override]
        return Trit((int(self) * int(other)) % 3)

    def __neg__(self) -> Trit:
        return Trit((3 - int(self)) % 3)

    @staticmethod
    def is_balanced(trits: list[Trit]) -> bool:
        """Conservation law: sum of trits == 0 (mod 3) via functools.reduce."""
        return reduce(operator.add, (int(t) for t in trits), 0) % 3 == 0

    @property
    def role(self) -> str:
        match int(self):
            case 0: return "coordinator"
            case 1: return "generator"
            case 2: return "verifier"
            case _: raise ValueError(f"impossible trit value: {self}")


# ---------------------------------------------------------------------------
# StrEnum taxonomies — post-modern enum style
# ---------------------------------------------------------------------------

class Framework(StrEnum):
    """Epistemological framework for weighting claims."""
    EMPIRICAL   = "empirical"
    RESPONSIBLE = "responsible"
    HARMONIC    = "harmonic"
    PLURALISTIC = "pluralistic"


class SourceKind(StrEnum):
    """Classification of evidence provenance."""
    ACADEMIC  = "academic"
    AUTHORITY = "authority"
    URL       = "url"
    ANECDOTAL = "anecdotal"


class WitnessRole(StrEnum):
    """Attestation capacity of a witness."""
    PEER_REVIEWER = "peer-reviewer"
    AUTHOR        = "author"
    EDITOR        = "editor"
    PUBLISHER     = "publisher"
    SELF          = "self"


class DerivationKind(StrEnum):
    """How a claim is derived from a source."""
    DIRECT              = "direct"
    DEDUCTIVE           = "deductive"
    APPEAL_TO_AUTHORITY = "appeal-to-authority"
    ANALOGICAL          = "analogical"


class CocycleKind(StrEnum):
    """H^1 obstruction classification."""
    CONTRADICTION   = "contradiction"
    UNSUPPORTED     = "unsupported"
    CIRCULAR        = "circular"
    TRIT_VIOLATION  = "trit-violation"
    WEAK_AUTHORITY  = "weak-authority"


# ---------------------------------------------------------------------------
# DblTheory — type-level schema for the epistemic double category
# ---------------------------------------------------------------------------

@dataclass(frozen=True, slots=True)
class ObType:
    """An object type in the double theory."""
    name: str

    def __hash__(self) -> int:
        return hash(self.name)


@dataclass(frozen=True, slots=True)
class MorType:
    """A morphism type in the double theory: dom -> cod."""
    name: str
    dom: ObType
    cod: ObType

    def __hash__(self) -> int:
        return hash((self.name, self.dom, self.cod))


@dataclass(frozen=True, slots=True)
class PathSegment:
    """One segment in a composable morphism chain."""
    mor_type: MorType
    label: str

    def __hash__(self) -> int:
        return hash((self.mor_type, self.label))


# A Path is an immutable chain of PathSegments — composition by concatenation.
type Path = tuple[PathSegment, ...]


@dataclass(frozen=True, slots=True)
class ObOp:
    """An operation on objects (functorial action)."""
    name: str
    input_type: ObType
    output_type: ObType


@dataclass(frozen=True, slots=True)
class MorOp:
    """An operation on morphisms (natural transformation component)."""
    name: str
    input_type: MorType
    output_type: MorType


@dataclass(frozen=True)
class EpistemicTheory:
    """The DblTheory for epistemological verification.

    Defines the type structure: which ObTypes and MorTypes exist,
    and supports `in` checks for membership.
    """
    ob_types: frozenset[ObType]
    mor_types: frozenset[MorType]
    ob_ops: frozenset[ObOp] = frozenset()
    mor_ops: frozenset[MorOp] = frozenset()

    def __contains__(self, item: ObType | MorType | ObOp | MorOp) -> bool:
        match item:
            case ObType():   return item in self.ob_types
            case MorType():  return item in self.mor_types
            case ObOp():     return item in self.ob_ops
            case MorOp():    return item in self.mor_ops
            case _:          return False


# Canonical types for the epistemic theory
_OB_CLAIM   = ObType("Claim")
_OB_SOURCE  = ObType("Source")
_OB_WITNESS = ObType("Witness")

_MOR_DERIVATION  = MorType("Derivation", dom=_OB_SOURCE, cod=_OB_CLAIM)
_MOR_ATTESTATION = MorType("Attestation", dom=_OB_WITNESS, cod=_OB_SOURCE)

EPISTEMIC_THEORY = EpistemicTheory(
    ob_types=frozenset({_OB_CLAIM, _OB_SOURCE, _OB_WITNESS}),
    mor_types=frozenset({_MOR_DERIVATION, _MOR_ATTESTATION}),
)


# ---------------------------------------------------------------------------
# ACSet value types — frozen + slots for immutability and memory efficiency
# ---------------------------------------------------------------------------
# NOTE: Claim uses frozen=False because downstream code (including tests)
# mutates `confidence` after initial construction.  All other value types
# are fully frozen.

@dataclass(slots=True)
class Claim:
    """A generator-role assertion in the epistemic category."""
    id: str
    text: str
    trit: Trit          # generator = 1
    hash: str           # SHA-256 of normalized text
    confidence: float   # 0.0 - 1.0 (mutable: updated after cocycle detection)
    framework: str      # epistemological framework

    def __hash__(self) -> int:
        return int(self.hash, 16) % (2**61 - 1)


@dataclass(frozen=True, slots=True)
class Source:
    """A verifier-role evidence node in the epistemic category."""
    id: str
    citation: str
    trit: Trit          # verifier = 2
    hash: str
    kind: str           # SourceKind value

    def __hash__(self) -> int:
        return int(self.hash, 16) % (2**61 - 1)


@dataclass(frozen=True, slots=True)
class Witness:
    """A coordinator-role attestation party."""
    id: str
    name: str
    trit: Trit          # coordinator = 0
    role: str           # WitnessRole value
    weight: float       # 0.0 - 1.0

    def __hash__(self) -> int:
        return hash((self.id, self.name, int(self.trit)))


@dataclass(frozen=True, slots=True)
class Derivation:
    """A morphism from Source to Claim in the epistemic category."""
    id: str
    source_id: str
    claim_id: str
    kind: str           # DerivationKind value
    strength: float     # 0.0 - 1.0

    def __hash__(self) -> int:
        return hash(self.id)


@dataclass(frozen=True, slots=True)
class Cocycle:
    """An H^1 sheaf obstruction between claims."""
    claim_a: str
    claim_b: str
    kind: str           # CocycleKind value
    severity: float     # 0.0 - 1.0

    def __hash__(self) -> int:
        return hash((self.claim_a, self.claim_b, self.kind))


@dataclass(frozen=True, slots=True)
class ManipulationPattern:
    """A detected manipulation pattern with severity."""
    kind: str
    evidence: str
    severity: float


# ---------------------------------------------------------------------------
# TypedDict result shapes for MCP tool responses
# ---------------------------------------------------------------------------

class GF3Counts(TypedDict):
    coordinator: int
    generator: int
    verifier: int


class AnalyzeResult(TypedDict):
    world: dict[str, Any]
    gf3_balanced: bool
    gf3_counts: GF3Counts
    h1_dimension: int
    confidence: float


class SourceDetail(TypedDict):
    source: dict[str, Any]
    witnesses: list[dict[str, Any]]
    derivation_kind: str
    strength: float


class ValidateResult(TypedDict):
    sources: list[SourceDetail]
    total_sources: int
    framework: str
    framework_boost: str
    gf3_balanced: bool
    gf3_sum_mod3: int


class ManipulationResult(TypedDict):
    patterns: list[dict[str, Any]]
    total_patterns: int
    total_severity: float
    max_severity: float
    manipulation_score: float
    verdict: str


# ---------------------------------------------------------------------------
# NamedTuple lightweight returns
# ---------------------------------------------------------------------------

class GF3Balance(NamedTuple):
    """Result of a GF(3) conservation check."""
    balanced: bool
    counts: dict[str, int]


class SheafConsistency(NamedTuple):
    """Result of an H^1 cohomology check."""
    h1: int
    cocycles: list[Cocycle]


# ---------------------------------------------------------------------------
# DblModel — the runtime epistemic model (mutable instance of the theory)
# ---------------------------------------------------------------------------

@dataclass
class EpistemicModel:
    """A DblModel over the EpistemicTheory.

    This is the runtime container (the ACSet instance) validated against
    the theory at construction time via __post_init__.
    """
    theory: EpistemicTheory = field(default=EPISTEMIC_THEORY)
    claims: dict[str, Claim] = field(default_factory=dict)
    sources: dict[str, Source] = field(default_factory=dict)
    witnesses: dict[str, Witness] = field(default_factory=dict)
    derivations: list[Derivation] = field(default_factory=list)
    cocycles: list[Cocycle] = field(default_factory=list)

    def __post_init__(self) -> None:
        """Validate that the theory contains the required types."""
        assert _OB_CLAIM in self.theory, "Theory must contain Claim ObType"
        assert _OB_SOURCE in self.theory, "Theory must contain Source ObType"
        assert _OB_WITNESS in self.theory, "Theory must contain Witness ObType"
        assert _MOR_DERIVATION in self.theory, "Theory must contain Derivation MorType"


# Backward-compatible alias
ClaimWorld = EpistemicModel


# ---------------------------------------------------------------------------
# Trit collection helpers — itertools.chain + Counter
# ---------------------------------------------------------------------------

def _all_trits(world: ClaimWorld) -> list[Trit]:
    """Gather every trit from claims, sources, and witnesses via itertools.chain."""
    return list(chain(
        (c.trit for c in world.claims.values()),
        (s.trit for s in world.sources.values()),
        (w.trit for w in world.witnesses.values()),
    ))


def _gf3_balance(world: ClaimWorld) -> GF3Balance:
    """Check GF(3) conservation using Counter for role aggregation."""
    trits = _all_trits(world)
    role_counts = Counter(t.role for t in trits)
    counts: dict[str, int] = {
        "coordinator": role_counts.get("coordinator", 0),
        "generator":   role_counts.get("generator", 0),
        "verifier":    role_counts.get("verifier", 0),
    }
    return GF3Balance(Trit.is_balanced(trits), counts)


def _sheaf_consistency(world: ClaimWorld) -> SheafConsistency:
    """Report H^1 dimension and cocycle list."""
    return SheafConsistency(len(world.cocycles), world.cocycles)


# Patch methods onto ClaimWorld for backward compat
ClaimWorld.all_trits = _all_trits          # type: ignore[attr-defined]
ClaimWorld.gf3_balance = _gf3_balance      # type: ignore[attr-defined]
ClaimWorld.sheaf_consistency = _sheaf_consistency  # type: ignore[attr-defined]


def _world_to_dict(world: ClaimWorld) -> dict[str, Any]:
    """Serialize the entire world for JSON output."""
    return {
        "claims":      [_dc_dict(c) for c in world.claims.values()],
        "sources":     [_dc_dict(s) for s in world.sources.values()],
        "witnesses":   [_dc_dict(w) for w in world.witnesses.values()],
        "derivations": [_dc_dict(d) for d in world.derivations],
        "cocycles":    [_dc_dict(cy) for cy in world.cocycles],
    }


ClaimWorld.to_dict = _world_to_dict  # type: ignore[attr-defined]


# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------

def content_hash(text: str) -> str:
    """SHA-256 content-addressable identity over normalized text."""
    return hashlib.sha256(text.strip().lower().encode()).hexdigest()


def _dc_dict(obj: Any) -> dict[str, Any]:
    """Dataclass -> dict, coercing Trit enums to int for JSON serialization."""
    d = asdict(obj)
    if "trit" in d:
        d["trit"] = int(d["trit"])
    return d


# ---------------------------------------------------------------------------
# Pre-compiled regex patterns (module-level for performance)
# ---------------------------------------------------------------------------

_RE_AUTHORITY = re.compile(
    r"(?i)(?:according to|cited by|reported by)\s+([^,\.]+)")
_RE_STUDY = re.compile(
    r"(?i)(?:study|research|paper)\s+(?:by|from|in)\s+([^,\.]+)")
_RE_JOURNAL = re.compile(
    r"(?i)(?:published in|journal of)\s+([^,\.]+)")
_RE_URL = re.compile(
    r"(?i)(https?://\S+)")

_SOURCE_PATTERNS: list[tuple[re.Pattern[str], str]] = [
    (_RE_AUTHORITY, SourceKind.AUTHORITY),
    (_RE_STUDY,     SourceKind.ACADEMIC),
    (_RE_JOURNAL,   SourceKind.ACADEMIC),
    (_RE_URL,       SourceKind.URL),
]

# Manipulation detection patterns (pre-compiled)
_RE_FEAR = re.compile(
    r"(?i)(fear|terrif|alarm|panic|dread|catastroph)")
_RE_URGENCY = re.compile(
    r"(?i)(act now|limited time|don't wait|expires|hurry|last chance|before it's too late)")
_RE_FALSE_CONSENSUS = re.compile(
    r"(?i)(everyone knows|nobody (?:believes|wants|thinks)|all experts|unanimous|widely accepted)")
_RE_APPEAL_AUTHORITY = re.compile(
    r"(?i)(experts say|scientists (?:claim|prove)|studies show|research proves|doctors recommend)")
_RE_SCARCITY = re.compile(
    r"(?i)(exclusive|rare opportunity|only \d+ left|limited (?:edition|supply|spots))")
_RE_SOCIAL_PRESSURE = re.compile(
    r"(?i)(you don't want to be|don't miss out|join .* (?:others|people)|be the first)")
_RE_LOADED = re.compile(
    r"(?i)(obviously|clearly|undeniably|unquestionably|beyond doubt)")
_RE_DICHOTOMY = re.compile(
    r"(?i)(either .* or|only (?:two|2) (?:options|choices)|if you don't .* then)")
_RE_CIRCULAR = re.compile(
    r"(?i)(because .* therefore .* because|true because .* which is true)")
_RE_AD_HOMINEM = re.compile(
    r"(?i)((?:stupid|idiot|moron|fool|ignorant|naive) .* (?:think|believe|say))")

_MANIPULATION_CHECKS: list[tuple[str, re.Pattern[str], float]] = [
    ("emotional_fear",      _RE_FEAR,             0.7),
    ("urgency",             _RE_URGENCY,           0.8),
    ("false_consensus",     _RE_FALSE_CONSENSUS,   0.6),
    ("appeal_authority",    _RE_APPEAL_AUTHORITY,   0.5),
    ("artificial_scarcity", _RE_SCARCITY,          0.7),
    ("social_pressure",     _RE_SOCIAL_PRESSURE,   0.6),
    ("loaded_language",     _RE_LOADED,            0.4),
    ("false_dichotomy",     _RE_DICHOTOMY,         0.6),
    ("circular_reasoning",  _RE_CIRCULAR,          0.9),
    ("ad_hominem",          _RE_AD_HOMINEM,        0.8),
]

# ---------------------------------------------------------------------------
# Mapping tables — SourceKind -> (WitnessRole, weight, DerivationKind, strength)
# ---------------------------------------------------------------------------

_KIND_WITNESS_ROLE: dict[str, str] = {
    SourceKind.ACADEMIC:  WitnessRole.PEER_REVIEWER,
    SourceKind.AUTHORITY: WitnessRole.AUTHOR,
    SourceKind.URL:       WitnessRole.PUBLISHER,
    SourceKind.ANECDOTAL: WitnessRole.SELF,
}

_KIND_WITNESS_WEIGHT: dict[str, float] = {
    SourceKind.ACADEMIC:  0.9,
    SourceKind.AUTHORITY: 0.6,
    SourceKind.URL:       0.4,
    SourceKind.ANECDOTAL: 0.2,
}

_KIND_DERIVATION: dict[str, str] = {
    SourceKind.ACADEMIC:  DerivationKind.DEDUCTIVE,
    SourceKind.AUTHORITY: DerivationKind.APPEAL_TO_AUTHORITY,
    SourceKind.URL:       DerivationKind.DIRECT,
    SourceKind.ANECDOTAL: DerivationKind.ANALOGICAL,
}

_KIND_STRENGTH: dict[str, float] = {
    SourceKind.ACADEMIC:  0.85,
    SourceKind.AUTHORITY: 0.5,
    SourceKind.URL:       0.3,
    SourceKind.ANECDOTAL: 0.1,
}


# ---------------------------------------------------------------------------
# Source extraction pipeline — generator with yield from
# ---------------------------------------------------------------------------

def _source_candidates(text: str):
    """Generator yielding (citation, kind, hash_prefix) from regex matches."""
    for pattern, kind in _SOURCE_PATTERNS:
        yield from (
            (m.group(1).strip(), kind, content_hash(m.group(1).strip())[:12])
            for m in pattern.finditer(text)
        )


def extract_sources(text: str) -> list[Source]:
    """Extract and deduplicate sources via generator pipeline."""
    seen: set[str] = set()
    sources: list[Source] = []
    for citation, kind, sid in _source_candidates(text):
        if sid not in seen:
            seen.add(sid)
            sources.append(Source(
                id=sid,
                citation=citation,
                trit=Trit.TWO,
                hash=content_hash(citation),
                kind=kind,
            ))
    return sources


def extract_witnesses(src: Source) -> list[Witness]:
    """Derive a witness from a source's attestation morphism."""
    return [Witness(
        id=f"w-{src.id}",
        name=src.citation,
        trit=Trit.ZERO,
        role=_KIND_WITNESS_ROLE.get(src.kind, WitnessRole.SELF),
        weight=_KIND_WITNESS_WEIGHT.get(src.kind, 0.2),
    )]


def classify_derivation(src: Source) -> str:
    """Map source kind to derivation morphism type."""
    return _KIND_DERIVATION.get(src.kind, DerivationKind.ANALOGICAL)


def source_strength(src: Source) -> float:
    """Map source kind to derivation strength."""
    return _KIND_STRENGTH.get(src.kind, 0.1)


# ---------------------------------------------------------------------------
# Confidence computation — framework dispatch via match/case
# ---------------------------------------------------------------------------

def compute_confidence(world: ClaimWorld, claim: Claim, framework: str) -> float:
    """Compute cocycle-penalized, framework-weighted confidence."""
    if not world.sources:
        return 0.1

    strengths = [d.strength for d in world.derivations if d.claim_id == claim.id]
    if not strengths:
        return 0.1

    avg = reduce(operator.add, strengths) / len(strengths)

    match framework:
        case Framework.EMPIRICAL:
            academic_count = sum(
                1 for s in world.sources.values() if s.kind == SourceKind.ACADEMIC
            )
            if academic_count > 0:
                avg *= 1.0 + 0.1 * academic_count
        case Framework.RESPONSIBLE:
            lower = claim.text.lower()
            if "community" in lower or "benefit" in lower:
                avg *= 1.1
        case Framework.HARMONIC:
            if len(world.sources) >= 3:
                avg *= 1.15
        case Framework.PLURALISTIC | _:
            pass  # no special boost — raw structural quality

    cocycle_penalty = 0.15 * len(world.cocycles)
    return max(0.0, min(1.0, avg - cocycle_penalty))


# ---------------------------------------------------------------------------
# Cocycle detection (H^1 obstructions) — match/case for kind dispatch
# ---------------------------------------------------------------------------

def detect_cocycles(world: ClaimWorld) -> list[Cocycle]:
    """Detect sheaf obstructions across the epistemic model."""
    cocycles: list[Cocycle] = []

    # Unsupported claims (no derivation chain)
    for claim in world.claims.values():
        if not any(d.claim_id == claim.id for d in world.derivations):
            cocycles.append(Cocycle(
                claim_a=claim.id,
                claim_b="",
                kind=CocycleKind.UNSUPPORTED,
                severity=0.9,
            ))

    # Weak authority without proper verification
    for d in world.derivations:
        match d:
            case Derivation(kind=DerivationKind.APPEAL_TO_AUTHORITY, strength=s) if s < 0.6:
                cocycles.append(Cocycle(
                    claim_a=d.claim_id,
                    claim_b=d.source_id,
                    kind=CocycleKind.WEAK_AUTHORITY,
                    severity=0.5,
                ))

    # GF(3) conservation violation
    balanced, _ = world.gf3_balance()
    if not balanced:
        cocycles.append(Cocycle(
            claim_a="",
            claim_b="",
            kind=CocycleKind.TRIT_VIOLATION,
            severity=0.3,
        ))

    return cocycles


# ---------------------------------------------------------------------------
# Core analysis functions
# ---------------------------------------------------------------------------

def analyze_claim(text: str, framework: str = "pluralistic") -> ClaimWorld:
    """Parse text into a cat-clad structure and check consistency."""
    world = ClaimWorld()

    claim = Claim(
        id=content_hash(text)[:12],
        text=text,
        trit=Trit.ONE,
        hash=content_hash(text),
        confidence=0.0,
        framework=framework,
    )
    world.claims[claim.id] = claim

    sources = extract_sources(text)
    for src in sources:
        world.sources[src.id] = src
        world.derivations.append(Derivation(
            id=f"d-{src.id}-{claim.id}",
            source_id=src.id,
            claim_id=claim.id,
            kind=classify_derivation(src),
            strength=source_strength(src),
        ))

    for src in sources:
        for w in extract_witnesses(src):
            world.witnesses[w.id] = w

    world.cocycles = detect_cocycles(world)
    claim.confidence = compute_confidence(world, claim, framework)

    return world


def validate_sources(text: str, framework: str = "pluralistic") -> ValidateResult:
    """Extract and classify sources, compute witness weights."""
    sources = extract_sources(text)
    results: list[SourceDetail] = []

    for src in sources:
        witnesses = extract_witnesses(src)
        results.append({
            "source":          _dc_dict(src),
            "witnesses":       [_dc_dict(w) for w in witnesses],
            "derivation_kind": classify_derivation(src),
            "strength":        source_strength(src),
        })

    # Framework boost summary via match/case
    match framework:
        case Framework.EMPIRICAL:
            academic = sum(1 for s in sources if s.kind == SourceKind.ACADEMIC)
            boost = (
                f"empirical: +{academic * 10}% for {academic} academic source(s)"
                if academic > 0 else "none"
            )
        case Framework.RESPONSIBLE:
            boost = (
                "responsible: +10% for community/benefit language"
                if "community" in text.lower() or "benefit" in text.lower()
                else "none"
            )
        case Framework.HARMONIC:
            boost = (
                "harmonic: +15% for multi-source convergence"
                if len(sources) >= 3 else "none"
            )
        case _:
            boost = "none"

    # GF(3) check: chain claim trit + source trits + witness trits
    trits: list[Trit] = list(chain(
        [Trit.ONE],
        (s.trit for s in sources),
        (w.trit for s in sources for w in extract_witnesses(s)),
    ))

    return {
        "sources":        results,
        "total_sources":  len(sources),
        "framework":      framework,
        "framework_boost": boost,
        "gf3_balanced":   Trit.is_balanced(trits),
        "gf3_sum_mod3":   reduce(operator.add, (int(t) for t in trits), 0) % 3,
    }


# ---------------------------------------------------------------------------
# Manipulation detection — match/case verdict dispatch
# ---------------------------------------------------------------------------

def check_manipulation(text: str) -> ManipulationResult:
    """Detect manipulation patterns with severity scoring."""
    patterns_found: list[dict[str, Any]] = [
        {"kind": kind, "evidence": m.group(0), "severity": weight}
        for kind, pattern, weight in _MANIPULATION_CHECKS
        for m in pattern.finditer(text)
    ]

    total_severity = reduce(operator.add, (p["severity"] for p in patterns_found), 0.0)
    max_severity = max((p["severity"] for p in patterns_found), default=0.0)

    match patterns_found:
        case []:
            verdict = "clean"
        case _ if total_severity >= 2.0:
            verdict = "likely_manipulative"
        case _:
            verdict = "suspicious"

    return {
        "patterns":           patterns_found,
        "total_patterns":     len(patterns_found),
        "total_severity":     round(total_severity, 3),
        "max_severity":       max_severity,
        "manipulation_score": round(min(1.0, total_severity / 5.0), 3),
        "verdict":            verdict,
    }


# ---------------------------------------------------------------------------
# MCP Server
# ---------------------------------------------------------------------------

def create_server() -> Server:
    """Build and configure the MCP server with all three tools."""
    server = Server("anti-bullshit")

    @server.list_tools()
    async def list_tools() -> list[Tool]:
        return [
            Tool(
                name="analyze_claim",
                description=(
                    "Parse text into a cat-clad epistemological structure. "
                    "Extracts sources via regex, builds derivation morphisms, "
                    "computes GF(3)-weighted confidence, and detects sheaf "
                    "cocycles (H^1 obstructions)."
                ),
                inputSchema={
                    "type": "object",
                    "properties": {
                        "text": {
                            "type": "string",
                            "description": "The claim text to analyze.",
                        },
                        "framework": {
                            "type": "string",
                            "description": (
                                "Epistemological framework: empirical, responsible, "
                                "harmonic, or pluralistic (default)."
                            ),
                            "enum": [f.value for f in Framework],
                            "default": Framework.PLURALISTIC,
                        },
                    },
                    "required": ["text"],
                },
            ),
            Tool(
                name="validate_sources",
                description=(
                    "Extract and classify sources from text. Computes witness "
                    "weights and checks GF(3) conservation across the source graph."
                ),
                inputSchema={
                    "type": "object",
                    "properties": {
                        "text": {
                            "type": "string",
                            "description": "Text containing citations/sources to validate.",
                        },
                        "framework": {
                            "type": "string",
                            "description": "Epistemological framework for weighting.",
                            "enum": [f.value for f in Framework],
                            "default": Framework.PLURALISTIC,
                        },
                    },
                    "required": ["text"],
                },
            ),
            Tool(
                name="check_manipulation",
                description=(
                    "Scan text for 10 manipulation patterns (fear, urgency, "
                    "false consensus, etc.) with severity scoring."
                ),
                inputSchema={
                    "type": "object",
                    "properties": {
                        "text": {
                            "type": "string",
                            "description": "Text to check for manipulation patterns.",
                        },
                    },
                    "required": ["text"],
                },
            ),
        ]

    @server.call_tool()
    async def call_tool(name: str, arguments: dict[str, Any]) -> list[TextContent]:
        text = arguments.get("text", "")
        framework = arguments.get("framework", Framework.PLURALISTIC)

        match name:
            case "analyze_claim":
                world = analyze_claim(text, framework)
                balanced, counts = world.gf3_balance()
                h1, cocycles = world.sheaf_consistency()
                result: AnalyzeResult = {
                    "world":        world.to_dict(),
                    "gf3_balanced": balanced,
                    "gf3_counts":   counts,
                    "h1_dimension": h1,
                    "confidence": (
                        next(iter(world.claims.values())).confidence
                        if world.claims else 0.0
                    ),
                }
                return [TextContent(type="text", text=json.dumps(result, indent=2))]

            case "validate_sources":
                return [TextContent(
                    type="text",
                    text=json.dumps(validate_sources(text, framework), indent=2),
                )]

            case "check_manipulation":
                return [TextContent(
                    type="text",
                    text=json.dumps(check_manipulation(text), indent=2),
                )]

            case _:
                return [TextContent(
                    type="text",
                    text=json.dumps({"error": f"Unknown tool: {name}"}),
                )]

    return server


# ---------------------------------------------------------------------------
# Entry point
# ---------------------------------------------------------------------------

async def main() -> None:
    server = create_server()
    options = server.create_initialization_options()
    async with stdio_server() as (read_stream, write_stream):
        await server.run(read_stream, write_stream, options)


if __name__ == "__main__":
    asyncio.run(main())
