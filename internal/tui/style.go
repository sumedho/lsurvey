package tui

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"lsurvey/internal/project"
)

type stylePane int

const (
	stylePaneGroups stylePane = iota
	stylePaneCodes
	stylePanePalette
	stylePaneActions
)

type styleForm int

const (
	styleFormNone styleForm = iota
	styleFormGroupAdd
	styleFormGroupEdit
	styleFormCodeAdd
	styleFormCodeEdit
	styleFormImport
	styleFormExport
	styleFormDelete
)

type styleField struct {
	label string
	input textinput.Model
}

type StyleState struct {
	Pane          stylePane
	GroupIndex    int
	CodeIndex     int
	ACI           int
	form          styleForm
	fields        []styleField
	field         int
	deleteCommand string
	deleteLabel   string
	pendingCode   string
}

func newStyleState() StyleState {
	return StyleState{Pane: stylePaneGroups, ACI: 1}
}

func (m *Model) enterStyle() {
	m.mode = ModeStyle
	m.style.ensureSelection(m.project)
	m.message = "code styling"
	m.lastErr = ""
}

func (s *StyleState) ensureSelection(p *project.Project) {
	groups := p.SortedGroups()
	if len(groups) == 0 {
		s.GroupIndex = 0
	} else if s.GroupIndex >= len(groups) {
		s.GroupIndex = len(groups) - 1
	}
	codes := sortedStyleCodes(p)
	if len(codes) == 0 {
		s.CodeIndex = 0
	} else if s.CodeIndex >= len(codes) {
		s.CodeIndex = len(codes) - 1
	}
	if s.ACI < 1 || s.ACI > 255 {
		s.ACI = 1
	}
}

func sortedStyleCodes(p *project.Project) []string {
	codes := make([]string, 0, len(p.PointCodeStyles))
	for code := range p.PointCodeStyles {
		codes = append(codes, code)
	}
	sort.Strings(codes)
	return codes
}

func (m Model) updateStyle(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		m.quitting = true
		return m, tea.Quit
	case "f1":
		m.openHelp("")
		return m, nil
	}
	if m.style.form == styleFormDelete {
		switch msg.String() {
		case "y", "Y", "enter":
			m.confirmStyleDelete()
		case "n", "N", "esc":
			m.cancelStyleForm()
		}
		return m, nil
	}
	if m.style.form != styleFormNone {
		return m.updateStyleForm(msg)
	}
	switch msg.String() {
	case "esc", "f3":
		m.mode = ModeMain
	case "tab":
		m.style.Pane = (m.style.Pane + 1) % 4
	case "shift+tab":
		m.style.Pane = (m.style.Pane + 3) % 4
	case "up":
		m.moveStyleSelection(-1, 0)
	case "down":
		m.moveStyleSelection(1, 0)
	case "left":
		m.moveStyleSelection(0, -1)
	case "right":
		m.moveStyleSelection(0, 1)
	case "pgup":
		m.moveStyleSelection(-m.styleTableRowLimit(), 0)
	case "pgdown":
		m.moveStyleSelection(m.styleTableRowLimit(), 0)
	case "g":
		m.startGroupForm(false)
	case "c":
		m.startCodeForm(false)
	case "e", "enter":
		switch m.style.Pane {
		case stylePaneGroups:
			m.startGroupForm(true)
		case stylePaneCodes:
			m.startCodeForm(true)
		}
	case "d":
		m.beginStyleDelete()
	case "i":
		m.startPathForm(styleFormImport)
	case "x":
		m.startPathForm(styleFormExport)
	}
	return m, nil
}

