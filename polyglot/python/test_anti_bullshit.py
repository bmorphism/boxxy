"""Tests for the cat-clad anti-bullshit MCP server.

Run with:
    pytest test_anti_bullshit.py -v

Requirements:
    pip install mcp pytest pytest-asyncio
"""

from __future__ import annotations

import json

import pytest

from mcp.types import (
    CallToolRequest,
    CallToolRequestParams,
    ListToolsRequest,
)

from anti_bullshit_mcp import (
    ClaimWorld,
    Cocycle,
    Claim,
    Derivation,
    ManipulationPattern,
    Source,
    Trit,
    Witness,
    analyze_claim,
    check_manipulation,
    classify_derivation,
    compute_confidence,
    content_hash,
    create_server,
    detect_cocycles,
    extract_sources,
    extract_witnesses,
    source_strength,
    validate_sources,
)


# ---------------------------------------------------------------------------
# GF(3) conservation tests
# ---------------------------------------------------------------------------

class TestGF3:
    """Test GF(3) arithmetic and conservation law."""

    def test_trit_addition(self) -> None:
        assert Trit.ZERO + Trit.ZERO == Trit.ZERO
        assert Trit.ONE + Trit.TWO == Trit.ZERO
        assert Trit.TWO + Trit.ONE == Trit.ZERO
        assert Trit.ONE + Trit.ONE == Trit.TWO
        assert Trit.TWO + Trit.TWO == Trit.ONE

    def test_trit_multiplication(self) -> None:
        assert Trit.ZERO * Trit.ONE == Trit.ZERO
        assert Trit.ONE * Trit.ONE == Trit.ONE
        assert Trit.TWO * Trit.TWO == Trit.ONE
        assert Trit.ONE * Trit.TWO == Trit.TWO

    def test_trit_negation(self) -> None:
        assert -Trit.ZERO == Trit.ZERO
        assert -Trit.ONE == Trit.TWO
        assert -Trit.TWO == Trit.ONE

    def test_trit_roles(self) -> None:
        assert Trit.ZERO.role == "coordinator"
        assert Trit.ONE.role == "generator"
        assert Trit.TWO.role == "verifier"

    def test_balanced_empty(self) -> None:
        assert Trit.is_balanced([]) is True

    def test_balanced_single_zero(self) -> None:
        assert Trit.is_balanced([Trit.ZERO]) is True

    def test_balanced_generator_verifier_coordinator(self) -> None:
        # 1 + 2 + 0 = 3 = 0 mod 3
        assert Trit.is_balanced([Trit.ONE, Trit.TWO, Trit.ZERO]) is True

    def test_balanced_three_generators(self) -> None:
        # 1 + 1 + 1 = 3 = 0 mod 3
        assert Trit.is_balanced([Trit.ONE, Trit.ONE, Trit.ONE]) is True

    def test_unbalanced(self) -> None:
        # 1 + 1 = 2 != 0 mod 3
        assert Trit.is_balanced([Trit.ONE, Trit.ONE]) is False

    def test_unbalanced_single_generator(self) -> None:
        assert Trit.is_balanced([Trit.ONE]) is False

    def test_conservation_claim_source_witness_triple(self) -> None:
        """One claim (1) + one source (2) + one witness (0) = 3 = 0 mod 3."""
        world = ClaimWorld()
        world.claims["c1"] = Claim("c1", "test", Trit.ONE, "h1", 0.5, "pluralistic")
        world.sources["s1"] = Source("s1", "citation", Trit.TWO, "h2", "academic")
        world.witnesses["w1"] = Witness("w1", "name", Trit.ZERO, "author", 0.8)
        balanced, counts = world.gf3_balance()
        assert balanced is True
        assert counts == {"coordinator": 1, "generator": 1, "verifier": 1}

    def test_conservation_violation(self) -> None:
        """Two claims (1+1=2) with no sources or witnesses is not balanced."""
        world = ClaimWorld()
        world.claims["c1"] = Claim("c1", "a", Trit.ONE, "h1", 0.5, "pluralistic")
        world.claims["c2"] = Claim("c2", "b", Trit.ONE, "h2", 0.5, "pluralistic")
        balanced, counts = world.gf3_balance()
        assert balanced is False
        assert counts["generator"] == 2


# ---------------------------------------------------------------------------
# Source extraction tests
# ---------------------------------------------------------------------------

