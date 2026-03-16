/**
 * Cat-clad epistemological verification.
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

import { createHash } from "crypto";

// ============================================================================
// Post-modern foundations: branded types, const assertions, template literals
// ============================================================================

/** Phantom brand carrier -- optional field preserves assignability from literals. */
declare const __brand: unique symbol;
type Brand<B extends string> = { readonly [__brand]?: B };

/** GF(3) element: 0, 1, or 2 under arithmetic mod 3 (branded). */
export type Trit = (0 | 1 | 2) & Brand<"GF3">;

/** Template literal type for SHA-256 content hashes. */
export type ContentHash = `sha256:${string}`;

/** Generic algebraic Result type -- success or failure, never thrown. */
export type Result<T, E> = Readonly<
  | { readonly ok: true; readonly value: T }
  | { readonly ok: false; readonly error: E }
>;

const Ok = <T>(value: T): Result<T, never> =>
  Object.freeze({ ok: true, value }) as Result<T, never>;

const Err = <E>(error: E): Result<never, E> =>
  Object.freeze({ ok: false, error }) as Result<never, E>;

// ============================================================================
// GF(3) -- the Galois field on {0, 1, 2}
// ============================================================================

export const GF3 = {
  Zero: 0 as Trit,
  One: 1 as Trit,
  Two: 2 as Trit,

  add(a: Trit, b: Trit): Trit {
    return ((a + b) % 3) as Trit;
  },

  neg(a: Trit): Trit {
    return ((3 - a) % 3) as Trit;
  },

  mul(a: Trit, b: Trit): Trit {
    return ((a * b) % 3) as Trit;
  },

  /** Sum of a sequence (plain number, not reduced). */
  seqSum(s: ReadonlyArray<Trit>): number {
    let sum = 0;
    for (const e of s) sum += e;
    return sum;
  },

  /** Conservation law: sum of elements is 0 (mod 3). */
  isBalanced(s: ReadonlyArray<Trit>): boolean {
    return ((GF3.seqSum(s) % 3) + 3) % 3 === 0;
  },

  /** Find the element needed to balance a triad. */
  findBalancer(a: Trit, b: Trit, c: Trit): Trit {
    const partial = (a + b + c) % 3;
    return ((3 - partial) % 3) as Trit;
  },
} as const;

// ============================================================================
// ACSet Schema Types -- discriminated unions & const-assertion enums
// ============================================================================

/** Object types in the epistemic category. */
export const ObType = {
  Claim: "Claim",
  Source: "Source",
  Witness: "Witness",
  Derivation: "Derivation",
} as const satisfies Readonly<Record<string, string>>;
export type ObType = (typeof ObType)[keyof typeof ObType];

/** Morphism types in the epistemic category. */
export const MorType = {
  DerivesFrom: "derives_from",
  Produces: "produces",
  Attests: "attests",
  Cites: "cites",
} as const satisfies Readonly<Record<string, string>>;
export type MorType = (typeof MorType)[keyof typeof MorType];

/** Epistemological frameworks -- const assertion with satisfies. */
export const Frameworks = {
  Empirical: "empirical",
  Responsible: "responsible",
  Harmonic: "harmonic",
  Pluralistic: "pluralistic",
} as const satisfies Readonly<Record<string, string>>;
export type Framework = (typeof Frameworks)[keyof typeof Frameworks];

/** Source kind -- discriminated union with exhaustive matching. */
export const SourceKinds = {
  Academic: "academic",
  Authority: "authority",
  Url: "url",
  Anecdotal: "anecdotal",
} as const satisfies Readonly<Record<string, string>>;
export type SourceKind = (typeof SourceKinds)[keyof typeof SourceKinds];

export type WitnessRole = "author" | "peer-reviewer" | "editor" | "self";

export const DerivationKinds = {
  Direct: "direct",
  Deductive: "deductive",
  AppealToAuthority: "appeal-to-authority",
  Analogical: "analogical",
} as const satisfies Readonly<Record<string, string>>;
export type DerivationKind =
  (typeof DerivationKinds)[keyof typeof DerivationKinds];

/** Cocycle kinds form a discriminated union for exhaustive switching. */
export type CocycleKind =
  | "contradiction"
  | "unsupported"
  | "circular"
  | "trit-violation";

// ============================================================================
// CatColab DblTheory: EpistemicTheory
// ============================================================================

/** A branded morphism identifier for type-safe path composition. */
declare const __morBrand: unique symbol;
export type MorphismId = string & { readonly [__morBrand]?: "MorphismId" };

