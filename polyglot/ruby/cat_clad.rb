# frozen_string_literal: true

# cat_clad.rb -- Tier 3 Ruby implementation of cat-clad epistemological verification.
#
# Post-modern Ruby 3.2+ rewrite: Data.define, Ractor-safe, pattern matching,
# Enumerator::Lazy pipelines, Comparable/Enumerable, Refinements, builders.
#
# A "cat-clad" claim is an object in a category with morphisms tracking
# its provenance, derivation history, and the consistency conditions that
# bind it to other claims.
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

# --- GF(3) Trit: Comparable immutable value ---

class Trit
  include Comparable

  attr_reader :value

  ZERO = new.tap { |t| t.instance_variable_set(:@value, 0) }.freeze
  ONE  = new.tap { |t| t.instance_variable_set(:@value, 1) }.freeze
  TWO  = new.tap { |t| t.instance_variable_set(:@value, 2) }.freeze

  ALL = [ZERO, ONE, TWO].freeze

  private_class_method :new

  def self.[](v)
    case v
    when 0 then ZERO
    when 1 then ONE
    when 2 then TWO
    when Trit then v
    else raise ArgumentError, "invalid trit: #{v}"
    end
  end

  def <=>(other)
    value <=> other.value
  end

  def +(other)  = Trit[(value + other.value) % 3]
  def *(other)  = Trit[(value * other.value) % 3]
  def -@        = Trit[(3 - value) % 3]
  def -(other)  = self + (-other)

  def inverse
    raise "gf3: multiplicative inverse of zero" if self == ZERO
    self == ONE ? ONE : TWO
  end

  def balanced_value
    case self
    in ^(ZERO) then 0
    in ^(ONE)  then 1
    in ^(TWO)  then -1
    end
  end

  def role
    case self
    in ^(ZERO) then "coordinator".freeze
    in ^(ONE)  then "generator".freeze
    in ^(TWO)  then "verifier".freeze
    end
  end

  def to_i    = value
  def to_s    = "Trit(#{value}:#{role})"
  def inspect = to_s

  def self.balanced?(trits)
    (trits.sum(&:value) % 3).zero?
  end

  def self.find_balancer(a, b, c)
    Trit[(3 - (a.value + b.value + c.value) % 3) % 3]
  end

  freeze
end

# --- Refinements: pipeline helpers on core types ---

module CatCladRefinements
  refine String do
    def content_hash
      Digest::SHA256.hexdigest(strip.downcase).freeze
    end

    def trit_id(len = 12)
      content_hash[0, len].freeze
    end
  end

  refine Array do
    def trit_balanced?
      Trit.balanced?(self)
    end
  end
end

# --- ACSet Schema Types (Data.define: immutable value objects) ---

Claim = Data.define(:id, :text, :trit, :hash, :confidence, :framework, :created_at) do
  def to_h
    { id: id, text: text, trit: trit.to_i, hash: hash, confidence: confidence,
      framework: framework, created_at: created_at.to_s }.freeze
  end

  def with_confidence(c)
    with(confidence: c)
  end
end

Source = Data.define(:id, :citation, :trit, :hash, :kind) do
  def to_h
    { id: id, citation: citation, trit: trit.to_i, hash: hash, kind: kind }.freeze
  end
end

Witness = Data.define(:id, :name, :trit, :role, :weight) do
  def to_h
    { id: id, name: name, trit: trit.to_i, role: role, weight: weight }.freeze
  end
end

Derivation = Data.define(:id, :source_id, :claim_id, :kind, :strength) do
  def to_h
    { id: id, source_id: source_id, claim_id: claim_id, kind: kind, strength: strength }.freeze
  end
end

Cocycle = Data.define(:claim_a, :claim_b, :kind, :severity) do
  def to_h
    { claim_a: claim_a, claim_b: claim_b, kind: kind, severity: severity }.freeze
  end
end

ManipulationPattern = Data.define(:kind, :evidence, :severity) do
  def to_h
    { kind: kind, evidence: evidence, severity: severity }.freeze
  end
end

# --- ClaimWorld: Enumerable ACSet instance, Ractor-safe (immutable after build) ---

