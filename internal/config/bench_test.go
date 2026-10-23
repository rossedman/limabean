package config

import (
	"os"
	"path/filepath"
	"testing"
)

func BenchmarkLoad(b *testing.B) {
	path := filepath.Join(b.TempDir(), "config")
	if err := os.WriteFile(path, []byte(bashConfig+"CPUS=\"4\"\nDISK=\"30GiB\"\n"), 0o600); err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	for b.Loop() {
		c, err := Load(New(), path)
		if err != nil {
			b.Fatal(err)
		}
		if c.Path != path {
			b.Fatalf("Path = %q", c.Path)
		}
	}
}