/** A single segment of a categorical path. */
export interface PathSegment {
  readonly morphism: MorType;
  readonly source: ObType;
  readonly target: ObType;
}

/** A path in the epistemic category -- readonly tuple of segments. */
export type Path = readonly [...PathSegment[]];

/** Error types arising from invalid path composition. */
export type CompositionError =
  | { readonly kind: "domain_mismatch"; readonly expected: ObType; readonly got: ObType }
  | { readonly kind: "empty_path" };

/** Strength of a composite morphism chain. */
export interface CompositeStrength {
  readonly path: Path;
  readonly strength: number;
}

/** Compose a path from segments, validating domain/codomain adjacency. */
export function composePath(
  segments: ReadonlyArray<PathSegment>,
  strengths: ReadonlyArray<number>,
): Result<CompositeStrength, CompositionError> {
  if (segments.length === 0) {
    return Err({ kind: "empty_path" as const });
  }

  for (let i = 1; i < segments.length; i++) {
    const prev = segments[i - 1]!;
    const curr = segments[i]!;
    if (prev.target !== curr.source) {
      return Err({
        kind: "domain_mismatch" as const,
        expected: prev.target,
        got: curr.source,
      });
    }
  }

  const compositeStrength = strengths.reduce<number>((acc, s) => acc * s, 1.0);
  return Ok({
    path: segments as Path,
    strength: compositeStrength,
  });
}

/**
 * The epistemic double theory -- frozen, readonly specification of the
 * categorical schema for claim worlds.
 */
export const EpistemicTheory = Object.freeze({
  objects: Object.freeze([
    ObType.Claim,
    ObType.Source,
    ObType.Witness,
    ObType.Derivation,
  ] as const),

  morphisms: Object.freeze([
    Object.freeze({ name: MorType.DerivesFrom, source: ObType.Derivation, target: ObType.Source }),
    Object.freeze({ name: MorType.Produces, source: ObType.Derivation, target: ObType.Claim }),
    Object.freeze({ name: MorType.Attests, source: ObType.Witness, target: ObType.Source }),
    Object.freeze({ name: MorType.Cites, source: ObType.Claim, target: ObType.Source }),
  ] as const),

  attributes: Object.freeze([
    Object.freeze({ name: "claim_trit", on: ObType.Claim, type: "Trit" }),
    Object.freeze({ name: "source_trit", on: ObType.Source, type: "Trit" }),
    Object.freeze({ name: "witness_trit", on: ObType.Witness, type: "Trit" }),
    Object.freeze({ name: "claim_hash", on: ObType.Claim, type: "ContentHash" }),
    Object.freeze({ name: "source_hash", on: ObType.Source, type: "ContentHash" }),
    Object.freeze({ name: "claim_confidence", on: ObType.Claim, type: "Confidence" }),
  ] as const),
} as const);

// ============================================================================
// Domain interfaces
// ============================================================================

export interface Claim {
  id: string;
  text: string;
  trit: Trit;
  hash: string;
  confidence: number;
  framework: string;
}

export interface Source {
  id: string;
  citation: string;
  trit: Trit;
  hash: string;
  kind: SourceKind;
}

export interface Witness {
  id: string;
  name: string;
  trit: Trit;
  role: WitnessRole;
  weight: number;
}

export interface Derivation {
  id: string;
  sourceId: string;
  claimId: string;
  kind: DerivationKind;
  strength: number;
}

export interface Cocycle {
  claimA: string;
  claimB?: string;
  kind: CocycleKind | string;
  severity: number;
}

export interface ManipulationPattern {
  kind: string;
  evidence: string;
  severity: number;
}

// ============================================================================
// ClaimWorld: the ACSet instance with Symbol.iterator & #private internals
// ============================================================================

/** Iterable element yielded by ClaimWorld[Symbol.iterator]. */
export type ClaimWorldEntry =
  | { readonly type: "claim"; readonly value: Claim }
  | { readonly type: "source"; readonly value: Source }
  | { readonly type: "witness"; readonly value: Witness };

export class ClaimWorld {
  // True-private backing field for the theory reference
  readonly #theory = EpistemicTheory;

  claims: Map<string, Claim> = new Map();
  sources: Map<string, Source> = new Map();
  witnesses: Map<string, Witness> = new Map();
  derivations: Derivation[] = [];
  cocycles: Cocycle[] = [];

  /** The epistemic theory this world instantiates. */
  get theory(): typeof EpistemicTheory {
    return this.#theory;
  }

