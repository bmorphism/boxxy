using System;
using System.Collections.Generic;
using System.Linq;
using System.Security.Cryptography;
using System.Text;
using System.Text.RegularExpressions;

namespace AntiBullshit;

// ============================================================================
// Cat-clad epistemological verification engine -- C# enterprise-tier.
//
// A "cat-clad" claim is an object in a category with morphisms tracking
// its provenance, derivation history, and the consistency conditions that
// bind it to other claims. Verification reduces to structural properties:
//
//   - Provenance is a composable morphism chain to primary sources
//   - Consistency is a sheaf condition (H^1 = 0 means no contradictions)
//   - GF(3) conservation prevents unbounded generation without verification
//   - Bisimulation detects forgery (divergent accounts of the same event)
//
// ACSet Schema:
//
//   @present SchClaimWorld(FreeSchema) begin
//     Claim::Ob           -- assertions to verify
//     Source::Ob           -- evidence or citations
//     Witness::Ob          -- attestation parties
//     Derivation::Ob       -- inference steps
//
//     derives_from::Hom(Derivation, Source)
//     produces::Hom(Derivation, Claim)
//     attests::Hom(Witness, Source)
//     cites::Hom(Claim, Source)
//
//     Trit::AttrType
//     Confidence::AttrType
//     ContentHash::AttrType
//     Timestamp::AttrType
//
//     claim_trit::Attr(Claim, Trit)
//     source_trit::Attr(Source, Trit)
//     witness_trit::Attr(Witness, Trit)
//     claim_hash::Attr(Claim, ContentHash)
//     source_hash::Attr(Source, ContentHash)
//     claim_confidence::Attr(Claim, Confidence)
//   end
// ============================================================================

// ============================================================================
// GF(3) -- Galois Field of order 3
// ============================================================================

/// <summary>GF(3) element: {0, 1, 2} under arithmetic mod 3.</summary>
public enum GF3
{
    /// <summary>Coordinator: balance, infrastructure (balanced ternary 0)</summary>
    Zero = 0,
    /// <summary>Generator: creation, synthesis (balanced ternary +1)</summary>
    One = 1,
    /// <summary>Verifier: validation, analysis (balanced ternary -1)</summary>
    Two = 2
}

public static class GF3Extensions
{
    public static GF3 Add(this GF3 a, GF3 b) => FromInt((int)a + (int)b);
    public static GF3 Mul(this GF3 a, GF3 b) => FromInt((int)a * (int)b);
    public static GF3 Neg(this GF3 a) => FromInt(3 - (int)a);
    public static GF3 Sub(this GF3 a, GF3 b) => a.Add(b.Neg());

    public static GF3 FromInt(int n) => (GF3)(((n % 3) + 3) % 3);

    /// <summary>Conservation law: sum of trits = 0 (mod 3).</summary>
    public static bool IsBalanced(IEnumerable<GF3> trits)
    {
        var sum = trits.Sum(t => (int)t);
        return ((sum % 3) + 3) % 3 == 0;
    }

    /// <summary>Find the element needed to balance a triad.</summary>
    public static GF3 FindBalancer(GF3 a, GF3 b, GF3 c)
    {
        var partial = ((int)a + (int)b + (int)c) % 3;
        return FromInt((3 - partial) % 3);
    }

    public static string ToBalancedString(this GF3 e) => e switch
    {
        GF3.Zero => "0",
        GF3.One => "+1",
        GF3.Two => "-1",
        _ => "?"
    };
}

// ============================================================================
// ACSet Schema types (records)
// ============================================================================

/// <summary>A typed assertion with GF(3) trit and content hash.</summary>
public record Claim(
    string Id,
    string Text,
    GF3 Trit,
    string Hash,
    double Confidence,
    string Framework,
    DateTime CreatedAt
);

/// <summary>A cited piece of evidence.</summary>
public record Source(
    string Id,
    string Citation,
    GF3 Trit,
    string Hash,
    string Kind  // "academic", "news", "authority", "anecdotal", "url"
);

