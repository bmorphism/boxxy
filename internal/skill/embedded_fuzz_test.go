//go:build tinygo

package skill

import "testing"

func FuzzParseEmbeddedSkillLine(f *testing.F) {
	seeds := []string{
		"name=alife;description=Artificial Life;trit=1",
		"name=bad name;description=desc;trit=2",
		"name=ok;description=;trit=0",
		"name=ok;description=desc;trit=3",
		"just-a-bad-line",
	}
	for _, seed := range seeds {
		f.Add([]byte(seed))
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 8*1024 {
			return
		}
		s, _ := ParseEmbeddedSkillLine(string(data))
		if s != nil {
			_ = s.ValidateEmbedded()
		}
	})
}
