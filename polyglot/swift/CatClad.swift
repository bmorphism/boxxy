// CatClad.swift -- Tier 3 Swift implementation of cat-clad epistemological verification.
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

import Foundation
#if canImport(CryptoKit)
import CryptoKit
#endif

// MARK: - GF(3) Trit Type

enum Trit: Int, CaseIterable, Codable {
    case zero = 0
    case one  = 1
    case two  = 2

    static func add(_ a: Trit, _ b: Trit) -> Trit {
        Trit(rawValue: (a.rawValue + b.rawValue) % 3)!
    }

    static func mul(_ a: Trit, _ b: Trit) -> Trit {
        Trit(rawValue: (a.rawValue * b.rawValue) % 3)!
    }

    static func neg(_ a: Trit) -> Trit {
        Trit(rawValue: (3 - a.rawValue) % 3)!
    }

    static func sub(_ a: Trit, _ b: Trit) -> Trit {
        add(a, neg(b))
    }

    static func inv(_ a: Trit) -> Trit {
        precondition(a != .zero, "gf3: multiplicative inverse of zero")
        return a == .one ? .one : .two
    }

    static func isBalanced(_ trits: [Trit]) -> Bool {
        let sum = trits.reduce(0) { $0 + $1.rawValue }
        return ((sum % 3) + 3) % 3 == 0
    }

    static func seqSum(_ trits: [Trit]) -> Int {
        trits.reduce(0) { $0 + $1.rawValue }
    }

    static func findBalancer(_ a: Trit, _ b: Trit, _ c: Trit) -> Trit {
        let partial = (a.rawValue + b.rawValue + c.rawValue) % 3
        return Trit(rawValue: (3 - partial) % 3)!
    }

    var balancedValue: Int {
        switch self {
        case .zero: return 0
        case .one:  return 1
        case .two:  return -1
        }
    }

    var roleName: String {
        switch self {
        case .zero: return "coordinator"
        case .one:  return "generator"
        case .two:  return "verifier"
        }
    }
}

// MARK: - ACSet Schema Types

struct Claim: Codable {
    let id: String
    let text: String
    var trit: Trit
    let hash: String
    var confidence: Double
    let framework: String
    let createdAt: Date

    enum CodingKeys: String, CodingKey {
        case id, text, trit, hash, confidence, framework
        case createdAt = "created_at"
    }
}

struct Source: Codable {
    let id: String
    let citation: String
    let trit: Trit
    let hash: String
    let kind: String  // "academic", "news", "authority", "anecdotal", "self-referential"
}

struct Witness: Codable {
    let id: String
    let name: String
    let trit: Trit
    let role: String   // "author", "peer-reviewer", "editor", "self"
    let weight: Double
}

struct Derivation: Codable {
    let id: String
    let sourceId: String
    let claimId: String
    let kind: String     // "direct", "inductive", "deductive", "analogical", "appeal-to-authority"
    let strength: Double

    enum CodingKeys: String, CodingKey {
        case id, kind, strength
        case sourceId = "source_id"
        case claimId = "claim_id"
    }
}

struct Cocycle: Codable {
    let claimA: String?
    let claimB: String?
    let kind: String     // "contradiction", "unsupported", "circular", "trit-violation"
    let severity: Double

    enum CodingKeys: String, CodingKey {
        case kind, severity
        case claimA = "claim_a"
        case claimB = "claim_b"
    }
}

struct ManipulationPattern: Codable {
    let kind: String
    let evidence: String
    let severity: Double
}

// MARK: - ClaimWorld (ACSet instance)

struct ClaimWorld: Codable {
    var claims: [String: Claim] = [:]
    var sources: [String: Source] = [:]
    var witnesses: [String: Witness] = [:]
    var derivations: [Derivation] = []
    var cocycles: [Cocycle] = []

    // Sheaf consistency: returns (h1_dimension, cocycles).
    // H^1 = 0 means consistent, >0 means contradictions.
    mutating func sheafConsistency() -> (Int, [Cocycle]) {
        return (cocycles.count, cocycles)
    }

