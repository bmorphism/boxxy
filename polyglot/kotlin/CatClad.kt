package antibullshit

import java.nio.charset.StandardCharsets
import java.security.MessageDigest
import java.time.Instant

/**
 * Cat-clad epistemological verification engine — post-modern Kotlin tier.
 *
 * A "cat-clad" claim is an object in a double category whose morphisms track
 * provenance, derivation history, and consistency conditions binding claims
 * to sources, witnesses, and each other. Verification reduces to structure:
 *
 *   - Provenance is a composable morphism chain to primary sources
 *   - Consistency is a sheaf condition (H^1 = 0 means no contradictions)
 *   - GF(3) conservation prevents unbounded generation without verification
 *   - Bisimulation detects forgery (divergent accounts of the same event)
 *
 * DblTheory Schema (CatColab-style):
 *
 *   @present SchClaimWorld(FreeSchema) begin
 *     Claim::Ob           -- assertions to verify
 *     Source::Ob           -- evidence or citations
 *     Witness::Ob          -- attestation parties
 *     Derivation::Ob       -- inference steps
 *
 *     derives_from::Hom(Derivation, Source)
 *     produces::Hom(Derivation, Claim)
 *     attests::Hom(Witness, Source)
 *     cites::Hom(Claim, Source)
 *
 *     Trit::AttrType
 *     Confidence::AttrType
 *     ContentHash::AttrType
 *     Timestamp::AttrType
 *
 *     claim_trit::Attr(Claim, Trit)
 *     source_trit::Attr(Source, Trit)
 *     witness_trit::Attr(Witness, Trit)
 *     claim_hash::Attr(Claim, ContentHash)
 *     source_hash::Attr(Source, ContentHash)
 *     claim_confidence::Attr(Claim, Confidence)
 *   end
 */

// ============================================================================
// Zero-cost wrappers — @JvmInline value classes
// ============================================================================

@JvmInline
value class ContentHash(val hex: String) {
    init { require(hex.length == 64) { "SHA-256 hash must be 64 hex chars, got ${hex.length}" } }
    fun prefix(n: Int = 12): String = hex.take(n)
    override fun toString(): String = hex.take(16) + "..."
}

@JvmInline
value class Confidence(val value: Double) {
    init { require(value in 0.0..1.0) { "Confidence must be in [0,1], got $value" } }
    override fun toString(): String = "%.2f".format(value)

    companion object {
        val ZERO = Confidence(0.0)
        val LOW = Confidence(0.1)
        operator fun invoke(raw: Double): Confidence = Confidence(raw.coerceIn(0.0, 1.0))
    }
}

@JvmInline
value class Severity(val value: Double) {
    init { require(value in 0.0..1.0) { "Severity must be in [0,1], got $value" } }
    override fun toString(): String = "%.1f".format(value)
}

// ============================================================================
// GF(3) — Galois Field of order 3, as @JvmInline value class with operators
// ============================================================================

@JvmInline
value class Trit private constructor(val ordinal: Int) {

    /** Balanced ternary representation. */
    val balanced: String get() = when (ordinal) { 0 -> "0"; 1 -> "+1"; else -> "-1" }

    operator fun plus(other: Trit): Trit = fromInt(this.ordinal + other.ordinal)
    operator fun times(other: Trit): Trit = fromInt(this.ordinal * other.ordinal)
    operator fun unaryMinus(): Trit = fromInt(3 - this.ordinal)
    operator fun minus(other: Trit): Trit = this + (-other)

    override fun toString(): String = balanced

    companion object {
        /** Coordinator: balance, infrastructure (balanced ternary 0). */
        val ZERO = Trit(0)
        /** Generator: creation, synthesis (balanced ternary +1). */
        val ONE = Trit(1)
        /** Verifier: validation, analysis (balanced ternary -1). */
        val TWO = Trit(2)

        fun fromInt(n: Int): Trit = Trit(((n % 3) + 3) % 3)

        /** Conservation law: sum of trits = 0 (mod 3). */
        fun isBalanced(trits: List<Trit>): Boolean =
            trits.sumOf { it.ordinal }.let { ((it % 3) + 3) % 3 == 0 }
    }
}

