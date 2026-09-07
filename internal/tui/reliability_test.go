package tui

import (
	"lsurvey/internal/project"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestQualityCommandsUseSessionAndPreserveReadOnlyState(t *testing.T) {
	m := NewModel(project.New("qa"), "")
	path := filepath.Join(t.TempDir(), "job")
	for _, cmd := range []string{"pt add S 0 0", "pt add T 20 1", "trav start S", "trav leg 0 9", "trav leg 90 9", "trav close T", "save " + path, "check", "trav report transit"} {
		m.ExecuteCommand(cmd)
		if m.lastErr != "" {
			t.Fatalf("%s: %s", cmd, m.lastErr)
		}
	}
	if m.dirty || !strings.Contains(m.message, "transit") {
		t.Fatal("QA command handling failed")
	}
	m.ExecuteCommand("trav adjust compass")
	if m.lastErr != "" || !m.dirty || !m.project.Traverse.Adjusted {
		t.Fatalf("adjust: %s", m.lastErr)
	}
	m.ExecuteCommand("undo")
	if m.lastErr != "" || m.project.Traverse.Adjusted {
		t.Fatal("undo failed")
	}
}

func TestRecoveryResetsUIAndLoadsPreviousSave(t *testing.T) {
	m := NewModel(project.New("qa"), "")
	path := filepath.Join(t.TempDir(), "job")
	for _, cmd := range []string{"pt add A 1 2", "save " + path, "pt add B 3 4", "save", "recover " + path} {
		m.ExecuteCommand(cmd)
		if m.lastErr != "" {
			t.Fatalf("%s: %s", cmd, m.lastErr)
		}
	}
	if len(m.project.Points) != 1 || !m.dirty || m.path != "" {
		t.Fatal("recovery did not load detached dirty project")
	}
	m.ExecuteCommand("save")
	if m.lastErr == "" {
		t.Fatal("recovery silently reused original path")
	}
}

func TestLegacyProjectSaveAsCreatesSQLite(t *testing.T) {
	dir := t.TempDir()
	legacy := filepath.Join(dir, "legacy.srv")
	target := filepath.Join(dir, "native")
	data := []byte(`{"schema_version":8,"name":"old","points":{},"history":[]}`)
	if err := os.WriteFile(legacy, data, 0600); err != nil {
		t.Fatal(err)
	}
	m := NewModel(project.New("test"), "")
	m.ExecuteCommand("open " + legacy)
	if m.lastErr != "" {
		t.Fatal(m.lastErr)
	}
	m.ExecuteCommand("pt add A 1 2")
	m.ExecuteCommand("save")
	if m.lastErr == "" || !m.dirty {
		t.Fatal("legacy save should fail and retain dirty state")
	}
	m.ExecuteCommand("saveas " + target)
	if m.lastErr != "" || m.dirty {
		t.Fatalf("saveas failed: %s", m.lastErr)
	}
	got, err := os.ReadFile(target + ".srv")
	if err != nil || string(got[:16]) != "SQLite format 3\x00" {
		t.Fatal("not SQLite")
	}
	got, err = os.ReadFile(legacy)
	if err != nil || string(got) != string(data) {
		t.Fatal("legacy overwritten")
	}
	m.ExecuteCommand("open " + target)
	if m.lastErr != "" || len(m.project.Points) != 1 {
		t.Fatal("native reopen failed")
	}
}
