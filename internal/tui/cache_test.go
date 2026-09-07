package tui

import (
	"lsurvey/internal/project"
	"testing"
)

func TestTableCacheReusesRenderAndInvalidatesEdits(t *testing.T) {
	m := NewModel(project.New("test"), "")
	m.width = 80
	m.height = 24
	m.syncMainViewports()
	builds := m.tables.builds
	for i := 0; i < 10; i++ {
		m.View()
	}
	if m.tables.builds != builds {
		t.Fatal("View rebuilt tables")
	}
	m.ExecuteCommand("pt add 1 100 200")
	if m.tables.builds <= builds || m.visiblePoints != 1 {
		t.Fatal("edit did not refresh")
	}
	builds = m.tables.builds
	m.filter = "missing"
	m.syncMainViewports()
	if m.tables.builds <= builds || m.visiblePoints != 0 {
		t.Fatal("filter did not refresh")
	}
}