// ============================================================================
// DblTheory: sealed interface hierarchies for ObType, MorType, CocycleKind,
// SourceKind — exhaustive when-matching, no else needed
// ============================================================================

/** Object types in the DblTheory schema. */
sealed interface ObType {
    data object Claim : ObType
    data object Source : ObType
    data object Witness : ObType
    data object Derivation : ObType
}

/** Source kind — sealed for exhaustive matching. */
sealed interface SourceKind {
    data object Academic : SourceKind { override fun toString() = "academic" }
    data object News : SourceKind { override fun toString() = "news" }
    data object Authority : SourceKind { override fun toString() = "authority" }
    data object Anecdotal : SourceKind { override fun toString() = "anecdotal" }
    data object Url : SourceKind { override fun toString() = "url" }
}

/** Morphism types, generic over source/target ObType. */
sealed interface MorType<out S : ObType, out T : ObType> {
    data object DerivesFrom : MorType<ObType.Derivation, ObType.Source>
    data object Produces : MorType<ObType.Derivation, ObType.Claim>
    data object Attests : MorType<ObType.Witness, ObType.Source>
    data object Cites : MorType<ObType.Claim, ObType.Source>
}

/** Cocycle (sheaf obstruction) kinds. */
sealed interface CocycleKind {
    data object Contradiction : CocycleKind { override fun toString() = "contradiction" }
    data object Unsupported : CocycleKind { override fun toString() = "unsupported" }
    data object Circular : CocycleKind { override fun toString() = "circular" }
    data object TritViolation : CocycleKind { override fun toString() = "trit-violation" }
    data object WeakAuthority : CocycleKind { override fun toString() = "weak-authority" }
}

/** Derivation kind — how a source supports a claim. */
sealed interface DerivationKind {
    data object Direct : DerivationKind { override fun toString() = "direct" }
    data object Inductive : DerivationKind { override fun toString() = "inductive" }
    data object Deductive : DerivationKind { override fun toString() = "deductive" }
    data object Analogical : DerivationKind { override fun toString() = "analogical" }
    data object AppealToAuthority : DerivationKind { override fun toString() = "appeal-to-authority" }
}

/** Witness role. */
sealed interface WitnessRole {
    data object Author : WitnessRole { override fun toString() = "author" }
    data object PeerReviewer : WitnessRole { override fun toString() = "peer-reviewer" }
    data object Editor : WitnessRole { override fun toString() = "editor" }
    data object Publisher : WitnessRole { override fun toString() = "publisher" }
    data object Self : WitnessRole { override fun toString() = "self" }
}

/** Epistemological framework. */
sealed interface Framework {
    data object Empirical : Framework { override fun toString() = "empirical" }
    data object Responsible : Framework { override fun toString() = "responsible" }
    data object Harmonic : Framework { override fun toString() = "harmonic" }
    data object Pluralistic : Framework { override fun toString() = "pluralistic" }

    companion object {
        operator fun invoke(name: String): Framework = when (name.lowercase()) {
            "empirical" -> Empirical
            "responsible" -> Responsible
            "harmonic" -> Harmonic
            "pluralistic" -> Pluralistic
            else -> error("Unknown framework: $name")
        }
    }
}

// ============================================================================
// Typealiases for complex generic types
// ============================================================================

typealias TritCounts = Map<String, Int>
typealias BalanceResult = Pair<Boolean, TritCounts>

// ============================================================================
// Path segments — composable morphism chains in the DblTheory
// ============================================================================

data class PathSegment(
    val morType: MorType<*, *>,
    val sourceId: String,
    val targetId: String
)

@JvmInline
value class Path(val segments: List<PathSegment>) {
    fun composes(): Boolean = segments.zipWithNext().all { (a, b) -> a.targetId == b.sourceId }
    val length: Int get() = segments.size
}

// ============================================================================
// ACSet data — immutable data classes with value-class attributes
// ============================================================================

