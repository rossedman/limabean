package lima

import (
	"strings"
	"testing"
)

func BenchmarkParseList(b *testing.B) {
	out := []byte(strings.Repeat(listJSON, 10))
	b.ReportAllocs()
	for b.Loop() {
		if list, err := parseList(out); err != nil || len(list) != 20 {
			b.Fatal(len(list), err)
		}
	}
}
