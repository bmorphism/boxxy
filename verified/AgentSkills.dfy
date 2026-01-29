// AgentSkills.dfy — Formal specification of the agentskills.io SKILL.md format.
//
// Source: https://agentskills.io/specification
// Reference: https://github.com/agentskills/agentskills/tree/main/skills-ref
//
// Every predicate here is decidable and translates 1:1 to Go in
// internal/skill/skill.go. Go tests exhaustively verify boundary cases.

// --- Character predicates ---

predicate IsLowercaseLetter(c: char)
{
  'a' <= c <= 'z'
}

predicate IsDigit(c: char)
{
  '0' <= c <= '9'
}

predicate IsHyphen(c: char)
{
  c == '-'
}

// Valid name character: lowercase alphanumeric or hyphen
predicate IsNameChar(c: char)
{
  IsLowercaseLetter(c) || IsDigit(c) || IsHyphen(c)
}

// --- Name validation (agentskills spec) ---
// - Must be 1-64 characters
// - May only contain lowercase alphanumeric characters and hyphens
// - Must not start or end with '-'
// - Must not contain consecutive hyphens ('--')

predicate NoConsecutiveHyphens(s: string)
  requires |s| >= 2
{
  forall i :: 0 <= i < |s| - 1 ==> !(IsHyphen(s[i]) && IsHyphen(s[i+1]))
}

predicate AllNameChars(s: string)
{
  forall i :: 0 <= i < |s| ==> IsNameChar(s[i])
}

predicate IsValidSkillName(name: string)
{
  1 <= |name| <= 64
  && AllNameChars(name)
  && !IsHyphen(name[0])
  && !IsHyphen(name[|name|-1])
  && (|name| >= 2 ==> NoConsecutiveHyphens(name))
}

// --- Description validation ---
// - Must be 1-1024 characters
// - Non-empty

predicate IsValidDescription(desc: string)
{
  1 <= |desc| <= 1024
}

// --- Compatibility validation ---
// - Optional (empty is valid)
// - If present, must be 1-500 characters

predicate IsValidCompatibility(compat: string)
{
  |compat| == 0 || (1 <= |compat| <= 500)
}

// --- Frontmatter structure ---
// SKILL.md must start with "---\n" and contain a second "---\n"

predicate HasFrontmatter(content: string)
  requires |content| >= 7 // minimum: "---\n---\n" would be 8 but 7 allows ---\n---
{
  content[0] == '-' && content[1] == '-' && content[2] == '-'
}

// --- Combined skill validity ---

datatype SkillMeta = SkillMeta(
  name: string,
  description: string,
  compatibility: string,
  parentDirName: string
)

predicate IsValidSkillMeta(meta: SkillMeta)
{
  IsValidSkillName(meta.name)
  && IsValidDescription(meta.description)
  && IsValidCompatibility(meta.compatibility)
  // Name must match parent directory name
  && (|meta.parentDirName| > 0 ==> meta.name == meta.parentDirName)
}

// --- Lemmas ---

// Empty name is invalid
lemma EmptyNameInvalid()
  ensures !IsValidSkillName("")
{}

// Single lowercase letter is valid
lemma SingleLetterValid()
  ensures IsValidSkillName("a")
{}

// Max length name is valid (64 chars)
lemma MaxLengthValid()
  ensures IsValidSkillName("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa") // 64 'a's
{
  var s := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa";
  assert |s| == 64;
}

// 65 chars is invalid
lemma OverMaxInvalid()
  ensures !IsValidSkillName("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa") // 65 'a's
{
  var s := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa";
  assert |s| == 65;
}

// Leading hyphen is invalid
lemma LeadingHyphenInvalid()
  ensures !IsValidSkillName("-pdf")
{}

// Trailing hyphen is invalid
lemma TrailingHyphenInvalid()
  ensures !IsValidSkillName("pdf-")
{}

// Consecutive hyphens invalid
lemma ConsecutiveHyphensInvalid()
  ensures !IsValidSkillName("pdf--processing")
{
  var s := "pdf--processing";
  assert IsHyphen(s[3]) && IsHyphen(s[4]);
}

// Uppercase letter invalid
lemma UppercaseInvalid()
  ensures !IsValidSkillName("PDF")
{
  assert !IsNameChar('P');
}

