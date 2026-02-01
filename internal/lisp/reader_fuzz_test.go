//go:build darwin

package lisp

import (
	"strings"
	"testing"
)

func FuzzReaderReadAll(f *testing.F) {
	seeds := []string{
		"(a b c)",
		"[1 2 3]",
		"{:a 1 :b 2}",
		"\"str\\nwith\\tescapes\"",
		"nil true false 123 4.56",
		"#\n! /usr/bin/env boxxy\n(hello)",
	}
	for _, s := range seeds {
		f.Add([]byte(s))
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		r := NewReader(strings.NewReader(string(data)))
		_, _ = r.ReadAll()
	})
}
