package main

import "testing"

func TestResolveVersionUsesInjectedValue(t *testing.T) {
	original := version
	version = "1.2.3"
	t.Cleanup(func() {
		version = original
	})

	if got := resolveVersion(); got != "1.2.3" {
		t.Fatalf("resolveVersion()=%q want %q", got, "1.2.3")
	}
}

func TestResolveVersionFallsBackToDev(t *testing.T) {
	original := version
	version = ""
	t.Cleanup(func() {
		version = original
	})

	if got := resolveVersion(); got != "dev" {
		t.Fatalf("resolveVersion()=%q want %q", got, "dev")
	}
}
