package tui

import (
	"github.com/charmbracelet/bubbles/viewport"
	"lsurvey/internal/project"
)

type tableKey struct {
	project                                      *project.Project
	revision                                     uint64
	width, precision, points, features, contours int
	filter                                       string
	sort                                         SortField
	ascending                                    bool
}
type tableCache struct {
	key                       tableKey
	builds                    int
	points, lines             viewport.Model
	pointHeader, lineHeader   string
	pointDetails, lineDetails []string
	visible                   int
}

func (m *Model) cachedTables(width int) {
	if m.tables == nil {
		m.tables = &tableCache{}
	}
	key := tableKey{m.project, m.tableRevision, width, m.project.DisplayPrecision(), len(m.project.Points), len(m.project.Features), len(m.project.ContourSets), m.filter, m.sort, m.sortAsc}
	pOffset, lOffset := m.pointsView.YOffset, m.linesView.YOffset
	pHeight, lHeight := m.pointsView.Height, m.linesView.Height
	if key != m.tables.key || m.tables.builds == 0 {
		m.buildTables(width)
		*m.tables = tableCache{key: key, builds: m.tables.builds + 1, points: m.pointsView, lines: m.linesView, pointHeader: m.pointHeader, lineHeader: m.lineHeader, pointDetails: m.pointDetails, lineDetails: m.lineDetails, visible: m.visiblePoints}
	}
	c := m.tables
	m.pointsView, m.linesView = c.points, c.lines
	m.pointsView.Height = pHeight
	m.linesView.Height = lHeight
	m.pointsView.SetYOffset(pOffset)
	m.linesView.SetYOffset(lOffset)
	m.pointHeader, m.lineHeader = c.pointHeader, c.lineHeader
	m.pointDetails, m.lineDetails = c.pointDetails, c.lineDetails
	m.visiblePoints = c.visible
}