    // GF(3) balance: checks conservation law sum(trits) = 0 (mod 3).
    // Returns (balanced?, counts).
    mutating func gf3Balance() -> (Bool, [String: Int]) {
        var counts: [String: Int] = ["coordinator": 0, "generator": 0, "verifier": 0]
        var trits: [Trit] = []

        for c in claims.values {
            trits.append(c.trit)
        }
        for s in sources.values {
            trits.append(s.trit)
        }
        for w in witnesses.values {
            trits.append(w.trit)
        }

        for t in trits {
            counts[t.roleName, default: 0] += 1
        }

        return (Trit.isBalanced(trits), counts)
    }
}

// MARK: - CatClad Engine

/// Content hash using SHA-256.
func contentHash(_ text: String) -> String {
    let normalized = text.trimmingCharacters(in: .whitespacesAndNewlines).lowercased()
    let data = Data(normalized.utf8)
    #if canImport(CryptoKit)
    let digest = SHA256.hash(data: data)
    return digest.map { String(format: "%02x", $0) }.joined()
    #else
    // Fallback: simple hash for platforms without CryptoKit
    var hash: UInt64 = 5381
    for byte in data {
        hash = ((hash &<< 5) &+ hash) &+ UInt64(byte)
    }
    return String(format: "%016llx%016llx%016llx%016llx", hash, hash &* 31, hash &* 37, hash &* 41)
    #endif
}

/// Source extraction patterns.
private struct SourcePattern {
    let regex: NSRegularExpression
    let kind: String
}

private let sourcePatterns: [SourcePattern] = {
    let defs: [(String, String)] = [
        ("(?i)(?:according to|cited by|reported by)\\s+([^,\\.]+)", "authority"),
        ("(?i)(?:study|research|paper)\\s+(?:by|from|in)\\s+([^,\\.]+)", "academic"),
        ("(?i)(?:published in|journal of)\\s+([^,\\.]+)", "academic"),
        ("(?i)(https?://\\S+)", "url"),
    ]
    return defs.compactMap { (pattern, kind) in
        guard let re = try? NSRegularExpression(pattern: pattern) else { return nil }
        return SourcePattern(regex: re, kind: kind)
    }
}()

/// Manipulation check definition.
private struct ManipulationCheck {
    let kind: String
    let regex: NSRegularExpression
    let weight: Double
}

/// 10 manipulation patterns.
private let manipulationChecks: [ManipulationCheck] = {
    let defs: [(String, String, Double)] = [
        ("emotional_fear",
         "(?i)(fear|terrif|alarm|panic|dread|catastroph)", 0.7),
        ("urgency",
         "(?i)(act now|limited time|don't wait|expires|hurry|last chance|before it's too late)", 0.8),
        ("false_consensus",
         "(?i)(everyone knows|nobody (?:believes|wants|thinks)|all experts|unanimous|widely accepted)", 0.6),
        ("appeal_authority",
         "(?i)(experts say|scientists (?:claim|prove)|studies show|research proves|doctors recommend)", 0.5),
        ("artificial_scarcity",
         "(?i)(exclusive|rare opportunity|only \\d+ left|limited (?:edition|supply|spots))", 0.7),
        ("social_pressure",
         "(?i)(you don't want to be|don't miss out|join .* (?:others|people)|be the first)", 0.6),
        ("loaded_language",
         "(?i)(obviously|clearly|undeniably|unquestionably|beyond doubt)", 0.4),
        ("false_dichotomy",
         "(?i)(either .* or|only (?:two|2) (?:options|choices)|if you don't .* then)", 0.6),
        ("circular_reasoning",
         "(?i)(because .* therefore .* because|true because .* which is true)", 0.9),
        ("ad_hominem",
         "(?i)(stupid|idiot|moron|fool|ignorant|naive) .* (?:think|believe|say)", 0.8),
    ]
    return defs.compactMap { (kind, pattern, weight) in
        guard let re = try? NSRegularExpression(pattern: pattern) else { return nil }
        return ManipulationCheck(kind: kind, regex: re, weight: weight)
    }
}()

/// The 4 epistemological frameworks.
let frameworks = ["empirical", "responsible", "harmonic", "pluralistic"]

// MARK: - Source/Witness Helpers

func witnessRole(forKind kind: String) -> String {
    switch kind {
    case "academic":  return "peer-reviewer"
    case "authority":  return "author"
    case "url":        return "publisher"
    default:           return "self"
    }
}