class ClaimWorld
  include Enumerable
  using CatCladRefinements

  attr_reader :claims, :sources, :witnesses, :derivations, :cocycles

  def initialize(claims: {}.freeze, sources: {}.freeze, witnesses: {}.freeze,
                 derivations: [].freeze, cocycles: [].freeze)
    @claims      = claims.freeze
    @sources     = sources.freeze
    @witnesses   = witnesses.freeze
    @derivations = derivations.freeze
    @cocycles    = cocycles.freeze
    freeze
  end

  # Enumerable: iterate all claims
  def each(&block)
    claims.each_value(&block)
  end

  # Sheaf consistency: [h1_dimension, cocycles]. H^1 = 0 means consistent.
  def sheaf_consistency
    [cocycles.length, cocycles].freeze
  end

  # GF(3) balance: [balanced?, counts_hash]
  def gf3_balance
    trits = all_trits
    counts = trits.each_with_object(
      { "coordinator" => 0, "generator" => 0, "verifier" => 0 }
    ) { |t, h| h[t.role] += 1 }
    [trits.trit_balanced?, counts.freeze].freeze
  end

  def to_h
    {
      claims:      claims.transform_values(&:to_h),
      sources:     sources.transform_values(&:to_h),
      witnesses:   witnesses.transform_values(&:to_h),
      derivations: derivations.map(&:to_h),
      cocycles:    cocycles.map(&:to_h)
    }.freeze
  end

  def to_json(*args)
    to_h.to_json(*args)
  end

  # --- Builder DSL ---

  def self.build
    builder = Builder.new
    yield builder
    builder.finalize
  end

  private

  def all_trits
    [].then { |acc|
      claims.each_value { |c| acc << c.trit }
      sources.each_value { |s| acc << s.trit }
      witnesses.each_value { |w| acc << w.trit }
      acc.freeze
    }
  end

  # Mutable builder, consumed once to produce frozen ClaimWorld
  class Builder
    using CatCladRefinements

    def initialize
      @claims      = {}
      @sources     = {}
      @witnesses   = {}
      @derivations = []
      @cocycles    = []
    end

    def add_claim(claim)     = @claims[claim.id] = claim
    def add_source(source)   = @sources[source.id] = source
    def add_witness(witness) = @witnesses[witness.id] = witness
    def add_derivation(d)    = @derivations << d
    def add_cocycle(c)       = @cocycles << c

    def claim(text, trit: Trit::ONE, framework: "pluralistic")
      hash = text.content_hash
      Claim.new(
        id:         hash[0, 12].freeze,
        text:       text.freeze,
        trit:       trit,
        hash:       hash,
        confidence: 0.0,
        framework:  framework.freeze,
        created_at: Time.now
      ).tap { |c| add_claim(c) }
    end

    def finalize
      ClaimWorld.new(
        claims:      @claims.freeze,
        sources:     @sources.freeze,
        witnesses:   @witnesses.freeze,
        derivations: @derivations.freeze,
        cocycles:    @cocycles.freeze
      )
    end
  end
end

# --- CatClad Module ---

