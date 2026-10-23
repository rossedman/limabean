package project

import "testing"

func BenchmarkParse(b *testing.B) {
	body := []byte(projectSamples[1] + projectSamples[2])
	b.ReportAllocs()
	for b.Loop() {
		if _, err := Parse(body, "/repo/app/.limabean.yaml"); err != nil {
			b.Fatal(err)
		}
	}
}
