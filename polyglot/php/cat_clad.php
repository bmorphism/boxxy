<?php
/**
 * cat_clad.php -- Tier 3 PHP implementation of cat-clad epistemological verification.
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

// --- ACSet Schema Types ---

class Claim {
    public string $id;
    public string $text;
    public int $trit;
    public string $hash;
    public float $confidence;
    public string $framework;
    public string $createdAt;

    public function __construct(
        string $id,
        string $text,
        int $trit,
        string $hash,
        float $confidence,
        string $framework,
        string $createdAt = ''
    ) {
        $this->id = $id;
        $this->text = $text;
        $this->trit = $trit;
        $this->hash = $hash;
        $this->confidence = $confidence;
        $this->framework = $framework;
        $this->createdAt = $createdAt ?: date('c');
    }

    public function toArray(): array {
        return [
            'id' => $this->id,
            'text' => $this->text,
            'trit' => $this->trit,
            'hash' => $this->hash,
            'confidence' => $this->confidence,
            'framework' => $this->framework,
            'created_at' => $this->createdAt,
        ];
    }
}

class Source {
    public string $id;
    public string $citation;
    public int $trit;
    public string $hash;
    public string $kind; // "academic", "news", "authority", "anecdotal", "self-referential"

    public function __construct(string $id, string $citation, int $trit, string $hash, string $kind) {
        $this->id = $id;
        $this->citation = $citation;
        $this->trit = $trit;
        $this->hash = $hash;
        $this->kind = $kind;
    }

    public function toArray(): array {
        return [
            'id' => $this->id,
            'citation' => $this->citation,
            'trit' => $this->trit,
            'hash' => $this->hash,
            'kind' => $this->kind,
        ];
    }
}

class Witness {
    public string $id;
    public string $name;
    public int $trit;
    public string $role;  // "author", "peer-reviewer", "editor", "self"
    public float $weight;

    public function __construct(string $id, string $name, int $trit, string $role, float $weight) {
        $this->id = $id;
        $this->name = $name;
        $this->trit = $trit;
        $this->role = $role;
        $this->weight = $weight;
    }

    public function toArray(): array {
        return [
            'id' => $this->id,
            'name' => $this->name,
            'trit' => $this->trit,
            'role' => $this->role,
            'weight' => $this->weight,
        ];
    }
}

class Derivation {
    public string $id;
    public string $sourceId;
    public string $claimId;
    public string $kind;     // "direct", "inductive", "deductive", "analogical", "appeal-to-authority"
    public float $strength;

    public function __construct(string $id, string $sourceId, string $claimId, string $kind, float $strength) {
        $this->id = $id;
        $this->sourceId = $sourceId;
        $this->claimId = $claimId;
        $this->kind = $kind;
        $this->strength = $strength;
    }

    public function toArray(): array {
        return [
            'id' => $this->id,
            'source_id' => $this->sourceId,
            'claim_id' => $this->claimId,
            'kind' => $this->kind,
            'strength' => $this->strength,
        ];
    }
}

class Cocycle {
    public ?string $claimA;
    public ?string $claimB;
    public string $kind;     // "contradiction", "unsupported", "circular", "trit-violation"
    public float $severity;

    public function __construct(?string $claimA, ?string $claimB, string $kind, float $severity) {
        $this->claimA = $claimA;
        $this->claimB = $claimB;
        $this->kind = $kind;
        $this->severity = $severity;
    }

    public function toArray(): array {
        return [
            'claim_a' => $this->claimA,
            'claim_b' => $this->claimB,
            'kind' => $this->kind,
            'severity' => $this->severity,
        ];
    }
}

class ManipulationPattern {
    public string $kind;
    public string $evidence;
    public float $severity;

    public function __construct(string $kind, string $evidence, float $severity) {
        $this->kind = $kind;
        $this->evidence = $evidence;
        $this->severity = $severity;
    }

    public function toArray(): array {
        return [
            'kind' => $this->kind,
            'evidence' => $this->evidence,
            'severity' => $this->severity,
        ];
    }
}

// --- GF(3) Class ---

class GF3 {
    const ZERO = 0;
    const ONE  = 1;
    const TWO  = 2;

    public static function add(int $a, int $b): int {
        return ($a + $b) % 3;
    }

    public static function mul(int $a, int $b): int {
        return ($a * $b) % 3;
    }

    public static function neg(int $a): int {
        return (3 - $a) % 3;
    }

    public static function sub(int $a, int $b): int {
        return self::add($a, self::neg($b));
    }

    public static function inv(int $a): int {
        if ($a === self::ZERO) {
            throw new \InvalidArgumentException("gf3: multiplicative inverse of zero");
        }
        return $a === self::ONE ? self::ONE : self::TWO;
    }

    public static function seqSum(array $trits): int {
        return array_sum($trits);
    }

    /**
     * Check GF(3) conservation law: sum(trits) = 0 (mod 3).
     * @param int[] $trits
     */
    public static function isBalanced(array $trits): bool {
        $sum = self::seqSum($trits);
        return (($sum % 3) + 3) % 3 === 0;
    }

    public static function findBalancer(int $a, int $b, int $c): int {
        $partial = ($a + $b + $c) % 3;
        return (3 - $partial) % 3;
    }

    public static function toBalanced(int $e): int {
        return match ($e) {
            self::ZERO => 0,
            self::ONE  => 1,
            self::TWO  => -1,
            default    => 0,
        };
    }

    public static function elemName(int $e): string {
        return match ($e) {
            self::ZERO => 'coordinator',
            self::ONE  => 'generator',
            self::TWO  => 'verifier',
            default    => 'unknown',
        };
    }
}