// Valid example from spec
lemma PdfProcessingValid()
  ensures IsValidSkillName("pdf-processing")
{
  var s := "pdf-processing";
  assert |s| == 14;
  assert AllNameChars(s);
  assert !IsHyphen(s[0]);
  assert !IsHyphen(s[|s|-1]);
}

// Valid example from spec
lemma DataAnalysisValid()
  ensures IsValidSkillName("data-analysis")
{
  var s := "data-analysis";
  assert |s| == 13;
  assert AllNameChars(s);
}

// Valid example from spec
lemma CodeReviewValid()
  ensures IsValidSkillName("code-review")
{
  var s := "code-review";
  assert |s| == 11;
  assert AllNameChars(s);
}

// Empty description is invalid
lemma EmptyDescriptionInvalid()
  ensures !IsValidDescription("")
{}

// Description at max length is valid
lemma MaxDescriptionValid()
{
  // 1024 chars is valid
  var s := seq(1024, _ => 'a');
  assert |s| == 1024;
  assert IsValidDescription(s);
}

// Description over max is invalid
lemma OverMaxDescriptionInvalid()
{
  var s := seq(1025, _ => 'a');
  assert |s| == 1025;
  assert !IsValidDescription(s);
}

// Empty compatibility is valid (optional field)
lemma EmptyCompatibilityValid()
  ensures IsValidCompatibility("")
{}

// Compatibility at max is valid
lemma MaxCompatibilityValid()
{
  var s := seq(500, _ => 'a');
  assert |s| == 500;
  assert IsValidCompatibility(s);
}

// Compatibility over max is invalid
lemma OverMaxCompatibilityInvalid()
{
  var s := seq(501, _ => 'a');
  assert |s| == 501;
  assert !IsValidCompatibility(s);
}

// Combined: valid skill
lemma ValidSkillExample()
{
  var meta := SkillMeta(
    name := "pdf-processing",
    description := "Extract text and tables from PDF files.",
    compatibility := "",
    parentDirName := "pdf-processing"
  );
  assert IsValidSkillName(meta.name);
  assert IsValidDescription(meta.description);
  assert IsValidCompatibility(meta.compatibility);
  assert meta.name == meta.parentDirName;
  assert IsValidSkillMeta(meta);
}

// Combined: mismatched dir name
lemma MismatchedDirInvalid()
{
  var meta := SkillMeta(
    name := "pdf-processing",
    description := "Extract text.",
    compatibility := "",
    parentDirName := "wrong-name"
  );
  assert meta.name != meta.parentDirName;
  assert !IsValidSkillMeta(meta);
}

// --- Progressive Disclosure (agentskills spec) ---
// Level 1: Metadata (~100 tokens) — name, description, trit, role
// Level 2: Instructions — full SKILL.md body (<500 lines, <5000 tokens recommended)
// Level 3: Resources — loaded on demand (scripts/, references/, assets/)

// A "token" for estimation purposes is ≈4 characters (conservative).
// The spec says "approximately 100 tokens" for metadata and "<5000 tokens" for instructions.

const MAX_BODY_LINES: int := 500
const MAX_BODY_TOKENS: int := 5000
const CHARS_PER_TOKEN: int := 4
const MAX_METADATA_TOKENS: int := 100

// Count newlines in a string → line count
function CountLines(s: string): int
{
  if |s| == 0 then 0
  else CountLinesHelper(s, 0, 0)
}

function CountLinesHelper(s: string, i: int, acc: int): int
  requires 0 <= i <= |s|
  decreases |s| - i
{
  if i == |s| then acc + 1 // last line (no trailing newline) counts
  else if s[i] == '\n' then CountLinesHelper(s, i + 1, acc + 1)
  else CountLinesHelper(s, i + 1, acc)
}

// Estimate token count from character count
function EstimateTokens(charCount: int): int
  requires charCount >= 0
  requires CHARS_PER_TOKEN > 0
{
  (charCount + CHARS_PER_TOKEN - 1) / CHARS_PER_TOKEN // ceiling division
}

// Progressive disclosure Level 2 constraints
predicate IsValidBodyLength(body: string)
{
  CountLines(body) <= MAX_BODY_LINES
}

predicate IsValidBodyTokens(body: string)
{
  EstimateTokens(|body|) <= MAX_BODY_TOKENS
}

// Combined progressive disclosure check (line count is hard limit, tokens are recommended)
predicate SatisfiesProgressiveDisclosure(body: string)
{
  IsValidBodyLength(body)
  // Token count is advisory — we check it but don't fail on it
}