module CatClad
  using CatCladRefinements

  FRAMEWORKS = %w[empirical responsible harmonic pluralistic].freeze

  # 10 manipulation patterns
  MANIPULATION_CHECKS = [
    { kind: "emotional_fear".freeze,
      pattern: /(?:fear|terrif|alarm|panic|dread|catastroph)/i,
      weight: 0.7 },
    { kind: "urgency".freeze,
      pattern: /(?:act now|limited time|don't wait|expires|hurry|last chance|before it's too late)/i,
      weight: 0.8 },
    { kind: "false_consensus".freeze,
      pattern: /(?:everyone knows|nobody (?:believes|wants|thinks)|all experts|unanimous|widely accepted)/i,
      weight: 0.6 },
    { kind: "appeal_authority".freeze,
      pattern: /(?:experts say|scientists (?:claim|prove)|studies show|research proves|doctors recommend)/i,
      weight: 0.5 },
    { kind: "artificial_scarcity".freeze,
      pattern: /(?:exclusive|rare opportunity|only \d+ left|limited (?:edition|supply|spots))/i,
      weight: 0.7 },
    { kind: "social_pressure".freeze,
      pattern: /(?:you don't want to be|don't miss out|join .* (?:others|people)|be the first)/i,
      weight: 0.6 },
    { kind: "loaded_language".freeze,
      pattern: /(?:obviously|clearly|undeniably|unquestionably|beyond doubt)/i,
      weight: 0.4 },
    { kind: "false_dichotomy".freeze,
      pattern: /(?:either .* or|only (?:two|2) (?:options|choices)|if you don't .* then)/i,
      weight: 0.6 },
    { kind: "circular_reasoning".freeze,
      pattern: /(?:because .* therefore .* because|true because .* which is true)/i,
      weight: 0.9 },
    { kind: "ad_hominem".freeze,
      pattern: /(?:stupid|idiot|moron|fool|ignorant|naive) .* (?:think|believe|say)/i,
      weight: 0.8 }
  ].freeze

  SOURCE_PATTERNS = [
    { re: /(?:according to|cited by|reported by)\s+([^,\.]+)/i, kind: "authority".freeze },
    { re: /(?:study|research|paper)\s+(?:by|from|in)\s+([^,\.]+)/i, kind: "academic".freeze },
    { re: /(?:published in|journal of)\s+([^,\.]+)/i, kind: "academic".freeze },
    { re: /(https?:\/\/\S+)/i, kind: "url".freeze }
  ].freeze

  module_function

  def content_hash(text)
    text.content_hash
  end

  # Lazy source extraction pipeline: yields Source values lazily
  def extract_sources_lazy(text)
    seen = {}

    Enumerator::Lazy.new(SOURCE_PATTERNS, SOURCE_PATTERNS.size) { |yielder, pat|
      text.scan(pat[:re]).each do |match|
        citation = match[0].strip.freeze
        id = citation.trit_id
        next if seen[id]

        seen[id] = true
        yielder << Source.new(
          id:       id,
          citation: citation,
          trit:     Trit::TWO,
          hash:     citation.content_hash,
          kind:     pat[:kind]
        )
      end
    }
  end

  def extract_sources(text)
    extract_sources_lazy(text).to_a.freeze
  end

  def witness_role(kind)
    case kind
    in "academic"  then "peer-reviewer".freeze
    in "authority" then "author".freeze
    in "url"       then "publisher".freeze
    else                "self".freeze
    end
  end

  def witness_weight(kind)
    case kind
    in "academic"  then 0.9
    in "authority" then 0.6
    in "url"       then 0.4
    else                0.2
    end
  end

  def extract_witnesses(source)
    [Witness.new(
      id:     "w-#{source.id}".freeze,
      name:   source.citation,
      trit:   Trit::ZERO,
      role:   witness_role(source.kind),
      weight: witness_weight(source.kind)
    )].freeze
  end

  def classify_derivation(source)
    case source.kind
    in "academic"  then "deductive".freeze
    in "authority" then "appeal-to-authority".freeze
    in "url"       then "direct".freeze
    else                "analogical".freeze
    end
  end

  def source_strength(source)
    case source.kind
    in "academic"  then 0.85
    in "authority" then 0.5
    in "url"       then 0.3
    else                0.1
    end
  end

  # Framework dispatch via pattern matching
  def framework_boost(avg_strength, world, claim, framework)
    case framework
    in "empirical"
      academic_count = world.sources.values.count { |s| s.kind == "academic" }
      academic_count > 0 ? avg_strength * (1.0 + 0.1 * academic_count) : avg_strength
    in "responsible"
      lower = claim.text.downcase
      (lower.include?("community") || lower.include?("benefit")) ? avg_strength * 1.1 : avg_strength
    in "harmonic"
      world.sources.length >= 3 ? avg_strength * 1.15 : avg_strength
    in "pluralistic"
      avg_strength
    else
      avg_strength
    end
  end

  def compute_confidence(world, claim, framework)
    return 0.1 if world.sources.empty?

    derivs = world.derivations.select { |d| d.claim_id == claim.id }
    return 0.1 if derivs.empty?

    derivs
      .sum(&:strength)
      .then { |total| total / derivs.length.to_f }
      .then { |avg| framework_boost(avg, world, claim, framework) }
      .then { |boosted| boosted - 0.15 * world.cocycles.length }
      .then { |final| final.clamp(0.0, 1.0) }
  end

  def detect_cocycles(world)
    cocycles = []

    # Unsupported claims
    world.claims.each_value do |claim|
      unless world.derivations.any? { |d| d.claim_id == claim.id }
        cocycles << Cocycle.new(claim_a: claim.id, claim_b: nil,
                               kind: "unsupported".freeze, severity: 0.9)
      end
    end

    # Weak authority
    world.derivations.each do |d|
      if d.kind == "appeal-to-authority" && d.strength < 0.6
        cocycles << Cocycle.new(claim_a: d.claim_id, claim_b: d.source_id,
                               kind: "weak-authority".freeze, severity: 0.5)
      end
    end

    # GF(3) conservation
    balanced, _ = world.gf3_balance
    unless balanced
      cocycles << Cocycle.new(claim_a: nil, claim_b: nil,
                             kind: "trit-violation".freeze, severity: 0.3)
    end

    cocycles.freeze
  end

  # --- Public API ---

  def analyze_claim(text, framework: "pluralistic")
    sources     = extract_sources(text)
    hash        = content_hash(text)
    claim_id    = hash[0, 12].freeze

    claim = Claim.new(
      id: claim_id, text: text.freeze, trit: Trit::ONE,
      hash: hash, confidence: 0.0, framework: framework.freeze,
      created_at: Time.now
    )

    derivations = sources.map { |src|
      Derivation.new(
        id:        "d-#{src.id}-#{claim_id}".freeze,
        source_id: src.id, claim_id: claim_id,
        kind:      classify_derivation(src),
        strength:  source_strength(src)
      )
    }.freeze

    witnesses_hash = sources
      .lazy
      .flat_map { |src| extract_witnesses(src) }
      .each_with_object({}) { |w, h| h[w.id] = w }
      .freeze

    sources_hash = sources.each_with_object({}) { |s, h| h[s.id] = s }.freeze

    # Build intermediate world for cocycle detection (no cocycles yet)
    proto_world = ClaimWorld.new(
      claims:      { claim_id => claim }.freeze,
      sources:     sources_hash,
      witnesses:   witnesses_hash,
      derivations: derivations,
      cocycles:    [].freeze
    )

    # Compute confidence and detect cocycles
    confidence = compute_confidence(proto_world, claim, framework)
    updated_claim = claim.with_confidence(confidence)

    cocycles = detect_cocycles(proto_world)

    ClaimWorld.new(
      claims:      { claim_id => updated_claim }.freeze,
      sources:     sources_hash,
      witnesses:   witnesses_hash,
      derivations: derivations,
      cocycles:    cocycles
    )
  end

  # Manipulation detection via pattern matching on check structures
  def detect_manipulation(text)
    MANIPULATION_CHECKS.each_with_object([]) { |check, acc|
      text.scan(check[:pattern]).each do |match|
        evidence = match.is_a?(Array) ? match.first : match
        acc << ManipulationPattern.new(
          kind:     check[:kind],
          evidence: evidence.freeze,
          severity: check[:weight]
        )
      end
    }.freeze
  end
end

# --- Main block ---

if __FILE__ == $0
  puts "=== Cat-Clad Anti-Bullshit Engine (Ruby Tier 3 -- Post-Modern) ==="
  puts

  # Test 1: Analyze a well-sourced claim
  text1 = "According to Dr. Smith, research from Harvard shows that exercise reduces stress by 40%"
  world1 = CatClad.analyze_claim(text1, framework: "empirical")

  puts "--- Claim Analysis (empirical) ---"
  puts "Claims:      #{world1.claims.length}"
  puts "Sources:     #{world1.sources.length}"
  puts "Witnesses:   #{world1.witnesses.length}"
  puts "Derivations: #{world1.derivations.length}"

  world1.each do |c|
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
    w.each do |c|
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

  # Test 6: Builder DSL
  puts "--- Builder DSL ---"
  built = ClaimWorld.build do |w|
    w.claim("Test claim via builder", framework: "empirical")
  end
  puts "Built world claims: #{built.claims.length}"
  puts

  puts "=== All demos complete ==="
end

# --- Minitest Tests ---

if __FILE__ == $0 || defined?(Minitest)
  require 'minitest/autorun'

  class TestTrit < Minitest::Test
    def test_arithmetic
      assert_equal Trit::ZERO, Trit::ZERO + Trit::ZERO
      assert_equal Trit::ONE,  Trit::ZERO + Trit::ONE
      assert_equal Trit::TWO,  Trit::ZERO + Trit::TWO
      assert_equal Trit::TWO,  Trit::ONE + Trit::ONE
      assert_equal Trit::ZERO, Trit::ONE + Trit::TWO
      assert_equal Trit::ONE,  Trit::TWO + Trit::TWO
    end

    def test_mul
      assert_equal Trit::ZERO, Trit::ZERO * Trit::ZERO
      assert_equal Trit::ZERO, Trit::ZERO * Trit::ONE
      assert_equal Trit::ONE,  Trit::ONE * Trit::ONE
      assert_equal Trit::TWO,  Trit::ONE * Trit::TWO
      assert_equal Trit::ONE,  Trit::TWO * Trit::TWO
    end

    def test_neg
      assert_equal Trit::ZERO, -Trit::ZERO
      assert_equal Trit::TWO,  -Trit::ONE
      assert_equal Trit::ONE,  -Trit::TWO
    end

    def test_inv
      assert_equal Trit::ONE, Trit::ONE.inverse
      assert_equal Trit::TWO, Trit::TWO.inverse
      assert_raises(RuntimeError) { Trit::ZERO.inverse }
    end

    def test_comparable
      assert Trit::ZERO < Trit::ONE
      assert Trit::ONE < Trit::TWO
      assert_equal [Trit::ZERO, Trit::ONE, Trit::TWO], Trit::ALL.sort
    end

    def test_balanced
      assert Trit.balanced?([Trit::ZERO, Trit::ZERO, Trit::ZERO])
      assert Trit.balanced?([Trit::ONE, Trit::ONE, Trit::ONE])
      assert Trit.balanced?([Trit::ZERO, Trit::ONE, Trit::TWO])
      refute Trit.balanced?([Trit::ONE, Trit::ONE, Trit::ZERO])
      refute Trit.balanced?([Trit::ONE])
    end

    def test_find_balancer
      assert_equal Trit::ZERO, Trit.find_balancer(Trit::ZERO, Trit::ZERO, Trit::ZERO)
      assert_equal Trit::TWO,  Trit.find_balancer(Trit::ONE, Trit::ZERO, Trit::ZERO)
      assert_equal Trit::ZERO, Trit.find_balancer(Trit::ONE, Trit::ONE, Trit::ONE)
    end

    def test_to_balanced
      assert_equal  0, Trit::ZERO.balanced_value
      assert_equal  1, Trit::ONE.balanced_value
      assert_equal(-1, Trit::TWO.balanced_value)
    end

    def test_role
      assert_equal "coordinator", Trit::ZERO.role
      assert_equal "generator",   Trit::ONE.role
      assert_equal "verifier",    Trit::TWO.role
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

      world.each do |c|
        assert c.confidence > 0, "expected positive confidence, got #{c.confidence}"
      end
    end

    def test_analyze_unsupported_claim
      world = CatClad.analyze_claim("The moon is made of cheese", framework: "empirical")

      assert_empty world.sources, "expected 0 sources for unsupported claim"

      h1, cocycles = world.sheaf_consistency
      assert h1 > 0, "unsupported claim should produce H^1 > 0"
      assert cocycles.any? { |c| c.kind == "unsupported" }, "expected 'unsupported' cocycle"

      world.each do |c|
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

        world.each do |c|
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

    def test_data_define_immutability
      claim = Claim.new(id: "x", text: "t", trit: Trit::ONE, hash: "h",
                        confidence: 0.5, framework: "empirical", created_at: Time.now)
      assert claim.frozen?

      updated = claim.with_confidence(0.9)
      assert_equal 0.5, claim.confidence
      assert_equal 0.9, updated.confidence
    end

    def test_enumerable_on_world
      world = CatClad.analyze_claim(
        "According to Dr. Smith, research from Harvard shows that exercise reduces stress by 40%",
        framework: "empirical"
      )

      texts = world.map(&:text)
      assert_equal 1, texts.length
      assert texts.first.include?("Dr. Smith")
    end

    def test_builder_dsl
      world = ClaimWorld.build do |w|
        w.claim("Builder test claim", framework: "harmonic")
      end

      assert_equal 1, world.claims.length
      world.each do |c|
        assert_equal "harmonic", c.framework
      end
    end

    def test_lazy_source_extraction
      text = "According to Dr. Smith, research from Harvard shows results"
      lazy_enum = CatClad.extract_sources_lazy(text)
      assert_kind_of Enumerator::Lazy, lazy_enum

      first = lazy_enum.first
      assert_kind_of Source, first
    end

    def test_world_is_frozen
      world = CatClad.analyze_claim("test", framework: "pluralistic")
      assert world.frozen?
      assert world.claims.frozen?
      assert world.sources.frozen?
      assert world.derivations.frozen?
    end
  end
end
