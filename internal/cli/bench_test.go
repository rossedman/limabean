package cli

import (
	"strings"
	"testing"
)

func BenchmarkParseListening(b *testing.B) {
	out := strings.Repeat(ssOutput, 10)
	b.ReportAllocs()
	for b.Loop() {
		if len(parseListening(out)) != 4 {
			b.Fatal("parseListening")
		}
	}
}

func BenchmarkValidVMName(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		if validVMName("my-dev-vm-2") != nil {
			b.Fatal("validVMName")
		}
	}
}

func BenchmarkValidSnapshotTag(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		if validSnapshotTag("before-upgrade_v1.2") != nil {
			b.Fatal("validSnapshotTag")
		}
	}
}

func BenchmarkCheckNames(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		if checkNames("dev", "node", "") != nil {
			b.Fatal("checkNames")
		}
	}
}

func BenchmarkExpandAllow(b *testing.B) {
	items := []string{"GitHub", "*.Example.com", "registry.npmjs.org", "go", "npm", "python", "claude"}
	b.ReportAllocs()
	for b.Loop() {
		if _, err := expandAllow(items); err != nil {
			b.Fatal(err)
		}
	}
}