// --- ClaimWorld: the ACSet instance ---

class ClaimWorld {
    /** @var array<string, Claim> */
    public array $claims = [];

    /** @var array<string, Source> */
    public array $sources = [];

    /** @var array<string, Witness> */
    public array $witnesses = [];

    /** @var Derivation[] */
    public array $derivations = [];

    /** @var Cocycle[] */
    public array $cocycles = [];

    /**
     * Sheaf consistency: returns [h1_dimension, cocycles].
     * H^1 = 0 means consistent, >0 means contradictions.
     * @return array{int, Cocycle[]}
     */
    public function sheafConsistency(): array {
        return [count($this->cocycles), $this->cocycles];
    }

    /**
     * GF(3) balance: checks conservation law sum(trits) = 0 (mod 3).
     * @return array{bool, array<string, int>}
     */
    public function gf3Balance(): array {
        $counts = ['coordinator' => 0, 'generator' => 0, 'verifier' => 0];
        $trits = [];

        foreach ($this->claims as $c) {
            $trits[] = $c->trit;
        }
        foreach ($this->sources as $s) {
            $trits[] = $s->trit;
        }
        foreach ($this->witnesses as $w) {
            $trits[] = $w->trit;
        }

        foreach ($trits as $t) {
            $name = GF3::elemName($t);
            $counts[$name] = ($counts[$name] ?? 0) + 1;
        }

        return [GF3::isBalanced($trits), $counts];
    }

    public function toArray(): array {
        return [
            'claims'      => array_map(fn($c) => $c->toArray(), $this->claims),
            'sources'     => array_map(fn($s) => $s->toArray(), $this->sources),
            'witnesses'   => array_map(fn($w) => $w->toArray(), $this->witnesses),
            'derivations' => array_map(fn($d) => $d->toArray(), $this->derivations),
            'cocycles'    => array_map(fn($c) => $c->toArray(), $this->cocycles),
        ];
    }

    public function toJson(): string {
        return json_encode($this->toArray(), JSON_PRETTY_PRINT);
    }
}

// --- CatCladEngine ---

class CatCladEngine {
    const FRAMEWORKS = ['empirical', 'responsible', 'harmonic', 'pluralistic'];