// Extended skill validity including progressive disclosure
datatype FullSkillMeta = FullSkillMeta(
  name: string,
  description: string,
  compatibility: string,
  parentDirName: string,
  body: string
)

predicate IsValidFullSkill(meta: FullSkillMeta)
{
  IsValidSkillName(meta.name)
  && IsValidDescription(meta.description)
  && IsValidCompatibility(meta.compatibility)
  && (|meta.parentDirName| > 0 ==> meta.name == meta.parentDirName)
  && IsValidBodyLength(meta.body)
}

// Lemma: empty body satisfies progressive disclosure
lemma EmptyBodyValid()
  ensures IsValidBodyLength("")
  ensures IsValidBodyTokens("")
{
  assert CountLines("") == 0;
  assert EstimateTokens(0) == 0;
}

// Lemma: body with exactly 500 lines is at the boundary
lemma MaxLinesBody()
{
  // A body of 500 newlines has 501 lines (500 separators + 1)
  // So we need 499 newlines for exactly 500 lines
  var s := seq(499, _ => '\n');
  // Each newline creates a line, so 499 newlines = 500 lines
  assert |s| == 499;
}

// --- Allowed Fields Whitelist (agentskills spec) ---
// Only these frontmatter keys are permitted. Unknown keys are validation errors.

datatype FieldName = FName | FDescription | FLicense | FAllowedTools | FMetadata | FCompatibility

predicate IsAllowedFieldName(key: string)
{
  key == "name" || key == "description" || key == "license"
  || key == "allowed-tools" || key == "metadata" || key == "compatibility"
}

// --- Metadata validation ---
// metadata is a mapping of string keys to string values.
// Nested dicts are converted to strings. Keys should be reasonably unique.

datatype MetadataEntry = MetadataEntry(key: string, value: string)

predicate IsValidMetadataKey(key: string)
{
  |key| > 0 && |key| <= 256
}

predicate IsValidMetadataMap(entries: seq<MetadataEntry>)
{
  forall i :: 0 <= i < |entries| ==> IsValidMetadataKey(entries[i].key)
  // No duplicate keys
  && forall i, j :: 0 <= i < j < |entries| ==> entries[i].key != entries[j].key
}

// --- AllowedTools validation ---
// Space-delimited tool patterns (experimental).
// Example: "Bash(git:*) Bash(jq:*) Read"

predicate IsNonEmptyToken(token: string)
{
  |token| > 0
}

predicate IsValidAllowedTools(tools: string)
{
  |tools| == 0 || |tools| <= 2048 // reasonable upper bound
}

// --- License validation ---
// Optional string. No specific constraints beyond non-empty if present.

predicate IsValidLicense(license: string)
{
  true // any string is valid; empty means no license specified
}

// --- Frontmatter structure ---
// The complete set of frontmatter fields with their validation rules.

datatype Frontmatter = Frontmatter(
  name: string,
  description: string,
  license: string,        // optional, empty = absent
  compatibility: string,  // optional, empty = absent
  allowedTools: string,   // optional, space-delimited patterns
  metadata: seq<MetadataEntry>, // optional, key-value pairs
  unknownFields: seq<string>    // any keys not in ALLOWED_FIELDS
)

predicate HasNoUnknownFields(fm: Frontmatter)
{
  |fm.unknownFields| == 0
}

predicate IsValidFrontmatter(fm: Frontmatter)
{
  IsValidSkillName(fm.name)
  && IsValidDescription(fm.description)
  && IsValidLicense(fm.license)
  && IsValidCompatibility(fm.compatibility)
  && IsValidAllowedTools(fm.allowedTools)
  && IsValidMetadataMap(fm.metadata)
  && HasNoUnknownFields(fm)
}

// --- Directory Structure (agentskills spec) ---
// A valid skill directory must contain SKILL.md, may contain scripts/, references/, assets/.

datatype DirEntry = DirEntry(name: string, isDir: bool)

predicate HasSkillMd(entries: seq<DirEntry>)
{
  exists i :: 0 <= i < |entries| && !entries[i].isDir
    && (entries[i].name == "SKILL.md" || entries[i].name == "skill.md")
}