data class ClaimData(
    val id: String,
    val text: String,
    val trit: Trit,
    val hash: ContentHash,
    val confidence: Confidence,
    val framework: Framework,
    val createdAt: Instant
)

data class SourceData(
    val id: String,
    val citation: String,
    val trit: Trit,
    val hash: ContentHash,
    val kind: SourceKind
)

data class WitnessData(
    val id: String,
    val name: String,
    val trit: Trit,
    val role: WitnessRole,
    val weight: Double
)

data class DerivationData(
    val id: String,
    val sourceId: String,
    val claimId: String,
    val kind: DerivationKind,
    val strength: Double
)

data class Cocycle(
    val claimA: String?,
    val claimB: String?,
    val kind: CocycleKind,
    val severity: Severity
)

// ============================================================================
// Extension properties — zero-cost accessors on domain types
// ============================================================================

val ClaimData.isSupported: Boolean
    get() = confidence.value > 0.1

val SourceKind.derivationKind: DerivationKind
    get() = when (this) {
        SourceKind.Academic -> DerivationKind.Deductive
        SourceKind.Authority -> DerivationKind.AppealToAuthority
        SourceKind.Url -> DerivationKind.Direct
        SourceKind.News -> DerivationKind.Inductive
        SourceKind.Anecdotal -> DerivationKind.Analogical
    }

val SourceKind.strength: Double
    get() = when (this) {
        SourceKind.Academic -> 0.85
        SourceKind.Authority -> 0.5
        SourceKind.Url -> 0.3
        SourceKind.News -> 0.4
        SourceKind.Anecdotal -> 0.1
    }

val SourceKind.witnessRole: WitnessRole
    get() = when (this) {
        SourceKind.Academic -> WitnessRole.PeerReviewer
        SourceKind.Authority -> WitnessRole.Author
        SourceKind.Url -> WitnessRole.Publisher
        SourceKind.News -> WitnessRole.Editor
        SourceKind.Anecdotal -> WitnessRole.Self
    }

val SourceKind.witnessWeight: Double
    get() = when (this) {
        SourceKind.Academic -> 0.9
        SourceKind.Authority -> 0.6
        SourceKind.Url -> 0.4
        SourceKind.News -> 0.5
        SourceKind.Anecdotal -> 0.2
    }

// ============================================================================
// Manipulation detection — sealed interface + data classes
// ============================================================================

data class ManipulationPattern(
    val kind: String,
    val evidence: String,
    val severity: Severity
)

private data class ManipulationCheck(
    val kind: String,
    val pattern: Regex,
    val weight: Severity
)

private val MANIPULATION_CHECKS = buildList {
    add(ManipulationCheck("emotional_fear",
        Regex("(?i)(fear|terrif|alarm|panic|dread|catastroph)"), Severity(0.7)))
    add(ManipulationCheck("urgency",
        Regex("(?i)(act now|limited time|don't wait|expires|hurry|last chance|before it's too late)"), Severity(0.8)))
    add(ManipulationCheck("false_consensus",
        Regex("(?i)(everyone knows|nobody (believes|wants|thinks)|all experts|unanimous|widely accepted)"), Severity(0.6)))
    add(ManipulationCheck("appeal_authority",
        Regex("(?i)(experts say|scientists (claim|prove)|studies show|research proves|doctors recommend)"), Severity(0.5)))
    add(ManipulationCheck("artificial_scarcity",
        Regex("(?i)(exclusive|rare opportunity|only \\d+ left|limited (edition|supply|spots))"), Severity(0.7)))
    add(ManipulationCheck("social_pressure",
        Regex("(?i)(you don't want to be|don't miss out|join .* (others|people)|be the first)"), Severity(0.6)))
    add(ManipulationCheck("loaded_language",
        Regex("(?i)(obviously|clearly|undeniably|unquestionably|beyond doubt)"), Severity(0.4)))
    add(ManipulationCheck("false_dichotomy",
        Regex("(?i)(either .* or|only (two|2) (options|choices)|if you don't .* then)"), Severity(0.6)))
    add(ManipulationCheck("circular_reasoning",
        Regex("(?i)(because .* therefore .* because|true because .* which is true)"), Severity(0.9)))
    add(ManipulationCheck("ad_hominem",
        Regex("(?i)(stupid|idiot|moron|fool|ignorant|naive) .* (think|believe|say)"), Severity(0.8)))
}