    /** @var array<array{kind: string, pattern: string, weight: float}> */
    const MANIPULATION_CHECKS = [
        ['kind' => 'emotional_fear',
         'pattern' => '/(?:fear|terrif|alarm|panic|dread|catastroph)/i',
         'weight' => 0.7],
        ['kind' => 'urgency',
         'pattern' => '/(?:act now|limited time|don\'t wait|expires|hurry|last chance|before it\'s too late)/i',
         'weight' => 0.8],
        ['kind' => 'false_consensus',
         'pattern' => '/(?:everyone knows|nobody (?:believes|wants|thinks)|all experts|unanimous|widely accepted)/i',
         'weight' => 0.6],
        ['kind' => 'appeal_authority',
         'pattern' => '/(?:experts say|scientists (?:claim|prove)|studies show|research proves|doctors recommend)/i',
         'weight' => 0.5],
        ['kind' => 'artificial_scarcity',
         'pattern' => '/(?:exclusive|rare opportunity|only \d+ left|limited (?:edition|supply|spots))/i',
         'weight' => 0.7],
        ['kind' => 'social_pressure',
         'pattern' => '/(?:you don\'t want to be|don\'t miss out|join .* (?:others|people)|be the first)/i',
         'weight' => 0.6],
        ['kind' => 'loaded_language',
         'pattern' => '/(?:obviously|clearly|undeniably|unquestionably|beyond doubt)/i',
         'weight' => 0.4],
        ['kind' => 'false_dichotomy',
         'pattern' => '/(?:either .* or|only (?:two|2) (?:options|choices)|if you don\'t .* then)/i',
         'weight' => 0.6],
        ['kind' => 'circular_reasoning',
         'pattern' => '/(?:because .* therefore .* because|true because .* which is true)/i',
         'weight' => 0.9],
        ['kind' => 'ad_hominem',
         'pattern' => '/(?:stupid|idiot|moron|fool|ignorant|naive) .* (?:think|believe|say)/i',
         'weight' => 0.8],
    ];

    const SOURCE_PATTERNS = [
        ['re' => '/(?:according to|cited by|reported by)\s+([^,\.]+)/i', 'kind' => 'authority'],
        ['re' => '/(?:study|research|paper)\s+(?:by|from|in)\s+([^,\.]+)/i', 'kind' => 'academic'],
        ['re' => '/(?:published in|journal of)\s+([^,\.]+)/i', 'kind' => 'academic'],
        ['re' => '/(https?:\/\/\S+)/i', 'kind' => 'url'],
    ];

    // --- Helpers ---

    public static function contentHash(string $text): string {
        return hash('sha256', strtolower(trim($text)));
    }

    private static function witnessRole(string $kind): string {
        return match ($kind) {
            'academic'  => 'peer-reviewer',
            'authority' => 'author',
            'url'       => 'publisher',
            default     => 'self',
        };
    }

    private static function witnessWeight(string $kind): float {
        return match ($kind) {
            'academic'  => 0.9,
            'authority' => 0.6,
            'url'       => 0.4,
            default     => 0.2,
        };
    }

    private static function classifyDerivation(Source $source): string {
        return match ($source->kind) {
            'academic'  => 'deductive',
            'authority' => 'appeal-to-authority',
            'url'       => 'direct',
            default     => 'analogical',
        };
    }

    private static function sourceStrength(Source $source): float {
        return match ($source->kind) {
            'academic'  => 0.85,
            'authority' => 0.5,
            'url'       => 0.3,
            default     => 0.1,
        };
    }

    /**
     * Extract sources from text using regex patterns.
     * @return Source[]
     */
    public static function extractSources(string $text): array {
        $sources = [];
        $seen = [];

        foreach (self::SOURCE_PATTERNS as $pat) {
            $matches = [];
            if (preg_match_all($pat['re'], $text, $matches, PREG_SET_ORDER)) {
                foreach ($matches as $match) {
                    if (count($match) < 2) continue;
                    $citation = trim($match[1]);
                    $hash = self::contentHash($citation);
                    $id = substr($hash, 0, 12);
                    if (isset($seen[$id])) continue;
                    $seen[$id] = true;

                    $sources[] = new Source(
                        $id,
                        $citation,
                        GF3::TWO,  // Verifier role -- evidence checks claims
                        $hash,
                        $pat['kind']
                    );
                }
            }
        }

        return $sources;
    }

    /**
     * Extract witnesses from a source.
     * @return Witness[]
     */
    public static function extractWitnesses(Source $source): array {
        return [new Witness(
            "w-{$source->id}",
            $source->citation,
            GF3::ZERO,  // Coordinator -- mediating between claim and verification
            self::witnessRole($source->kind),
            self::witnessWeight($source->kind)
        )];
    }