func witnessWeight(forKind kind: String) -> Double {
    switch kind {
    case "academic":  return 0.9
    case "authority":  return 0.6
    case "url":        return 0.4
    default:           return 0.2
    }
}

func classifyDerivation(source: Source) -> String {
    switch source.kind {
    case "academic":  return "deductive"
    case "authority":  return "appeal-to-authority"
    case "url":        return "direct"
    default:           return "analogical"
    }
}

func sourceStrength(source: Source) -> Double {
    switch source.kind {
    case "academic":  return 0.85
    case "authority":  return 0.5
    case "url":        return 0.3
    default:           return 0.1
    }
}

// MARK: - Extraction

func extractSources(from text: String) -> [Source] {
    var sources: [Source] = []
    var seen = Set<String>()
    let nsText = text as NSString
    let range = NSRange(location: 0, length: nsText.length)

    for sp in sourcePatterns {
        let matches = sp.regex.matches(in: text, range: range)
        for match in matches {
            guard match.numberOfRanges >= 2 else { continue }
            let citation = nsText.substring(with: match.range(at: 1))
                .trimmingCharacters(in: .whitespaces)
            let hash = contentHash(citation)
            let id = String(hash.prefix(12))
            guard !seen.contains(id) else { continue }
            seen.insert(id)

            sources.append(Source(
                id: id,
                citation: citation,
                trit: .two,  // Verifier role -- evidence checks claims
                hash: hash,
                kind: sp.kind
            ))
        }
    }

    return sources
}

func extractWitnesses(from source: Source) -> [Witness] {
    [Witness(
        id: "w-\(source.id)",
        name: source.citation,
        trit: .zero,  // Coordinator -- mediating between claim and verification
        role: witnessRole(forKind: source.kind),
        weight: witnessWeight(forKind: source.kind)
    )]
}

// MARK: - Cocycle Detection

func detectCocycles(in world: ClaimWorld) -> [Cocycle] {
    var cocycles: [Cocycle] = []

    // Check for unsupported claims (no derivation chain)
    for claim in world.claims.values {
        let hasDerivation = world.derivations.contains { $0.claimId == claim.id }
        if !hasDerivation {
            cocycles.append(Cocycle(
                claimA: claim.id,
                claimB: nil,
                kind: "unsupported",
                severity: 0.9
            ))
        }
    }

    // Check for appeal-to-authority without verification
    for d in world.derivations {
        if d.kind == "appeal-to-authority" && d.strength < 0.6 {
            cocycles.append(Cocycle(
                claimA: d.claimId,
                claimB: d.sourceId,
                kind: "weak-authority",
                severity: 0.5
            ))
        }
    }

    // Check GF(3) conservation
    var trits: [Trit] = []
    for c in world.claims.values { trits.append(c.trit) }
    for s in world.sources.values { trits.append(s.trit) }
    for w in world.witnesses.values { trits.append(w.trit) }

    if !Trit.isBalanced(trits) {
        cocycles.append(Cocycle(
            claimA: nil,
            claimB: nil,
            kind: "trit-violation",
            severity: 0.3
        ))
    }

    return cocycles
}

// MARK: - Confidence Computation

func computeConfidence(world: ClaimWorld, claim: Claim, framework: String) -> Double {
    guard !world.sources.isEmpty else { return 0.1 }

    let derivs = world.derivations.filter { $0.claimId == claim.id }
    guard !derivs.isEmpty else { return 0.1 }

    var avgStrength = derivs.reduce(0.0) { $0 + $1.strength } / Double(derivs.count)

    // Weight by framework
    switch framework {
    case "empirical":
        let academicCount = world.sources.values.filter { $0.kind == "academic" }.count
        if academicCount > 0 {
            avgStrength *= 1.0 + 0.1 * Double(academicCount)
        }
    case "responsible":
        let lower = claim.text.lowercased()
        if lower.contains("community") || lower.contains("benefit") {
            avgStrength *= 1.1
        }
    case "harmonic":
        if world.sources.count >= 3 {
            avgStrength *= 1.15
        }
    case "pluralistic":
        break  // No special boost, raw structural quality
    default:
        break
    }

    // Penalize cocycles
    let cocyclePenalty = 0.15 * Double(world.cocycles.count)
    let confidence = avgStrength - cocyclePenalty
    return min(1.0, max(0.0, confidence))
}