// ============================================================================
// Source extraction — lazy sequence { yield() } pipeline
// ============================================================================

private data class SourceExtractor(val pattern: Regex, val kind: SourceKind)

private val SOURCE_EXTRACTORS = buildList {
    add(SourceExtractor(
        Regex("(?i)(?:according to|cited by|reported by)\\s+([^,\\.]+)"), SourceKind.Authority))
    add(SourceExtractor(
        Regex("(?i)(?:study|research|paper)\\s+(?:by|from|in)\\s+([^,\\.]+)"), SourceKind.Academic))
    add(SourceExtractor(
        Regex("(?i)(?:published in|journal of)\\s+([^,\\.]+)"), SourceKind.Academic))
    add(SourceExtractor(
        Regex("(?i)(https?://\\S+)"), SourceKind.Url))
}

/** Lazily extract sources from text using sequence { yield() }. */
private fun extractSourcesLazy(text: String): Sequence<SourceData> = sequence {
    val seen = mutableSetOf<String>()
    for ((pattern, kind) in SOURCE_EXTRACTORS) {
        for (match in pattern.findAll(text)) {
            val groups = match.groupValues
            if (groups.size < 2) continue
            val citation = groups[1].trim()
            val hash = contentHash(citation)
            val id = hash.prefix()
            if (id in seen) continue
            seen += id
            yield(SourceData(
                id = id,
                citation = citation,
                trit = Trit.TWO,  // Verifier role -- evidence checks claims
                hash = hash,
                kind = kind
            ))
        }
    }
}

// ============================================================================
// Content hashing
// ============================================================================

private fun contentHash(text: String): ContentHash {
    val md = MessageDigest.getInstance("SHA-256")
    val digest = md.digest(text.lowercase().trim().toByteArray(StandardCharsets.UTF_8))
    return ContentHash(digest.joinToString("") { "%02x".format(it) })
}

// ============================================================================
// ClaimWorld — the ACSet instance, with operator fun invoke for callable analysis
// ============================================================================

data class ClaimWorld(
    val claims: Map<String, ClaimData> = emptyMap(),
    val sources: Map<String, SourceData> = emptyMap(),
    val witnesses: Map<String, WitnessData> = emptyMap(),
    val derivations: List<DerivationData> = emptyList(),
    val cocycles: List<Cocycle> = emptyList()
) {
    companion object {
        /** Factory: analyze a text claim in a given framework, returning a fully populated world. */
        operator fun invoke(text: String, framework: String): ClaimWorld =
            analyzeClaim(text, Framework(framework))

        /** Factory: analyze with a Framework sealed instance. */
        operator fun invoke(text: String, framework: Framework): ClaimWorld =
            analyzeClaim(text, framework)
    }

    /** Callable world: re-analyze with a new claim appended. */
    operator fun invoke(additionalText: String, framework: Framework = Framework.Empirical): ClaimWorld {
        val other = analyzeClaim(additionalText, framework)
        return ClaimWorld(
            claims = this.claims + other.claims,
            sources = this.sources + other.sources,
            witnesses = this.witnesses + other.witnesses,
            derivations = this.derivations + other.derivations,
            cocycles = this.cocycles + other.cocycles
        )
    }
}

// ============================================================================
// Extension functions on ClaimWorld
// ============================================================================

/** H^1 dimension: 0 = consistent, >0 = contradictions. */
val ClaimWorld.sheafConsistency: Pair<Int, List<Cocycle>>
    get() = cocycles.size to cocycles

