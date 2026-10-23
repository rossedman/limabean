package secrets

import (
	"strings"
	"testing"
)

func BenchmarkParse(b *testing.B) {
	in := strings.Repeat(secretsSample, 20)
	b.ReportAllocs()
	for b.Loop() {
		if got, _ := Parse(strings.NewReader(in), "secrets.env"); len(got) == 0 {
			b.Fatal("no secrets")
		}
	}
}

func BenchmarkUnquote(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		if unquote(`"op://Private/OpenAI/key"`) != "op://Private/OpenAI/key" {
			b.Fatal("unquote")
		}
	}
}