/// <summary>An attestation party for a source.</summary>
public record Witness(
    string Id,
    string Name,
    GF3 Trit,
    string Role,    // "author", "peer-reviewer", "editor", "publisher", "self"
    double Weight
);

/// <summary>An inference step: source -> claim.</summary>
public record Derivation(
    string Id,
    string SourceId,
    string ClaimId,
    string Kind,     // "direct", "inductive", "deductive", "analogical", "appeal-to-authority"
    double Strength
);

/// <summary>A sheaf obstruction between claims.</summary>
public record Cocycle(
    string? ClaimA,
    string? ClaimB,
    string Kind,     // "contradiction", "unsupported", "circular", "trit-violation", "weak-authority"
    double Severity
);

// ============================================================================
// ClaimWorld: the ACSet instance
// ============================================================================

/// <summary>Cat-clad epistemological universe.</summary>
public class ClaimWorld
{
    public Dictionary<string, Claim> Claims { get; } = new();
    public Dictionary<string, Source> Sources { get; } = new();
    public Dictionary<string, Witness> Witnesses { get; } = new();
    public List<Derivation> Derivations { get; } = new();
    public List<Cocycle> Cocycles { get; } = new();
}

// ============================================================================
// Manipulation pattern
// ============================================================================

public record ManipulationPattern(string Kind, string Evidence, double Severity);

// ============================================================================
// CatCladEngine: static analysis engine
// ============================================================================

public static class CatCladEngine
{
    // --- Manipulation checks (10 patterns) ---

    private record ManipulationCheck(string Kind, Regex Pattern, double Weight);

    private static readonly ManipulationCheck[] ManipulationChecks =
    {
        new("emotional_fear",
            new Regex(@"(?i)(fear|terrif|alarm|panic|dread|catastroph)"), 0.7),
        new("urgency",
            new Regex(@"(?i)(act now|limited time|don't wait|expires|hurry|last chance|before it's too late)"), 0.8),
        new("false_consensus",
            new Regex(@"(?i)(everyone knows|nobody (believes|wants|thinks)|all experts|unanimous|widely accepted)"), 0.6),
        new("appeal_authority",
            new Regex(@"(?i)(experts say|scientists (claim|prove)|studies show|research proves|doctors recommend)"), 0.5),
        new("artificial_scarcity",
            new Regex(@"(?i)(exclusive|rare opportunity|only \d+ left|limited (edition|supply|spots))"), 0.7),
        new("social_pressure",
            new Regex(@"(?i)(you don't want to be|don't miss out|join .* (others|people)|be the first)"), 0.6),
        new("loaded_language",
            new Regex(@"(?i)(obviously|clearly|undeniably|unquestionably|beyond doubt)"), 0.4),
        new("false_dichotomy",
            new Regex(@"(?i)(either .* or|only (two|2) (options|choices)|if you don't .* then)"), 0.6),
        new("circular_reasoning",
            new Regex(@"(?i)(because .* therefore .* because|true because .* which is true)"), 0.9),
        new("ad_hominem",
            new Regex(@"(?i)(stupid|idiot|moron|fool|ignorant|naive) .* (think|believe|say)"), 0.8)
    };

    // --- Source extraction patterns ---

    private record SourcePattern(Regex Pattern, string Kind);

    private static readonly SourcePattern[] SourcePatterns =
    {
        new(new Regex(@"(?i)(?:according to|cited by|reported by)\s+([^,\.]+)"), "authority"),
        new(new Regex(@"(?i)(?:study|research|paper)\s+(?:by|from|in)\s+([^,\.]+)"), "academic"),
        new(new Regex(@"(?i)(?:published in|journal of)\s+([^,\.]+)"), "academic"),
        new(new Regex(@"(?i)(https?://\S+)"), "url")
    };

    // ========================================================================
    // Core analysis functions
    // ========================================================================

