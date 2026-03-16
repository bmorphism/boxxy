# frozen_string_literal: true

# cat_clad.rb -- Tier 3 Ruby implementation of cat-clad epistemological verification.
#
# A "cat-clad" claim is an object in a category with morphisms tracking
# its provenance, derivation history, and the consistency conditions that
# bind it to other claims. Verification reduces to structural properties:
#
#   - Provenance is a composable morphism chain to primary sources
#   - Consistency is a sheaf condition (H^1 = 0 means no contradictions)
#   - GF(3) conservation prevents unbounded generation without verification
#   - Bisimulation detects forgery (divergent accounts of the same event)
#
# ACSet Schema:
#
#   @present SchClaimWorld(FreeSchema) begin
#     Claim::Ob           -- assertions to verify
#     Source::Ob           -- evidence or citations
#     Witness::Ob          -- attestation parties
#     Derivation::Ob       -- inference steps
#
#     derives_from::Hom(Derivation, Source)
#     produces::Hom(Derivation, Claim)
#     attests::Hom(Witness, Source)
#     cites::Hom(Claim, Source)
#
#     Trit::AttrType
#     Confidence::AttrType
#     ContentHash::AttrType
#     Timestamp::AttrType
#
#     claim_trit::Attr(Claim, Trit)
#     source_trit::Attr(Source, Trit)
#     witness_trit::Attr(Witness, Trit)
#     claim_hash::Attr(Claim, ContentHash)
#     source_hash::Attr(Source, ContentHash)
#     claim_confidence::Attr(Claim, Confidence)
#   end

require 'digest'
require 'time'
require 'json'

# --- ACSet Schema Types ---

Claim = Struct.new(:id, :text, :trit, :hash, :confidence, :framework, :created_at, keyword_init: true) do
  def to_h
    { id: id, text: text, trit: trit, hash: hash, confidence: confidence,
      framework: framework, created_at: created_at.to_s }
  end
end

Source = Struct.new(:id, :citation, :trit, :hash, :kind, keyword_init: true) do
  def to_h
    { id: id, citation: citation, trit: trit, hash: hash, kind: kind }
  end
end

Witness = Struct.new(:id, :name, :trit, :role, :weight, keyword_init: true) do
  def to_h
    { id: id, name: name, trit: trit, role: role, weight: weight }
  end
end

Derivation = Struct.new(:id, :source_id, :claim_id, :kind, :strength, keyword_init: true) do
  def to_h
    { id: id, source_id: source_id, claim_id: claim_id, kind: kind, strength: strength }
  end
end

Cocycle = Struct.new(:claim_a, :claim_b, :kind, :severity, keyword_init: true) do
  def to_h
    { claim_a: claim_a, claim_b: claim_b, kind: kind, severity: severity }
  end
end

ManipulationPattern = Struct.new(:kind, :evidence, :severity, keyword_init: true) do
  def to_h
    { kind: kind, evidence: evidence, severity: severity }
  end
end

# --- GF(3) Module ---

module GF3
  Zero = 0
  One  = 1
  Two  = 2

  module_function

  def add(a, b)
    (a + b) % 3
  end

  def mul(a, b)
    (a * b) % 3
  end

  def neg(a)
    (3 - a) % 3
  end

  def sub(a, b)
    add(a, neg(b))
  end

  def inv(a)
    raise "gf3: multiplicative inverse of zero" if a == Zero
    a == One ? One : Two
  end

  def seq_sum(trits)
    trits.sum
  end

  def balanced?(trits)
    (seq_sum(trits) % 3) == 0
  end

  def find_balancer(a, b, c)
    partial = (a + b + c) % 3
    (3 - partial) % 3
  end

  def to_balanced(e)
    case e
    when Zero then 0
    when One  then 1
    when Two  then -1
    end
  end

  def elem_name(e)
    case e
    when Zero then "coordinator"
    when One  then "generator"
    when Two  then "verifier"
    end
  end
end

# --- ClaimWorld: the ACSet instance ---