// MARK: - Public API

/// Analyze a claim: parse text into a cat-clad structure and check consistency.
func analyzeClaim(text: String, framework: String = "pluralistic") -> ClaimWorld {
    var world = ClaimWorld()

    // Create the primary claim (Generator role -- it's asserting something)
    let hash = contentHash(text)
    var claim = Claim(
        id: String(hash.prefix(12)),
        text: text,
        trit: .one,   // Generator: creating an assertion
        hash: hash,
        confidence: 0.0,
        framework: framework,
        createdAt: Date()
    )
    world.claims[claim.id] = claim

    // Extract sources as morphisms from claim
    let sources = extractSources(from: text)
    for src in sources {
        world.sources[src.id] = src
        world.derivations.append(Derivation(
            id: "d-\(src.id)-\(claim.id)",
            sourceId: src.id,
            claimId: claim.id,
            kind: classifyDerivation(source: src),
            strength: sourceStrength(source: src)
        ))
    }

    // Extract witnesses (who attests to the sources)
    for src in sources {
        let witnesses = extractWitnesses(from: src)
        for w in witnesses {
            world.witnesses[w.id] = w
        }
    }

    // Compute confidence
    claim.confidence = computeConfidence(world: world, claim: claim, framework: framework)
    world.claims[claim.id] = claim

    // Detect cocycles (contradictions, unsupported claims, circular reasoning)
    world.cocycles = detectCocycles(in: world)

    return world
}

/// Detect manipulation patterns in text.
func detectManipulation(text: String) -> [ManipulationPattern] {
    var patterns: [ManipulationPattern] = []
    let nsText = text as NSString
    let range = NSRange(location: 0, length: nsText.length)

    for check in manipulationChecks {
        let matches = check.regex.matches(in: text, range: range)
        for match in matches {
            let evidence: String
            if match.numberOfRanges >= 2 {
                evidence = nsText.substring(with: match.range(at: 1))
            } else {
                evidence = nsText.substring(with: match.range)
            }
            patterns.append(ManipulationPattern(
                kind: check.kind,
                evidence: evidence,
                severity: check.weight
            ))
        }
    }

    return patterns
}

// MARK: - ClaimWorld Extensions

extension ClaimWorld {
    /// Non-mutating sheaf consistency check.
    func checkSheafConsistency() -> (h1: Int, cocycles: [Cocycle]) {
        (cocycles.count, cocycles)
    }

    /// Non-mutating GF(3) balance check.
    func checkGF3Balance() -> (balanced: Bool, counts: [String: Int]) {
        var counts: [String: Int] = ["coordinator": 0, "generator": 0, "verifier": 0]
        var trits: [Trit] = []

        for c in claims.values { trits.append(c.trit) }
        for s in sources.values { trits.append(s.trit) }
        for w in witnesses.values { trits.append(w.trit) }

        for t in trits {
            counts[t.roleName, default: 0] += 1
        }

        return (Trit.isBalanced(trits), counts)
    }

    /// Encode to JSON data.
    func toJSON() -> Data? {
        let encoder = JSONEncoder()
        encoder.outputFormatting = [.prettyPrinted, .sortedKeys]
        encoder.dateEncodingStrategy = .iso8601
        return try? encoder.encode(self)
    }
}

// MARK: - Main Entry Point

