package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

func box(title, body string, width, height int) string {
	width = max(12, width)
	height = max(3, height)
	innerWidth := max(0, width-4)
	innerHeight := max(1, height-3)

	lines := []string{boxTitleStyle.Render(truncate(title, innerWidth))}
	lines = append(lines, boundedLines(body, innerWidth, innerHeight)...)
	content := strings.Join(lines, "\n")

	return boxStyle.
		Width(width - 2).
		Height(height - 2).
		Render(content)
}

func boundedLines(body string, width, height int) []string {
	rendered := make([]string, 0, height)
	for _, line := range strings.Split(body, "\n") {
		if len(rendered) >= height {
			break
		}
		rendered = append(rendered, truncate(line, width))
	}
	for len(rendered) < height {
		rendered = append(rendered, "")
	}
	return rendered
}

func truncate(value string, width int) string {
	if width <= 0 {
		return ""
	}
	if lipgloss.Width(value) <= width {
		return value
	}
	runes := []rune(value)
	for len(runes) > 0 && lipgloss.Width(string(runes)+"…") > width {
		runes = runes[:len(runes)-1]
	}
	return string(runes) + "…"
}
