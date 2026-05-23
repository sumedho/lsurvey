package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"lsurvey/internal/project"
)

func TestCommandCompletionAcceptsTemplate(t *testing.T) {
	model := NewModel(project.New("test"), "")
	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	updated, _ = updated.(Model).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	updated, _ = updated.(Model).Update(tea.KeyMsg{Type: tea.KeyTab})
	got := updated.(Model)
	if !strings.HasPrefix(got.input.Value(), "radiate <from>") {
		t.Fatalf("input=%q want radiate template", got.input.Value())
	}
}

func TestCommandCompletionIncludesProjectPointHints(t *testing.T) {
	m := NewModel(project.New("test"), "")
	m.ExecuteCommand("pt add 1 100 200 PEG")
	m.input.SetValue("inverse 1")
	m.refreshCompletions()
	if len(m.input.MatchedSuggestions()) == 0 {
		t.Fatal("expected matched point suggestions")
	}
}
