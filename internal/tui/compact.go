package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

func (m Model) renderCompactStyle(width, height int) string {
	rows := max(1, height-10)
	title, body := "Style Groups", m.renderStyleGroups(rows)
	switch m.style.Pane {
	case stylePaneCodes:
		title = "Point Code Defaults"
		body = m.renderStyleCodes(rows)
	case stylePanePalette:
		title = "ACI Palette"
		body = m.renderStylePalette(max(12, width-4))
	case stylePaneActions:
		title = "Actions"
		body = m.renderStyleActions()
	}
	footer := "Tab: pane  g: group  c: code  e: edit  d: delete\ni: import  x: export  arrows/page/wheel: scroll"
	if m.style.form == styleFormDelete {
		footer = m.renderStyleActions()
	}
	if m.lastErr != "" {
		footer += "\n" + m.lastErr
	} else {
		footer += "\n" + m.message
	}
	if height < 12 {
		return box("Code Styling · "+title, footer+"\n"+body, width, height)
	}
	return lipgloss.JoinVertical(lipgloss.Left, box("Code Styling · "+title, body, width, height-6), box("Actions", footer, width, 6))
}

func (m Model) compactConversionRows(start, end, width int) []string {
	widths := []int{8, max(9, (width-24)/2), max(9, (width-24)/2), max(5, width-11-2*max(9, (width-24)/2))}
	rows := []string{tableCells([]string{"ID", "SOURCE", "RESULT", "STATUS"}, widths)}
	for i := start; i < end; i++ {
		row := m.convert.Rows[i]
		status := "staged"
		if row.Output.Grid != nil || row.Output.Geographic != nil {
			status = "computed"
		}
		if row.Error != "" {
			status = "error"
		}
		id := " " + row.ID
		if i == m.convert.Selected {
			id = ">" + row.ID
		}
		rows = append(rows, tableCells([]string{id, row.sourceText(m.convert.Source), row.resultText(), status}, widths))
	}
	return rows
}

func (m Model) renderCompactConversion(width, height int) string {
	body := fmt.Sprintf("%s → %s\na:add i:CSV g:grid s/t:source/target\nr:calculate c/C:commit selected/all\narrows:select Enter:details Esc:back", m.convert.Source, m.convert.Target)
	if m.convert.confirm {
		body = "Confirm existing points use target CRS?\ny: confirm  n/Esc: cancel"
	}
	if m.lastErr != "" {
		body += "\n" + m.lastErr
	}
	if len(m.convert.Rows) > 0 {
		body += "\n" + strings.Join(m.compactConversionRows(m.convert.Selected, m.convert.Selected+1, width-4), "\n")
	}
	return box("Coordinate Conversion", body, width, height)
}
