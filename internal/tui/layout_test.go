package tui

import (
	"fmt"
	"github.com/charmbracelet/lipgloss"
	"lsurvey/internal/geom"
	"lsurvey/internal/project"
	"strings"
	"testing"
)

func TestViewsStayInsideTerminal(t *testing.T) {
	for _, mode := range []Mode{ModeMain, ModeMap, ModeHelp, ModeStyle, ModeConvert, ModeResults} {
		for _, size := range [][2]int{{40, 12}, {80, 24}, {120, 40}} {
			m := NewModel(project.New("test"), "")
			m.mode = mode
			m.width = size[0]
			m.height = size[1]
			view := m.View()
			if lipgloss.Width(view) > m.width || len(strings.Split(view, "\n")) > m.height {
				t.Fatalf("mode %d size %v overflow: %dx%d", mode, size, lipgloss.Width(view), len(strings.Split(view, "\n")))
			}
		}
	}
}

func TestPinnedHeaderAndFullRowDetails(t *testing.T) {
	p := project.New("test")
	for i := 0; i < 20; i++ {
		id := fmt.Sprint(i)
		p.Points[id] = geom.Point{ID: id, Easting: 123456.789012, Description: strings.Repeat("long description ", 20)}
	}
	m := NewModel(p, "")
	m.width = 80
	m.height = 24
	m.syncMainViewports()
	m.pointsView.ScrollDown(5)
	if !strings.Contains(m.View(), "Easting") {
		t.Fatal("header scrolled away")
	}
	m.focus = focusPoints
	m.openRowDetails()
	if !strings.Contains(m.results[m.resultIndex].Text, "123456.789012") || !strings.Contains(m.results[m.resultIndex].Text, p.Points["5"].Description) {
		t.Fatal("details truncated")
	}
}
