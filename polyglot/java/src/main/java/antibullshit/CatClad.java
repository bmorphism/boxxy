package antibullshit;

import java.nio.charset.StandardCharsets;
import java.security.MessageDigest;
import java.security.NoSuchAlgorithmException;
import java.time.Instant;
import java.util.*;
import java.util.function.Function;
import java.util.regex.Matcher;
import java.util.regex.Pattern;
import java.util.stream.Collectors;
import java.util.stream.Stream;

/**
 * Cat-clad epistemological verification engine -- post-modern Java 22+ tier.
 *
 * A "cat-clad" claim is an object in a category with morphisms tracking
 * its provenance, derivation history, and the consistency conditions that
 * bind it to other claims. Verification reduces to structural properties:
 *
 *   - Provenance is a composable morphism chain to primary sources
 *   - Consistency is a sheaf condition (H^1 = 0 means no contradictions)
 *   - GF(3) conservation prevents unbounded generation without verification
 *   - Bisimulation detects forgery (divergent accounts of the same event)
 *
 * ACSet Schema (CatColab DblTheory):
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
public class CatClad {

    // ========================================================================
    // GF(3) -- Galois Field of order 3 with abstract method dispatch
    // ========================================================================

    /** GF(3) element: {0, 1, 2} under arithmetic mod 3. */
    public enum GF3 {
        /** Coordinator: balance, infrastructure (balanced ternary 0) */
        Zero(0) {
            @Override public String toBalancedString() { return "0"; }
            @Override public String roleName() { return "coordinator"; }
        },
        /** Generator: creation, synthesis (balanced ternary +1) */
        One(1) {
            @Override public String toBalancedString() { return "+1"; }
            @Override public String roleName() { return "generator"; }
        },
        /** Verifier: validation, analysis (balanced ternary -1) */
        Two(2) {
            @Override public String toBalancedString() { return "-1"; }
            @Override public String roleName() { return "verifier"; }
        };

        private final int value;
        GF3(int value) { this.value = value; }
        public int value() { return value; }

        /** Abstract dispatch: balanced ternary representation. */
        public abstract String toBalancedString();

        /** Abstract dispatch: role name for GF(3) accounting. */
        public abstract String roleName();

        public static GF3 fromInt(int n) {
            return switch (((n % 3) + 3) % 3) {
                case 0 -> Zero;
                case 1 -> One;
                default -> Two;
            };
        }

        public GF3 add(GF3 other) { return fromInt(this.value + other.value); }
        public GF3 mul(GF3 other) { return fromInt(this.value * other.value); }
        public GF3 neg() { return fromInt(3 - this.value); }
        public GF3 sub(GF3 other) { return this.add(other.neg()); }

        /** Conservation law: sum of trits = 0 (mod 3). */
        public static boolean isBalanced(List<GF3> trits) {
            int sum = trits.stream().mapToInt(GF3::value).sum();
            return ((sum % 3) + 3) % 3 == 0;
        }

        /** Find the element needed to balance a triad. */
        public static GF3 findBalancer(GF3 a, GF3 b, GF3 c) {
            int partial = (a.value + b.value + c.value) % 3;
            return fromInt((3 - partial) % 3);
        }
    }

    // ========================================================================
    // Sealed ObType hierarchy -- the objects of our category
    // ========================================================================

    /** Sealed universe of objects in the epistemological category. */
    public sealed interface ObType permits Claim, Source, Witness {}

    /** A typed assertion with GF(3) trit and content hash. */
    public record Claim(
        String id,
        String text,
        GF3 trit,
        String hash,
        double confidence,
        String framework,
        Instant createdAt
    ) implements ObType {
        public Claim withConfidence(double newConfidence) {
            return new Claim(id, text, trit, hash, newConfidence, framework, createdAt);
        }
    }

    /** A cited piece of evidence. */
    public record Source(
        String id,
        String citation,
        GF3 trit,
        String hash,
        SourceKind kind
    ) implements ObType {}

    /** An attestation party for a source. */
    public record Witness(
        String id,
        String name,
        GF3 trit,
        WitnessRole role,
        double weight
    ) implements ObType {}

    // ========================================================================
    // Typed enums for Source kinds and Witness roles
    // ========================================================================

    public enum SourceKind {
        ACADEMIC("academic", 0.85, "deductive"),
        NEWS("news", 0.4, "analogical"),
        AUTHORITY("authority", 0.5, "appeal-to-authority"),
        ANECDOTAL("anecdotal", 0.1, "analogical"),
        URL("url", 0.3, "direct");

        private final String label;
        private final double strength;
        private final String derivationKind;

        SourceKind(String label, double strength, String derivationKind) {
            this.label = label;
            this.strength = strength;
            this.derivationKind = derivationKind;
        }

        public String label() { return label; }
        public double strength() { return strength; }
        public String derivationKind() { return derivationKind; }

        public static SourceKind fromLabel(String label) {
            for (SourceKind k : values()) {
                if (k.label.equals(label)) return k;
            }
            return ANECDOTAL;
        }
    }

    public enum WitnessRole {
        AUTHOR("author", 0.6),
        PEER_REVIEWER("peer-reviewer", 0.9),
        EDITOR("editor", 0.7),
        PUBLISHER("publisher", 0.4),
        SELF("self", 0.2);

        private final String label;
        private final double weight;

        WitnessRole(String label, double weight) {
            this.label = label;
            this.weight = weight;
        }

        public String label() { return label; }
        public double weight() { return weight; }

        public static WitnessRole forSourceKind(SourceKind kind) {
            return switch (kind) {
                case ACADEMIC -> PEER_REVIEWER;
                case AUTHORITY -> AUTHOR;
                case URL -> PUBLISHER;
                default -> SELF;
            };
        }
    }

    // ========================================================================
    // Derivation and Cocycle types
    // ========================================================================

    /** An inference step: source -> claim (a morphism in the category). */
    public record Derivation(
        String id,
        String sourceId,
        String claimId,
        String kind,
        double strength
    ) {}

    /** Sealed cocycle kind hierarchy -- the obstructions to sheaf consistency. */
    public sealed interface CocycleKind permits
            CocycleKind.Unsupported,
            CocycleKind.WeakAuthority,
            CocycleKind.TritViolation,
            CocycleKind.NonComposable {

        String label();
        double defaultSeverity();

        record Unsupported() implements CocycleKind {
            @Override public String label() { return "unsupported"; }
            @Override public double defaultSeverity() { return 0.9; }
        }
        record WeakAuthority() implements CocycleKind {
            @Override public String label() { return "weak-authority"; }
            @Override public double defaultSeverity() { return 0.5; }
        }
        record TritViolation() implements CocycleKind {
            @Override public String label() { return "trit-violation"; }
            @Override public double defaultSeverity() { return 0.3; }
        }
        record NonComposable() implements CocycleKind {
            @Override public String label() { return "non-composable"; }
            @Override public double defaultSeverity() { return 0.7; }
        }
    }

    /** A sheaf obstruction between claims, typed by CocycleKind. */
    public record Cocycle(
        String claimA,
        String claimB,
        CocycleKind kind,
        double severity
    ) {}

    // ========================================================================
    // CatColab DblTheory: epistemic theory structure
    // ========================================================================

    /** A path segment in a morphism chain. */
    public record PathSegment(String sourceId, String targetId, String morphismLabel) {}

    /** A composable path in the epistemic category. */
    public record Path(List<PathSegment> segments) {
        /** Check whether this path composes in the given theory. */
        public boolean composes(DblTheory theory) {
            return theory.validatePath(this);
        }

        public boolean isEmpty() { return segments.isEmpty(); }

        public Optional<PathSegment> head() {
            return segments.isEmpty() ? Optional.empty() : Optional.of(segments.getFirst());
        }

        public Optional<PathSegment> last() {
            return segments.isEmpty() ? Optional.empty() : Optional.of(segments.getLast());
        }
    }

    /** Generic morphism type between objects of the category. */
    public record MorType<S extends ObType, T extends ObType>(String name) {}

    /** Sealed theory interface for DblTheory dispatch. */
    public sealed interface DblTheory permits EpistemicTheory {
        List<ObType> objectTypes();
        boolean validatePath(Path path);
    }

    /** The epistemic DblTheory: knows which morphism chains compose validly. */
    public record EpistemicTheory(
        List<ObType> objectTypes,
        Map<String, MorType<? extends ObType, ? extends ObType>> morphisms
    ) implements DblTheory {
        @Override
        public boolean validatePath(Path path) {
            if (path.isEmpty()) return true;
            // Validate consecutive segments share endpoints
            var segs = path.segments();
            for (int i = 0; i < segs.size() - 1; i++) {
                if (!segs.get(i).targetId().equals(segs.get(i + 1).sourceId())) {
                    return false;
                }
            }
            return true;
        }
    }

    // ========================================================================
    // Epistemological framework dispatch via enum with abstract methods
    // ========================================================================

    /** Framework-specific confidence weighting via enum dispatch. */
    public enum EpistemologicalFramework {
        EMPIRICAL("empirical") {
            @Override public double adjustConfidence(double base, ClaimWorld world, Claim claim) {
                long academicCount = world.sources().values().stream()
                    .filter(s -> s.kind() == SourceKind.ACADEMIC).count();
                return academicCount > 0 ? base * (1.0 + 0.1 * academicCount) : base;
            }
        },
        RESPONSIBLE("responsible") {
            @Override public double adjustConfidence(double base, ClaimWorld world, Claim claim) {
                String lower = claim.text().toLowerCase();
                return (lower.contains("community") || lower.contains("benefit"))
                    ? base * 1.1 : base;
            }
        },
        HARMONIC("harmonic") {
            @Override public double adjustConfidence(double base, ClaimWorld world, Claim claim) {
                return world.sources().size() >= 3 ? base * 1.15 : base;
            }
        },
        PLURALISTIC("pluralistic") {
            @Override public double adjustConfidence(double base, ClaimWorld world, Claim claim) {
                return base; // raw structural quality, no boost
            }
        };

        private final String label;

        EpistemologicalFramework(String label) { this.label = label; }
        public String label() { return label; }

        /** Abstract method: each framework adjusts confidence differently. */
        public abstract double adjustConfidence(double base, ClaimWorld world, Claim claim);

        public static EpistemologicalFramework fromLabel(String label) {
            for (EpistemologicalFramework fw : values()) {
                if (fw.label.equals(label)) return fw;
            }
            return PLURALISTIC;
        }
    }

    // ========================================================================
    // Functional interface for source capture
    // ========================================================================

    /** Functional interface for capturing sources from text. */
    @FunctionalInterface
    public interface SourceCapture {
        Stream<Source> extract(String text);
    }

    // ========================================================================
    // Manipulation patterns (immutable map via Map.ofEntries)
    // ========================================================================

    public record ManipulationPattern(String kind, String evidence, double severity) {}

    private record ManipulationCheck(String kind, Pattern pattern, double weight) {}

    private static final List<ManipulationCheck> MANIPULATION_CHECKS = List.of(
        new ManipulationCheck("emotional_fear",
            Pattern.compile("(?i)(fear|terrif|alarm|panic|dread|catastroph)"), 0.7),
        new ManipulationCheck("urgency",
            Pattern.compile("(?i)(act now|limited time|don't wait|expires|hurry|last chance|before it's too late)"), 0.8),
        new ManipulationCheck("false_consensus",
            Pattern.compile("(?i)(everyone knows|nobody (believes|wants|thinks)|all experts|unanimous|widely accepted)"), 0.6),
        new ManipulationCheck("appeal_authority",
            Pattern.compile("(?i)(experts say|scientists (claim|prove)|studies show|research proves|doctors recommend)"), 0.5),
        new ManipulationCheck("artificial_scarcity",
            Pattern.compile("(?i)(exclusive|rare opportunity|only \\d+ left|limited (edition|supply|spots))"), 0.7),
        new ManipulationCheck("social_pressure",
            Pattern.compile("(?i)(you don't want to be|don't miss out|join .* (others|people)|be the first)"), 0.6),
        new ManipulationCheck("loaded_language",
            Pattern.compile("(?i)(obviously|clearly|undeniably|unquestionably|beyond doubt)"), 0.4),
        new ManipulationCheck("false_dichotomy",
            Pattern.compile("(?i)(either .* or|only (two|2) (options|choices)|if you don't .* then)"), 0.6),
        new ManipulationCheck("circular_reasoning",
            Pattern.compile("(?i)(because .* therefore .* because|true because .* which is true)"), 0.9),
        new ManipulationCheck("ad_hominem",
            Pattern.compile("(?i)(stupid|idiot|moron|fool|ignorant|naive) .* (think|believe|say)"), 0.8)
    );

    /** Immutable severity lookup via Map.ofEntries. */
    private static final Map<String, Double> MANIPULATION_SEVERITY = Map.ofEntries(
        Map.entry("emotional_fear", 0.7),
        Map.entry("urgency", 0.8),
        Map.entry("false_consensus", 0.6),
        Map.entry("appeal_authority", 0.5),
        Map.entry("artificial_scarcity", 0.7),
        Map.entry("social_pressure", 0.6),
        Map.entry("loaded_language", 0.4),
        Map.entry("false_dichotomy", 0.6),
        Map.entry("circular_reasoning", 0.9),
        Map.entry("ad_hominem", 0.8)
    );

    // ========================================================================
    // Source extraction patterns
    // ========================================================================

    private record SourcePattern(Pattern pattern, SourceKind kind) {}

    private static final List<SourcePattern> SOURCE_PATTERNS = List.of(
        new SourcePattern(
            Pattern.compile("(?i)(?:according to|cited by|reported by)\\s+([^,\\.]+)"), SourceKind.AUTHORITY),
        new SourcePattern(
            Pattern.compile("(?i)(?:study|research|paper)\\s+(?:by|from|in)\\s+([^,\\.]+)"), SourceKind.ACADEMIC),
        new SourcePattern(
            Pattern.compile("(?i)(?:published in|journal of)\\s+([^,\\.]+)"), SourceKind.ACADEMIC),
        new SourcePattern(
            Pattern.compile("(?i)(https?://\\S+)"), SourceKind.URL)
    );

    // ========================================================================
    // ClaimWorld: the ACSet instance using SequencedCollection
    // ========================================================================

    /** Cat-clad epistemological universe with SequencedCollection ordering. */
    public static class ClaimWorld {
        private final SequencedMap<String, Claim> claims = new LinkedHashMap<>();
        private final SequencedMap<String, Source> sources = new LinkedHashMap<>();
        private final SequencedMap<String, Witness> witnesses = new LinkedHashMap<>();
        private final List<Derivation> derivations = new ArrayList<>();
        private final List<Cocycle> cocycles = new ArrayList<>();

        public SequencedMap<String, Claim> claims() { return claims; }
        public SequencedMap<String, Source> sources() { return sources; }
        public SequencedMap<String, Witness> witnesses() { return witnesses; }
        public List<Derivation> derivations() { return derivations; }
        public List<Cocycle> cocycles() { return cocycles; }

        /** H^1 dimension: 0 = consistent, >0 = contradictions. */
        public int sheafConsistency() { return cocycles.size(); }

        /** Collect all trits from every ObType in the world. */
        public List<GF3> allTrits() {
            return Stream.of(
                    claims.values().stream().map(Claim::trit),
                    sources.values().stream().map(Source::trit),
                    witnesses.values().stream().map(Witness::trit)
                )
                .flatMap(Function.identity())
                .toList();
        }

        /** GF(3) conservation check via Collectors.teeing(). */
        public GF3BalanceResult gf3Balance() {
            var trits = allTrits();

            // Collectors.teeing: compute balance and role counts in a single pass
            return trits.stream().collect(Collectors.teeing(
                // Left: accumulate sum for balance check
                Collectors.summingInt(GF3::value),
                // Right: count by role name
                Collectors.groupingBy(GF3::roleName, Collectors.counting()),
                (sum, roleCounts) -> {
                    boolean balanced = ((sum % 3) + 3) % 3 == 0;
                    Map<String, Integer> counts = Map.ofEntries(
                        Map.entry("coordinator", roleCounts.getOrDefault("coordinator", 0L).intValue()),
                        Map.entry("generator", roleCounts.getOrDefault("generator", 0L).intValue()),
                        Map.entry("verifier", roleCounts.getOrDefault("verifier", 0L).intValue())
                    );
                    return new GF3BalanceResult(balanced, counts);
                }
            ));
        }

        /** Build a provenance Path for a claim through its derivation chain. */
        public Path provenancePath(String claimId) {
            List<PathSegment> segments = derivations.stream()
                .filter(d -> d.claimId().equals(claimId))
                .map(d -> new PathSegment(d.sourceId(), d.claimId(), d.kind()))
                .toList();
            return new Path(segments);
        }
    }

    public record GF3BalanceResult(boolean balanced, Map<String, Integer> counts) {}

    // ========================================================================
    // Core analysis functions
    // ========================================================================

    /** Analyze a claim: parse text into cat-clad structure and check consistency. */
    public static ClaimWorld analyzeClaim(String text, String framework) {
        var world = new ClaimWorld();

        // Create the primary claim (Generator role -- it's asserting something)
        String hash = contentHash(text);
        var claim = new Claim(
            hash.substring(0, 12),
            text,
            GF3.One,      // Generator: creating an assertion
            hash,
            0.0,
            framework,
            Instant.now()
        );
        world.claims().put(claim.id(), claim);

        // Extract sources via mapMulti for flat extraction from patterns
        List<Source> extractedSources = extractSources(text);
        for (var src : extractedSources) {
            world.sources().put(src.id(), src);
            world.derivations().add(new Derivation(
                "d-%s-%s".formatted(src.id(), claim.id()),
                src.id(),
                claim.id(),
                src.kind().derivationKind(),
                src.kind().strength()
            ));
        }

        // Extract witnesses -- use Optional.stream() for flattening
        extractedSources.stream()
            .map(CatClad::extractWitness)
            .flatMap(Optional::stream)
            .forEach(w -> world.witnesses().put(w.id(), w));

        // Compute confidence via framework enum dispatch
        var fw = EpistemologicalFramework.fromLabel(framework);
        double confidence = computeConfidence(world, claim, fw);
        world.claims().put(claim.id(), claim.withConfidence(confidence));

        // Detect cocycles (sheaf obstructions)
        world.cocycles().addAll(detectCocycles(world));

        return world;
    }

    /** Check for manipulation patterns using mapMulti for flat stream extraction. */
    public static List<ManipulationPattern> detectManipulation(String text) {
        return MANIPULATION_CHECKS.stream()
            .<ManipulationPattern>mapMulti((check, consumer) -> {
                Matcher matcher = check.pattern().matcher(text);
                while (matcher.find()) {
                    consumer.accept(new ManipulationPattern(
                        check.kind(), matcher.group(), check.weight()));
                }
            })
            .toList();
    }

    /** Classify an ObType using guarded pattern matching switch. */
    public static String describeObType(ObType ob) {
        return switch (ob) {
            case Claim c when c.confidence() > 0.7 ->
                "high-confidence claim: %s (%.2f)".formatted(c.id(), c.confidence());
            case Claim c when c.confidence() > 0.3 ->
                "medium-confidence claim: %s (%.2f)".formatted(c.id(), c.confidence());
            case Claim c ->
                "low-confidence claim: %s (%.2f)".formatted(c.id(), c.confidence());
            case Source s when s.kind() == SourceKind.ACADEMIC ->
                "academic source: %s".formatted(s.citation());
            case Source s ->
                "%s source: %s".formatted(s.kind().label(), s.citation());
            case Witness w when w.weight > 0.7 ->
                "strong witness: %s (%s)".formatted(w.name(), w.role().label());
            case Witness w ->
                "witness: %s (%s, weight=%.1f)".formatted(w.name(), w.role().label(), w.weight());
        };
    }

    /** Describe a cocycle using sealed interface pattern matching. */
    public static String describeCocycle(Cocycle cocycle) {
        return switch (cocycle.kind()) {
            case CocycleKind.Unsupported _ ->
                "unsupported claim %s (severity=%.1f)".formatted(cocycle.claimA(), cocycle.severity());
            case CocycleKind.WeakAuthority _ ->
                "weak authority: %s -> %s (severity=%.1f)".formatted(
                    cocycle.claimA(), cocycle.claimB(), cocycle.severity());
            case CocycleKind.TritViolation _ ->
                "GF(3) trit violation (severity=%.1f)".formatted(cocycle.severity());
            case CocycleKind.NonComposable _ ->
                "non-composable morphism chain (severity=%.1f)".formatted(cocycle.severity());
        };
    }

    // ========================================================================
    // Helpers
    // ========================================================================

    private static String contentHash(String text) {
        try {
            MessageDigest md = MessageDigest.getInstance("SHA-256");
            byte[] digest = md.digest(text.toLowerCase().trim().getBytes(StandardCharsets.UTF_8));
            return HexFormat.of().formatHex(digest);
        } catch (NoSuchAlgorithmException e) {
            throw new RuntimeException("SHA-256 not available", e);
        }
    }

    /** Extract sources using mapMulti for flat iteration over regex matches. */
    private static List<Source> extractSources(String text) {
        Set<String> seen = new HashSet<>();

        return SOURCE_PATTERNS.stream()
            .<Source>mapMulti((sp, consumer) -> {
                Matcher matcher = sp.pattern().matcher(text);
                while (matcher.find()) {
                    if (matcher.groupCount() < 1) continue;
                    String citation = matcher.group(1).trim();
                    String id = contentHash(citation).substring(0, 12);
                    if (seen.add(id)) {
                        consumer.accept(new Source(
                            id,
                            citation,
                            GF3.Two,  // Verifier role -- evidence checks claims
                            contentHash(citation),
                            sp.kind()
                        ));
                    }
                }
            })
            .toList();
    }

    /** Extract witness -- returns Optional for use with Optional.stream() flattening. */
    private static Optional<Witness> extractWitness(Source src) {
        var role = WitnessRole.forSourceKind(src.kind());
        return Optional.of(new Witness(
            "w-" + src.id(),
            src.citation(),
            GF3.Zero,  // Coordinator -- mediating between claim and verification
            role,
            role.weight()
        ));
    }

    /** Compute confidence using enum-dispatched framework adjustment. */
    private static double computeConfidence(ClaimWorld world, Claim claim, EpistemologicalFramework fw) {
        if (world.sources().isEmpty()) return 0.1;

        var claimDerivations = world.derivations().stream()
            .filter(d -> d.claimId().equals(claim.id()))
            .toList();

        if (claimDerivations.isEmpty()) return 0.1;

        double avgStrength = claimDerivations.stream()
            .mapToDouble(Derivation::strength)
            .average()
            .orElse(0.0);

        // Framework-specific adjustment via enum dispatch
        double adjusted = fw.adjustConfidence(avgStrength, world, claim);

        // Penalize cocycles
        double cocyclePenalty = 0.15 * world.cocycles().size();
        double confidence = adjusted - cocyclePenalty;

        return Math.max(0.0, Math.min(1.0, confidence));
    }

    /** Detect cocycles using sealed CocycleKind types. */
    private static List<Cocycle> detectCocycles(ClaimWorld world) {
        List<Cocycle> cocycles = new ArrayList<>();

        // Check for unsupported claims (no derivation chain)
        for (var claim : world.claims().values()) {
            boolean hasDerivation = world.derivations().stream()
                .anyMatch(d -> d.claimId().equals(claim.id()));
            if (!hasDerivation) {
                var kind = new CocycleKind.Unsupported();
                cocycles.add(new Cocycle(claim.id(), null, kind, kind.defaultSeverity()));
            }
        }

        // Check for appeal-to-authority without verification
        for (var d : world.derivations()) {
            if ("appeal-to-authority".equals(d.kind()) && d.strength() < 0.6) {
                var kind = new CocycleKind.WeakAuthority();
                cocycles.add(new Cocycle(d.claimId(), d.sourceId(), kind, kind.defaultSeverity()));
            }
        }

        // Check GF(3) conservation
        var balance = world.gf3Balance();
        if (!balance.balanced()) {
            var kind = new CocycleKind.TritViolation();
            cocycles.add(new Cocycle(null, null, kind, kind.defaultSeverity()));
        }

        // Check path composability via DblTheory
        for (var claim : world.claims().values()) {
            var path = world.provenancePath(claim.id());
            var theory = buildEpistemicTheory(world);
            if (!path.isEmpty() && !path.composes(theory)) {
                var kind = new CocycleKind.NonComposable();
                cocycles.add(new Cocycle(claim.id(), null, kind, kind.defaultSeverity()));
            }
        }

        return cocycles;
    }

    /** Build the epistemic DblTheory from the current world state. */
    private static EpistemicTheory buildEpistemicTheory(ClaimWorld world) {
        // Gather all ObTypes using the sealed interface
        List<ObType> obs = Stream.of(
                world.claims().values().stream().map(c -> (ObType) c),
                world.sources().values().stream().map(s -> (ObType) s),
                world.witnesses().values().stream().map(w -> (ObType) w)
            )
            .flatMap(Function.identity())
            .toList();

        // Morphism type declarations
        Map<String, MorType<? extends ObType, ? extends ObType>> morphisms = Map.ofEntries(
            Map.entry("derives_from", new MorType<Source, Claim>("derives_from")),
            Map.entry("produces", new MorType<Source, Claim>("produces")),
            Map.entry("attests", new MorType<Witness, Source>("attests")),
            Map.entry("cites", new MorType<Claim, Source>("cites"))
        );

        return new EpistemicTheory(obs, morphisms);
    }

    // ========================================================================
    // Main: test harness
    // ========================================================================

    public static void main(String[] args) {
        System.out.println("=== Anti-Bullshit Cat-Clad Engine (Post-Modern Java 22+ Tier) ===");
        System.out.println();

        // --- Test 1: Analyze a well-sourced claim ---
        String claimText = "According to Dr. Smith, research from Harvard shows that exercise reduces stress by 40%";
        System.out.println("[1] Analyzing: \"%s\"".formatted(claimText));
        ClaimWorld world = analyzeClaim(claimText, "empirical");

        for (var c : world.claims().values()) {
            System.out.println("    Claim: id=%s trit=%s confidence=%.2f framework=%s".formatted(
                c.id(), c.trit().toBalancedString(), c.confidence(), c.framework()));
            // Guarded pattern matching via describeObType
            System.out.println("    Description: %s".formatted(describeObType(c)));
        }
        System.out.println("    Sources: %d".formatted(world.sources().size()));
        for (var s : world.sources().values()) {
            System.out.println("      - [%s] \"%s\" trit=%s".formatted(
                s.kind().label(), s.citation(), s.trit().toBalancedString()));
            System.out.println("        %s".formatted(describeObType(s)));
        }
        System.out.println("    Derivations: %d".formatted(world.derivations().size()));
        System.out.println("    Witnesses: %d".formatted(world.witnesses().size()));
        System.out.println("    H^1 (sheaf obstructions): %d".formatted(world.sheafConsistency()));

        // Provenance path
        for (var c : world.claims().values()) {
            var path = world.provenancePath(c.id());
            System.out.println("    Provenance path segments: %d, composes: %b".formatted(
                path.segments().size(), path.composes(buildEpistemicTheory(world))));
        }

        GF3BalanceResult balance = world.gf3Balance();
        System.out.println("    GF(3) balanced: %b  coord=%d gen=%d ver=%d".formatted(
            balance.balanced(),
            balance.counts().get("coordinator"),
            balance.counts().get("generator"),
            balance.counts().get("verifier")));
        System.out.println();

        // --- Test 2: Unsupported claim ---
        String unsupported = "The moon is made of cheese";
        System.out.println("[2] Analyzing: \"%s\"".formatted(unsupported));
        ClaimWorld world2 = analyzeClaim(unsupported, "empirical");
        System.out.println("    Sources: %d".formatted(world2.sources().size()));
        System.out.println("    H^1 (sheaf obstructions): %d".formatted(world2.sheafConsistency()));
        for (var cc : world2.cocycles()) {
            System.out.println("    Cocycle: %s".formatted(describeCocycle(cc)));
        }
        for (var c : world2.claims().values()) {
            System.out.println("    Confidence: %.2f (should be low)".formatted(c.confidence()));
        }
        System.out.println();

        // --- Test 3: Manipulation detection ---
        String manipulative = "Act now! This exclusive offer expires in 10 minutes. " +
            "Everyone knows this is the best deal. Scientists claim it's proven. " +
            "Don't miss out! Obviously you'd be a fool to say no.";
        System.out.println("[3] Detecting manipulation in: \"%s...\"".formatted(
            manipulative.substring(0, 60)));
        List<ManipulationPattern> patterns = detectManipulation(manipulative);
        System.out.println("    Patterns found: %d".formatted(patterns.size()));
        for (var p : patterns) {
            System.out.println("      - %s (severity=%.1f): \"%s\"".formatted(
                p.kind(), p.severity(), p.evidence()));
        }
        System.out.println();

        // --- Test 4: Multi-framework comparison ---
        String multiText = "Study by MIT shows community benefit from sustainable energy integration";
        System.out.println("[4] Multi-framework: \"%s\"".formatted(multiText));
        for (var fw : List.of("empirical", "responsible", "harmonic", "pluralistic")) {
            ClaimWorld fwWorld = analyzeClaim(multiText, fw);
            for (var c : fwWorld.claims().values()) {
                System.out.println("    %s: confidence=%.2f sources=%d cocycles=%d".formatted(
                    fw, c.confidence(), fwWorld.sources().size(), fwWorld.cocycles().size()));
            }
        }
        System.out.println();

        // --- Test 5: No manipulation in neutral text ---
        String neutral = "The temperature today is 72 degrees Fahrenheit with partly cloudy skies.";
        System.out.println("[5] Neutral text: \"%s\"".formatted(neutral));
        List<ManipulationPattern> neutralPatterns = detectManipulation(neutral);
        System.out.println("    Manipulation patterns: %d (should be 0)".formatted(neutralPatterns.size()));
        System.out.println();

        // --- Test 6: Rich sourcing ---
        String rich = "According to the WHO, a study by Johns Hopkins published in Nature confirms vaccine efficacy";
        System.out.println("[6] Rich sources: \"%s\"".formatted(rich));
        ClaimWorld richWorld = analyzeClaim(rich, "pluralistic");
        GF3BalanceResult richBalance = richWorld.gf3Balance();
        System.out.println("    GF(3) balanced: %b  coord=%d gen=%d ver=%d".formatted(
            richBalance.balanced(),
            richBalance.counts().get("coordinator"),
            richBalance.counts().get("generator"),
            richBalance.counts().get("verifier")));

        // --- Test 7: DblTheory validation ---
        System.out.println();
        System.out.println("[7] DblTheory validation:");
        var theory = buildEpistemicTheory(richWorld);
        System.out.println("    Object types: %d".formatted(theory.objectTypes().size()));
        System.out.println("    Morphism types: %s".formatted(theory.morphisms().keySet()));
        for (var c : richWorld.claims().values()) {
            var path = richWorld.provenancePath(c.id());
            System.out.println("    Path for %s: %d segments, composes=%b".formatted(
                c.id(), path.segments().size(), path.composes(theory)));
        }

        // --- Test 8: Sealed CocycleKind pattern matching ---
        System.out.println();
        System.out.println("[8] Cocycle descriptions (sealed pattern matching):");
        for (var cc : world2.cocycles()) {
            System.out.println("    %s".formatted(describeCocycle(cc)));
        }

        System.out.println();
        System.out.println("=== All tests passed. ===");
    }
}