class TestSourceExtraction:
    """Test regex-based source extraction."""

    def test_authority_source(self) -> None:
        text = "According to the World Health Organization, vaccines are safe."
        sources = extract_sources(text)
        assert len(sources) >= 1
        src = sources[0]
        assert src.kind == "authority"
        assert "World Health Organization" in src.citation
        assert src.trit == Trit.TWO

    def test_academic_source(self) -> None:
        text = "A study by Harvard researchers found a correlation."
        sources = extract_sources(text)
        assert len(sources) >= 1
        assert any(s.kind == "academic" for s in sources)

    def test_url_source(self) -> None:
        text = "See https://example.com/paper.pdf for details."
        sources = extract_sources(text)
        assert len(sources) >= 1
        assert any(s.kind == "url" for s in sources)

    def test_multiple_sources(self) -> None:
        text = (
            "According to NASA, the Earth is warming. "
            "A study by MIT confirmed this. "
            "See https://climate.nasa.gov for data."
        )
        sources = extract_sources(text)
        assert len(sources) >= 3

    def test_no_sources(self) -> None:
        text = "The sky is blue and grass is green."
        sources = extract_sources(text)
        assert len(sources) == 0

    def test_deduplication(self) -> None:
        text = "According to NASA, it's real. According to NASA, it's confirmed."
        sources = extract_sources(text)
        # Same citation should be deduplicated by hash
        assert len(sources) == 1

    def test_journal_source(self) -> None:
        text = "Published in Nature, the results were striking."
        sources = extract_sources(text)
        assert len(sources) >= 1
        assert any(s.kind == "academic" for s in sources)


# ---------------------------------------------------------------------------
# Witness extraction tests
# ---------------------------------------------------------------------------

class TestWitnessExtraction:
    """Test witness extraction from sources."""

    def test_academic_witness_role(self) -> None:
        src = Source("s1", "Nature", Trit.TWO, "h", "academic")
        witnesses = extract_witnesses(src)
        assert len(witnesses) == 1
        assert witnesses[0].role == "peer-reviewer"
        assert witnesses[0].weight == 0.9

    def test_authority_witness_role(self) -> None:
        src = Source("s1", "WHO", Trit.TWO, "h", "authority")
        witnesses = extract_witnesses(src)
        assert witnesses[0].role == "author"
        assert witnesses[0].weight == 0.6

    def test_url_witness_role(self) -> None:
        src = Source("s1", "https://example.com", Trit.TWO, "h", "url")
        witnesses = extract_witnesses(src)
        assert witnesses[0].role == "publisher"
        assert witnesses[0].weight == 0.4

    def test_witness_trit_is_coordinator(self) -> None:
        src = Source("s1", "test", Trit.TWO, "h", "academic")
        witnesses = extract_witnesses(src)
        assert witnesses[0].trit == Trit.ZERO


# ---------------------------------------------------------------------------
# Derivation classification tests
# ---------------------------------------------------------------------------

class TestDerivation:
    """Test derivation kind and strength classification."""

    def test_academic_derivation(self) -> None:
        src = Source("s1", "c", Trit.TWO, "h", "academic")
        assert classify_derivation(src) == "deductive"
        assert source_strength(src) == 0.85

    def test_authority_derivation(self) -> None:
        src = Source("s1", "c", Trit.TWO, "h", "authority")
        assert classify_derivation(src) == "appeal-to-authority"
        assert source_strength(src) == 0.5

    def test_url_derivation(self) -> None:
        src = Source("s1", "c", Trit.TWO, "h", "url")
        assert classify_derivation(src) == "direct"
        assert source_strength(src) == 0.3

    def test_anecdotal_derivation(self) -> None:
        src = Source("s1", "c", Trit.TWO, "h", "anecdotal")
        assert classify_derivation(src) == "analogical"
        assert source_strength(src) == 0.1


# ---------------------------------------------------------------------------
# Cocycle detection tests
# ---------------------------------------------------------------------------

