package tui

import (
	"fmt"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"strings"
)

func (m Model) mainLayout() (width, info, points, lines, command int) {
	width = max(12, m.width)
	height := max(24, m.height) - tabBarHeight
	info = 5
	command = 5
	lines = max(5, height/4)
	points = height - info - lines - command
	return
}

func (m Model) View() string {
	if m.width <= 0 {
		m.width = 80
	}
	if m.height <= 0 {
		m.height = 24
	}
	view := m.renderView()
	if m.width <= 0 || m.height <= 0 {
		return view
	}
	lines := strings.Split(view, "\n")
	if len(lines) > m.height {
		lines = lines[:m.height]
	}
	for i := range lines {
		lines[i] = ansi.Truncate(lines[i], m.width, "…")
	}
	return strings.Join(lines, "\n")
}

func (m Model) projectStatus(visible int) string {
	state := "saved"
	if m.dirty {
		state = "UNSAVED"
	}
	if m.path == "" {
		state = "UNSAVED (no file)"
	}
	first := fmt.Sprintf("%s · %s · points %d/%d · %s", state, m.project.Name, visible, len(m.project.Points), m.path)
	second := fmt.Sprintf("CRS: %s · sort %s %s · filter %s", m.project.CoordinateLabel(), m.sort, m.sortDirection(), quoteBlank(m.filter))
	if m.project.Traverse != nil {
		second = fmt.Sprintf("CRS: %s · trav current=%s next=%s", m.project.CoordinateLabel(), m.project.Traverse.Current, m.project.NextPointID())
	}
	return first + "\n" + second
}

func tableCells(values []string, widths []int) string {
	cells := make([]string, 0, len(values))
	for i, v := range values {
		if i >= len(widths) {
			break
		}
		cell := truncate(v, widths[i])
		cells = append(cells, cell+strings.Repeat(" ", max(0, widths[i]-lipgloss.Width(cell))))
	}
	return strings.Join(cells, " ")
}

func (m *Model) buildTables(width int) {
	points := FilterAndSortPoints(m.project, m.filter, m.sort, m.sortAsc)
	m.visiblePoints = len(points)
	widths := []int{8, 14, 14, 10, 8, max(1, width-59)}
	if width < sixtyColumns {
		widths = []int{8, max(8, (width-12)/2), max(8, (width-12)/2)}
	}
	m.pointHeader = tableCells([]string{"ID", "Easting", "Northing", "Elevation", "Code", "Description"}, widths)
	rows := make([]string, 0, len(points))
	m.pointDetails = nil
	for _, p := range points {
		elev := "2D"
		if p.Elevation != nil {
			elev = formatDecimal(*p.Elevation, m.project.DisplayPrecision())
		}
		rows = append(rows, tableCells([]string{p.ID, formatDecimal(p.Easting, m.project.DisplayPrecision()), formatDecimal(p.Northing, m.project.DisplayPrecision()), elev, p.Code, p.Description}, widths))
		z := "2D"
		if p.Elevation != nil {
			z = fmt.Sprintf("%g", *p.Elevation)
		}
		m.pointDetails = append(m.pointDetails, fmt.Sprintf("ID: %s\nEasting: %g\nNorthing: %g\nElevation: %s\nCode: %s\nGroup: %s\nLayer: %s\nDescription: %s", p.ID, p.Easting, p.Northing, z, p.Code, p.GroupID, groupLayer(m.project.Groups, p.GroupID), p.Description))
	}
	m.pointsView.SetContent(strings.Join(rows, "\n"))
	widths = []int{8, 9, max(8, width-43), 8, max(1, width-8-9-max(8, width-43)-8-4)}
	m.lineHeader = tableCells([]string{"ID", "Type", "Points", "Code", "Description"}, widths)
	rows = nil
	m.lineDetails = nil
	for _, f := range m.project.SortedFeatures() {
		rows = append(rows, tableCells([]string{f.ID, string(f.Kind), strings.Join(f.PointIDs, ","), f.Code, f.Description}, widths))
		m.lineDetails = append(m.lineDetails, fmt.Sprintf("ID: %s\nType: %s\nPoints: %s\nCode: %s\nGroup: %s\nLayer: %s\nDescription: %s", f.ID, f.Kind, strings.Join(f.PointIDs, ", "), f.Code, f.GroupID, groupLayer(m.project.Groups, f.GroupID), f.Description))
	}
	for _, set := range m.project.SortedContourSets() {
		row := fmt.Sprintf("%s contour interval=%g polylines=%d stale=%t", set.ID, set.Interval, len(set.Polylines), set.Stale)
		rows = append(rows, row)
		m.lineDetails = append(m.lineDetails, row)
	}
	m.linesView.SetContent(strings.Join(rows, "\n"))
}

const sixtyColumns = 60

func (m *Model) openRowDetails() {
	details, index := m.pointDetails, m.pointsView.YOffset
	if m.focus == focusLines {
		details, index = m.lineDetails, m.linesView.YOffset
	}
	if len(details) == 0 {
		return
	}
	m.appendResult(commandResult{"Row details", details[min(index, len(details)-1)]})
	m.openResults()
}

func (m Model) compactMain() string {
	rows := m.pointsView.View()
	header := m.pointHeader
	if m.focus == focusLines {
		rows = m.linesView.View()
		header = m.lineHeader
	}
	body := strings.Split(m.projectStatus(m.visiblePoints)+"\n"+header+"\n"+rows, "\n")
	available := max(0, m.height-5)
	body = boundedLines(strings.Join(body, "\n"), max(1, m.width), available)
	message := m.message
	if m.lastErr != "" {
		message = m.lastErr
	}
	return m.renderWithTabs(strings.Join(body, "\n")+"\n"+box("Command · F6 results", m.input.View()+"\n"+message, m.width, 4), m.width)
}
