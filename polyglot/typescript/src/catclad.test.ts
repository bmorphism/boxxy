/**
 * Tests for cat-clad epistemological verification.
 *
 * Mirrors the Go test suite in internal/antibullshit/catclad_test.go.
 */

import { describe, it, expect } from "vitest";
import {
  analyzeClaim,
  detectManipulation,
  validateSources,
  contentHash,
  ClaimWorld,
  GF3,
  type Trit,
} from "./catclad.js";

// --- GF(3) unit tests ---

describe("GF3", () => {
  it("add is commutative and mod 3", () => {
    expect(GF3.add(1, 2)).toBe(0);
    expect(GF3.add(2, 2)).toBe(1);
    expect(GF3.add(0, 0)).toBe(0);
    expect(GF3.add(1, 1)).toBe(2);
  });

  it("neg is the additive inverse", () => {
    expect(GF3.neg(0)).toBe(0);
    expect(GF3.neg(1)).toBe(2);
    expect(GF3.neg(2)).toBe(1);
  });

  it("mul is correct mod 3", () => {
    expect(GF3.mul(2, 2)).toBe(1);
    expect(GF3.mul(1, 2)).toBe(2);
    expect(GF3.mul(0, 2)).toBe(0);
  });

  it("isBalanced checks sum mod 3 = 0", () => {
    expect(GF3.isBalanced([0, 0, 0])).toBe(true);
    expect(GF3.isBalanced([1, 2, 0])).toBe(true); // 1+2+0 = 3
    expect(GF3.isBalanced([1, 1, 1])).toBe(true); // 1+1+1 = 3
    expect(GF3.isBalanced([2, 2, 2])).toBe(true); // 2+2+2 = 6
    expect(GF3.isBalanced([1, 0, 0])).toBe(false); // 1
    expect(GF3.isBalanced([1, 1, 0])).toBe(false); // 2
  });

  it("findBalancer returns correct balancing element", () => {
    expect(GF3.findBalancer(1, 2, 0)).toBe(0); // 1+2+0 = 3, need 0
    expect(GF3.findBalancer(1, 1, 0)).toBe(1); // 1+1+0 = 2, need 1
    expect(GF3.findBalancer(0, 0, 0)).toBe(0); // 0+0+0 = 0, need 0
    expect(GF3.findBalancer(1, 0, 0)).toBe(2); // 1+0+0 = 1, need 2
  });
});

// --- Content hash tests ---

describe("contentHash", () => {
  it("is deterministic", () => {
    const h1 = contentHash("hello world");
    const h2 = contentHash("hello world");
    expect(h1).toBe(h2);
  });

  it("is case-insensitive", () => {
    const h1 = contentHash("hello world");
    const h2 = contentHash("Hello World");
    expect(h1).toBe(h2);
  });

  it("trims whitespace", () => {
    const h1 = contentHash("hello world");
    const h2 = contentHash("  hello world  ");
    expect(h1).toBe(h2);
  });

  it("returns a hex string", () => {
    const h = contentHash("test");
    expect(h).toMatch(/^[0-9a-f]{64}$/);
  });
});

// --- analyzeClaim tests ---