class TestCocycleDetection:
    """Test sheaf consistency checks."""

    def test_unsupported_claim_cocycle(self) -> None:
        world = ClaimWorld()
        world.claims["c1"] = Claim("c1", "test", Trit.ONE, "h", 0.5, "pluralistic")
        # No derivations = unsupported
        cocycles = detect_cocycles(world)
        assert any(c.kind == "unsupported" for c in cocycles)

    def test_weak_authority_cocycle(self) -> None:
        world = ClaimWorld()
        world.claims["c1"] = Claim("c1", "test", Trit.ONE, "h", 0.5, "pluralistic")
        world.sources["s1"] = Source("s1", "citation", Trit.TWO, "h", "authority")
        world.derivations.append(Derivation(
            "d1", "s1", "c1", "appeal-to-authority", 0.5  # < 0.6 threshold
        ))
        cocycles = detect_cocycles(world)
        assert any(c.kind == "weak-authority" for c in cocycles)

    def test_trit_violation_cocycle(self) -> None:
        world = ClaimWorld()
        world.claims["c1"] = Claim("c1", "a", Trit.ONE, "h1", 0.5, "pluralistic")
        world.claims["c2"] = Claim("c2", "b", Trit.ONE, "h2", 0.5, "pluralistic")
        # 1 + 1 = 2 != 0 mod 3
        world.derivations.append(Derivation("d1", "c1", "c1", "direct", 0.5))
        world.derivations.append(Derivation("d2", "c2", "c2", "direct", 0.5))
        cocycles = detect_cocycles(world)
        assert any(c.kind == "trit-violation" for c in cocycles)

    def test_no_cocycles_balanced(self) -> None:
        world = ClaimWorld()
        world.claims["c1"] = Claim("c1", "test", Trit.ONE, "h", 0.5, "pluralistic")
        world.sources["s1"] = Source("s1", "study by MIT", Trit.TWO, "h", "academic")
        world.witnesses["w1"] = Witness("w1", "MIT", Trit.ZERO, "peer-reviewer", 0.9)
        world.derivations.append(Derivation("d1", "s1", "c1", "deductive", 0.85))
        cocycles = detect_cocycles(world)
        # Should have no unsupported, no trit-violation (1+2+0=3=0 mod 3),
        # and no weak-authority (kind is deductive, not appeal-to-authority)
        assert len(cocycles) == 0


# ---------------------------------------------------------------------------
# analyze_claim integration tests
# ---------------------------------------------------------------------------

class TestAnalyzeClaim:
    """Integration tests for analyze_claim."""

    def test_basic_analysis(self) -> None:
        text = "According to NASA, the Earth is warming."
        world = analyze_claim(text, "empirical")
        assert len(world.claims) == 1
        claim = list(world.claims.values())[0]
        assert claim.trit == Trit.ONE
        assert claim.framework == "empirical"
        assert 0.0 <= claim.confidence <= 1.0
        assert len(world.sources) >= 1

    def test_no_sources_low_confidence(self) -> None:
        text = "The sky is falling tomorrow."
        world = analyze_claim(text, "pluralistic")
        claim = list(world.claims.values())[0]
        assert claim.confidence == 0.1
        assert any(c.kind == "unsupported" for c in world.cocycles)

    def test_academic_empirical_boost(self) -> None:
        text = "A study by Harvard researchers shows this effect."
        world_emp = analyze_claim(text, "empirical")
        world_plur = analyze_claim(text, "pluralistic")
        claim_emp = list(world_emp.claims.values())[0]
        claim_plur = list(world_plur.claims.values())[0]
        # Empirical framework should boost academic sources
        assert claim_emp.confidence >= claim_plur.confidence

    def test_responsible_framework_boost(self) -> None:
        text = "According to WHO, this benefits the community greatly."
        world = analyze_claim(text, "responsible")
        claim = list(world.claims.values())[0]
        # Should get the responsible boost for "community" / "benefit"
        assert claim.confidence > 0.1

    def test_harmonic_multi_source(self) -> None:
        text = (
            "According to NASA, warming is real. "
            "A study by MIT confirms this. "
            "Published in Nature, the data is clear."
        )
        world = analyze_claim(text, "harmonic")
        claim = list(world.claims.values())[0]
        assert claim.confidence > 0.1
        assert len(world.sources) >= 3

    def test_gf3_conservation_in_analysis(self) -> None:
        """Each source gets one claim (1) + one source (2) + one witness (0).
        Per triple: 1+2+0 = 3 = 0 mod 3.
        With N sources: claim(1) + N*source(2) + N*witness(0) = 1 + 2N.
        Balanced when (1 + 2N) mod 3 == 0, i.e. N = 1, 4, 7, ...
        """
        text = "According to NASA, it is confirmed."
        world = analyze_claim(text, "pluralistic")
        balanced, counts = world.gf3_balance()
        # 1 claim (trit=1) + 1 source (trit=2) + 1 witness (trit=0) = 3 = 0 mod 3
        assert balanced is True

    def test_world_serialization(self) -> None:
        text = "According to NOAA, the data is clear."
        world = analyze_claim(text, "pluralistic")
        d = world.to_dict()
        assert "claims" in d
        assert "sources" in d
        assert "witnesses" in d
        assert "derivations" in d
        assert "cocycles" in d
        # Should be JSON-serializable
        json.dumps(d)

    def test_content_hash_deterministic(self) -> None:
        h1 = content_hash("Hello World")
        h2 = content_hash("  Hello World  ")
        h3 = content_hash("hello world")
        assert h1 == h2 == h3


