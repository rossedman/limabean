package pins

import "testing"

func BenchmarkParse(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		if len(Parse(sample)) == 0 {
			b.Fatal("no pins")
		}
	}
}

func BenchmarkSeries(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		if Series("temurin-25.0.4+101.0.LTS") != "temurin-25" {
			b.Fatal("Series")
		}
	}
}

func BenchmarkPrefix(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		if Prefix("temurin-25.0.4+101.0.LTS") != "temurin" {
			b.Fatal("Prefix")
		}
	}
}

func BenchmarkCompareVersions(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		if compareVersions("1.10.0-rc2", "1.9.9") <= 0 {
			b.Fatal("compareVersions")
		}
	}
}