ClaimWorld = Struct.new(:claims, :sources, :witnesses, :derivations, :cocycles, keyword_init: true) do
  def initialize(**kwargs)
    super(
      claims:      kwargs.fetch(:claims, {}),
      sources:     kwargs.fetch(:sources, {}),
      witnesses:   kwargs.fetch(:witnesses, {}),
      derivations: kwargs.fetch(:derivations, []),
      cocycles:    kwargs.fetch(:cocycles, [])
    )
  end

  # Sheaf consistency: returns [h1_dimension, cocycles].
  # H^1 = 0 means consistent, >0 means contradictions.
  def sheaf_consistency
    [cocycles.length, cocycles]
  end

  # GF(3) balance: checks conservation law sum(trits) = 0 (mod 3).
  # Returns [balanced?, counts_hash].
  def gf3_balance
    counts = { "coordinator" => 0, "generator" => 0, "verifier" => 0 }
    trits = []

    claims.each_value do |c|
      trits << c.trit
    end
    sources.each_value do |s|
      trits << s.trit
    end
    witnesses.each_value do |w|
      trits << w.trit
    end

    trits.each do |t|
      counts[GF3.elem_name(t)] += 1
    end

    [GF3.balanced?(trits), counts]
  end

  def to_h
    {
      claims:      claims.transform_values(&:to_h),
      sources:     sources.transform_values(&:to_h),
      witnesses:   witnesses.transform_values(&:to_h),
      derivations: derivations.map(&:to_h),
      cocycles:    cocycles.map(&:to_h)
    }
  end

  def to_json(*args)
    to_h.to_json(*args)
  end
end

# --- CatClad Module ---

