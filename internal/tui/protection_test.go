package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"lsurvey/internal/project"
	"testing"
)

func TestLifecycleGuardCancelAndDiscard(t *testing.T) {
	for _, command := range []string{"new next", "open missing", "recover missing", "quit"} {
		m := NewModel(project.New("original"), "")
		m.ExecuteCommand("pt add 1 100 200")
		m.dispatchCommand(command)
		if m.pendingAction == "" {
			t.Fatalf("unguarded %s", command)
		}
		updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
		m = updated.(Model)
		if m.pendingAction != "" || !m.dirty || m.quitting {
			t.Fatalf("cancel changed project: %s", command)
		}
	}
	m := NewModel(project.New("original"), "")
	m.dirty = true
	m.dispatchCommand("quit")
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("d")})
	if !updated.(Model).quitting || cmd == nil {
		t.Fatal("discard did not quit")
	}
}

func TestQuitGuardAppliesToEveryScreen(t *testing.T) {
	for _, mode := range []Mode{ModeMain, ModeHelp, ModeMap, ModeStyle, ModeConvert, ModeResults} {
		m := NewModel(project.New("test"), "")
		m.dirty = true
		m.mode = mode
		updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
		m = updated.(Model)
		if m.pendingAction != "quit" || m.quitting {
			t.Fatalf("mode %d bypassed guard", mode)
		}
	}
}