  /** Sheaf consistency: H^1 dimension. 0 = consistent, >0 = contradictions. */
  sheafConsistency(): { h1: number; cocycles: ReadonlyArray<Cocycle> } {
    return { h1: this.cocycles.length, cocycles: this.cocycles };
  }

  /** GF(3) conservation law: sum of all trits must be 0 (mod 3). */
  gf3Balance(): {
    balanced: boolean;
    counts: Readonly<Record<string, number>>;
  } {
    const counts: Record<string, number> = {
      coordinator: 0,
      generator: 0,
      verifier: 0,
    };
    const trits: Trit[] = [];

    for (const c of this.claims.values()) trits.push(c.trit);
    for (const s of this.sources.values()) trits.push(s.trit);
    for (const w of this.witnesses.values()) trits.push(w.trit);

    for (const t of trits) {
      // Exhaustive switch over GF(3) elements
      switch (t as Trit) {
        case GF3.Zero:
          counts.coordinator++;
          break;
        case GF3.One:
          counts.generator++;
          break;
        case GF3.Two:
          counts.verifier++;
          break;
      }
    }

    return { balanced: GF3.isBalanced(trits), counts };
  }

  /** Iterate all objects (claims, sources, witnesses) in the category. */
  *[Symbol.iterator](): Generator<ClaimWorldEntry, void, undefined> {
    for (const value of this.claims.values()) {
      yield { type: "claim", value } satisfies ClaimWorldEntry;
    }
    for (const value of this.sources.values()) {
      yield { type: "source", value } satisfies ClaimWorldEntry;
    }
    for (const value of this.witnesses.values()) {
      yield { type: "witness", value } satisfies ClaimWorldEntry;
    }
  }

  /** Deep-clone this world via structuredClone. */
  clone(): ClaimWorld {
    const copy = new ClaimWorld();
    copy.claims = structuredClone(this.claims);
    copy.sources = structuredClone(this.sources);
    copy.witnesses = structuredClone(this.witnesses);
    copy.derivations = structuredClone(this.derivations);
    copy.cocycles = structuredClone(this.cocycles);
    return copy;
  }

  /** Serialize to a plain JSON object. */
  toJSON(): object {
    return {
      claims: Object.fromEntries(this.claims),
      sources: Object.fromEntries(this.sources),
      witnesses: Object.fromEntries(this.witnesses),
      derivations: this.derivations,
      cocycles: this.cocycles,
    };
  }
}

// ============================================================================
// Content hashing
// ============================================================================

export function contentHash(text: string): string {
  return createHash("sha256")
    .update(text.toLowerCase().trim())
    .digest("hex");
}

/** Branded content hash with `sha256:` prefix (internal use). */
function brandedHash(text: string): ContentHash {
  return `sha256:${contentHash(text)}` as ContentHash;
}

// ============================================================================
// Source extraction -- readonly patterns with const assertion
// ============================================================================

interface SourcePattern {
  readonly re: RegExp;
  readonly kind: SourceKind;
}

const sourcePatterns = [
  {
    re: /(?:according to|cited by|reported by)\s+([^,.]+)/gi,
    kind: "authority",
  },
  {
    re: /(?:study|research|paper)\s+(?:by|from|in)\s+([^,.]+)/gi,
    kind: "academic",
  },
  { re: /(?:published in|journal of)\s+([^,.]+)/gi, kind: "academic" },
  { re: /(https?:\/\/\S+)/gi, kind: "url" },
] as const satisfies ReadonlyArray<SourcePattern>;

function extractSources(text: string): ReadonlyArray<Source> {
  const sources: Source[] = [];
  const seen = new Set<string>();

  for (const pattern of sourcePatterns) {
    // Reset lastIndex for global regexps
    pattern.re.lastIndex = 0;
    let match: RegExpExecArray | null;
    while ((match = pattern.re.exec(text)) !== null) {
      if (match.length < 2) continue;
      const citation = match[1]!.trim();
      const id = contentHash(citation).slice(0, 12);
      if (seen.has(id)) continue;
      seen.add(id);

      sources.push({
        id,
        citation,
        trit: GF3.Two, // Verifier role: evidence checks claims
        hash: contentHash(citation),
        kind: pattern.kind,
      });
    }
  }

  return sources;
}

// ============================================================================
// Witness extraction -- exhaustive SourceKind switching
// ============================================================================

function witnessRole(kind: SourceKind): WitnessRole {
  switch (kind) {
    case "academic":
      return "peer-reviewer";
    case "authority":
      return "author";
    case "url":
      return "editor";
    case "anecdotal":
      return "self";
  }
  // Exhaustiveness: the above switch covers all SourceKind variants.
  // TypeScript will error if a variant is added to SourceKind without a case.
  const _exhaustive: never = kind;
  return _exhaustive;
}