func (m Model) updateStyleForm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if (m.style.form == styleFormGroupAdd || m.style.form == styleFormGroupEdit) && m.style.Pane == stylePanePalette {
		switch msg.String() {
		case "esc", "ctrl+p":
			m.style.Pane = stylePaneGroups
			return m, nil
		case "enter":
			m.acceptStylePalette()
			m.style.Pane = stylePaneGroups
			return m, nil
		case "up":
			m.moveStyleSelection(-1, 0)
			return m, nil
		case "down":
			m.moveStyleSelection(1, 0)
			return m, nil
		case "left":
			m.moveStyleSelection(0, -1)
			return m, nil
		case "right":
			m.moveStyleSelection(0, 1)
			return m, nil
		case "pgup":
			m.style.ACI = max(1, m.style.ACI-24)
			return m, nil
		case "pgdown":
			m.style.ACI = min(255, m.style.ACI+24)
			return m, nil
		}
	}
	switch msg.String() {
	case "esc":
		m.cancelStyleForm()
		return m, nil
	case "tab":
		m.cycleStyleField(1)
		return m, nil
	case "shift+tab":
		m.cycleStyleField(-1)
		return m, nil
	case "enter":
		m.submitStyleForm()
		return m, nil
	case "ctrl+g":
		if m.style.form == styleFormCodeAdd || m.style.form == styleFormCodeEdit {
			m.style.pendingCode = m.style.fields[0].input.Value()
			m.startGroupForm(false)
			m.message = "create group, then the point-code mapping will resume"
		}
		return m, nil
	case "ctrl+p":
		if m.style.form == styleFormGroupAdd || m.style.form == styleFormGroupEdit {
			m.style.Pane = stylePanePalette
			m.message = "select a nominal ACI color and press Enter"
		}
		return m, nil
	}
	if len(m.style.fields) == 0 {
		return m, nil
	}
	var cmd tea.Cmd
	field := &m.style.fields[m.style.field]
	field.input, cmd = field.input.Update(msg)
	return m, cmd
}

func (m *Model) cycleStyleField(step int) {
	if len(m.style.fields) == 0 {
		return
	}
	m.style.fields[m.style.field].input.Blur()
	m.style.field = (m.style.field + step + len(m.style.fields)) % len(m.style.fields)
	m.style.fields[m.style.field].input.Focus()
}

func (m *Model) moveStyleSelection(vertical, horizontal int) {
	switch m.style.Pane {
	case stylePaneGroups:
		groups := m.project.SortedGroups()
		m.style.GroupIndex = clampStyleIndex(m.style.GroupIndex+vertical, len(groups))
	case stylePaneCodes:
		codes := sortedStyleCodes(m.project)
		m.style.CodeIndex = clampStyleIndex(m.style.CodeIndex+vertical, len(codes))
	case stylePanePalette:
		step := vertical
		if horizontal != 0 {
			step = horizontal
		} else {
			step *= 8
		}
		m.style.ACI = min(255, max(1, m.style.ACI+step))
	}
}

func (m Model) styleTableRowLimit() int {
	height := max(18, m.height)
	topHeight := max(9, (height-3)/2)
	return max(1, topHeight-4)
}

func (m Model) handleStyleMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	if !isWheelMouse(msg) || m.style.form != styleFormNone {
		return m, nil
	}
	step := 0
	switch msg.Button {
	case tea.MouseButtonWheelUp:
		step = -1
	case tea.MouseButtonWheelDown:
		step = 1
	}
	if step != 0 {
		m.moveStyleSelection(step, 0)
	}
	return m, nil
}

func clampStyleIndex(index, length int) int {
	if length == 0 {
		return 0
	}
	return min(length-1, max(0, index))
}

func (m *Model) startGroupForm(edit bool) {
	values := []string{"", "", "1", ""}
	m.style.form = styleFormGroupAdd
	if edit {
		groups := m.project.SortedGroups()
		if len(groups) == 0 {
			m.setError("no style group selected")
			return
		}
		group := groups[clampStyleIndex(m.style.GroupIndex, len(groups))]
		values = []string{group.ID, group.Layer, strconv.Itoa(group.Color), group.Description}
		m.style.form = styleFormGroupEdit
		m.style.ACI = group.Color
	}
	m.setStyleFields([]string{"ID", "Layer", "ACI color", "Description"}, values)
	if edit {
		m.style.fields[0].input.Blur()
		m.style.field = 1
		m.style.fields[1].input.Focus()
	}
	m.lastErr = ""
}

func (m *Model) startCodeForm(edit bool) {
	values := []string{"", ""}
	m.style.form = styleFormCodeAdd
	if edit {
		codes := sortedStyleCodes(m.project)
		if len(codes) == 0 {
			m.setError("no point code default selected")
			return
		}
		code := codes[clampStyleIndex(m.style.CodeIndex, len(codes))]
		values = []string{code, m.project.PointCodeStyles[code]}
		m.style.form = styleFormCodeEdit
	}
	m.setStyleFields([]string{"Code", "Group"}, values)
	m.lastErr = ""
}