describe("analyzeClaim", () => {
  it("analyzes a claim with sources", () => {
    const world = analyzeClaim(
      "According to Dr. Smith, research from Harvard shows that exercise reduces stress by 40%",
      "empirical"
    );

    expect(world.claims.size).toBe(1);
    expect(world.sources.size).toBeGreaterThan(0);
    expect(world.derivations.length).toBeGreaterThan(0);

    // Claim should have positive confidence
    const claim = Array.from(world.claims.values())[0];
    expect(claim.confidence).toBeGreaterThan(0);
    expect(claim.framework).toBe("empirical");
    expect(claim.trit).toBe(GF3.One); // Generator
  });

  it("detects unsupported claims", () => {
    const world = analyzeClaim("The moon is made of cheese", "empirical");

    expect(world.sources.size).toBe(0);

    const { h1, cocycles } = world.sheafConsistency();
    expect(h1).toBeGreaterThan(0);

    const unsupported = cocycles.find((c) => c.kind === "unsupported");
    expect(unsupported).toBeDefined();

    // Confidence should be very low
    const claim = Array.from(world.claims.values())[0];
    expect(claim.confidence).toBeLessThanOrEqual(0.2);
  });

  it("assigns correct GF(3) trits", () => {
    const world = analyzeClaim(
      "According to the WHO, a study by Johns Hopkins published in Nature confirms vaccine efficacy",
      "pluralistic"
    );

    // Claims = Generator (1)
    for (const c of world.claims.values()) {
      expect(c.trit).toBe(GF3.One);
    }
    // Sources = Verifier (2)
    for (const s of world.sources.values()) {
      expect(s.trit).toBe(GF3.Two);
    }
    // Witnesses = Coordinator (0)
    for (const w of world.witnesses.values()) {
      expect(w.trit).toBe(GF3.Zero);
    }
  });

  it("works with all frameworks", () => {
    const text =
      "Study by MIT shows community benefit from sustainable energy integration";
    const frameworks = [
      "empirical",
      "responsible",
      "harmonic",
      "pluralistic",
    ] as const;

    for (const fw of frameworks) {
      const world = analyzeClaim(text, fw);
      const claim = Array.from(world.claims.values())[0];
      expect(claim.framework).toBe(fw);
      expect(claim.confidence).toBeGreaterThanOrEqual(0);
      expect(claim.confidence).toBeLessThanOrEqual(1);
    }
  });
});

// --- GF(3) balance tests ---

describe("gf3Balance", () => {
  it("reports balance counts", () => {
    const world = analyzeClaim(
      "According to the WHO, a study by Johns Hopkins published in Nature confirms vaccine efficacy",
      "pluralistic"
    );

    const { counts } = world.gf3Balance();
    expect(counts.generator).toBeGreaterThan(0);
    expect(counts.verifier).toBeGreaterThan(0);
  });

  it("correctly detects balanced triad", () => {
    // Manually construct a balanced world: 1 claim (trit=1), 1 source (trit=2), 1 witness (trit=0)
    // Sum = 1+2+0 = 3 => 0 (mod 3) => balanced
    const world = new ClaimWorld();
    world.claims.set("c1", {
      id: "c1",
      text: "test",
      trit: 1,
      hash: "abc",
      confidence: 0.5,
      framework: "pluralistic",
    });
    world.sources.set("s1", {
      id: "s1",
      citation: "source",
      trit: 2,
      hash: "def",
      kind: "academic",
    });
    world.witnesses.set("w1", {
      id: "w1",
      name: "witness",
      trit: 0,
      role: "author",
      weight: 0.5,
    });

    const { balanced, counts } = world.gf3Balance();
    expect(balanced).toBe(true);
    expect(counts.generator).toBe(1);
    expect(counts.verifier).toBe(1);
    expect(counts.coordinator).toBe(1);
  });

  it("detects unbalanced configuration", () => {
    // Only generators: sum = 1 => not balanced
    const world = new ClaimWorld();
    world.claims.set("c1", {
      id: "c1",
      text: "test",
      trit: 1,
      hash: "abc",
      confidence: 0.5,
      framework: "pluralistic",
    });

    const { balanced } = world.gf3Balance();
    expect(balanced).toBe(false);
  });
});

// --- sheafConsistency tests ---

describe("sheafConsistency", () => {
  it("returns h1=0 for consistent world", () => {
    const world = new ClaimWorld();
    const { h1 } = world.sheafConsistency();
    expect(h1).toBe(0);
  });

  it("h1 equals cocycle count", () => {
    const world = new ClaimWorld();
    world.cocycles.push({
      claimA: "a",
      kind: "unsupported",
      severity: 0.9,
    });
    world.cocycles.push({
      claimA: "b",
      kind: "contradiction",
      severity: 0.8,
    });

    const { h1, cocycles } = world.sheafConsistency();
    expect(h1).toBe(2);
    expect(cocycles).toHaveLength(2);
  });
});