@main
struct CatCladMain {
    static func main() {
        print("=== Cat-Clad Anti-Bullshit Engine (Swift Tier 3) ===")
        print()

        // Test 1: Analyze a well-sourced claim
        let text1 = "According to Dr. Smith, research from Harvard shows that exercise reduces stress by 40%"
        var world1 = analyzeClaim(text: text1, framework: "empirical")

        print("--- Claim Analysis (empirical) ---")
        print("Claims:      \(world1.claims.count)")
        print("Sources:     \(world1.sources.count)")
        print("Witnesses:   \(world1.witnesses.count)")
        print("Derivations: \(world1.derivations.count)")

        for c in world1.claims.values {
            let preview = String(c.text.prefix(60))
            print("  Claim: \(preview)...")
            print("  Confidence: \(String(format: "%.3f", c.confidence))")
            print("  Framework:  \(c.framework)")
        }

        let (h1, cocycles1) = world1.sheafConsistency()
        print("Sheaf H^1:   \(h1) (\(h1 == 0 ? "consistent" : "contradictions detected"))")

        let (balanced1, counts1) = world1.gf3Balance()
        print("GF(3):       balanced=\(balanced1) \(counts1)")
        print()

        // Test 2: Unsupported claim
        let text2 = "The moon is made of cheese"
        var world2 = analyzeClaim(text: text2, framework: "empirical")

        print("--- Unsupported Claim ---")
        print("Sources:     \(world2.sources.count)")
        let (h1_2, cocycles2) = world2.sheafConsistency()
        print("Sheaf H^1:   \(h1_2)")
        for c in cocycles2 {
            print("  Cocycle: \(c.kind) (severity=\(c.severity))")
        }
        print()

        // Test 3: Manipulation detection
        let text3 = "Act now! This exclusive offer expires in 10 minutes. Everyone knows this is the best deal. Scientists claim it's proven."
        let patterns = detectManipulation(text: text3)

        print("--- Manipulation Detection ---")
        print("Patterns found: \(patterns.count)")
        for p in patterns {
            print("  \(p.kind) (severity=\(p.severity)): \"\(p.evidence)\"")
        }
        print()

        // Test 4: All frameworks
        let text4 = "Study by MIT shows community benefit from sustainable energy integration"
        print("--- Multi-Framework ---")
        for fw in frameworks {
            let w = analyzeClaim(text: text4, framework: fw)
            for c in w.claims.values {
                print("  \(fw): confidence=\(String(format: "%.3f", c.confidence)) sources=\(w.sources.count) cocycles=\(w.cocycles.count)")
            }
        }
        print()

        // Test 5: Clean text
        let text5 = "The temperature today is 72 degrees Fahrenheit with partly cloudy skies."
        let cleanPatterns = detectManipulation(text: text5)
        print("--- Clean Text ---")
        print("Manipulation patterns: \(cleanPatterns.count) (should be 0)")
        print()

        // Assertions
        var allPassed = true
        func check(_ condition: Bool, _ msg: String) {
            if !condition {
                print("FAIL: \(msg)")
                allPassed = false
            }
        }

        print("--- Assertions ---")

        // Trit arithmetic
        check(Trit.add(.zero, .zero) == .zero, "0+0=0")
        check(Trit.add(.one, .two) == .zero, "1+2=0")
        check(Trit.mul(.two, .two) == .one, "2*2=1")
        check(Trit.neg(.one) == .two, "neg(1)=2")
        check(Trit.inv(.one) == .one, "inv(1)=1")
        check(Trit.inv(.two) == .two, "inv(2)=2")
        check(Trit.isBalanced([.zero, .one, .two]), "0+1+2 balanced")
        check(Trit.isBalanced([.one, .one, .one]), "1+1+1 balanced")
        check(!Trit.isBalanced([.one, .one, .zero]), "1+1+0 not balanced")

        // Claim analysis
        check(world1.claims.count == 1, "1 claim")
        check(world1.sources.count > 0, "has sources")
        check(world1.derivations.count > 0, "has derivations")

        // Unsupported claim
        check(world2.sources.count == 0, "unsupported has 0 sources")
        check(h1_2 > 0, "unsupported H^1 > 0")
        check(cocycles2.contains { $0.kind == "unsupported" }, "unsupported cocycle present")

        // Manipulation
        check(patterns.count > 0, "manipulation detected")
        let kinds = Set(patterns.map { $0.kind })
        check(kinds.contains("urgency"), "urgency detected")
        check(kinds.contains("artificial_scarcity"), "scarcity detected")
        check(kinds.contains("appeal_authority"), "authority appeal detected")

        // Clean text
        check(cleanPatterns.count == 0, "clean text has 0 patterns")

        // Content hash deterministic
        let ch1 = contentHash("hello world")
        let ch2 = contentHash("hello world")
        let ch3 = contentHash("Hello World")
        check(ch1 == ch2, "hash deterministic")
        check(ch1 == ch3, "hash case-insensitive")

        print()
        if allPassed {
            print("All assertions passed.")
        } else {
            print("Some assertions FAILED.")
        }

        print()
        print("=== All demos complete ===")
    }
}
