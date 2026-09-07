package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"lsurvey/internal/project"
)

func TestResultsRetainAndExportWithoutOverwrite(t *testing.T) {
	m := NewModel(project.New("test"), "")
	m.ExecuteCommand("check")
	m.ExecuteCommand("precision 4")
	if len(m.results) != 2 {
		t.Fatalf("results: %d", len(m.results))
	}
	m.openResults()
	path := filepath.Join(t.TempDir(), "report.txt")
	if err := m.exportResult(path); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	if !strings.Contains(string(data), m.results[m.resultIndex].Text) {
		t.Fatal("missing result")
	}
	if err := m.exportResult(path); err == nil {
		t.Fatal("overwrote report")
	}
}