func (m *Model) startPathForm(form styleForm) {
	m.style.form = form
	m.setStyleFields([]string{"Library path"}, []string{""})
	m.lastErr = ""
}

func (m *Model) setStyleFields(labels, values []string) {
	m.style.fields = nil
	m.style.field = 0
	for i, label := range labels {
		input := textinput.New()
		input.Prompt = ""
		input.Width = 42
		input.CharLimit = 256
		input.SetValue(values[i])
		if i == 0 {
			input.Focus()
		}
		m.style.fields = append(m.style.fields, styleField{label: label, input: input})
	}
}

func (m *Model) cancelStyleForm() {
	m.style.form = styleFormNone
	m.style.fields = nil
	m.style.field = 0
	m.style.deleteCommand = ""
	m.style.deleteLabel = ""
	m.style.pendingCode = ""
	m.lastErr = ""
	m.message = "code styling"
}

func (m *Model) acceptStylePalette() {
	if m.style.form != styleFormGroupAdd && m.style.form != styleFormGroupEdit {
		return
	}
	if len(m.style.fields) >= 3 {
		m.style.fields[2].input.SetValue(strconv.Itoa(m.style.ACI))
		m.message = fmt.Sprintf("selected ACI %d", m.style.ACI)
	}
}

func (m *Model) submitStyleForm() {
	var command string
	var groupID string
	switch m.style.form {
	case styleFormGroupAdd, styleFormGroupEdit:
		id := strings.TrimSpace(m.style.fields[0].input.Value())
		layer := strings.TrimSpace(m.style.fields[1].input.Value())
		color := strings.TrimSpace(m.style.fields[2].input.Value())
		desc := strings.TrimSpace(m.style.fields[3].input.Value())
		action := "add"
		if m.style.form == styleFormGroupEdit {
			action = "edit"
		}
		command = fmt.Sprintf("group %s %s layer=%s color=%s", action, quoteStyleField(id), quoteStyleField(layer), color)
		if desc != "" || m.style.form == styleFormGroupEdit {
			command += " desc=" + quoteStyleField(desc)
		}
		groupID = id
	case styleFormCodeAdd, styleFormCodeEdit:
		code := strings.TrimSpace(m.style.fields[0].input.Value())
		group := strings.TrimSpace(m.style.fields[1].input.Value())
		command = fmt.Sprintf("code style set %s group=%s", quoteStyleField(code), quoteStyleField(group))
	case styleFormImport:
		command = "import codes " + quoteStyleField(strings.TrimSpace(m.style.fields[0].input.Value()))
	case styleFormExport:
		command = "export codes " + quoteStyleField(strings.TrimSpace(m.style.fields[0].input.Value()))
	default:
		return
	}
	if !m.executeStyleCommand(command) {
		return
	}
	if groupID != "" && m.style.pendingCode != "" {
		code := m.style.pendingCode
		m.style.pendingCode = ""
		m.style.form = styleFormCodeAdd
		m.setStyleFields([]string{"Code", "Group"}, []string{code, groupID})
		m.message = "group created; press Enter to save the pending point-code mapping"
		return
	}
	m.style.form = styleFormNone
	m.style.fields = nil
}

func (m *Model) executeStyleCommand(command string) bool {
	outcome, err := m.session.Execute(command)
	if err != nil {
		m.setError(err.Error())
		return false
	}
	m.syncSessionState()
	if outcome.ProjectReplaced {
		m.mapState = newMapState()
	}
	m.style.ensureSelection(m.project)
	m.refreshCompletions()
	m.syncMainViewports()
	m.lastErr = ""
	m.message = outcome.Message
	return true
}

func quoteStyleField(value string) string {
	return strconv.Quote(value)
}

