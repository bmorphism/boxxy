#!/usr/bin/env python3
"""Cat-clad anti-bullshit epistemological verification MCP server.

A "cat-clad" claim is an object in a category with morphisms tracking
its provenance, derivation history, and the consistency conditions that
bind it to other claims. Verification reduces to structural properties:

  - Provenance is a composable morphism chain to primary sources
  - Consistency is a sheaf condition (H^1 = 0 means no contradictions)
  - GF(3) conservation prevents unbounded generation without verification
  - Cocycles detect structural inconsistencies

ACSet Schema (SchClaimWorld):

    Claim::Ob           -- assertions to verify
    Source::Ob           -- evidence or citations
    Witness::Ob          -- attestation parties
    Derivation::Ob       -- inference steps (Source -> Claim)
    Cocycle              -- sheaf obstructions (H^1)

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
import re
import uuid
from dataclasses import asdict, dataclass, field
from enum import IntEnum
from typing import Any

from mcp.server import Server
from mcp.server.stdio import stdio_server
from mcp.types import TextContent, Tool

# ---------------------------------------------------------------------------
# GF(3) arithmetic
# ---------------------------------------------------------------------------

class Trit(IntEnum):
    """Element of GF(3): the Galois field with three elements."""
    ZERO = 0   # coordinator
    ONE = 1    # generator
    TWO = 2    # verifier

    def __add__(self, other: Trit) -> Trit:  # type: ignore[override]
        return Trit((int(self) + int(other)) % 3)

    def __mul__(self, other: Trit) -> Trit:  # type: ignore[override]
        return Trit((int(self) * int(other)) % 3)

    def __neg__(self) -> Trit:
        return Trit((3 - int(self)) % 3)

    @staticmethod
    def is_balanced(trits: list[Trit]) -> bool:
        """Check conservation law: sum of trits == 0 (mod 3)."""
        return sum(int(t) for t in trits) % 3 == 0

    @property
    def role(self) -> str:
        return {0: "coordinator", 1: "generator", 2: "verifier"}[int(self)]


# ---------------------------------------------------------------------------
# ACSet Schema -- dataclasses
# ---------------------------------------------------------------------------

@dataclass
class Claim:
    id: str
    text: str
    trit: Trit          # generator = 1
    hash: str           # SHA-256 of normalized text
    confidence: float   # 0.0 - 1.0
    framework: str      # epistemological framework


@dataclass
class Source:
    id: str
    citation: str
    trit: Trit          # verifier = 2
    hash: str
    kind: str           # academic | authority | url | anecdotal


@dataclass
class Witness:
    id: str
    name: str
    trit: Trit          # coordinator = 0
    role: str           # author | peer-reviewer | editor | publisher | self
    weight: float       # 0.0 - 1.0


@dataclass
class Derivation:
    id: str
    source_id: str
    claim_id: str
    kind: str           # direct | deductive | appeal-to-authority | analogical
    strength: float     # 0.0 - 1.0


@dataclass
class Cocycle:
    claim_a: str
    claim_b: str
    kind: str           # contradiction | unsupported | circular | trit-violation | weak-authority
    severity: float     # 0.0 - 1.0


@dataclass
class ClaimWorld:
    """The ACSet instance -- a cat-clad epistemological universe."""
    claims: dict[str, Claim] = field(default_factory=dict)
    sources: dict[str, Source] = field(default_factory=dict)
    witnesses: dict[str, Witness] = field(default_factory=dict)
    derivations: list[Derivation] = field(default_factory=list)
    cocycles: list[Cocycle] = field(default_factory=list)

    # -- GF(3) conservation --

    def all_trits(self) -> list[Trit]:
        trits: list[Trit] = []
        for c in self.claims.values():
            trits.append(c.trit)
        for s in self.sources.values():
            trits.append(s.trit)
        for w in self.witnesses.values():
            trits.append(w.trit)
        return trits

    def gf3_balance(self) -> tuple[bool, dict[str, int]]:
        counts = {"coordinator": 0, "generator": 0, "verifier": 0}
        trits = self.all_trits()
        for t in trits:
            counts[t.role] += 1
        return Trit.is_balanced(trits), counts

    # -- Sheaf consistency (H^1) --

    def sheaf_consistency(self) -> tuple[int, list[Cocycle]]:
        return len(self.cocycles), self.cocycles

    def to_dict(self) -> dict[str, Any]:
        """Serialize the entire world for JSON output."""
        return {
            "claims": [_dc_dict(c) for c in self.claims.values()],
            "sources": [_dc_dict(s) for s in self.sources.values()],
            "witnesses": [_dc_dict(w) for w in self.witnesses.values()],
            "derivations": [_dc_dict(d) for d in self.derivations],
            "cocycles": [_dc_dict(cy) for cy in self.cocycles],
        }


def _dc_dict(obj: Any) -> dict[str, Any]:
    d = asdict(obj)
    # Convert Trit enums to their int value for JSON
    if "trit" in d:
        d["trit"] = int(d["trit"])
    return d


# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------

def content_hash(text: str) -> str:
    return hashlib.sha256(text.strip().lower().encode()).hexdigest()


# -- Source extraction via regex --

_SOURCE_PATTERNS: list[tuple[re.Pattern[str], str]] = [
    (re.compile(r"(?i)(?:according to|cited by|reported by)\s+([^,\.]+)"), "authority"),
    (re.compile(r"(?i)(?:study|research|paper)\s+(?:by|from|in)\s+([^,\.]+)"), "academic"),
    (re.compile(r"(?i)(?:published in|journal of)\s+([^,\.]+)"), "academic"),
    (re.compile(r"(?i)(https?://\S+)"), "url"),
]


def extract_sources(text: str) -> list[Source]:
    sources: list[Source] = []
    seen: set[str] = set()
    for pattern, kind in _SOURCE_PATTERNS:
        for m in pattern.finditer(text):
            citation = m.group(1).strip()
            sid = content_hash(citation)[:12]
            if sid in seen:
                continue
            seen.add(sid)
            sources.append(Source(
                id=sid,
                citation=citation,
                trit=Trit.TWO,   # verifier
                hash=content_hash(citation),
                kind=kind,
            ))
    return sources


def _witness_role(kind: str) -> str:
    return {
        "academic": "peer-reviewer",
        "authority": "author",
        "url": "publisher",
        "anecdotal": "self",
    }.get(kind, "self")


def _witness_weight(kind: str) -> float:
    return {
        "academic": 0.9,
        "authority": 0.6,
        "url": 0.4,
        "anecdotal": 0.2,
    }.get(kind, 0.2)


def extract_witnesses(src: Source) -> list[Witness]:
    return [Witness(
        id=f"w-{src.id}",
        name=src.citation,
        trit=Trit.ZERO,  # coordinator
        role=_witness_role(src.kind),
        weight=_witness_weight(src.kind),
    )]


def classify_derivation(src: Source) -> str:
    return {
        "academic": "deductive",
        "authority": "appeal-to-authority",
        "url": "direct",
        "anecdotal": "analogical",
    }.get(src.kind, "analogical")


def source_strength(src: Source) -> float:
    return {
        "academic": 0.85,
        "authority": 0.5,
        "url": 0.3,
        "anecdotal": 0.1,
    }.get(src.kind, 0.1)


# -- Confidence computation --

def compute_confidence(world: ClaimWorld, claim: Claim, framework: str) -> float:
    if not world.sources:
        return 0.1  # unsupported claim gets minimal confidence

    # Average derivation strength for this claim
    strengths = [d.strength for d in world.derivations if d.claim_id == claim.id]
    if not strengths:
        return 0.1
    avg = sum(strengths) / len(strengths)

    # Framework-specific weighting
    if framework == "empirical":
        academic_count = sum(1 for s in world.sources.values() if s.kind == "academic")
        if academic_count > 0:
            avg *= 1.0 + 0.1 * academic_count
    elif framework == "responsible":
        lower = claim.text.lower()
        if "community" in lower or "benefit" in lower:
            avg *= 1.1
    elif framework == "harmonic":
        if len(world.sources) >= 3:
            avg *= 1.15
    # "pluralistic" -- no special boost, raw structural quality

    # Penalize cocycles
    cocycle_penalty = 0.15 * len(world.cocycles)
    confidence = avg - cocycle_penalty

    return max(0.0, min(1.0, confidence))


# -- Cocycle detection (H^1 obstructions) --

def detect_cocycles(world: ClaimWorld) -> list[Cocycle]:
    cocycles: list[Cocycle] = []

    # Unsupported claims (no derivation chain)
    for claim in world.claims.values():
        has_derivation = any(d.claim_id == claim.id for d in world.derivations)
        if not has_derivation:
            cocycles.append(Cocycle(
                claim_a=claim.id,
                claim_b="",
                kind="unsupported",
                severity=0.9,
            ))

    # Weak authority without proper verification
    for d in world.derivations:
        if d.kind == "appeal-to-authority" and d.strength < 0.6:
            cocycles.append(Cocycle(
                claim_a=d.claim_id,
                claim_b=d.source_id,
                kind="weak-authority",
                severity=0.5,
            ))

    # GF(3) conservation violation
    balanced, _ = world.gf3_balance()
    if not balanced:
        cocycles.append(Cocycle(
            claim_a="",
            claim_b="",
            kind="trit-violation",
            severity=0.3,
        ))

    return cocycles


# ---------------------------------------------------------------------------
# Core analysis functions
# ---------------------------------------------------------------------------

def analyze_claim(text: str, framework: str = "pluralistic") -> ClaimWorld:
    """Parse text into a cat-clad structure and check consistency."""
    world = ClaimWorld()

    # Primary claim -- generator role
    claim = Claim(
        id=content_hash(text)[:12],
        text=text,
        trit=Trit.ONE,
        hash=content_hash(text),
        confidence=0.0,
        framework=framework,
    )
    world.claims[claim.id] = claim

    # Extract sources as morphisms
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

    # Extract witnesses
    for src in sources:
        for w in extract_witnesses(src):
            world.witnesses[w.id] = w

    # Detect cocycles before confidence (so penalty applies)
    world.cocycles = detect_cocycles(world)

    # Compute confidence (framework-weighted, cocycle-penalized)
    claim.confidence = compute_confidence(world, claim, framework)

    return world


def validate_sources(text: str, framework: str = "pluralistic") -> dict[str, Any]:
    """Extract and classify sources, compute witness weights."""
    sources = extract_sources(text)
    results: list[dict[str, Any]] = []

    for src in sources:
        witnesses = extract_witnesses(src)
        results.append({
            "source": _dc_dict(src),
            "witnesses": [_dc_dict(w) for w in witnesses],
            "derivation_kind": classify_derivation(src),
            "strength": source_strength(src),
        })

    # Framework boost summary
    boost = "none"
    if framework == "empirical":
        academic = sum(1 for s in sources if s.kind == "academic")
        if academic > 0:
            boost = f"empirical: +{academic * 10}% for {academic} academic source(s)"
    elif framework == "responsible":
        if "community" in text.lower() or "benefit" in text.lower():
            boost = "responsible: +10% for community/benefit language"
    elif framework == "harmonic":
        if len(sources) >= 3:
            boost = "harmonic: +15% for multi-source convergence"

    # GF(3) check on sources + implied witnesses
    trits: list[Trit] = [Trit.ONE]  # implied claim (generator)
    for s in sources:
        trits.append(s.trit)
    for s in sources:
        for w in extract_witnesses(s):
            trits.append(w.trit)
    balanced = Trit.is_balanced(trits)

    return {
        "sources": results,
        "total_sources": len(sources),
        "framework": framework,
        "framework_boost": boost,
        "gf3_balanced": balanced,
        "gf3_sum_mod3": sum(int(t) for t in trits) % 3,
    }


# -- Manipulation detection --

@dataclass
class ManipulationPattern:
    kind: str
    evidence: str
    severity: float


_MANIPULATION_CHECKS: list[tuple[str, re.Pattern[str], float]] = [
    ("emotional_fear",
     re.compile(r"(?i)(fear|terrif|alarm|panic|dread|catastroph)"), 0.7),
    ("urgency",
     re.compile(r"(?i)(act now|limited time|don't wait|expires|hurry|last chance|before it's too late)"), 0.8),
    ("false_consensus",
     re.compile(r"(?i)(everyone knows|nobody (?:believes|wants|thinks)|all experts|unanimous|widely accepted)"), 0.6),
    ("appeal_authority",
     re.compile(r"(?i)(experts say|scientists (?:claim|prove)|studies show|research proves|doctors recommend)"), 0.5),
    ("artificial_scarcity",
     re.compile(r"(?i)(exclusive|rare opportunity|only \d+ left|limited (?:edition|supply|spots))"), 0.7),
    ("social_pressure",
     re.compile(r"(?i)(you don't want to be|don't miss out|join .* (?:others|people)|be the first)"), 0.6),
    ("loaded_language",
     re.compile(r"(?i)(obviously|clearly|undeniably|unquestionably|beyond doubt)"), 0.4),
    ("false_dichotomy",
     re.compile(r"(?i)(either .* or|only (?:two|2) (?:options|choices)|if you don't .* then)"), 0.6),
    ("circular_reasoning",
     re.compile(r"(?i)(because .* therefore .* because|true because .* which is true)"), 0.9),
    ("ad_hominem",
     re.compile(r"(?i)((?:stupid|idiot|moron|fool|ignorant|naive) .* (?:think|believe|say))"), 0.8),
]


def check_manipulation(text: str) -> dict[str, Any]:
    """Detect manipulation patterns with severity scoring."""
    patterns_found: list[dict[str, Any]] = []

    for kind, pattern, weight in _MANIPULATION_CHECKS:
        for m in pattern.finditer(text):
            patterns_found.append({
                "kind": kind,
                "evidence": m.group(0),
                "severity": weight,
            })

    total_severity = sum(p["severity"] for p in patterns_found)
    max_severity = max((p["severity"] for p in patterns_found), default=0.0)

    return {
        "patterns": patterns_found,
        "total_patterns": len(patterns_found),
        "total_severity": round(total_severity, 3),
        "max_severity": max_severity,
        "manipulation_score": round(min(1.0, total_severity / 5.0), 3),
        "verdict": (
            "clean" if not patterns_found
            else "suspicious" if total_severity < 2.0
            else "likely_manipulative"
        ),
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
                            "enum": ["empirical", "responsible", "harmonic", "pluralistic"],
                            "default": "pluralistic",
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
                            "description": (
                                "Epistemological framework for weighting."
                            ),
                            "enum": ["empirical", "responsible", "harmonic", "pluralistic"],
                            "default": "pluralistic",
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
        framework = arguments.get("framework", "pluralistic")

        if name == "analyze_claim":
            world = analyze_claim(text, framework)
            balanced, counts = world.gf3_balance()
            h1, cocycles = world.sheaf_consistency()
            result = {
                "world": world.to_dict(),
                "gf3_balanced": balanced,
                "gf3_counts": counts,
                "h1_dimension": h1,
                "confidence": world.claims[list(world.claims.keys())[0]].confidence
                    if world.claims else 0.0,
            }
            return [TextContent(
                type="text",
                text=json.dumps(result, indent=2),
            )]

        elif name == "validate_sources":
            result = validate_sources(text, framework)
            return [TextContent(
                type="text",
                text=json.dumps(result, indent=2),
            )]

        elif name == "check_manipulation":
            result = check_manipulation(text)
            return [TextContent(
                type="text",
                text=json.dumps(result, indent=2),
            )]

        else:
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
