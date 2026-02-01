package skill

import "testing"

func FuzzParseSkill(f *testing.F) {
	seeds := []string{
		"---\nname: test-skill\ndescription: test\n---\n# Title\nBody",
		"# Only body, no frontmatter",
		"---\nname: x\ndescription: y\nversion: 1.0.0\n---\n",
		"---\nname: bad name\ndescription: y\n---\n",
	}
	for _, seed := range seeds {
		f.Add([]byte(seed))
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 64*1024 {
			return
		}
		s, _ := ParseSkill(string(data), "/skills/test-skill/SKILL.md")
		if s != nil {
			_ = s.Validate()
		}
	})
}
