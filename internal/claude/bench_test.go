package claude

import "testing"

func BenchmarkParseObject(b *testing.B) {
	data := []byte(existing)
	b.ReportAllocs()
	for b.Loop() {
		if _, err := parseObject(data); err != nil {
			b.Fatal(err)
		}
	}
}