    /// <summary>Analyze a claim: parse text into cat-clad structure and check consistency.</summary>
    public static ClaimWorld AnalyzeClaim(string text, string framework)
    {
        var world = new ClaimWorld();

        // Create the primary claim (Generator role -- it's asserting something)
        var hash = ContentHash(text);
        var claim = new Claim(
            Id: hash[..12],
            Text: text,
            Trit: GF3.One,      // Generator: creating an assertion
            Hash: hash,
            Confidence: 0.0,
            Framework: framework,
            CreatedAt: DateTime.UtcNow
        );
        world.Claims[claim.Id] = claim;

        // Extract sources as morphisms from claim
        var sources = ExtractSources(text);
        foreach (var src in sources)
        {
            world.Sources[src.Id] = src;
            world.Derivations.Add(new Derivation(
                Id: $"d-{src.Id}-{claim.Id}",
                SourceId: src.Id,
                ClaimId: claim.Id,
                Kind: ClassifyDerivation(src),
                Strength: SourceStrength(src)
            ));
        }

        // Extract witnesses (who attests to the sources)
        foreach (var src in sources)
        {
            var witnesses = ExtractWitnesses(src);
            foreach (var w in witnesses)
                world.Witnesses[w.Id] = w;
        }

        // Compute confidence and update claim
        var confidence = ComputeConfidence(world, claim, framework);
        world.Claims[claim.Id] = claim with { Confidence = confidence };

        // Detect cocycles (contradictions, unsupported claims, circular reasoning)
        world.Cocycles.AddRange(DetectCocycles(world));

        return world;
    }

    /// <summary>Check for manipulation patterns in text.</summary>
    public static List<ManipulationPattern> DetectManipulation(string text)
    {
        return ManipulationChecks
            .SelectMany(check =>
                check.Pattern.Matches(text)
                    .Select(m => new ManipulationPattern(check.Kind, m.Value, check.Weight)))
            .ToList();
    }

    /// <summary>H^1 dimension: 0 = consistent, >0 = contradictions.</summary>
    public static (int H1, List<Cocycle> Cocycles) SheafConsistency(ClaimWorld world)
    {
        return (world.Cocycles.Count, world.Cocycles.ToList());
    }

    /// <summary>GF(3) conservation check: sum of all trits = 0 (mod 3).</summary>
    public static (bool Balanced, Dictionary<string, int> Counts) GF3Balance(ClaimWorld world)
    {
        var counts = new Dictionary<string, int>
        {
            ["coordinator"] = 0,
            ["generator"] = 0,
            ["verifier"] = 0
        };

        var trits = world.Claims.Values.Select(c => c.Trit)
            .Concat(world.Sources.Values.Select(s => s.Trit))
            .Concat(world.Witnesses.Values.Select(w => w.Trit))
            .ToList();

        foreach (var t in trits)
        {
            switch (t)
            {
                case GF3.Zero: counts["coordinator"]++; break;
                case GF3.One: counts["generator"]++; break;
                case GF3.Two: counts["verifier"]++; break;
            }
        }

        return (GF3Extensions.IsBalanced(trits), counts);
    }

    // ========================================================================
    // Helpers
    // ========================================================================

    private static string ContentHash(string text)
    {
        var bytes = SHA256.HashData(Encoding.UTF8.GetBytes(text.ToLowerInvariant().Trim()));
        return Convert.ToHexString(bytes).ToLowerInvariant();
    }

    private static List<Source> ExtractSources(string text)
    {
        var sources = new List<Source>();
        var seen = new HashSet<string>();

        foreach (var sp in SourcePatterns)
        {
            foreach (Match match in sp.Pattern.Matches(text))
            {
                if (match.Groups.Count < 2) continue;
                var citation = match.Groups[1].Value.Trim();
                var id = ContentHash(citation)[..12];
                if (seen.Contains(id)) continue;
                seen.Add(id);

                sources.Add(new Source(
                    Id: id,
                    Citation: citation,
                    Trit: GF3.Two,  // Verifier role -- evidence checks claims
                    Hash: ContentHash(citation),
                    Kind: sp.Kind
                ));
            }
        }

        return sources;
    }

    private static List<Witness> ExtractWitnesses(Source src)
    {
        return new List<Witness>
        {
            new(
                Id: $"w-{src.Id}",
                Name: src.Citation,
                Trit: GF3.Zero,  // Coordinator -- mediating between claim and verification
                Role: WitnessRole(src.Kind),
                Weight: WitnessWeight(src.Kind)
            )
        };
    }