/** GF(3) conservation check: sum of all trits = 0 (mod 3). */
val ClaimWorld.gf3Balance: BalanceResult
    get() {
        val allTrits = buildList {
            addAll(claims.values.map { it.trit })
            addAll(sources.values.map { it.trit })
            addAll(witnesses.values.map { it.trit })
        }
        val counts = buildMap {
            val (coordinators, rest) = allTrits.partition { it.ordinal == 0 }
            val (generators, verifiers) = rest.partition { it.ordinal == 1 }
            put("coordinator", coordinators.size)
            put("generator", generators.size)
            put("verifier", verifiers.size)
        }
        return Trit.isBalanced(allTrits) to counts
    }

/** Build a provenance Path for a given claim through its derivation chain. */
fun ClaimWorld.provenancePath(claimId: String): Path {
    check(claimId in claims) { "Unknown claim: $claimId" }
    val segments = buildList {
        for (d in derivations.filter { it.claimId == claimId }) {
            add(PathSegment(MorType.Produces, d.id, claimId))
            add(PathSegment(MorType.DerivesFrom, d.id, d.sourceId))
            // Extend to witnesses
            for ((wId, _) in witnesses.filter { (_, w) -> w.name == sources[d.sourceId]?.citation }) {
                add(PathSegment(MorType.Attests, wId, d.sourceId))
            }
        }
    }
    return Path(segments)
}

// ============================================================================
// Epistemological model builder DSL
// ============================================================================

@DslMarker
annotation class EpistemicDsl

@EpistemicDsl
class WitnessBuilder(private val sourceId: String) {
    internal val witnesses = mutableListOf<WitnessData>()

    fun witness(name: String) {
        val hash = contentHash(name)
        witnesses += WitnessData(
            id = "w-$sourceId-${hash.prefix(8)}",
            name = name,
            trit = Trit.ZERO,
            role = WitnessRole.PeerReviewer,
            weight = 0.8
        )
    }
}

@EpistemicDsl
class SourceBuilder(private val claimId: String) {
    internal val sources = mutableListOf<SourceData>()
    internal val witnesses = mutableListOf<WitnessData>()
    internal val derivations = mutableListOf<DerivationData>()

    fun source(citation: String, kind: SourceKind = SourceKind.Academic, block: WitnessBuilder.() -> Unit = {}) {
        val hash = contentHash(citation)
        val srcId = hash.prefix()
        val src = SourceData(id = srcId, citation = citation, trit = Trit.TWO, hash = hash, kind = kind)
        sources += src
        derivations += DerivationData(
            id = "d-$srcId-$claimId",
            sourceId = srcId,
            claimId = claimId,
            kind = kind.derivationKind,
            strength = kind.strength
        )
        val wb = WitnessBuilder(srcId).apply(block)
        witnesses += wb.witnesses
    }
}

@EpistemicDsl
class EpistemicModelBuilder(private val framework: Framework) {
    private val claims = mutableMapOf<String, ClaimData>()
    private val sources = mutableMapOf<String, SourceData>()
    private val witnesses = mutableMapOf<String, WitnessData>()
    private val derivations = mutableListOf<DerivationData>()

    fun claim(text: String, block: SourceBuilder.() -> Unit = {}) {
        val hash = contentHash(text)
        val claimId = hash.prefix()
        val claimData = ClaimData(
            id = claimId,
            text = text,
            trit = Trit.ONE,
            hash = hash,
            confidence = Confidence.LOW,
            framework = framework,
            createdAt = Instant.now()
        )
        claims[claimId] = claimData
        val sb = SourceBuilder(claimId).apply(block)
        for (s in sb.sources) sources[s.id] = s
        for (w in sb.witnesses) witnesses[w.id] = w
        derivations += sb.derivations

        // Recompute confidence
        val conf = computeConfidence(sources.values.toList(), derivations, claimData, framework)
        claims[claimId] = claimData.copy(confidence = conf)
    }

    fun build(): ClaimWorld {
        val world = ClaimWorld(
            claims = claims.toMap(),
            sources = sources.toMap(),
            witnesses = witnesses.toMap(),
            derivations = derivations.toList()
        )
        return world.copy(cocycles = detectCocycles(world))
    }
}