func (m *Model) beginStyleDelete() {
	switch m.style.Pane {
	case stylePaneGroups:
		groups := m.project.SortedGroups()
		if len(groups) == 0 {
			m.setError("no style group selected")
			return
		}
		id := groups[clampStyleIndex(m.style.GroupIndex, len(groups))].ID
		m.style.deleteCommand = "group del " + quoteStyleField(id)
		m.style.deleteLabel = "style group " + id
	case stylePaneCodes:
		codes := sortedStyleCodes(m.project)
		if len(codes) == 0 {
			m.setError("no point code default selected")
			return
		}
		code := codes[clampStyleIndex(m.style.CodeIndex, len(codes))]
		m.style.deleteCommand = "code style del " + quoteStyleField(code)
		m.style.deleteLabel = "point code default " + code
	default:
		return
	}
	m.style.form = styleFormDelete
	m.lastErr = ""
}

func (m *Model) confirmStyleDelete() {
	if m.executeStyleCommand(m.style.deleteCommand) {
		m.style.form = styleFormNone
		m.style.deleteCommand = ""
		m.style.deleteLabel = ""
	}
}

func (m Model) renderStyle(width, height int) string {
	m.style.ensureSelection(m.project)
	innerWidth := max(56, width-4)
	leftWidth := innerWidth / 2
	rightWidth := innerWidth - leftWidth
	topHeight := max(9, (height-3)/2)
	bottomHeight := max(9, height-3-topHeight)
	tableRows := max(1, topHeight-4)
	groups := m.project.SortedGroups()
	codes := sortedStyleCodes(m.project)
	top := lipgloss.JoinHorizontal(
		lipgloss.Top,
		box(styleTablePaneTitle("Style Groups", m.style.Pane == stylePaneGroups, len(groups), m.style.GroupIndex, tableRows), m.renderStyleGroups(tableRows), leftWidth, topHeight),
		box(styleTablePaneTitle("Point Code Defaults", m.style.Pane == stylePaneCodes, len(codes), m.style.CodeIndex, tableRows), m.renderStyleCodes(tableRows), rightWidth, topHeight),
	)
	bottom := lipgloss.JoinHorizontal(
		lipgloss.Top,
		box(stylePaneTitle("ACI Palette", m.style.Pane == stylePanePalette), m.renderStylePalette(leftWidth), leftWidth, bottomHeight),
		box(stylePaneTitle("Actions", m.style.Pane == stylePaneActions), m.renderStyleActions(), rightWidth, bottomHeight),
	)
	body := lipgloss.JoinVertical(lipgloss.Left, top, bottom)
	return box("Code Styling", body, width, height)
}

func (m Model) renderStyleGroups(limit int) string {
	rows := []string{mutedStyle.Render(fmt.Sprintf("  %-10s %-15s %-7s %s", "GROUP", "LAYER", "ACI", "DESCRIPTION"))}
	groups := m.project.SortedGroups()
	if len(groups) == 0 {
		rows = append(rows, mutedStyle.Render("  No style groups. Press g to add one."))
	}
	for i, group := range visibleStyleSlice(groups, m.style.GroupIndex, limit) {
		absolute := visibleStyleStart(len(groups), m.style.GroupIndex, limit) + i
		prefix := "  "
		if m.style.Pane == stylePaneGroups && absolute == m.style.GroupIndex {
			prefix = "> "
		}
		rows = append(rows, fmt.Sprintf("%s%-10s %-15s %-7s %s",
			prefix, group.ID, group.Layer, styleACISwatch(group.Color), group.Description))
	}
	return strings.Join(rows, "\n")
}

func (m Model) renderStyleCodes(limit int) string {
	rows := []string{mutedStyle.Render(fmt.Sprintf("  %-10s %-10s %-14s %s", "CODE", "GROUP", "LAYER", "ACI"))}
	codes := sortedStyleCodes(m.project)
	if len(codes) == 0 {
		rows = append(rows, mutedStyle.Render("  No defaults. Press c to assign a point code."))
	}
	start := visibleStyleStart(len(codes), m.style.CodeIndex, limit)
	for i, code := range visibleStyleSlice(codes, m.style.CodeIndex, limit) {
		absolute := start + i
		groupID := m.project.PointCodeStyles[code]
		group := m.project.Groups[groupID]
		prefix := "  "
		if m.style.Pane == stylePaneCodes && absolute == m.style.CodeIndex {
			prefix = "> "
		}
		rows = append(rows, fmt.Sprintf("%s%-10s %-10s %-14s %s",
			prefix, code, groupID, group.Layer, styleACISwatch(group.Color)))
	}
	return strings.Join(rows, "\n")
}