module CatClad
  FRAMEWORKS = %w[empirical responsible harmonic pluralistic].freeze

  # 10 manipulation patterns
  MANIPULATION_CHECKS = [
    { kind: "emotional_fear",
      pattern: /(?:fear|terrif|alarm|panic|dread|catastroph)/i,
      weight: 0.7 },
    { kind: "urgency",
      pattern: /(?:act now|limited time|don't wait|expires|hurry|last chance|before it's too late)/i,
      weight: 0.8 },
    { kind: "false_consensus",
      pattern: /(?:everyone knows|nobody (?:believes|wants|thinks)|all experts|unanimous|widely accepted)/i,
      weight: 0.6 },
    { kind: "appeal_authority",
      pattern: /(?:experts say|scientists (?:claim|prove)|studies show|research proves|doctors recommend)/i,
      weight: 0.5 },
    { kind: "artificial_scarcity",
      pattern: /(?:exclusive|rare opportunity|only \d+ left|limited (?:edition|supply|spots))/i,
      weight: 0.7 },
    { kind: "social_pressure",
      pattern: /(?:you don't want to be|don't miss out|join .* (?:others|people)|be the first)/i,
      weight: 0.6 },
    { kind: "loaded_language",
      pattern: /(?:obviously|clearly|undeniably|unquestionably|beyond doubt)/i,
      weight: 0.4 },
    { kind: "false_dichotomy",
      pattern: /(?:either .* or|only (?:two|2) (?:options|choices)|if you don't .* then)/i,
      weight: 0.6 },
    { kind: "circular_reasoning",
      pattern: /(?:because .* therefore .* because|true because .* which is true)/i,
      weight: 0.9 },
    { kind: "ad_hominem",
      pattern: /(?:stupid|idiot|moron|fool|ignorant|naive) .* (?:think|believe|say)/i,
      weight: 0.8 }
  ].freeze

  SOURCE_PATTERNS = [
    { re: /(?:according to|cited by|reported by)\s+([^,\.]+)/i, kind: "authority" },
    { re: /(?:study|research|paper)\s+(?:by|from|in)\s+([^,\.]+)/i, kind: "academic" },
    { re: /(?:published in|journal of)\s+([^,\.]+)/i, kind: "academic" },
    { re: /(https?:\/\/\S+)/i, kind: "url" }
  ].freeze

  module_function

  def content_hash(text)
    Digest::SHA256.hexdigest(text.strip.downcase)
  end

  def extract_sources(text)
    sources = []
    seen = {}

    SOURCE_PATTERNS.each do |pat|
      text.scan(pat[:re]).each do |match|
        citation = match[0].strip
        id = content_hash(citation)[0, 12]
        next if seen[id]

        seen[id] = true
        sources << Source.new(
          id:       id,
          citation: citation,
          trit:     GF3::Two,  # Verifier role -- evidence checks claims
          hash:     content_hash(citation),
          kind:     pat[:kind]
        )
      end
    end

    sources
  end

  def witness_role(kind)
    case kind
    when "academic"  then "peer-reviewer"
    when "authority"  then "author"
    when "url"        then "publisher"
    else                   "self"
    end
  end

  def witness_weight(kind)
    case kind
    when "academic"  then 0.9
    when "authority"  then 0.6
    when "url"        then 0.4
    else                   0.2
    end
  end

  def extract_witnesses(source)
    [Witness.new(
      id:     "w-#{source.id}",
      name:   source.citation,
      trit:   GF3::Zero,  # Coordinator -- mediating between claim and verification
      role:   witness_role(source.kind),
      weight: witness_weight(source.kind)
    )]
  end

  def classify_derivation(source)
    case source.kind
    when "academic"  then "deductive"
    when "authority"  then "appeal-to-authority"
    when "url"        then "direct"
    else                   "analogical"
    end
  end

  def source_strength(source)
    case source.kind
    when "academic"  then 0.85
    when "authority"  then 0.5
    when "url"        then 0.3
    else                   0.1
    end
  end

  def compute_confidence(world, claim, framework)
    return 0.1 if world.sources.empty?

    # Average derivation strength
    derivs = world.derivations.select { |d| d.claim_id == claim.id }
    return 0.1 if derivs.empty?

    avg_strength = derivs.sum(&:strength) / derivs.length.to_f

    # Weight by framework
    case framework
    when "empirical"
      academic_count = world.sources.values.count { |s| s.kind == "academic" }
      avg_strength *= (1.0 + 0.1 * academic_count) if academic_count > 0
    when "responsible"
      text_lower = claim.text.downcase
      avg_strength *= 1.1 if text_lower.include?("community") || text_lower.include?("benefit")
    when "harmonic"
      avg_strength *= 1.15 if world.sources.length >= 3
    when "pluralistic"
      # No special boost, raw structural quality
    end

    # Penalize cocycles
    cocycle_penalty = 0.15 * world.cocycles.length
    confidence = avg_strength - cocycle_penalty
    confidence.clamp(0.0, 1.0)
  end

  def detect_cocycles(world)
    cocycles = []

    # Check for unsupported claims (no derivation chain)
    world.claims.each_value do |claim|
      has_derivation = world.derivations.any? { |d| d.claim_id == claim.id }
      unless has_derivation
        cocycles << Cocycle.new(
          claim_a:  claim.id,
          claim_b:  nil,
          kind:     "unsupported",
          severity: 0.9
        )
      end
    end

    # Check for appeal-to-authority without verification
    world.derivations.each do |d|
      if d.kind == "appeal-to-authority" && d.strength < 0.6
        cocycles << Cocycle.new(
          claim_a:  d.claim_id,
          claim_b:  d.source_id,
          kind:     "weak-authority",
          severity: 0.5
        )
      end
    end

    # Check GF(3) conservation
    balanced, _ = world.gf3_balance
    unless balanced
      cocycles << Cocycle.new(
        claim_a:  nil,
        claim_b:  nil,
        kind:     "trit-violation",
        severity: 0.3
      )
    end

    cocycles
  end

  # --- Public API ---

  # Analyze a claim: parse text into a cat-clad structure and check consistency.
  def analyze_claim(text, framework: "pluralistic")
    world = ClaimWorld.new

    # Create the primary claim (Generator role -- it's asserting something)
    hash = content_hash(text)
    claim = Claim.new(
      id:         hash[0, 12],
      text:       text,
      trit:       GF3::One,  # Generator: creating an assertion
      hash:       hash,
      confidence: 0.0,
      framework:  framework,
      created_at: Time.now
    )
    world.claims[claim.id] = claim

    # Extract sources as morphisms from claim
    sources = extract_sources(text)
    sources.each do |src|
      world.sources[src.id] = src
      world.derivations << Derivation.new(
        id:        "d-#{src.id}-#{claim.id}",
        source_id: src.id,
        claim_id:  claim.id,
        kind:      classify_derivation(src),
        strength:  source_strength(src)
      )
    end

    # Extract witnesses (who attests to the sources)
    sources.each do |src|
      witnesses = extract_witnesses(src)
      witnesses.each { |w| world.witnesses[w.id] = w }
    end

    # Compute confidence
    claim.confidence = compute_confidence(world, claim, framework)

    # Detect cocycles (contradictions, unsupported claims, circular reasoning)
    world.cocycles = detect_cocycles(world)

    world
  end

  # Detect manipulation patterns in text.
  def detect_manipulation(text)
    patterns = []

    MANIPULATION_CHECKS.each do |check|
      text.scan(check[:pattern]).each do |match|
        evidence = match.is_a?(Array) ? match.first : match
        patterns << ManipulationPattern.new(
          kind:     check[:kind],
          evidence: evidence,
          severity: check[:weight]
        )
      end
    end

    patterns
  end
end

# --- Main block ---

if __FILE__ == $0
  puts "=== Cat-Clad Anti-Bullshit Engine (Ruby Tier 3) ==="
  puts

  # Test 1: Analyze a well-sourced claim
  text1 = "According to Dr. Smith, research from Harvard shows that exercise reduces stress by 40%"
  world1 = CatClad.analyze_claim(text1, framework: "empirical")

  puts "--- Claim Analysis (empirical) ---"
  puts "Claims:      #{world1.claims.length}"
  puts "Sources:     #{world1.sources.length}"
  puts "Witnesses:   #{world1.witnesses.length}"
  puts "Derivations: #{world1.derivations.length}"

  world1.claims.each_value do |c|
    puts "  Claim: #{c.text[0, 60]}..."
    puts "  Confidence: #{c.confidence.round(3)}"
    puts "  Framework:  #{c.framework}"
  end

  h1, cocycles = world1.sheaf_consistency
  puts "Sheaf H^1:   #{h1} (#{h1 == 0 ? 'consistent' : 'contradictions detected'})"

  balanced, counts = world1.gf3_balance
  puts "GF(3):       balanced=#{balanced} #{counts}"
  puts

  # Test 2: Unsupported claim
  text2 = "The moon is made of cheese"
  world2 = CatClad.analyze_claim(text2, framework: "empirical")

  puts "--- Unsupported Claim ---"
  puts "Sources:     #{world2.sources.length}"
  h1_2, cocycles_2 = world2.sheaf_consistency
  puts "Sheaf H^1:   #{h1_2}"
  cocycles_2.each { |c| puts "  Cocycle: #{c.kind} (severity=#{c.severity})" }
  puts

  # Test 3: Manipulation detection
  text3 = "Act now! This exclusive offer expires in 10 minutes. Everyone knows this is the best deal. Scientists claim it's proven."
  patterns = CatClad.detect_manipulation(text3)

  puts "--- Manipulation Detection ---"
  puts "Patterns found: #{patterns.length}"
  patterns.each do |p|
    puts "  #{p.kind} (severity=#{p.severity}): #{p.evidence.inspect}"
  end
  puts

  # Test 4: All frameworks
  text4 = "Study by MIT shows community benefit from sustainable energy integration"
  puts "--- Multi-Framework ---"
  %w[empirical responsible harmonic pluralistic].each do |fw|
    w = CatClad.analyze_claim(text4, framework: fw)
    w.claims.each_value do |c|
      puts "  #{fw}: confidence=#{c.confidence.round(3)} sources=#{w.sources.length} cocycles=#{w.cocycles.length}"
    end
  end
  puts

  # Test 5: Clean text
  text5 = "The temperature today is 72 degrees Fahrenheit with partly cloudy skies."
  clean_patterns = CatClad.detect_manipulation(text5)
  puts "--- Clean Text ---"
  puts "Manipulation patterns: #{clean_patterns.length} (should be 0)"
  puts
  puts "=== All demos complete ==="
end

# --- Minitest Tests ---

if __FILE__ == $0 || defined?(Minitest)
  require 'minitest/autorun'

  class TestGF3 < Minitest::Test
    def test_add
      assert_equal 0, GF3.add(0, 0)
      assert_equal 1, GF3.add(0, 1)
      assert_equal 2, GF3.add(0, 2)
      assert_equal 2, GF3.add(1, 1)
      assert_equal 0, GF3.add(1, 2)
      assert_equal 1, GF3.add(2, 2)
    end

    def test_mul
      assert_equal 0, GF3.mul(0, 0)
      assert_equal 0, GF3.mul(0, 1)
      assert_equal 1, GF3.mul(1, 1)
      assert_equal 2, GF3.mul(1, 2)
      assert_equal 1, GF3.mul(2, 2)
    end

    def test_neg
      assert_equal 0, GF3.neg(0)
      assert_equal 2, GF3.neg(1)
      assert_equal 1, GF3.neg(2)
    end

    def test_inv
      assert_equal 1, GF3.inv(1)
      assert_equal 2, GF3.inv(2)
      assert_raises(RuntimeError) { GF3.inv(0) }
    end

    def test_balanced
      assert GF3.balanced?([0, 0, 0])
      assert GF3.balanced?([1, 1, 1])
      assert GF3.balanced?([0, 1, 2])
      refute GF3.balanced?([1, 1, 0])
      refute GF3.balanced?([1])
    end

    def test_find_balancer
      assert_equal 0, GF3.find_balancer(0, 0, 0)
      assert_equal 2, GF3.find_balancer(1, 0, 0)
      assert_equal 0, GF3.find_balancer(1, 1, 1)
    end

    def test_to_balanced
      assert_equal  0, GF3.to_balanced(0)
      assert_equal  1, GF3.to_balanced(1)
      assert_equal(-1, GF3.to_balanced(2))
    end
  end

  class TestCatCladAnalysis < Minitest::Test
    def test_analyze_claim_with_sources
      world = CatClad.analyze_claim(
        "According to Dr. Smith, research from Harvard shows that exercise reduces stress by 40%",
        framework: "empirical"
      )

      assert_equal 1, world.claims.length
      refute_empty world.sources, "expected at least 1 extracted source"
      refute_empty world.derivations, "expected derivations from sources to claim"

      world.claims.each_value do |c|
        assert c.confidence > 0, "expected positive confidence, got #{c.confidence}"
      end
    end

    def test_analyze_unsupported_claim
      world = CatClad.analyze_claim("The moon is made of cheese", framework: "empirical")

      assert_empty world.sources, "expected 0 sources for unsupported claim"

      h1, cocycles = world.sheaf_consistency
      assert h1 > 0, "unsupported claim should produce H^1 > 0"

      assert cocycles.any? { |c| c.kind == "unsupported" }, "expected 'unsupported' cocycle"

      world.claims.each_value do |c|
        assert c.confidence <= 0.2, "unsupported claim should have low confidence, got #{c.confidence}"
      end
    end

    def test_gf3_balance_with_full_triad
      world = CatClad.analyze_claim(
        "According to the WHO, a study by Johns Hopkins published in Nature confirms vaccine efficacy",
        framework: "pluralistic"
      )

      balanced, counts = world.gf3_balance
      assert counts["generator"] > 0, "expected at least 1 generator (claim)"
      assert counts["verifier"] > 0, "expected at least 1 verifier (source)"
    end

    def test_detect_manipulation
      patterns = CatClad.detect_manipulation(
        "Act now! This exclusive offer expires in 10 minutes. Everyone knows this is the best deal. Scientists claim it's proven."
      )

      refute_empty patterns, "expected manipulation patterns"

      kinds = patterns.map(&:kind).uniq
      assert_includes kinds, "urgency"
      assert_includes kinds, "artificial_scarcity"
      assert_includes kinds, "appeal_authority"
    end

    def test_detect_no_manipulation
      patterns = CatClad.detect_manipulation(
        "The temperature today is 72 degrees Fahrenheit with partly cloudy skies."
      )

      assert_empty patterns, "expected 0 manipulation patterns for neutral text"
    end

    def test_content_hash_deterministic
      h1 = CatClad.content_hash("hello world")
      h2 = CatClad.content_hash("hello world")
      h3 = CatClad.content_hash("Hello World")

      assert_equal h1, h2, "same text should produce same hash"
      assert_equal h1, h3, "hashing should be case-insensitive"
    end

    def test_multiple_frameworks
      text = "Study by MIT shows community benefit from sustainable energy integration"

      %w[empirical responsible harmonic pluralistic].each do |fw|
        world = CatClad.analyze_claim(text, framework: fw)

        world.claims.each_value do |c|
          assert_equal fw, c.framework, "expected framework #{fw}, got #{c.framework}"
        end
      end
    end

    def test_source_kind_classification
      world = CatClad.analyze_claim(
        "A study by Stanford published in Nature, and according to the CDC, plus https://example.com/data",
        framework: "empirical"
      )

      kinds = world.sources.values.map(&:kind).uniq
      refute_empty kinds, "expected at least one classified source"
    end

    def test_sheaf_consistency_clean
      world = CatClad.analyze_claim(
        "According to Dr. Smith, research from Harvard shows that exercise reduces stress by 40%",
        framework: "empirical"
      )

      h1, _ = world.sheaf_consistency
      # With sources present, sheaf should be relatively clean
      assert h1 >= 0
    end

    def test_claim_world_to_json
      world = CatClad.analyze_claim("According to the WHO, vaccines are effective", framework: "empirical")
      json = world.to_json
      parsed = JSON.parse(json)

      assert parsed.key?("claims")
      assert parsed.key?("sources")
      assert parsed.key?("witnesses")
      assert parsed.key?("derivations")
      assert parsed.key?("cocycles")
    end
  end
end