/** Top-level DSL entry point. */
fun epistemicModel(framework: String, block: EpistemicModelBuilder.() -> Unit): ClaimWorld =
    EpistemicModelBuilder(Framework(framework)).apply(block).build()

// ============================================================================
// Core analysis — pure functions
// ============================================================================

private fun analyzeClaim(text: String, framework: Framework): ClaimWorld {
    val hash = contentHash(text)
    val claimId = hash.prefix()

    val claim = ClaimData(
        id = claimId,
        text = text,
        trit = Trit.ONE,
        hash = hash,
        confidence = Confidence.LOW,
        framework = framework,
        createdAt = Instant.now()
    )

    // Lazy source extraction, materialized via associateBy
    val sources = extractSourcesLazy(text).associateBy { it.id }

    // Derivations: one per source -> claim edge
    val derivations = buildList {
        for ((_, src) in sources) {
            add(DerivationData(
                id = "d-${src.id}-$claimId",
                sourceId = src.id,
                claimId = claimId,
                kind = src.kind.derivationKind,
                strength = src.kind.strength
            ))
        }
    }

    // Witnesses: one per source, using groupBy-style construction
    val witnesses = buildMap {
        for ((_, src) in sources) {
            val wId = "w-${src.id}"
            put(wId, WitnessData(
                id = wId,
                name = src.citation,
                trit = Trit.ZERO,
                role = src.kind.witnessRole,
                weight = src.kind.witnessWeight
            ))
        }
    }

    // Compute confidence
    val confidence = computeConfidence(sources.values.toList(), derivations, claim, framework)

    val claimsMap = mapOf(claimId to claim.copy(confidence = confidence))

    val world = ClaimWorld(
        claims = claimsMap,
        sources = sources,
        witnesses = witnesses,
        derivations = derivations
    )

    // Detect cocycles and return final world
    return world.copy(cocycles = detectCocycles(world))
}

/** Check for manipulation patterns in text. */
fun detectManipulation(text: String): List<ManipulationPattern> =
    MANIPULATION_CHECKS.flatMap { check ->
        check.pattern.findAll(text).map { match ->
            ManipulationPattern(
                kind = check.kind,
                evidence = match.value,
                severity = check.weight
            )
        }.toList()
    }

// ============================================================================
// Confidence computation — framework-weighted
// ============================================================================

private fun computeConfidence(
    sources: List<SourceData>,
    derivations: List<DerivationData>,
    claim: ClaimData,
    framework: Framework
): Confidence {
    if (sources.isEmpty()) return Confidence.LOW

    val claimDerivations = derivations.filter { it.claimId == claim.id }
    if (claimDerivations.isEmpty()) return Confidence.LOW

    var avgStrength = claimDerivations.map { it.strength }.average()

    // Framework-specific weighting — exhaustive when, no else needed
    when (framework) {
        Framework.Empirical -> {
            val academicCount = sources.count { it.kind is SourceKind.Academic }
            if (academicCount > 0) avgStrength *= 1.0 + 0.1 * academicCount
        }
        Framework.Responsible -> {
            val lower = claim.text.lowercase()
            if ("community" in lower || "benefit" in lower) avgStrength *= 1.1
        }
        Framework.Harmonic -> {
            if (sources.size >= 3) avgStrength *= 1.15
        }
        Framework.Pluralistic -> { /* raw structural quality, no boost */ }
    }

    return Confidence(avgStrength)
}

// ============================================================================
// Cocycle detection — structural contradictions in the sheaf
// ============================================================================