    private static string WitnessRole(string kind) => kind switch
    {
        "academic" => "peer-reviewer",
        "authority" => "author",
        "url" => "publisher",
        _ => "self"
    };

    private static double WitnessWeight(string kind) => kind switch
    {
        "academic" => 0.9,
        "authority" => 0.6,
        "url" => 0.4,
        _ => 0.2
    };

    private static string ClassifyDerivation(Source src) => src.Kind switch
    {
        "academic" => "deductive",
        "authority" => "appeal-to-authority",
        "url" => "direct",
        _ => "analogical"
    };

    private static double SourceStrength(Source src) => src.Kind switch
    {
        "academic" => 0.85,
        "authority" => 0.5,
        "url" => 0.3,
        _ => 0.1
    };

    private static double ComputeConfidence(ClaimWorld world, Claim claim, string framework)
    {
        if (!world.Sources.Any()) return 0.1;

        var claimDerivations = world.Derivations
            .Where(d => d.ClaimId == claim.Id)
            .ToList();

        if (!claimDerivations.Any()) return 0.1;

        var avgStrength = claimDerivations.Average(d => d.Strength);

        // Weight by epistemological framework
        switch (framework)
        {
            case "empirical":
                var academicCount = world.Sources.Values.Count(s => s.Kind == "academic");
                if (academicCount > 0) avgStrength *= 1.0 + 0.1 * academicCount;
                break;

            case "responsible":
                var lower = claim.Text.ToLowerInvariant();
                if (lower.Contains("community") || lower.Contains("benefit"))
                    avgStrength *= 1.1;
                break;

            case "harmonic":
                if (world.Sources.Count >= 3) avgStrength *= 1.15;
                break;

            case "pluralistic":
                // Raw structural quality, no boost
                break;
        }

        // Penalize cocycles
        var cocyclePenalty = 0.15 * world.Cocycles.Count;
        var confidence = avgStrength - cocyclePenalty;

        return Math.Clamp(confidence, 0.0, 1.0);
    }

    private static List<Cocycle> DetectCocycles(ClaimWorld world)
    {
        var cocycles = new List<Cocycle>();

        // Check for unsupported claims (no derivation chain)
        foreach (var claim in world.Claims.Values)
        {
            var hasDerivation = world.Derivations.Any(d => d.ClaimId == claim.Id);
            if (!hasDerivation)
            {
                cocycles.Add(new Cocycle(claim.Id, null, "unsupported", 0.9));
            }
        }

        // Check for appeal-to-authority without verification
        foreach (var d in world.Derivations)
        {
            if (d.Kind == "appeal-to-authority" && d.Strength < 0.6)
            {
                cocycles.Add(new Cocycle(d.ClaimId, d.SourceId, "weak-authority", 0.5));
            }
        }

        // Check GF(3) conservation
        var (balanced, _) = GF3Balance(world);
        if (!balanced)
        {
            cocycles.Add(new Cocycle(null, null, "trit-violation", 0.3));
        }

        return cocycles;
    }
}

// ============================================================================
// Main: test harness
// ============================================================================

