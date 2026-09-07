package tui

import (
	"fmt"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/x/ansi"
	"strings"
)

// Forms use the whole screen, reserving space for feedback independently of
// the field window. The active field never scrolls behind a clipped footer.
func renderForm(title string, labels []string, inputs []textinput.Model, active int, feedback string, width, height int) string {
	available := max(1, height-7)
	start := max(0, active-available+1)
	end := min(len(labels), start+available)
	lines := []string{fmt.Sprintf("Field %d/%d · Tab/Shift+Tab: move · Enter: submit · Esc: cancel", active+1, len(labels))}
	for i := start; i < end; i++ {
		prefix := "  "
		if i == active {
			prefix = "> "
		}
		input := inputs[i]
		input.Width = max(1, width-len(labels[i])-10)
		lines = append(lines, prefix+labels[i]+": "+input.View())
	}
	for len(lines) < available+1 {
		lines = append(lines, "")
	}
	lines = append(lines, ansi.Hardwrap(feedback, max(1, width-4), true))
	return box(title, strings.Join(lines, "\n"), width, height)
}

func (m Model) renderConversionForm(width, height int) string {
	labels := make([]string, len(m.convert.fields))
	inputs := make([]textinput.Model, len(labels))
	for i, f := range m.convert.fields {
		labels[i] = f.label
		inputs[i] = f.input
	}
	feedback := m.message
	if m.lastErr != "" {
		feedback = m.lastErr
	}
	return renderForm("Coordinate conversion", labels, inputs, m.convert.field, feedback, width, height)
}

func (m Model) renderStyleForm(width, height int) string {
	labels := make([]string, len(m.style.fields))
	inputs := make([]textinput.Model, len(labels))
	for i, f := range m.style.fields {
		labels[i] = f.label
		inputs[i] = f.input
	}
	feedback := "ctrl+g: mapping group · ctrl+p: ACI palette\n" + m.message
	if m.lastErr != "" {
		feedback = m.lastErr
	}
	return renderForm(m.styleFormTitle(), labels, inputs, m.style.field, feedback, width, height)
}
