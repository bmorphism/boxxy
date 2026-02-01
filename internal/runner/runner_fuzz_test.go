//go:build darwin

package runner

import "testing"

func FuzzParseRunArgs(f *testing.F) {
	seed := [][]byte{
		[]byte(""),
		[]byte("--efi --iso x.iso"),
		[]byte("--linux --kernel vmlinuz"),
		[]byte("--macos"),
		[]byte("--guix --iso x.iso"),
		[]byte("--guix --kernel vmlinuz --guix-arch x86_64"),
	}
	for _, s := range seed {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		args := splitArgs(string(data))
		_, _ = parseRunArgs(args)
	})
}

func splitArgs(s string) []string {
	fields := make([]string, 0, 16)
	cur := ""
	for _, r := range s {
		if r == ' ' || r == '\n' || r == '\t' || r == '\r' {
			if cur != "" {
				fields = append(fields, cur)
				cur = ""
			}
			continue
		}
		cur += string(r)
	}
	if cur != "" {
		fields = append(fields, cur)
	}
	return fields
}