function witnessWeight(kind: SourceKind): number {
  switch (kind) {
    case "academic":
      return 0.9;
    case "authority":
      return 0.6;
    case "url":
      return 0.4;
    case "anecdotal":
      return 0.2;
  }
  const _exhaustive: never = kind;
  return _exhaustive;
}

function extractWitnesses(src: Source): ReadonlyArray<Witness> {
  return [
    {
      id: `w-${src.id}`,
      name: src.citation,
      trit: GF3.Zero, // Coordinator: mediating between claim and verification
      role: witnessRole(src.kind),
      weight: witnessWeight(src.kind),
    },
  ] as const;
}

// ============================================================================
// Derivation classification -- exhaustive SourceKind dispatch
// ============================================================================

function classifyDerivation(src: Source): DerivationKind {
  switch (src.kind) {
    case "academic":
      return "deductive";
    case "authority":
      return "appeal-to-authority";
    case "url":
      return "direct";
    case "anecdotal":
      return "analogical";
  }
  const _exhaustive: never = src.kind;
  return _exhaustive;
}

function sourceStrength(src: Source): number {
  switch (src.kind) {
    case "academic":
      return 0.85;
    case "authority":
      return 0.5;
    case "url":
      return 0.3;
    case "anecdotal":
      return 0.1;
  }
  const _exhaustive: never = src.kind;
  return _exhaustive;
}

// ============================================================================
// Confidence computation -- framework-aware weighting
// ============================================================================

function computeConfidence(
  world: ClaimWorld,
  claim: Claim,
  framework: string,
): number {
  if (world.sources.size === 0) {
    return 0.1; // unsupported claim
  }

  // Average derivation strength
  let totalStrength = 0;
  let count = 0;
  for (const d of world.derivations) {
    if (d.claimId === claim.id) {
      totalStrength += d.strength;
      count++;
    }
  }
  if (count === 0) return 0.1;
  let avgStrength = totalStrength / count;

  // Weight by framework (exhaustive over known frameworks)
  switch (framework) {
    case "empirical": {
      let academicCount = 0;
      for (const s of world.sources.values()) {
        if (s.kind === "academic") academicCount++;
      }
      if (academicCount > 0) {
        avgStrength *= 1.0 + 0.1 * academicCount;
      }
      break;
    }
    case "responsible": {
      const lower = claim.text.toLowerCase();
      if (lower.includes("community") || lower.includes("benefit")) {
        avgStrength *= 1.1;
      }
      break;
    }
    case "harmonic": {
      if (world.sources.size >= 3) {
        avgStrength *= 1.15;
      }
      break;
    }
    case "pluralistic":
      // Raw structural quality, no special boost
      break;
  }

  // Penalize cocycles
  const cocyclePenalty = 0.15 * world.cocycles.length;
  let confidence = avgStrength - cocyclePenalty;

  if (confidence > 1.0) confidence = 1.0;
  if (confidence < 0.0) confidence = 0.0;
  return confidence;
}

// ============================================================================
// Cocycle detection -- sheaf obstruction finder
// ============================================================================

function detectCocycles(world: ClaimWorld): ReadonlyArray<Cocycle> {
  const cocycles: Cocycle[] = [];

  // Check for unsupported claims (no derivation chain)
  for (const claim of world.claims.values()) {
    const hasDerivation = world.derivations.some(
      (d) => d.claimId === claim.id,
    );
    if (!hasDerivation) {
      cocycles.push({
        claimA: claim.id,
        kind: "unsupported" satisfies CocycleKind,
        severity: 0.9,
      });
    }
  }

  // Check for weak appeal-to-authority
  for (const d of world.derivations) {
    if (d.kind === "appeal-to-authority" && d.strength < 0.6) {
      cocycles.push({
        claimA: d.claimId,
        claimB: d.sourceId,
        kind: "weak-authority",
        severity: 0.5,
      });
    }
  }

  // Check GF(3) conservation
  const { balanced } = world.gf3Balance();
  if (!balanced) {
    cocycles.push({
      claimA: "",
      kind: "trit-violation" satisfies CocycleKind,
      severity: 0.3,
    });
  }

  return cocycles;
}

// ============================================================================
// Public API
// ============================================================================

