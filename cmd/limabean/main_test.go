package main

import "testing"

func TestResolveVersion(t *testing.T) {
	version = "1.2.3"
	if resolveVersion() != "1.2.3" {
		t.Fatal("ldflags version should win")
	}
	version = ""
	if v := resolveVersion(); v == "" {
		t.Fatal("resolveVersion should never be empty")
	}
}
