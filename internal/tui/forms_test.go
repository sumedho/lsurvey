package tui

import (
	"lsurvey/internal/project"
	"strings"
	"testing"
)

func TestConversionFormShowsLastFieldAndError(t *testing.T) {
	m := NewModel(project.New("test"), "")
	m.enterConvert()
	m.startConversionAdd()
	m.convert.field = len(m.convert.fields) - 1
	m.lastErr = "invalid coordinate"
	for _, height := range []int{12, 20, 30} {
		view := m.renderConvert(80, height)
		if !strings.Contains(view, "Description") || !strings.Contains(view, m.lastErr) {
			t.Fatalf("height %d: %s", height, view)
		}
	}
}

func TestStyleFormKeepsActiveFieldAndErrorVisible(t *testing.T) {
	m := NewModel(project.New("test"), "")
	m.enterStyle()
	m.startGroupForm(false)
	m.style.field = len(m.style.fields) - 1
	m.lastErr = "invalid color"
	view := m.renderStyle(60, 12)
	if !strings.Contains(view, m.style.fields[m.style.field].label) || !strings.Contains(view, m.lastErr) {
		t.Fatal(view)
	}
}