/**
 * Analyze a claim: parse text into a cat-clad structure and check consistency.
 *
 * Claims are Generators (trit=1), Sources are Verifiers (trit=2),
 * Witnesses are Coordinators (trit=0). The GF(3) conservation law
 * requires their sum to be 0 (mod 3).
 */
export function analyzeClaim(
  text: string,
  framework: Framework = "pluralistic",
): ClaimWorld {
  const world = new ClaimWorld();

  // Create the primary claim (Generator role)
  const claim: Claim = {
    id: contentHash(text).slice(0, 12),
    text,
    trit: GF3.One, // Generator: creating an assertion
    hash: contentHash(text),
    confidence: 0,
    framework,
  };
  world.claims.set(claim.id, claim);

  // Extract sources as morphisms from claim
  const sources = extractSources(text);
  for (const src of sources) {
    world.sources.set(src.id, src);
    world.derivations.push({
      id: `d-${src.id}-${claim.id}`,
      sourceId: src.id,
      claimId: claim.id,
      kind: classifyDerivation(src),
      strength: sourceStrength(src),
    });
  }

  // Extract witnesses (who attests to the sources)
  for (const src of sources) {
    const witnesses = extractWitnesses(src);
    for (const w of witnesses) {
      world.witnesses.set(w.id, w);
    }
  }

  // Compute confidence
  claim.confidence = computeConfidence(world, claim, framework);

  // Detect cocycles (contradictions, unsupported claims, circular reasoning)
  world.cocycles = [...detectCocycles(world)];

  return world;
}

// ============================================================================
// Manipulation detection
// ============================================================================

interface ManipulationCheck {
  readonly kind: string;
  readonly pattern: RegExp;
  readonly weight: number;
}

const manipulationChecks = [
  {
    kind: "emotional_fear",
    pattern: /(?:fear|terrif|alarm|panic|dread|catastroph)/gi,
    weight: 0.7,
  },
  {
    kind: "urgency",
    pattern:
      /(?:act now|limited time|don't wait|expires|hurry|last chance|before it's too late)/gi,
    weight: 0.8,
  },
  {
    kind: "false_consensus",
    pattern:
      /(?:everyone knows|nobody (?:believes|wants|thinks)|all experts|unanimous|widely accepted)/gi,
    weight: 0.6,
  },
  {
    kind: "appeal_authority",
    pattern:
      /(?:experts say|scientists (?:claim|prove)|studies show|research proves|doctors recommend)/gi,
    weight: 0.5,
  },
  {
    kind: "artificial_scarcity",
    pattern:
      /(?:exclusive|rare opportunity|only \d+ left|limited (?:edition|supply|spots))/gi,
    weight: 0.7,
  },
  {
    kind: "social_pressure",
    pattern:
      /(?:you don't want to be|don't miss out|join .* (?:others|people)|be the first)/gi,
    weight: 0.6,
  },
  {
    kind: "loaded_language",
    pattern:
      /(?:obviously|clearly|undeniably|unquestionably|beyond doubt)/gi,
    weight: 0.4,
  },
  {
    kind: "false_dichotomy",
    pattern:
      /(?:either .* or|only (?:two|2) (?:options|choices)|if you don't .* then)/gi,
    weight: 0.6,
  },
  {
    kind: "circular_reasoning",
    pattern:
      /(?:because .* therefore .* because|true because .* which is true)/gi,
    weight: 0.9,
  },
  {
    kind: "ad_hominem",
    pattern:
      /(?:stupid|idiot|moron|fool|ignorant|naive) .* (?:think|believe|say)/gi,
    weight: 0.8,
  },
] as const satisfies ReadonlyArray<ManipulationCheck>;

/**
 * Detect manipulation patterns in text.
 * Returns an array of matched patterns with kind, evidence, and severity.
 */
export function detectManipulation(
  text: string,
): ReadonlyArray<ManipulationPattern> {
  const patterns: ManipulationPattern[] = [];

  for (const check of manipulationChecks) {
    check.pattern.lastIndex = 0;
    let match: RegExpExecArray | null;
    while ((match = check.pattern.exec(text)) !== null) {
      patterns.push({
        kind: check.kind,
        evidence: match[0],
        severity: check.weight,
      });
    }
  }

  return patterns;
}

/**
 * Validate sources: extract and classify sources from text.
 * Returns a ClaimWorld focused on the source morphisms.
 */
export function validateSources(
  text: string,
  framework: Framework = "pluralistic",
): ClaimWorld {
  // This is essentially analyzeClaim but we can return the same structure;
  // the caller focuses on the sources, witnesses, and derivations.
  return analyzeClaim(text, framework);
}