# ---------------------------------------------------------------------------
# validate_sources tests
# ---------------------------------------------------------------------------

class TestValidateSources:
    """Test the validate_sources function."""

    def test_basic_validation(self) -> None:
        text = "According to CDC, masks work. A study by Oxford confirms this."
        result = validate_sources(text, "pluralistic")
        assert result["total_sources"] >= 2
        assert result["framework"] == "pluralistic"

    def test_empirical_boost_label(self) -> None:
        text = "A study by Harvard shows the effect."
        result = validate_sources(text, "empirical")
        assert "empirical" in result["framework_boost"]

    def test_no_sources_empty(self) -> None:
        text = "The weather is nice today."
        result = validate_sources(text, "pluralistic")
        assert result["total_sources"] == 0
        assert result["sources"] == []

    def test_gf3_balance_field(self) -> None:
        text = "According to WHO, this is true."
        result = validate_sources(text, "pluralistic")
        # Result includes gf3 balance info
        assert "gf3_balanced" in result
        assert "gf3_sum_mod3" in result


# ---------------------------------------------------------------------------
# check_manipulation tests
# ---------------------------------------------------------------------------

class TestCheckManipulation:
    """Test manipulation pattern detection."""

    def test_clean_text(self) -> None:
        text = "The research shows moderate results with some limitations."
        result = check_manipulation(text)
        assert result["total_patterns"] == 0
        assert result["verdict"] == "clean"

    def test_fear_pattern(self) -> None:
        text = "This catastrophic event will cause panic and dread."
        result = check_manipulation(text)
        assert any(p["kind"] == "emotional_fear" for p in result["patterns"])
        assert result["total_patterns"] >= 1
        assert result["verdict"] != "clean"

    def test_urgency_pattern(self) -> None:
        text = "Act now before it's too late! Limited time only!"
        result = check_manipulation(text)
        kinds = {p["kind"] for p in result["patterns"]}
        assert "urgency" in kinds

    def test_false_consensus(self) -> None:
        text = "Everyone knows this is true and it's widely accepted."
        result = check_manipulation(text)
        kinds = {p["kind"] for p in result["patterns"]}
        assert "false_consensus" in kinds

    def test_appeal_authority_pattern(self) -> None:
        text = "Experts say this is definitely correct. Studies show it works."
        result = check_manipulation(text)
        kinds = {p["kind"] for p in result["patterns"]}
        assert "appeal_authority" in kinds

    def test_loaded_language(self) -> None:
        text = "Obviously this is correct and clearly the best option, undeniably."
        result = check_manipulation(text)
        kinds = {p["kind"] for p in result["patterns"]}
        assert "loaded_language" in kinds

    def test_ad_hominem(self) -> None:
        text = "Only a fool would think otherwise."
        result = check_manipulation(text)
        kinds = {p["kind"] for p in result["patterns"]}
        assert "ad_hominem" in kinds

    def test_multiple_patterns_severity(self) -> None:
        text = (
            "Act now! Everyone knows this is true. "
            "This catastrophic fear is real. "
            "Obviously, experts say so. "
            "Don't miss out on this exclusive deal!"
        )
        result = check_manipulation(text)
        assert result["total_patterns"] >= 4
        assert result["total_severity"] > 0
        assert result["manipulation_score"] > 0
        assert result["verdict"] == "likely_manipulative"

    def test_social_pressure(self) -> None:
        text = "Don't miss out on what everyone else is doing!"
        result = check_manipulation(text)
        kinds = {p["kind"] for p in result["patterns"]}
        assert "social_pressure" in kinds

    def test_artificial_scarcity(self) -> None:
        text = "Only 5 left in stock! This is a rare opportunity."
        result = check_manipulation(text)
        kinds = {p["kind"] for p in result["patterns"]}
        assert "artificial_scarcity" in kinds