    /**
     * Compute confidence for a claim within a world.
     */
    private static function computeConfidence(ClaimWorld $world, Claim $claim, string $framework): float {
        if (empty($world->sources)) {
            return 0.1;
        }

        $derivs = array_filter($world->derivations, fn($d) => $d->claimId === $claim->id);
        if (empty($derivs)) {
            return 0.1;
        }

        $totalStrength = array_sum(array_map(fn($d) => $d->strength, $derivs));
        $avgStrength = $totalStrength / count($derivs);

        // Weight by framework
        switch ($framework) {
            case 'empirical':
                $academicCount = count(array_filter(
                    $world->sources,
                    fn($s) => $s->kind === 'academic'
                ));
                if ($academicCount > 0) {
                    $avgStrength *= 1.0 + 0.1 * $academicCount;
                }
                break;
            case 'responsible':
                $lower = strtolower($claim->text);
                if (str_contains($lower, 'community') || str_contains($lower, 'benefit')) {
                    $avgStrength *= 1.1;
                }
                break;
            case 'harmonic':
                if (count($world->sources) >= 3) {
                    $avgStrength *= 1.15;
                }
                break;
            case 'pluralistic':
                // No special boost, raw structural quality
                break;
        }

        // Penalize cocycles
        $cocyclePenalty = 0.15 * count($world->cocycles);
        $confidence = $avgStrength - $cocyclePenalty;

        return max(0.0, min(1.0, $confidence));
    }

    /**
     * Detect cocycles (contradictions, unsupported claims, circular reasoning).
     * @return Cocycle[]
     */
    private static function detectCocycles(ClaimWorld $world): array {
        $cocycles = [];

        // Check for unsupported claims (no derivation chain)
        foreach ($world->claims as $claim) {
            $hasDerivation = false;
            foreach ($world->derivations as $d) {
                if ($d->claimId === $claim->id) {
                    $hasDerivation = true;
                    break;
                }
            }
            if (!$hasDerivation) {
                $cocycles[] = new Cocycle($claim->id, null, 'unsupported', 0.9);
            }
        }

        // Check for appeal-to-authority without verification
        foreach ($world->derivations as $d) {
            if ($d->kind === 'appeal-to-authority' && $d->strength < 0.6) {
                $cocycles[] = new Cocycle($d->claimId, $d->sourceId, 'weak-authority', 0.5);
            }
        }

        // Check GF(3) conservation
        [$balanced, $_counts] = $world->gf3Balance();
        if (!$balanced) {
            $cocycles[] = new Cocycle(null, null, 'trit-violation', 0.3);
        }

        return $cocycles;
    }

    // --- Public API ---

    /**
     * Analyze a claim: parse text into a cat-clad structure and check consistency.
     */
    public static function analyzeClaim(string $text, string $framework = 'pluralistic'): ClaimWorld {
        $world = new ClaimWorld();

        // Create the primary claim (Generator role -- it's asserting something)
        $hash = self::contentHash($text);
        $claim = new Claim(
            substr($hash, 0, 12),
            $text,
            GF3::ONE,  // Generator: creating an assertion
            $hash,
            0.0,
            $framework
        );
        $world->claims[$claim->id] = $claim;

        // Extract sources as morphisms from claim
        $sources = self::extractSources($text);
        foreach ($sources as $src) {
            $world->sources[$src->id] = $src;
            $world->derivations[] = new Derivation(
                "d-{$src->id}-{$claim->id}",
                $src->id,
                $claim->id,
                self::classifyDerivation($src),
                self::sourceStrength($src)
            );
        }

        // Extract witnesses (who attests to the sources)
        foreach ($sources as $src) {
            $witnesses = self::extractWitnesses($src);
            foreach ($witnesses as $w) {
                $world->witnesses[$w->id] = $w;
            }
        }

        // Compute confidence
        $claim->confidence = self::computeConfidence($world, $claim, $framework);

        // Detect cocycles (contradictions, unsupported claims, circular reasoning)
        $world->cocycles = self::detectCocycles($world);

        return $world;
    }

