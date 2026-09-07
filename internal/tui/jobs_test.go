package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"lsurvey/internal/project"
	"path/filepath"
	"testing"
)

func TestJobPublishesAtomicallyAndCancellationDiscards(t *testing.T) {
	for _, cancel := range []bool{false, true} {
		m := NewModel(project.New("test"), "")
		m.asyncJobs = true
		m.dispatchCommand("pt add 1 100 200")
		if m.job == nil || m.queuedJob == nil {
			t.Fatal("no job")
		}
		if len(m.project.Points) != 0 {
			t.Fatal("published early")
		}
		if cancel {
			updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
			m = updated.(Model)
		}
		result := m.queuedJob()
		updated, _ := m.Update(result)
		m = updated.(Model)
		if m.job != nil {
			t.Fatal("still busy")
		}
		if (!cancel && len(m.project.Points) != 1) || (cancel && len(m.project.Points) != 0) {
			t.Fatal("wrong publication")
		}
	}
}

func TestBackgroundSaveFailureKeepsPendingAction(t *testing.T) {
	m := NewModel(project.New("test"), filepath.Join(t.TempDir(), "missing", "job.srv"))
	m.asyncJobs = true
	m.ExecuteCommand("pt add 1 100 200")
	m.dispatchCommand("quit")
	updated, _ := m.updateProtection(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("s")})
	m = updated.(Model)
	if m.job == nil || m.job.cancellable {
		t.Fatal("save must run non-cancellable")
	}
	updated, _ = m.Update(m.queuedJob())
	m = updated.(Model)
	if m.quitting || m.pendingAction != "quit" || !m.dirty || m.lastErr == "" {
		t.Fatal("failed save lost protection")
	}
}

func TestBackgroundJobDoesNotRaceRendering(t *testing.T) {
	m := NewModel(project.New("test"), "")
	m.asyncJobs = true
	m.width = 80
	m.height = 24
	m.dispatchCommand("pt add 1 100 200")
	done := make(chan tea.Msg, 1)
	worker := m.queuedJob
	go func() { done <- worker() }()
	for i := 0; i < 20; i++ {
		m.View()
	}
	updated, _ := m.Update(<-done)
	m = updated.(Model)
	m.ExecuteCommand("undo")
	if len(m.project.Points) != 0 {
		t.Fatal("job lost undo history")
	}
}