# ---------------------------------------------------------------------------
# MCP tool invocation tests (via server.call_tool)
# ---------------------------------------------------------------------------

class TestMCPTools:
    """Test the MCP tool invocations end-to-end via request_handlers."""

    @pytest.fixture
    def server(self):
        return create_server()

    @pytest.mark.asyncio
    async def test_analyze_claim_tool(self, server) -> None:
        handler = server.request_handlers[CallToolRequest]
        result = await handler(CallToolRequest(
            method="tools/call",
            params=CallToolRequestParams(
                name="analyze_claim",
                arguments={
                    "text": "According to IPCC, temperatures are rising.",
                    "framework": "empirical",
                },
            ),
        ))
        content = result.root.content
        assert len(content) == 1
        data = json.loads(content[0].text)
        assert "world" in data
        assert "gf3_balanced" in data
        assert "h1_dimension" in data
        assert "confidence" in data

    @pytest.mark.asyncio
    async def test_validate_sources_tool(self, server) -> None:
        handler = server.request_handlers[CallToolRequest]
        result = await handler(CallToolRequest(
            method="tools/call",
            params=CallToolRequestParams(
                name="validate_sources",
                arguments={
                    "text": "Study by Stanford researchers shows results.",
                },
            ),
        ))
        content = result.root.content
        assert len(content) == 1
        data = json.loads(content[0].text)
        assert "sources" in data
        assert data["total_sources"] >= 1

    @pytest.mark.asyncio
    async def test_check_manipulation_tool(self, server) -> None:
        handler = server.request_handlers[CallToolRequest]
        result = await handler(CallToolRequest(
            method="tools/call",
            params=CallToolRequestParams(
                name="check_manipulation",
                arguments={
                    "text": "Act now! Everyone knows this is clearly true!",
                },
            ),
        ))
        content = result.root.content
        assert len(content) == 1
        data = json.loads(content[0].text)
        assert data["total_patterns"] >= 1
        assert data["verdict"] != "clean"

    @pytest.mark.asyncio
    async def test_list_tools(self, server) -> None:
        handler = server.request_handlers[ListToolsRequest]
        result = await handler(ListToolsRequest(method="tools/list"))
        tool_names = {t.name for t in result.root.tools}
        assert tool_names == {"analyze_claim", "validate_sources", "check_manipulation"}

    @pytest.mark.asyncio
    async def test_unknown_tool(self, server) -> None:
        handler = server.request_handlers[CallToolRequest]
        result = await handler(CallToolRequest(
            method="tools/call",
            params=CallToolRequestParams(
                name="nonexistent_tool",
                arguments={"text": "test"},
            ),
        ))
        content = result.root.content
        data = json.loads(content[0].text)
        assert "error" in data


# ---------------------------------------------------------------------------
# Sheaf consistency integration
# ---------------------------------------------------------------------------

class TestSheafConsistency:
    """Test H^1 cohomology / sheaf consistency."""

    def test_h1_zero_means_consistent(self) -> None:
        world = ClaimWorld()
        h1, cocycles = world.sheaf_consistency()
        assert h1 == 0
        assert cocycles == []

    def test_h1_positive_means_contradictions(self) -> None:
        world = ClaimWorld()
        world.cocycles = [
            Cocycle("c1", "c2", "contradiction", 0.8),
            Cocycle("c1", "", "unsupported", 0.9),
        ]
        h1, cocycles = world.sheaf_consistency()
        assert h1 == 2
        assert len(cocycles) == 2

    def test_cocycle_penalty_reduces_confidence(self) -> None:
        """More cocycles should reduce confidence."""
        text = "According to WHO, this is true."
        world_clean = analyze_claim(text, "pluralistic")

        # Manually add extra cocycles to see penalty
        world_dirty = analyze_claim(text, "pluralistic")
        world_dirty.cocycles.append(Cocycle("x", "y", "contradiction", 0.8))
        world_dirty.cocycles.append(Cocycle("x", "z", "circular", 0.7))

        # Recompute confidence with extra cocycles
        claim = list(world_dirty.claims.values())[0]
        claim.confidence = compute_confidence(world_dirty, claim, "pluralistic")

        clean_conf = list(world_clean.claims.values())[0].confidence
        dirty_conf = claim.confidence
        assert dirty_conf < clean_conf
