package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const tabBarHeight = 1

type screenTab struct {
	Label string
	Mode  Mode
}

var screenTabs = []screenTab{
	{Label: "Project", Mode: ModeMain},
	{Label: "Help F1", Mode: ModeHelp},
	{Label: "Map F2", Mode: ModeMap},
	{Label: "Styles F3", Mode: ModeStyle},
	{Label: "Convert F4", Mode: ModeConvert},
}

func (m Model) renderWithTabs(body string, width int) string {
	return lipgloss.JoinVertical(lipgloss.Left, m.renderTabs(width), body)
}

func (m Model) renderTabs(width int) string {
	width = max(12, width)
	var rendered []string
	for _, tab := range screenTabs {
		label := " " + tab.Label + " "
		style := inactiveTabStyle
		if m.mode == tab.Mode {
			style = activeTabStyle
		}
		rendered = append(rendered, style.Render(label))
	}
	line := lipgloss.JoinHorizontal(lipgloss.Top, rendered...)
	if lipgloss.Width(line) < width {
		line += strings.Repeat(" ", width-lipgloss.Width(line))
	}
	return truncate(line, width)
}

func (m *Model) handleTabMouse(msg tea.MouseMsg) bool {
	if msg.Y != 0 || msg.Button != tea.MouseButtonLeft || msg.Action != tea.MouseActionPress {
		return false
	}
	if tab, ok := tabAtX(msg.X); ok {
		m.selectTab(tab.Mode)
		return true
	}
	return false
}

func tabAtX(x int) (screenTab, bool) {
	if x < 0 {
		return screenTab{}, false
	}
	left := 0
	for _, tab := range screenTabs {
		width := lipgloss.Width(" " + tab.Label + " ")
		if x >= left && x < left+width {
			return tab, true
		}
		left += width
	}
	return screenTab{}, false
}

func (m *Model) selectTab(mode Mode) {
	switch mode {
	case ModeMain:
		m.mode = ModeMain
	case ModeHelp:
		m.openHelp("")
	case ModeMap:
		m.enterMap()
		m.message = "map"
	case ModeStyle:
		m.enterStyle()
	case ModeConvert:
		m.enterConvert()
	}
}

var (
	activeTabStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("16")).
			Background(lipgloss.Color("86"))
	inactiveTabStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("252")).
				Background(lipgloss.Color("238"))
)