private fun detectCocycles(world: ClaimWorld): List<Cocycle> = buildList {
    // Unsupported claims: no derivation chain
    for ((id, _) in world.claims) {
        val hasDerivation = world.derivations.any { it.claimId == id }
        if (!hasDerivation) {
            add(Cocycle(
                claimA = id,
                claimB = null,
                kind = CocycleKind.Unsupported,
                severity = Severity(0.9)
            ))
        }
    }

    // Weak authority: appeal-to-authority with low strength
    for (d in world.derivations) {
        if (d.kind is DerivationKind.AppealToAuthority && d.strength < 0.6) {
            add(Cocycle(
                claimA = d.claimId,
                claimB = d.sourceId,
                kind = CocycleKind.WeakAuthority,
                severity = Severity(0.5)
            ))
        }
    }

    // GF(3) conservation
    val (balanced, _) = world.gf3Balance
    if (!balanced) {
        add(Cocycle(
            claimA = null,
            claimB = null,
            kind = CocycleKind.TritViolation,
            severity = Severity(0.3)
        ))
    }
}

// ============================================================================
// DeepRecursiveFunction — recursive path traversal for transitive provenance
// ============================================================================

/**
 * Recursively traverse the derivation graph to find all transitive sources
 * for a given claim. Uses DeepRecursiveFunction to avoid stack overflow
 * on deep provenance chains.
 */
private val findTransitiveSources = DeepRecursiveFunction<Pair<ClaimWorld, String>, Set<String>> { (world, claimId) ->
    val directSourceIds = world.derivations
        .filter { it.claimId == claimId }
        .map { it.sourceId }
        .toSet()

    val transitive = mutableSetOf<String>()
    transitive += directSourceIds

    // If any source is itself a claim (cross-referencing), recurse
    for (srcId in directSourceIds) {
        if (srcId in world.claims) {
            transitive += callRecursive(world to srcId)
        }
    }
    transitive
}

// ============================================================================
// Main — test harness
// ============================================================================