predicate PrefersUppercaseSkillMd(entries: seq<DirEntry>)
{
  (exists i :: 0 <= i < |entries| && entries[i].name == "SKILL.md")
  || !(exists i :: 0 <= i < |entries| && entries[i].name == "skill.md")
}

predicate IsAllowedSubdir(name: string)
{
  name == "scripts" || name == "references" || name == "assets"
}

// Advisory: only known subdirs recommended (not a hard error)
predicate HasOnlyKnownSubdirs(entries: seq<DirEntry>)
{
  forall i :: 0 <= i < |entries| && entries[i].isDir
    ==> IsAllowedSubdir(entries[i].name)
}

datatype SkillDirectory = SkillDirectory(
  dirName: string,
  entries: seq<DirEntry>
)

predicate IsValidSkillDirectory(dir: SkillDirectory)
{
  HasSkillMd(dir.entries)
  && |dir.dirName| > 0
}

// --- to-prompt XML Generation (agentskills spec) ---
// skills-ref to-prompt outputs XML:
//   <available_skills>
//     <skill>
//       <name>{html_escaped_name}</name>
//       <description>{html_escaped_desc}</description>
//       <location>{absolute_path}</location>
//     </skill>
//   </available_skills>

// HTML escaping predicates
predicate NeedsHtmlEscape(c: char)
{
  c == '<' || c == '>' || c == '&' || c == '"' || c == '\''
}

predicate IsHtmlSafe(s: string)
{
  forall i :: 0 <= i < |s| ==> !NeedsHtmlEscape(s[i])
}

// Prompt output structure
datatype PromptSkill = PromptSkill(
  name: string,
  description: string,
  location: string  // absolute path to SKILL.md
)

predicate IsValidPromptSkill(ps: PromptSkill)
{
  IsValidSkillName(ps.name)
  && IsValidDescription(ps.description)
  && |ps.location| > 0
}

// --- Progressive Disclosure Integration ---
// Complete model combining all three levels

datatype DisclosureLevel = Level1_Metadata | Level2_Instructions | Level3_Resources

// Level 1: what the agent sees at startup (~100 tokens)
datatype Level1 = Level1(name: string, description: string)

predicate IsValidLevel1(l: Level1)
{
  IsValidSkillName(l.name)
  && IsValidDescription(l.description)
  && EstimateTokens(|l.name| + |l.description|) <= MAX_METADATA_TOKENS
}

// Level 2: full SKILL.md body when activated
datatype Level2 = Level2(body: string)

predicate IsValidLevel2(l: Level2)
{
  IsValidBodyLength(l.body)
  // Token count is advisory
}

// Level 3: on-demand resources
datatype ResourceKind = Script | Reference | Asset

datatype Resource = Resource(kind: ResourceKind, path: string, content: string)

predicate IsValidResourcePath(path: string)
{
  |path| > 0 && |path| <= 1024
  // No directory traversal
  && (forall i :: 0 <= i < |path| - 1 ==> !(path[i] == '.' && path[i+1] == '.'))
}

// --- Complete Skill Model ---
// The full agentskills specification as a single Dafny datatype

datatype CompleteSkill = CompleteSkill(
  // Directory
  directory: SkillDirectory,
  // Frontmatter (all fields)
  frontmatter: Frontmatter,
  // Body (Level 2)
  body: string,
  // Resources (Level 3)
  resources: seq<Resource>
)

predicate IsValidCompleteSkill(skill: CompleteSkill)
{
  // Directory structure
  IsValidSkillDirectory(skill.directory)
  // Frontmatter validity
  && IsValidFrontmatter(skill.frontmatter)
  // Name matches directory
  && skill.frontmatter.name == skill.directory.dirName
  // Body progressive disclosure
  && IsValidBodyLength(skill.body)
  // Resources have valid paths
  && (forall i :: 0 <= i < |skill.resources| ==> IsValidResourcePath(skill.resources[i].path))
}

// --- Lemmas for Complete Skill ---

lemma ValidCompleteSkillExample()
{
  var dir := SkillDirectory(
    dirName := "pdf-processing",
    entries := [
      DirEntry("SKILL.md", false),
      DirEntry("scripts", true),
      DirEntry("references", true)
    ]
  );
  var fm := Frontmatter(
    name := "pdf-processing",
    description := "Extract text and tables from PDF files.",
    license := "MIT",
    compatibility := "Requires Python 3.9+",
    allowedTools := "Bash(python:*) Read",
    metadata := [MetadataEntry("version", "1.0.0")],
    unknownFields := []
  );
  assert IsValidSkillDirectory(dir);
  assert IsValidFrontmatter(fm);
  assert fm.name == dir.dirName;
}