// --- detectManipulation tests ---

describe("detectManipulation", () => {
  it("detects multiple manipulation patterns", () => {
    const patterns = detectManipulation(
      "Act now! This exclusive offer expires in 10 minutes. " +
        "Everyone knows this is the best deal. Scientists claim it's proven."
    );

    expect(patterns.length).toBeGreaterThan(0);

    const kinds = new Set(patterns.map((p) => p.kind));
    expect(kinds.has("urgency")).toBe(true);
    expect(kinds.has("artificial_scarcity")).toBe(true);
    expect(kinds.has("appeal_authority")).toBe(true);
  });

  it("returns empty for neutral text", () => {
    const patterns = detectManipulation(
      "The temperature today is 72 degrees Fahrenheit with partly cloudy skies."
    );
    expect(patterns).toHaveLength(0);
  });

  it("detects emotional fear", () => {
    const patterns = detectManipulation(
      "This catastrophic event will cause widespread panic and dread."
    );
    const kinds = new Set(patterns.map((p) => p.kind));
    expect(kinds.has("emotional_fear")).toBe(true);
  });

  it("detects loaded language", () => {
    const patterns = detectManipulation(
      "This is obviously the correct answer, undeniably true."
    );
    const kinds = new Set(patterns.map((p) => p.kind));
    expect(kinds.has("loaded_language")).toBe(true);
  });

  it("includes evidence substring and severity", () => {
    const patterns = detectManipulation("Act now before it's too late!");
    expect(patterns.length).toBeGreaterThan(0);
    for (const p of patterns) {
      expect(p.evidence).toBeTruthy();
      expect(p.severity).toBeGreaterThan(0);
      expect(p.severity).toBeLessThanOrEqual(1);
    }
  });
});

// --- validateSources tests ---

describe("validateSources", () => {
  it("extracts and classifies sources", () => {
    const world = validateSources(
      "A study by Stanford published in Nature, " +
        "and according to the CDC, " +
        "plus https://example.com/data",
      "empirical"
    );

    expect(world.sources.size).toBeGreaterThan(0);

    const kinds = new Set(
      Array.from(world.sources.values()).map((s) => s.kind)
    );
    // Should have at least one classified source
    const hasClassified =
      kinds.has("academic") || kinds.has("authority") || kinds.has("url");
    expect(hasClassified).toBe(true);
  });

  it("assigns verifier trit to all sources", () => {
    const world = validateSources(
      "According to NASA, research from MIT, https://example.com",
      "pluralistic"
    );

    for (const source of world.sources.values()) {
      expect(source.trit).toBe(GF3.Two); // Verifier
    }
  });

  it("creates derivation morphisms for each source", () => {
    const world = validateSources(
      "According to the WHO, study by Oxford shows results",
      "pluralistic"
    );

    // Each source should have a derivation linking it to the claim
    expect(world.derivations.length).toBe(world.sources.size);

    for (const d of world.derivations) {
      expect(d.sourceId).toBeTruthy();
      expect(d.claimId).toBeTruthy();
      expect(d.strength).toBeGreaterThan(0);
      expect(d.strength).toBeLessThanOrEqual(1);
    }
  });
});

// --- ClaimWorld serialization ---

describe("ClaimWorld.toJSON", () => {
  it("serializes to a plain object", () => {
    const world = analyzeClaim(
      "According to NASA, the earth orbits the sun",
      "empirical"
    );

    const json = world.toJSON() as Record<string, unknown>;
    expect(json).toHaveProperty("claims");
    expect(json).toHaveProperty("sources");
    expect(json).toHaveProperty("witnesses");
    expect(json).toHaveProperty("derivations");
    expect(json).toHaveProperty("cocycles");

    // Should be serializable
    const str = JSON.stringify(json);
    expect(str).toBeTruthy();
    const parsed = JSON.parse(str);
    expect(parsed.derivations).toBeInstanceOf(Array);
  });
});
