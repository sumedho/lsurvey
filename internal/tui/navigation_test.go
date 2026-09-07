package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"lsurvey/internal/project"
	"testing"
)

func TestHistoryPreservesDraftAndDoesNotWrap(t *testing.T) {
	m := NewModel(project.New("test"), "")
	m.recordHistory("one")
	m.recordHistory("two")
	m.input.SetValue("unfinished command")
	m.previousHistory()
	m.previousHistory()
	m.previousHistory()
	if m.input.Value() != "one" {
		t.Fatal("history wrapped")
	}
	m.nextHistory()
	m.nextHistory()
	if m.input.Value() != "unfinished command" {
		t.Fatal("draft lost")
	}
}

func TestScreenNavigationPreservesFormDraft(t *testing.T) {
	m := NewModel(project.New("test"), "")
	m.enterStyle()
	m.startGroupForm(false)
	m.style.fields[0].input.SetValue("DRAFT")
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyF4})
	m = updated.(Model)
	m.selectTab(ModeStyle)
	if m.style.form == styleFormNone || m.style.fields[0].input.Value() != "DRAFT" {
		t.Fatal("draft lost")
	}
}

func TestNavigationPreservesCommandAndReturnsFromHelp(t *testing.T) {
	m := NewModel(project.New("test"), "")
	m.input.SetValue("pt add draft")
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyF2})
	m = updated.(Model)
	if m.mode != ModeMap || m.input.Value() != "pt add draft" {
		t.Fatal("keyboard navigation lost draft")
	}
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyF1})
	m = updated.(Model)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = updated.(Model)
	if m.mode != ModeMap {
		t.Fatal("help lost origin")
	}
}