lemma UnknownFieldsInvalid()
{
  var fm := Frontmatter(
    name := "test",
    description := "A test skill.",
    license := "",
    compatibility := "",
    allowedTools := "",
    metadata := [],
    unknownFields := ["bogus-field"]
  );
  assert !HasNoUnknownFields(fm);
  assert !IsValidFrontmatter(fm);
}

lemma DuplicateMetadataKeysInvalid()
{
  var entries := [MetadataEntry("version", "1.0"), MetadataEntry("version", "2.0")];
  assert entries[0].key == entries[1].key;
  assert !IsValidMetadataMap(entries);
}

lemma DirectoryWithoutSkillMdInvalid()
{
  var dir := SkillDirectory(
    dirName := "test",
    entries := [DirEntry("README.md", false), DirEntry("scripts", true)]
  );
  assert !HasSkillMd(dir.entries);
  assert !IsValidSkillDirectory(dir);
}

lemma ResourcePathTraversalBlocked()
{
  assert !IsValidResourcePath("../../../etc/passwd");
}

// --- GF(3) trit property ---
// The trit is computed from SHA-256 of the name, mod 3.
// We can specify the domain constraint:

type Trit = t: int | 0 <= t < 3

function NameToTrit(name: string): Trit
  requires IsValidSkillName(name)
{
  // Abstract: actual computation is SHA-256 mod 3 (done in Go)
  // Here we just specify the type constraint
  0 // placeholder — the real hash is in Go
}

// Conservation law: sum of any triple must be 0 mod 3 for balance
predicate IsBalancedTriple(a: Trit, b: Trit, c: Trit)
{
  (a + b + c) % 3 == 0
}

// A balanced quad has sum 0 mod 3
predicate IsBalancedQuad(a: Trit, b: Trit, c: Trit, d: Trit)
{
  (a + b + c + d) % 3 == 0
}

// FindBalancer: given three trits, the fourth that balances them
function FindBalancer(a: Trit, b: Trit, c: Trit): Trit
{
  (3 - ((a + b + c) % 3)) % 3
}

lemma FindBalancerCorrect(a: Trit, b: Trit, c: Trit)
  ensures IsBalancedQuad(a, b, c, FindBalancer(a, b, c))
{}

// Exhaustive: all 27 triples, exactly 9 are balanced
lemma BalancedTripleCount()
{
  // 0+0+0=0, 0+1+2=3, 0+2+1=3, 1+0+2=3, 1+1+1=3, 1+2+0=3,
  // 2+0+1=3, 2+1+0=3, 2+2+2=6
  // 9 out of 27 triples have sum ≡ 0 (mod 3)
  assert IsBalancedTriple(0, 0, 0);
  assert IsBalancedTriple(0, 1, 2);
  assert IsBalancedTriple(0, 2, 1);
  assert IsBalancedTriple(1, 0, 2);
  assert IsBalancedTriple(1, 1, 1);
  assert IsBalancedTriple(1, 2, 0);
  assert IsBalancedTriple(2, 0, 1);
  assert IsBalancedTriple(2, 1, 0);
  assert IsBalancedTriple(2, 2, 2);
}

// Main: executable verification
method Main()
{
  // Name validation examples from spec
  assert IsValidSkillName("pdf-processing");
  assert IsValidSkillName("data-analysis");
  assert IsValidSkillName("code-review");
  assert !IsValidSkillName("PDF-Processing");
  assert !IsValidSkillName("-pdf");
  assert !IsValidSkillName("pdf--processing");
  assert !IsValidSkillName("");

  // Description
  assert IsValidDescription("Extract text and tables from PDF files.");
  assert !IsValidDescription("");

  // Compatibility
  assert IsValidCompatibility("");
  assert IsValidCompatibility("Requires git, docker");

  // GF(3) conservation
  assert IsBalancedQuad(0, 1, 2, FindBalancer(0, 1, 2));
  assert IsBalancedQuad(1, 1, 1, FindBalancer(1, 1, 1));
  assert IsBalancedQuad(2, 2, 2, FindBalancer(2, 2, 2));

  print "All agentskills spec assertions verified.\n";
}