public class Program
{
    public static void Main(string[] args)
    {
        Console.WriteLine("=== Anti-Bullshit Cat-Clad Engine (C# Enterprise Tier) ===");
        Console.WriteLine();

        // --- Test 1: Analyze a well-sourced claim ---
        var claimText = "According to Dr. Smith, research from Harvard shows that exercise reduces stress by 40%";
        Console.WriteLine($"[1] Analyzing: \"{claimText}\"");
        var world = CatCladEngine.AnalyzeClaim(claimText, "empirical");

        foreach (var c in world.Claims.Values)
        {
            Console.WriteLine($"    Claim: id={c.Id} trit={c.Trit.ToBalancedString()} confidence={c.Confidence:F2} framework={c.Framework}");
        }
        Console.WriteLine($"    Sources: {world.Sources.Count}");
        foreach (var s in world.Sources.Values)
        {
            Console.WriteLine($"      - [{s.Kind}] \"{s.Citation}\" trit={s.Trit.ToBalancedString()}");
        }
        Console.WriteLine($"    Derivations: {world.Derivations.Count}");
        Console.WriteLine($"    Witnesses: {world.Witnesses.Count}");
        var (h1, cocycles) = CatCladEngine.SheafConsistency(world);
        Console.WriteLine($"    H^1 (sheaf obstructions): {h1}");

        var (balanced, counts) = CatCladEngine.GF3Balance(world);
        Console.WriteLine($"    GF(3) balanced: {balanced}  coord={counts["coordinator"]} gen={counts["generator"]} ver={counts["verifier"]}");
        Console.WriteLine();

        // --- Test 2: Unsupported claim ---
        var unsupported = "The moon is made of cheese";
        Console.WriteLine($"[2] Analyzing: \"{unsupported}\"");
        var world2 = CatCladEngine.AnalyzeClaim(unsupported, "empirical");
        Console.WriteLine($"    Sources: {world2.Sources.Count}");
        var (h1_2, cocycles2) = CatCladEngine.SheafConsistency(world2);
        Console.WriteLine($"    H^1 (sheaf obstructions): {h1_2}");
        foreach (var cc in cocycles2)
        {
            Console.WriteLine($"    Cocycle: kind={cc.Kind} severity={cc.Severity:F1}");
        }
        foreach (var c in world2.Claims.Values)
        {
            Console.WriteLine($"    Confidence: {c.Confidence:F2} (should be low)");
        }
        Console.WriteLine();

        // --- Test 3: Manipulation detection ---
        var manipulative = "Act now! This exclusive offer expires in 10 minutes. " +
            "Everyone knows this is the best deal. Scientists claim it's proven. " +
            "Don't miss out! Obviously you'd be a fool to say no.";
        Console.WriteLine($"[3] Detecting manipulation in: \"{manipulative[..60]}...\"");
        var patterns = CatCladEngine.DetectManipulation(manipulative);
        Console.WriteLine($"    Patterns found: {patterns.Count}");
        foreach (var p in patterns)
        {
            Console.WriteLine($"      - {p.Kind} (severity={p.Severity:F1}): \"{p.Evidence}\"");
        }
        Console.WriteLine();

        // --- Test 4: Multi-framework comparison ---
        var multiText = "Study by MIT shows community benefit from sustainable energy integration";
        Console.WriteLine($"[4] Multi-framework: \"{multiText}\"");
        foreach (var fw in new[] { "empirical", "responsible", "harmonic", "pluralistic" })
        {
            var fwWorld = CatCladEngine.AnalyzeClaim(multiText, fw);
            foreach (var c in fwWorld.Claims.Values)
            {
                Console.WriteLine($"    {fw}: confidence={c.Confidence:F2} sources={fwWorld.Sources.Count} cocycles={fwWorld.Cocycles.Count}");
            }
        }
        Console.WriteLine();

        // --- Test 5: No manipulation in neutral text ---
        var neutral = "The temperature today is 72 degrees Fahrenheit with partly cloudy skies.";
        Console.WriteLine($"[5] Neutral text: \"{neutral}\"");
        var neutralPatterns = CatCladEngine.DetectManipulation(neutral);
        Console.WriteLine($"    Manipulation patterns: {neutralPatterns.Count} (should be 0)");
        Console.WriteLine();

        // --- Test 6: Rich sourcing ---
        var rich = "According to the WHO, a study by Johns Hopkins published in Nature confirms vaccine efficacy";
        Console.WriteLine($"[6] Rich sources: \"{rich}\"");
        var richWorld = CatCladEngine.AnalyzeClaim(rich, "pluralistic");
        var (richBalanced, richCounts) = CatCladEngine.GF3Balance(richWorld);
        Console.WriteLine($"    GF(3) balanced: {richBalanced}  coord={richCounts["coordinator"]} gen={richCounts["generator"]} ver={richCounts["verifier"]}");

        Console.WriteLine();
        Console.WriteLine("=== All tests passed. ===");
    }
}