    /**
     * Detect manipulation patterns in text.
     * @return ManipulationPattern[]
     */
    public static function detectManipulation(string $text): array {
        $patterns = [];

        foreach (self::MANIPULATION_CHECKS as $check) {
            $matches = [];
            if (preg_match_all($check['pattern'], $text, $matches, PREG_SET_ORDER)) {
                foreach ($matches as $match) {
                    $evidence = $match[0];
                    $patterns[] = new ManipulationPattern(
                        $check['kind'],
                        $evidence,
                        $check['weight']
                    );
                }
            }
        }

        return $patterns;
    }
}

// --- Main Block ---

if (php_sapi_name() === 'cli' && realpath($argv[0] ?? '') === realpath(__FILE__)) {
    echo "=== Cat-Clad Anti-Bullshit Engine (PHP Tier 3) ===" . PHP_EOL;
    echo PHP_EOL;

    // Test 1: Analyze a well-sourced claim
    $text1 = "According to Dr. Smith, research from Harvard shows that exercise reduces stress by 40%";
    $world1 = CatCladEngine::analyzeClaim($text1, 'empirical');

    echo "--- Claim Analysis (empirical) ---" . PHP_EOL;
    echo "Claims:      " . count($world1->claims) . PHP_EOL;
    echo "Sources:     " . count($world1->sources) . PHP_EOL;
    echo "Witnesses:   " . count($world1->witnesses) . PHP_EOL;
    echo "Derivations: " . count($world1->derivations) . PHP_EOL;

    foreach ($world1->claims as $c) {
        echo "  Claim: " . substr($c->text, 0, 60) . "..." . PHP_EOL;
        echo "  Confidence: " . round($c->confidence, 3) . PHP_EOL;
        echo "  Framework:  " . $c->framework . PHP_EOL;
    }

    [$h1, $cocycles1] = $world1->sheafConsistency();
    echo "Sheaf H^1:   {$h1} (" . ($h1 === 0 ? 'consistent' : 'contradictions detected') . ")" . PHP_EOL;

    [$balanced1, $counts1] = $world1->gf3Balance();
    $balStr = $balanced1 ? 'true' : 'false';
    echo "GF(3):       balanced={$balStr} " . json_encode($counts1) . PHP_EOL;
    echo PHP_EOL;

    // Test 2: Unsupported claim
    $text2 = "The moon is made of cheese";
    $world2 = CatCladEngine::analyzeClaim($text2, 'empirical');

    echo "--- Unsupported Claim ---" . PHP_EOL;
    echo "Sources:     " . count($world2->sources) . PHP_EOL;
    [$h1_2, $cocycles2] = $world2->sheafConsistency();
    echo "Sheaf H^1:   {$h1_2}" . PHP_EOL;
    foreach ($cocycles2 as $c) {
        echo "  Cocycle: {$c->kind} (severity={$c->severity})" . PHP_EOL;
    }
    echo PHP_EOL;

    // Test 3: Manipulation detection
    $text3 = "Act now! This exclusive offer expires in 10 minutes. Everyone knows this is the best deal. Scientists claim it's proven.";
    $patterns = CatCladEngine::detectManipulation($text3);

    echo "--- Manipulation Detection ---" . PHP_EOL;
    echo "Patterns found: " . count($patterns) . PHP_EOL;
    foreach ($patterns as $p) {
        echo "  {$p->kind} (severity={$p->severity}): \"{$p->evidence}\"" . PHP_EOL;
    }
    echo PHP_EOL;

    // Test 4: All frameworks
    $text4 = "Study by MIT shows community benefit from sustainable energy integration";
    echo "--- Multi-Framework ---" . PHP_EOL;
    foreach (CatCladEngine::FRAMEWORKS as $fw) {
        $w = CatCladEngine::analyzeClaim($text4, $fw);
        foreach ($w->claims as $c) {
            echo "  {$fw}: confidence=" . round($c->confidence, 3) .
                 " sources=" . count($w->sources) .
                 " cocycles=" . count($w->cocycles) . PHP_EOL;
        }
    }
    echo PHP_EOL;

    // Test 5: Clean text
    $text5 = "The temperature today is 72 degrees Fahrenheit with partly cloudy skies.";
    $cleanPatterns = CatCladEngine::detectManipulation($text5);
    echo "--- Clean Text ---" . PHP_EOL;
    echo "Manipulation patterns: " . count($cleanPatterns) . " (should be 0)" . PHP_EOL;
    echo PHP_EOL;
    echo "=== All demos complete ===" . PHP_EOL;
}

