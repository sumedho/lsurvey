package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"lsurvey/internal/project"
)

func TestRunBatchExportRequiresOutputPath(t *testing.T) {
	err := run([]string{"export", "dxf", "job.srv"})
	if err == nil {
		t.Fatal("expected usage error")
	}
	if !strings.Contains(err.Error(), "usage: export") {
		t.Fatalf("error=%q, want usage error", err)
	}
}

func TestRunBatchExportCSV(t *testing.T) {
	dir := t.TempDir()
	projectPath := filepath.Join(dir, "job.srv")
	outputPath := filepath.Join(dir, "points")
	if err := project.Save(projectPath, project.New("job")); err != nil {
		t.Fatal(err)
	}

	if err := run([]string{"export", "csv", projectPath, outputPath}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(outputPath + ".csv"); err != nil {
		t.Fatal(err)
	}
}

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
