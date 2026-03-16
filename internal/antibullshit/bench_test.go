//go:build darwin

package antibullshit

import (
	"strings"
	"testing"
)

func BenchmarkAnalyzeClaim(b *testing.B) {
	text := "According to Dr. Smith, research from Harvard published in Nature shows that exercise reduces stress by 40%"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		AnalyzeClaim(text, "empirical")
	}
}

func BenchmarkAnalyzeWithCatColab(b *testing.B) {
	text := "According to Dr. Smith, research from Harvard published in Nature shows that exercise reduces stress by 40%"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		AnalyzeWithCatColab(text, "empirical")
	}
}

func BenchmarkDetectManipulation(b *testing.B) {
	text := "Act now! This exclusive offer expires in 10 minutes. Everyone knows this is the best deal. Scientists claim it's proven. Don't miss out!"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		DetectManipulation(text)
	}
}

func BenchmarkContentHash(b *testing.B) {
	text := "benchmark content hashing performance"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		contentHash(text)
	}
}

func BenchmarkExtractSources(b *testing.B) {
	text := "According to WHO, study by MIT, research from Stanford, published in Nature, https://example.com/data"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		extractSources(text)
	}
}

func BenchmarkCatColabPathComposition(b *testing.B) {
	model := AnalyzeWithCatColab(
		"According to Dr. Smith, research from Harvard shows exercise helps",
		"empirical",
	)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, p := range model.Paths {
			p.Composes(model.Theory)
		}
	}
}

func BenchmarkGF3Balance(b *testing.B) {
	model := AnalyzeWithCatColab(
		"According to WHO, study by MIT published in Nature confirms vaccine efficacy",
		"pluralistic",
	)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		model.GF3Balance()
	}
}

func BenchmarkDetectManipulationLargeText(b *testing.B) {
	// Simulate a large document with many manipulation patterns
	parts := []string{
		"Act now before it's too late!",
		"Everyone knows this is obviously true.",
		"Scientists claim and research proves this works.",
		"This exclusive rare opportunity has only 5 left.",
		"Don't miss out! Be the first to join thousands of others.",
	}
	text := strings.Repeat(strings.Join(parts, " "), 20) // ~3KB
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		DetectManipulation(text)
	}
}

func BenchmarkAnalyzeClaimAllFrameworks(b *testing.B) {
	text := "Study by MIT shows community benefit from sustainable energy integration"
	frameworks := []string{"empirical", "responsible", "harmonic", "pluralistic"}
	for _, fw := range frameworks {
		b.Run(fw, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				AnalyzeClaim(text, fw)
			}
		})
	}
}