func (m Model) renderStylePalette(width int) string {
	columns := min(8, max(2, (width-6)/10))
	const rows = 3
	pageSize := columns * rows
	start := ((m.style.ACI - 1) / pageSize) * pageSize
	var lines []string
	for row := 0; row < rows; row++ {
		var cells []string
		for column := 0; column < columns; column++ {
			index := start + row*columns + column + 1
			if index > 255 {
				break
			}
			cell := styleACISwatch(index)
			if m.style.Pane == stylePanePalette && index == m.style.ACI {
				cell = "[" + cell + "]"
			} else {
				cell = " " + cell + " "
			}
			cells = append(cells, cell)
		}
		lines = append(lines, strings.Join(cells, " "))
	}
	selected, _ := aciColor(m.style.ACI)
	detail := fmt.Sprintf("ACI %d  %s", selected.Index, selected.RGB())
	if selected.Name != "" {
		detail += "  " + selected.Name
	}
	lines = append(lines, detail)
	return strings.Join(lines, "\n")
}

func (m Model) renderStyleActions() string {
	var content []string
	if m.style.form == styleFormDelete {
		content = append(content, fmt.Sprintf("Delete %s? y/Enter: confirm  n/Esc: cancel", m.style.deleteLabel))
	} else if m.style.form != styleFormNone {
		content = append(content, m.styleFormTitle())
		for i, field := range m.style.fields {
			prefix := "  "
			if i == m.style.field {
				prefix = "> "
			}
			content = append(content, fmt.Sprintf("%s%-14s %s", prefix, field.label+":", field.input.View()))
		}
		content = append(content, "Enter: save  Tab: next field  Esc: cancel  ctrl+g: create mapping group  ctrl+p: choose ACI")
	} else {
		content = append(content, "g: new group  c: new code default  e/Enter: edit active  d: delete active")
		content = append(content, "Tab: pane  arrows/page/wheel: scroll active table  i: import codes  x: export codes")
		content = append(content, "F1: help  F3/Esc: close")
	}
	message := m.message
	if m.lastErr != "" {
		message = errorStyle.Render(m.lastErr)
	}
	if strings.TrimSpace(message) != "" {
		content = append(content, message)
	}
	content = append(content, mutedStyle.Render("Nominal ACI preview; AutoCAD display/plot styles and background may alter appearance."))
	return strings.Join(content, "\n")
}

func (m Model) styleFormTitle() string {
	switch m.style.form {
	case styleFormGroupAdd:
		return "New style group"
	case styleFormGroupEdit:
		return "Edit style group"
	case styleFormCodeAdd:
		return "New point-code default"
	case styleFormCodeEdit:
		return "Edit point-code default"
	case styleFormImport:
		return "Import code library"
	case styleFormExport:
		return "Export code library"
	default:
		return ""
	}
}

func stylePaneTitle(title string, active bool) string {
	if active {
		title += " [active]"
	}
	return title
}

func styleTablePaneTitle(title string, active bool, count, selected, limit int) string {
	title = stylePaneTitle(title, active)
	if count == 0 {
		return title + " 0/0"
	}
	start := visibleStyleStart(count, selected, limit)
	end := min(count, start+limit)
	return fmt.Sprintf("%s %d-%d/%d", title, start+1, end, count)
}

func styleACISwatch(index int) string {
	color, ok := aciColor(index)
	if !ok {
		return "ACI ?"
	}
	foreground := "#FFFFFF"
	luminance := 299*int(color.R) + 587*int(color.G) + 114*int(color.B)
	if luminance > 150000 {
		foreground = "#000000"
	}
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color(foreground)).
		Background(lipgloss.Color(color.Hex())).
		Render(fmt.Sprintf("ACI %-3d", index))
}

func visibleStyleStart(length, selected, limit int) int {
	if length <= limit {
		return 0
	}
	start := selected - limit/2
	if start < 0 {
		return 0
	}
	if start+limit > length {
		return length - limit
	}
	return start
}

func visibleStyleSlice[T any](items []T, selected, limit int) []T {
	start := visibleStyleStart(len(items), selected, limit)
	end := min(len(items), start+limit)
	return items[start:end]
}