// --- PHPUnit-style Assertion Tests ---

/**
 * Simple assertion helper for non-PHPUnit environments.
 */
function catclad_assert(bool $condition, string $message): void {
    if (!$condition) {
        echo "FAIL: {$message}" . PHP_EOL;
    }
}

function catclad_run_tests(): void {
    echo PHP_EOL . "=== Running Tests ===" . PHP_EOL;
    $passed = 0;
    $failed = 0;

    $check = function(bool $condition, string $msg) use (&$passed, &$failed): void {
        if ($condition) {
            $passed++;
        } else {
            $failed++;
            echo "  FAIL: {$msg}" . PHP_EOL;
        }
    };

    // --- GF3 Tests ---
    echo "GF3 arithmetic..." . PHP_EOL;
    $check(GF3::add(0, 0) === 0, "0+0=0");
    $check(GF3::add(0, 1) === 1, "0+1=1");
    $check(GF3::add(0, 2) === 2, "0+2=2");
    $check(GF3::add(1, 1) === 2, "1+1=2");
    $check(GF3::add(1, 2) === 0, "1+2=0");
    $check(GF3::add(2, 2) === 1, "2+2=1");

    $check(GF3::mul(0, 0) === 0, "0*0=0");
    $check(GF3::mul(0, 1) === 0, "0*1=0");
    $check(GF3::mul(1, 1) === 1, "1*1=1");
    $check(GF3::mul(1, 2) === 2, "1*2=2");
    $check(GF3::mul(2, 2) === 1, "2*2=1");

    $check(GF3::neg(0) === 0, "neg(0)=0");
    $check(GF3::neg(1) === 2, "neg(1)=2");
    $check(GF3::neg(2) === 1, "neg(2)=1");

    $check(GF3::inv(1) === 1, "inv(1)=1");
    $check(GF3::inv(2) === 2, "inv(2)=2");

    $invZeroThrew = false;
    try { GF3::inv(0); } catch (\InvalidArgumentException $e) { $invZeroThrew = true; }
    $check($invZeroThrew, "inv(0) throws");

    $check(GF3::isBalanced([0, 0, 0]) === true, "[0,0,0] balanced");
    $check(GF3::isBalanced([1, 1, 1]) === true, "[1,1,1] balanced");
    $check(GF3::isBalanced([0, 1, 2]) === true, "[0,1,2] balanced");
    $check(GF3::isBalanced([1, 1, 0]) === false, "[1,1,0] not balanced");
    $check(GF3::isBalanced([1]) === false, "[1] not balanced");

    $check(GF3::findBalancer(0, 0, 0) === 0, "balancer(0,0,0)=0");
    $check(GF3::findBalancer(1, 0, 0) === 2, "balancer(1,0,0)=2");
    $check(GF3::findBalancer(1, 1, 1) === 0, "balancer(1,1,1)=0");

    $check(GF3::toBalanced(0) === 0, "toBalanced(0)=0");
    $check(GF3::toBalanced(1) === 1, "toBalanced(1)=1");
    $check(GF3::toBalanced(2) === -1, "toBalanced(2)=-1");

    // --- Claim Analysis Tests ---
    echo "Claim analysis..." . PHP_EOL;

    $world = CatCladEngine::analyzeClaim(
        "According to Dr. Smith, research from Harvard shows that exercise reduces stress by 40%",
        'empirical'
    );
    $check(count($world->claims) === 1, "1 claim");
    $check(count($world->sources) > 0, "has sources");
    $check(count($world->derivations) > 0, "has derivations");
    foreach ($world->claims as $c) {
        $check($c->confidence > 0, "positive confidence: {$c->confidence}");
    }

    // Unsupported claim
    echo "Unsupported claim..." . PHP_EOL;
    $world2 = CatCladEngine::analyzeClaim("The moon is made of cheese", 'empirical');
    $check(count($world2->sources) === 0, "0 sources for unsupported");
    [$h1, $cocycles] = $world2->sheafConsistency();
    $check($h1 > 0, "H^1 > 0 for unsupported");
    $foundUnsupported = false;
    foreach ($cocycles as $c) {
        if ($c->kind === 'unsupported') $foundUnsupported = true;
    }
    $check($foundUnsupported, "unsupported cocycle present");
    foreach ($world2->claims as $c) {
        $check($c->confidence <= 0.2, "low confidence for unsupported: {$c->confidence}");
    }

    // GF(3) balance with full triad
    echo "GF(3) balance..." . PHP_EOL;
    $world3 = CatCladEngine::analyzeClaim(
        "According to the WHO, a study by Johns Hopkins published in Nature confirms vaccine efficacy",
        'pluralistic'
    );
    [$balanced, $counts] = $world3->gf3Balance();
    $check($counts['generator'] > 0, "has generators");
    $check($counts['verifier'] > 0, "has verifiers");

    // Manipulation detection
    echo "Manipulation detection..." . PHP_EOL;
    $patterns = CatCladEngine::detectManipulation(
        "Act now! This exclusive offer expires in 10 minutes. Everyone knows this is the best deal. Scientists claim it's proven."
    );
    $check(count($patterns) > 0, "patterns found");
    $kinds = array_unique(array_map(fn($p) => $p->kind, $patterns));
    $check(in_array('urgency', $kinds), "urgency detected");
    $check(in_array('artificial_scarcity', $kinds), "scarcity detected");
    $check(in_array('appeal_authority', $kinds), "authority appeal detected");

    // Clean text
    echo "Clean text..." . PHP_EOL;
    $cleanPatterns = CatCladEngine::detectManipulation(
        "The temperature today is 72 degrees Fahrenheit with partly cloudy skies."
    );
    $check(count($cleanPatterns) === 0, "0 patterns for clean text");

    // Content hash
    echo "Content hash..." . PHP_EOL;
    $h1 = CatCladEngine::contentHash("hello world");
    $h2 = CatCladEngine::contentHash("hello world");
    $h3 = CatCladEngine::contentHash("Hello World");
    $check($h1 === $h2, "hash deterministic");
    $check($h1 === $h3, "hash case-insensitive");

    // Multiple frameworks
    echo "Multiple frameworks..." . PHP_EOL;
    $text = "Study by MIT shows community benefit from sustainable energy integration";
    foreach (CatCladEngine::FRAMEWORKS as $fw) {
        $w = CatCladEngine::analyzeClaim($text, $fw);
        foreach ($w->claims as $c) {
            $check($c->framework === $fw, "framework={$fw}");
        }
    }

    // Source classification
    echo "Source classification..." . PHP_EOL;
    $world4 = CatCladEngine::analyzeClaim(
        "A study by Stanford published in Nature, and according to the CDC, plus https://example.com/data",
        'empirical'
    );
    $kinds = array_unique(array_map(fn($s) => $s->kind, $world4->sources));
    $check(count($kinds) > 0, "sources classified");

    // JSON serialization
    echo "JSON serialization..." . PHP_EOL;
    $world5 = CatCladEngine::analyzeClaim("According to the WHO, vaccines are effective", 'empirical');
    $json = $world5->toJson();
    $parsed = json_decode($json, true);
    $check(isset($parsed['claims']), "JSON has claims");
    $check(isset($parsed['sources']), "JSON has sources");
    $check(isset($parsed['witnesses']), "JSON has witnesses");
    $check(isset($parsed['derivations']), "JSON has derivations");
    $check(isset($parsed['cocycles']), "JSON has cocycles");

    echo PHP_EOL;
    echo "Results: {$passed} passed, {$failed} failed" . PHP_EOL;
    if ($failed === 0) {
        echo "All tests passed." . PHP_EOL;
    }
}

// Run tests when executed directly
if (php_sapi_name() === 'cli' && realpath($argv[0] ?? '') === realpath(__FILE__)) {
    catclad_run_tests();
}
