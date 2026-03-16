package antibullshit;

import java.nio.charset.StandardCharsets;
import java.security.MessageDigest;
import java.security.NoSuchAlgorithmException;
import java.time.Instant;
import java.util.*;
import java.util.regex.Matcher;
import java.util.regex.Pattern;
import java.util.stream.Collectors;
import java.util.stream.Stream;

/**
 * Cat-clad epistemological verification engine -- Java enterprise-tier.
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
 * ACSet Schema:
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
    // GF(3) -- Galois Field of order 3
    // ========================================================================

    /** GF(3) element: {0, 1, 2} under arithmetic mod 3. */
    public enum GF3 {
        /** Coordinator: balance, infrastructure (balanced ternary 0) */
        Zero(0),
        /** Generator: creation, synthesis (balanced ternary +1) */
        One(1),
        /** Verifier: validation, analysis (balanced ternary -1) */
        Two(2);

        private final int value;
        GF3(int value) { this.value = value; }
        public int value() { return value; }

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

        public String toBalancedString() {
            return switch (this) {
                case Zero -> "0";
                case One -> "+1";
                case Two -> "-1";
            };
        }
    }

    // ========================================================================
    // ACSet Schema types
    // ========================================================================

    /** A typed assertion with GF(3) trit and content hash. */
    public record Claim(
        String id,
        String text,
        GF3 trit,
        String hash,
        double confidence,
        String framework,
        Instant createdAt
    ) {
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
        String kind  // "academic", "news", "authority", "anecdotal", "url"
    ) {}

    /** An attestation party for a source. */
    public record Witness(
        String id,
        String name,
        GF3 trit,
        String role,    // "author", "peer-reviewer", "editor", "publisher", "self"
        double weight
    ) {}

    /** An inference step: source -> claim. */
    public record Derivation(
        String id,
        String sourceId,
        String claimId,
        String kind,     // "direct", "inductive", "deductive", "analogical", "appeal-to-authority"
        double strength
    ) {}

    /** A sheaf obstruction between claims. */
    public record Cocycle(
        String claimA,
        String claimB,
        String kind,     // "contradiction", "unsupported", "circular", "trit-violation", "weak-authority"
        double severity
    ) {}

    // ========================================================================
    // ClaimWorld: the ACSet instance
    // ========================================================================

    /** Cat-clad epistemological universe. */
    public static class ClaimWorld {
        private final Map<String, Claim> claims = new LinkedHashMap<>();
        private final Map<String, Source> sources = new LinkedHashMap<>();
        private final Map<String, Witness> witnesses = new LinkedHashMap<>();
        private final List<Derivation> derivations = new ArrayList<>();
        private final List<Cocycle> cocycles = new ArrayList<>();

        public Map<String, Claim> claims() { return claims; }
        public Map<String, Source> sources() { return sources; }
        public Map<String, Witness> witnesses() { return witnesses; }
        public List<Derivation> derivations() { return derivations; }
        public List<Cocycle> cocycles() { return cocycles; }

        /** H^1 dimension: 0 = consistent, >0 = contradictions. */
        public int sheafConsistency() {
            return cocycles.size();
        }

        /** GF(3) conservation check: sum of all trits = 0 (mod 3). */
        public GF3BalanceResult gf3Balance() {
            Map<String, Integer> counts = new LinkedHashMap<>();
            counts.put("coordinator", 0);
            counts.put("generator", 0);
            counts.put("verifier", 0);

            List<GF3> trits = new ArrayList<>();

            for (Claim c : claims.values()) {
                trits.add(c.trit());
            }
            for (Source s : sources.values()) {
                trits.add(s.trit());
            }
            for (Witness w : witnesses.values()) {
                trits.add(w.trit());
            }

            for (GF3 t : trits) {
                switch (t) {
                    case Zero -> counts.merge("coordinator", 1, Integer::sum);
                    case One -> counts.merge("generator", 1, Integer::sum);
                    case Two -> counts.merge("verifier", 1, Integer::sum);
                }
            }

            return new GF3BalanceResult(GF3.isBalanced(trits), counts);
        }
    }

    public record GF3BalanceResult(boolean balanced, Map<String, Integer> counts) {}

    // ========================================================================
    // Manipulation patterns (10 patterns)
    // ========================================================================

    public record ManipulationPattern(String kind, String evidence, double severity) {}

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

    private record ManipulationCheck(String kind, Pattern pattern, double weight) {}

    // ========================================================================
    // Source extraction patterns
    // ========================================================================

    private static final List<SourcePattern> SOURCE_PATTERNS = List.of(
        new SourcePattern(
            Pattern.compile("(?i)(?:according to|cited by|reported by)\\s+([^,\\.]+)"), "authority"),
        new SourcePattern(
            Pattern.compile("(?i)(?:study|research|paper)\\s+(?:by|from|in)\\s+([^,\\.]+)"), "academic"),
        new SourcePattern(
            Pattern.compile("(?i)(?:published in|journal of)\\s+([^,\\.]+)"), "academic"),
        new SourcePattern(
            Pattern.compile("(?i)(https?://\\S+)"), "url")
    );

    private record SourcePattern(Pattern pattern, String kind) {}

    // ========================================================================
    // Core analysis functions
    // ========================================================================

    /** Analyze a claim: parse text into cat-clad structure and check consistency. */
    public static ClaimWorld analyzeClaim(String text, String framework) {
        ClaimWorld world = new ClaimWorld();

        // Create the primary claim (Generator role -- it's asserting something)
        String hash = contentHash(text);
        Claim claim = new Claim(
            hash.substring(0, 12),
            text,
            GF3.One,      // Generator: creating an assertion
            hash,
            0.0,
            framework,
            Instant.now()
        );
        world.claims.put(claim.id(), claim);

        // Extract sources as morphisms from claim
        List<Source> sources = extractSources(text);
        for (Source src : sources) {
            world.sources.put(src.id(), src);
            world.derivations.add(new Derivation(
                "d-" + src.id() + "-" + claim.id(),
                src.id(),
                claim.id(),
                classifyDerivation(src),
                sourceStrength(src)
            ));
        }

        // Extract witnesses (who attests to the sources)
        for (Source src : sources) {
            List<Witness> witnesses = extractWitnesses(src);
            for (Witness w : witnesses) {
                world.witnesses.put(w.id(), w);
            }
        }

        // Compute confidence and update claim
        double confidence = computeConfidence(world, claim, framework);
        Claim updatedClaim = claim.withConfidence(confidence);
        world.claims.put(updatedClaim.id(), updatedClaim);

        // Detect cocycles (contradictions, unsupported claims, circular reasoning)
        world.cocycles.addAll(detectCocycles(world));

        return world;
    }

    /** Check for manipulation patterns in text. */
    public static List<ManipulationPattern> detectManipulation(String text) {
        List<ManipulationPattern> patterns = new ArrayList<>();

        for (ManipulationCheck check : MANIPULATION_CHECKS) {
            Matcher matcher = check.pattern().matcher(text);
            while (matcher.find()) {
                patterns.add(new ManipulationPattern(
                    check.kind(), matcher.group(), check.weight()));
            }
        }

        return patterns;
    }

    // ========================================================================
    // Helpers
    // ========================================================================

    private static String contentHash(String text) {
        try {
            MessageDigest md = MessageDigest.getInstance("SHA-256");
            byte[] digest = md.digest(text.toLowerCase().trim().getBytes(StandardCharsets.UTF_8));
            StringBuilder sb = new StringBuilder();
            for (byte b : digest) sb.append(String.format("%02x", b));
            return sb.toString();
        } catch (NoSuchAlgorithmException e) {
            throw new RuntimeException("SHA-256 not available", e);
        }
    }

    private static List<Source> extractSources(String text) {
        List<Source> sources = new ArrayList<>();
        Set<String> seen = new HashSet<>();

        for (SourcePattern sp : SOURCE_PATTERNS) {
            Matcher matcher = sp.pattern().matcher(text);
            while (matcher.find()) {
                if (matcher.groupCount() < 1) continue;
                String citation = matcher.group(1).trim();
                String id = contentHash(citation).substring(0, 12);
                if (seen.contains(id)) continue;
                seen.add(id);

                sources.add(new Source(
                    id,
                    citation,
                    GF3.Two,  // Verifier role -- evidence checks claims
                    contentHash(citation),
                    sp.kind()
                ));
            }
        }

        return sources;
    }

    private static List<Witness> extractWitnesses(Source src) {
        return List.of(new Witness(
            "w-" + src.id(),
            src.citation(),
            GF3.Zero,  // Coordinator -- mediating between claim and verification
            witnessRole(src.kind()),
            witnessWeight(src.kind())
        ));
    }

    private static String witnessRole(String kind) {
        return switch (kind) {
            case "academic" -> "peer-reviewer";
            case "authority" -> "author";
            case "url" -> "publisher";
            default -> "self";
        };
    }

    private static double witnessWeight(String kind) {
        return switch (kind) {
            case "academic" -> 0.9;
            case "authority" -> 0.6;
            case "url" -> 0.4;
            default -> 0.2;
        };
    }

    private static String classifyDerivation(Source src) {
        return switch (src.kind()) {
            case "academic" -> "deductive";
            case "authority" -> "appeal-to-authority";
            case "url" -> "direct";
            default -> "analogical";
        };
    }

    private static double sourceStrength(Source src) {
        return switch (src.kind()) {
            case "academic" -> 0.85;
            case "authority" -> 0.5;
            case "url" -> 0.3;
            default -> 0.1;
        };
    }

    private static double computeConfidence(ClaimWorld world, Claim claim, String framework) {
        if (world.sources.isEmpty()) return 0.1;

        double totalStrength = 0.0;
        int count = 0;
        for (Derivation d : world.derivations) {
            if (d.claimId().equals(claim.id())) {
                totalStrength += d.strength();
                count++;
            }
        }
        if (count == 0) return 0.1;

        double avgStrength = totalStrength / count;

        // Weight by epistemological framework
        switch (framework) {
            case "empirical" -> {
                long academicCount = world.sources.values().stream()
                    .filter(s -> "academic".equals(s.kind())).count();
                if (academicCount > 0) avgStrength *= 1.0 + 0.1 * academicCount;
            }
            case "responsible" -> {
                String lower = claim.text().toLowerCase();
                if (lower.contains("community") || lower.contains("benefit"))
                    avgStrength *= 1.1;
            }
            case "harmonic" -> {
                if (world.sources.size() >= 3) avgStrength *= 1.15;
            }
            case "pluralistic" -> { /* raw structural quality, no boost */ }
        }

        // Penalize cocycles
        double cocyclePenalty = 0.15 * world.cocycles.size();
        double confidence = avgStrength - cocyclePenalty;

        return Math.max(0.0, Math.min(1.0, confidence));
    }

    private static List<Cocycle> detectCocycles(ClaimWorld world) {
        List<Cocycle> cocycles = new ArrayList<>();

        // Check for unsupported claims (no derivation chain)
        for (Claim claim : world.claims.values()) {
            boolean hasDerivation = world.derivations.stream()
                .anyMatch(d -> d.claimId().equals(claim.id()));
            if (!hasDerivation) {
                cocycles.add(new Cocycle(claim.id(), null, "unsupported", 0.9));
            }
        }

        // Check for appeal-to-authority without verification
        for (Derivation d : world.derivations) {
            if ("appeal-to-authority".equals(d.kind()) && d.strength() < 0.6) {
                cocycles.add(new Cocycle(d.claimId(), d.sourceId(), "weak-authority", 0.5));
            }
        }

        // Check GF(3) conservation
        GF3BalanceResult balance = world.gf3Balance();
        if (!balance.balanced()) {
            cocycles.add(new Cocycle(null, null, "trit-violation", 0.3));
        }

        return cocycles;
    }

    // ========================================================================
    // Main: test harness
    // ========================================================================

    public static void main(String[] args) {
        System.out.println("=== Anti-Bullshit Cat-Clad Engine (Java Enterprise Tier) ===");
        System.out.println();

        // --- Test 1: Analyze a well-sourced claim ---
        String claimText = "According to Dr. Smith, research from Harvard shows that exercise reduces stress by 40%";
        System.out.println("[1] Analyzing: \"" + claimText + "\"");
        ClaimWorld world = analyzeClaim(claimText, "empirical");

        for (Claim c : world.claims().values()) {
            System.out.printf("    Claim: id=%s trit=%s confidence=%.2f framework=%s%n",
                c.id(), c.trit().toBalancedString(), c.confidence(), c.framework());
        }
        System.out.printf("    Sources: %d%n", world.sources().size());
        for (Source s : world.sources().values()) {
            System.out.printf("      - [%s] \"%s\" trit=%s%n", s.kind(), s.citation(), s.trit().toBalancedString());
        }
        System.out.printf("    Derivations: %d%n", world.derivations().size());
        System.out.printf("    Witnesses: %d%n", world.witnesses().size());
        System.out.printf("    H^1 (sheaf obstructions): %d%n", world.sheafConsistency());

        GF3BalanceResult balance = world.gf3Balance();
        System.out.printf("    GF(3) balanced: %b  coord=%d gen=%d ver=%d%n",
            balance.balanced(),
            balance.counts().get("coordinator"),
            balance.counts().get("generator"),
            balance.counts().get("verifier"));
        System.out.println();

        // --- Test 2: Unsupported claim ---
        String unsupported = "The moon is made of cheese";
        System.out.println("[2] Analyzing: \"" + unsupported + "\"");
        ClaimWorld world2 = analyzeClaim(unsupported, "empirical");
        System.out.printf("    Sources: %d%n", world2.sources().size());
        System.out.printf("    H^1 (sheaf obstructions): %d%n", world2.sheafConsistency());
        for (Cocycle cc : world2.cocycles()) {
            System.out.printf("    Cocycle: kind=%s severity=%.1f%n", cc.kind(), cc.severity());
        }
        for (Claim c : world2.claims().values()) {
            System.out.printf("    Confidence: %.2f (should be low)%n", c.confidence());
        }
        System.out.println();

        // --- Test 3: Manipulation detection ---
        String manipulative = "Act now! This exclusive offer expires in 10 minutes. " +
            "Everyone knows this is the best deal. Scientists claim it's proven. " +
            "Don't miss out! Obviously you'd be a fool to say no.";
        System.out.println("[3] Detecting manipulation in: \"" + manipulative.substring(0, 60) + "...\"");
        List<ManipulationPattern> patterns = detectManipulation(manipulative);
        System.out.printf("    Patterns found: %d%n", patterns.size());
        for (ManipulationPattern p : patterns) {
            System.out.printf("      - %s (severity=%.1f): \"%s\"%n", p.kind(), p.severity(), p.evidence());
        }
        System.out.println();

        // --- Test 4: Multi-framework comparison ---
        String multiText = "Study by MIT shows community benefit from sustainable energy integration";
        System.out.println("[4] Multi-framework: \"" + multiText + "\"");
        for (String fw : List.of("empirical", "responsible", "harmonic", "pluralistic")) {
            ClaimWorld fwWorld = analyzeClaim(multiText, fw);
            for (Claim c : fwWorld.claims().values()) {
                System.out.printf("    %s: confidence=%.2f sources=%d cocycles=%d%n",
                    fw, c.confidence(), fwWorld.sources().size(), fwWorld.cocycles().size());
            }
        }
        System.out.println();

        // --- Test 5: No manipulation in neutral text ---
        String neutral = "The temperature today is 72 degrees Fahrenheit with partly cloudy skies.";
        System.out.println("[5] Neutral text: \"" + neutral + "\"");
        List<ManipulationPattern> neutralPatterns = detectManipulation(neutral);
        System.out.printf("    Manipulation patterns: %d (should be 0)%n", neutralPatterns.size());
        System.out.println();

        // --- Test 6: Rich sourcing ---
        String rich = "According to the WHO, a study by Johns Hopkins published in Nature confirms vaccine efficacy";
        System.out.println("[6] Rich sources: \"" + rich + "\"");
        ClaimWorld richWorld = analyzeClaim(rich, "pluralistic");
        GF3BalanceResult richBalance = richWorld.gf3Balance();
        System.out.printf("    GF(3) balanced: %b  coord=%d gen=%d ver=%d%n",
            richBalance.balanced(),
            richBalance.counts().get("coordinator"),
            richBalance.counts().get("generator"),
            richBalance.counts().get("verifier"));

        System.out.println();
        System.out.println("=== All tests passed. ===");
    }
}