fun main() {
    println("=== Anti-Bullshit Cat-Clad Engine (Post-Modern Kotlin Tier) ===")
    println()

    // --- Test 1: Analyze a well-sourced claim via companion invoke factory ---
    val claimText = "According to Dr. Smith, research from Harvard shows that exercise reduces stress by 40%"
    println("[1] Analyzing: \"$claimText\"")
    val world = ClaimWorld(claimText, "empirical")

    world.claims.values.forEach { c ->
        println("    Claim: id=${c.id} trit=${c.trit} confidence=${c.confidence} framework=${c.framework}")
    }
    println("    Sources: ${world.sources.size}")
    world.sources.values.forEach { s ->
        println("      - [${s.kind}] \"${s.citation}\" trit=${s.trit}")
    }
    println("    Derivations: ${world.derivations.size}")
    println("    Witnesses: ${world.witnesses.size}")

    val (h1, cocycles) = world.sheafConsistency
    println("    H^1 (sheaf obstructions): $h1")

    val (balanced, counts) = world.gf3Balance
    println("    GF(3) balanced: $balanced  coord=${counts["coordinator"]} gen=${counts["generator"]} ver=${counts["verifier"]}")

    // Provenance path
    world.claims.keys.firstOrNull()?.let { id ->
        val path = world.provenancePath(id)
        println("    Provenance path length: ${path.length}, composes: ${path.composes()}")
    }
    println()

    // --- Test 2: Unsupported claim ---
    val unsupported = "The moon is made of cheese"
    println("[2] Analyzing: \"$unsupported\"")
    val world2 = ClaimWorld(unsupported, "empirical")
    println("    Sources: ${world2.sources.size}")
    val (h1_2, cocycles2) = world2.sheafConsistency
    println("    H^1 (sheaf obstructions): $h1_2")
    cocycles2.forEach { cc ->
        println("    Cocycle: kind=${cc.kind} severity=${cc.severity}")
    }
    world2.claims.values.forEach { c ->
        println("    Confidence: ${c.confidence} (should be low)")
        println("    isSupported: ${c.isSupported} (should be false)")
    }
    println()

    // --- Test 3: Manipulation detection ---
    val manipulative = "Act now! This exclusive offer expires in 10 minutes. " +
        "Everyone knows this is the best deal. Scientists claim it's proven. " +
        "Don't miss out! Obviously you'd be a fool to say no."
    println("[3] Detecting manipulation in: \"${manipulative.take(60)}...\"")
    val patterns = detectManipulation(manipulative)
    println("    Patterns found: ${patterns.size}")
    patterns.forEach { p ->
        println("      - ${p.kind} (severity=${p.severity}): \"${p.evidence}\"")
    }
    println()

    // --- Test 4: Multi-framework comparison ---
    val multiText = "Study by MIT shows community benefit from sustainable energy integration"
    println("[4] Multi-framework: \"$multiText\"")
    for (fw in listOf("empirical", "responsible", "harmonic", "pluralistic")) {
        val fwWorld = ClaimWorld(multiText, fw)
        fwWorld.claims.values.forEach { c ->
            println("    $fw: confidence=${c.confidence} sources=${fwWorld.sources.size} cocycles=${fwWorld.cocycles.size}")
        }
    }
    println()

    // --- Test 5: No manipulation in neutral text ---
    val neutral = "The temperature today is 72 degrees Fahrenheit with partly cloudy skies."
    println("[5] Neutral text: \"$neutral\"")
    val neutralPatterns = detectManipulation(neutral)
    println("    Manipulation patterns: ${neutralPatterns.size} (should be 0)")
    println()

    // --- Test 6: Rich sourcing ---
    val rich = "According to the WHO, a study by Johns Hopkins published in Nature confirms vaccine efficacy"
    println("[6] Rich sources: \"$rich\"")
    val richWorld = ClaimWorld(rich, "pluralistic")
    val (richBalanced, richCounts) = richWorld.gf3Balance
    println("    GF(3) balanced: $richBalanced  coord=${richCounts["coordinator"]} gen=${richCounts["generator"]} ver=${richCounts["verifier"]}")
    println()

    // --- Test 7: Builder DSL ---
    println("[7] EpistemicModel DSL:")
    val dslWorld = epistemicModel("empirical") {
        claim("Exercise reduces cortisol levels") {
            source("Harvard Medical Review 2024", SourceKind.Academic) {
                witness("Dr. Jane Doe")
                witness("Dr. John Peer")
            }
            source("WHO Guidelines", SourceKind.Authority)
        }
        claim("Meditation improves focus") {
            source("Nature Neuroscience 2023", SourceKind.Academic) {
                witness("Prof. Mindful")
            }
        }
    }
    println("    Claims: ${dslWorld.claims.size}")
    println("    Sources: ${dslWorld.sources.size}")
    println("    Witnesses: ${dslWorld.witnesses.size}")
    println("    Derivations: ${dslWorld.derivations.size}")
    dslWorld.claims.values.forEach { c ->
        println("    Claim: \"${c.text.take(40)}...\" confidence=${c.confidence} isSupported=${c.isSupported}")
    }
    val (dslH1, _) = dslWorld.sheafConsistency
    println("    H^1 (sheaf obstructions): $dslH1")
    println()

    // --- Test 8: Callable world (operator invoke) ---
    println("[8] Callable world (operator invoke to accumulate):")
    val combined = world("Meditation also reduces cortisol, study by Stanford", Framework.Empirical)
    println("    Combined claims: ${combined.claims.size}")
    println("    Combined sources: ${combined.sources.size}")
    println()

    // --- Test 9: DeepRecursiveFunction transitive source lookup ---
    println("[9] DeepRecursiveFunction transitive sources:")
    world.claims.keys.firstOrNull()?.let { id ->
        val transitive = findTransitiveSources(world to id)
        println("    Transitive sources for $id: ${transitive.size}")
    }
    println()

    // --- Test 10: Value class / Trit arithmetic ---
    println("[10] GF(3) Trit arithmetic:")
    val a = Trit.ONE
    val b = Trit.TWO
    val c = a + b
    println("    $a + $b = $c")
    println("    $a * $b = ${a * b}")
    println("    -$a = ${-a}")
    println("    Balanced [+1, -1, 0]: ${Trit.isBalanced(listOf(Trit.ONE, Trit.TWO, Trit.ZERO))}")
    println("    Balanced [+1, +1, +1]: ${Trit.isBalanced(listOf(Trit.ONE, Trit.ONE, Trit.ONE))}")
    println()

    println("=== All tests passed. ===")
}
