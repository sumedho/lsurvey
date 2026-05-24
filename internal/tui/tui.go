package tui

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"lsurvey/internal/cogo"
	"lsurvey/internal/csvpoints"
	"lsurvey/internal/dxf"
	"lsurvey/internal/geojson"
	"lsurvey/internal/help"
	"lsurvey/internal/paths"
	"lsurvey/internal/project"
)

type Mode int

const (
	ModeSplash Mode = iota
	ModeMain
	ModeHelp
	ModeMap
)

const splashDuration = 1500 * time.Millisecond

type splashDoneMsg struct{}

type mainFocus int

const (
	focusCommand mainFocus = iota
	focusPoints
	focusLines
)

type Model struct {
	project *project.Project
	path    string
	version string
	dirty   bool

	input      textinput.Model
	help       viewport.Model
	pointsView viewport.Model
	linesView  viewport.Model
	mode       Mode
	prior      Mode
	mapState   MapState
	focus      mainFocus

	width  int
	height int

	filter   string
	sort     SortField
	sortAsc  bool
	history  []string
	histIdx  int
	message  string
	lastErr  string
	quitting bool
}

func NewModel(p *project.Project, path string) Model {
	return newModel(p, path, "dev", false)
}

func NewModelWithVersion(p *project.Project, path, version string) Model {
	return newModel(p, path, version, false)
}

func NewStartupModelWithVersion(p *project.Project, path, version string) Model {
	return newModel(p, path, version, true)
}

func newModel(p *project.Project, path, version string, showSplash bool) Model {
	input := textinput.New()
	input.Placeholder = "type command, F1/help for commands"
	input.Focus()
	input.Prompt = "> "
	input.CharLimit = 512
	input.Width = 80
	input.ShowSuggestions = true
	input.KeyMap.NextSuggestion = key.NewBinding(key.WithKeys("ctrl+n"))
	input.KeyMap.PrevSuggestion = key.NewBinding(key.WithKeys("ctrl+p"))

	helpView := viewport.New(80, 20)
	helpView.SetContent(help.Render(""))
	pointsView := viewport.New(80, 10)
	linesView := viewport.New(80, 6)

	m := Model{
		project:    p,
		path:       path,
		version:    version,
		input:      input,
		help:       helpView,
		pointsView: pointsView,
		linesView:  linesView,
		mode:       ModeMain,
		focus:      focusCommand,
		sort:       SortID,
		sortAsc:    true,
		histIdx:    -1,
		mapState:   newMapState(),
		message:    "F1 opens help. Tab accepts completions or switches panes when input is blank. Use / to filter, alt+s/alt+d to sort.",
	}
	if showSplash {
		m.mode = ModeSplash
	}
	m.syncMainViewports()
	m.refreshCompletions()
	return m
}

func Run(p *project.Project, path string) error {
	return RunWithVersion(p, path, "dev")
}

func RunWithVersion(p *project.Project, path, version string) error {
	_, err := tea.NewProgram(
		NewStartupModelWithVersion(p, path, version),
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	).Run()
	return err
}

func (m Model) Init() tea.Cmd {
	if m.mode == ModeSplash {
		return tea.Batch(textinput.Blink, dismissSplash())
	}
	return textinput.Blink
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.input.Width = max(20, msg.Width-6)
		m.help.Width = max(20, msg.Width-8)
		m.help.Height = max(5, msg.Height-5)
		m.syncMainViewports()
		return m, nil
	case splashDoneMsg:
		if m.mode == ModeSplash {
			m.mode = ModeMain
		}
		return m, nil
	case tea.MouseMsg:
		if m.mode == ModeMain {
			if handled, cmd := m.handleMainMouse(msg); handled {
				return m, cmd
			}
		}
	case tea.KeyMsg:
		if m.mode == ModeSplash {
			if msg.String() == "ctrl+c" {
				m.quitting = true
				return m, tea.Quit
			}
			m.mode = ModeMain
			return m, nil
		}
		if m.mode == ModeHelp {
			switch msg.String() {
			case "esc", "f1":
				m.mode = m.priorMode()
				return m, nil
			case "ctrl+c":
				m.quitting = true
				return m, tea.Quit
			}
			var cmd tea.Cmd
			m.help, cmd = m.help.Update(msg)
			return m, cmd
		}
		if m.mode == ModeMap {
			switch msg.String() {
			case "ctrl+c":
				m.quitting = true
				return m, tea.Quit
			case "esc", "f2":
				m.mode = ModeMain
				return m, nil
			case "f1":
				m.openHelp("")
				return m, nil
			case "l":
				m.mapState.ShowLines = !m.mapState.ShowLines
				return m, nil
			case "+", "=":
				m.mapState.zoomBy(m.project, 1.5)
				return m, nil
			case "-":
				m.mapState.zoomBy(m.project, 1.0/1.5)
				return m, nil
			case "left":
				m.mapState.panBy(m.project, -mapPanFraction, 0)
				return m, nil
			case "right":
				m.mapState.panBy(m.project, mapPanFraction, 0)
				return m, nil
			case "up":
				m.mapState.panBy(m.project, 0, mapPanFraction)
				return m, nil
			case "down":
				m.mapState.panBy(m.project, 0, -mapPanFraction)
				return m, nil
			case "f":
				m.mapState.fit()
				return m, nil
			}
			return m, nil
		}

		switch msg.String() {
		case "ctrl+c":
			m.quitting = true
			return m, tea.Quit
		case "enter":
			return m.executeInput()
		case "tab":
			if m.shouldCycleFocusOnTab() {
				m.cycleMainFocus(1)
				return m, nil
			}
			m.completeNextInput()
			return m, nil
		case "shift+tab":
			m.cycleMainFocus(-1)
			return m, nil
		case "up":
			if m.focus != focusCommand {
				return m.updateFocusedViewport(msg)
			}
			m.previousHistory()
			return m, nil
		case "down":
			if m.focus != focusCommand {
				return m.updateFocusedViewport(msg)
			}
			m.nextHistory()
			return m, nil
		case "pgup", "pgdown":
			if m.focus != focusCommand {
				return m.updateFocusedViewport(msg)
			}
		case "f1":
			m.openHelp("")
			return m, nil
		case "f2":
			if m.input.Value() == "" {
				m.toggleMap()
				return m, nil
			}
		case "/":
			if m.input.Value() == "" {
				m.input.SetValue("filter ")
				m.input.CursorEnd()
				m.refreshCompletions()
				return m, nil
			}
		case "alt+s":
			if m.input.Value() == "" {
				m.cycleSort()
				return m, nil
			}
		case "alt+d":
			if m.input.Value() == "" {
				m.sortAsc = !m.sortAsc
				m.message = "sort " + string(m.sort) + " " + m.sortDirection()
				return m, nil
			}
		}
	}

	var cmd tea.Cmd
	m.refreshCompletions()
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m Model) View() string {
	if m.quitting {
		return ""
	}
	if m.mode == ModeSplash {
		return renderSplash(max(60, m.width), max(18, m.height), m.version)
	}
	if m.mode == ModeHelp {
		width := max(60, m.width)
		height := max(18, m.height)
		body := mutedStyle.Render("Esc closes, arrows/page keys scroll") + "\n" + m.help.View()
		return box("Help", body, width, height)
	}
	if m.mode == ModeMap {
		return renderMap(m.project, m.mapState, max(60, m.width), max(18, m.height), m.project.DisplayPrecision())
	}

	m.syncMainViewports()

	width := max(60, m.width)
	height := max(18, m.height)
	infoHeight := 4
	commandHeight := 6
	lineHeight := max(5, height/4)
	pointHeight := max(6, height-infoHeight-commandHeight-lineHeight)

	points := FilterAndSortPoints(m.project, m.filter, m.sort, m.sortAsc)
	info := m.infoText(len(points))
	message := m.message
	if m.lastErr != "" {
		message = errorStyle.Render(m.lastErr)
	}
	commandLines := []string{m.input.View()}
	if strings.TrimSpace(message) != "" {
		commandLines = append(commandLines, message)
	}
	command := strings.Join(commandLines, "\n")
	return lipgloss.JoinVertical(
		lipgloss.Left,
		box("Info", info, width, infoHeight),
		box(m.viewportPaneTitle("Points", focusPoints, m.pointsView), m.pointsView.View(), width, pointHeight),
		box(m.viewportPaneTitle("Lines", focusLines, m.linesView), m.linesView.View(), width, lineHeight),
		box(m.mainPaneTitle("Command", focusCommand), command, width, commandHeight),
	)
}

func dismissSplashAfter(delay time.Duration) tea.Cmd {
	return tea.Tick(delay, func(time.Time) tea.Msg {
		return splashDoneMsg{}
	})
}

func dismissSplash() tea.Cmd {
	return dismissSplashAfter(splashDuration)
}

func renderSplash(width, height int, version string) string {
	width = max(60, width)
	height = max(18, height)
	body := lipgloss.JoinVertical(
		lipgloss.Center,
		splashTitleStyle.Render("LSurvey"),
		"",
		titleStyle.Render("Version "+version),
		"",
		mutedStyle.Render("Press any key to continue"),
	)
	innerWidth := max(0, width-4)
	innerHeight := max(1, height-3)
	return box("LSurvey", lipgloss.Place(innerWidth, innerHeight, lipgloss.Center, lipgloss.Center, body), width, height)
}

func (m *Model) ExecuteCommand(command string) tea.Cmd {
	command = strings.TrimSpace(command)
	if command == "" {
		return nil
	}
	fields, err := cogo.Fields(command)
	if err != nil {
		m.setError(err.Error())
		return nil
	}
	m.lastErr = ""
	defer m.refreshCompletions()
	defer m.syncMainViewports()
	switch fields[0] {
	case "quit", "exit":
		m.quitting = true
		return tea.Quit
	case "new":
		name := "untitled"
		if len(fields) > 1 {
			name = strings.Join(fields[1:], " ")
		}
		m.project = project.New(name)
		m.path = ""
		m.dirty = false
		m.mapState = newMapState()
		m.message = "new project: " + name
	case "open":
		if len(fields) != 2 {
			m.setError("usage: open <file>")
			return nil
		}
		path := paths.Project(fields[1])
		loaded, err := project.Load(path)
		if err != nil {
			m.setError(err.Error())
			return nil
		}
		m.project = loaded
		m.path = path
		m.dirty = false
		m.mapState = newMapState()
		m.message = "opened " + m.path
	case "save":
		if len(fields) > 1 {
			m.path = paths.Project(fields[1])
		}
		if m.path == "" {
			m.setError("usage: save <file>")
			return nil
		}
		m.path = paths.Project(m.path)
		m.project.AppVersion = m.version
		if err := project.Save(m.path, m.project); err != nil {
			m.setError(err.Error())
			return nil
		}
		m.dirty = false
		m.message = "saved " + m.path
	case "saveas":
		if len(fields) != 2 {
			m.setError("usage: saveas <file>")
			return nil
		}
		m.path = paths.Project(fields[1])
		m.project.AppVersion = m.version
		if err := project.Save(m.path, m.project); err != nil {
			m.setError(err.Error())
			return nil
		}
		m.dirty = false
		m.message = "saved " + m.path
	case "export":
		if len(fields) != 3 {
			m.setError("usage: export dxf|csv|geojson <file>")
			return nil
		}
		switch fields[1] {
		case "dxf":
			path := paths.DXF(fields[2])
			if err := writeDXF(path, m.project); err != nil {
				m.setError(err.Error())
				return nil
			}
			m.message = "exported " + path
		case "csv":
			path := paths.CSV(fields[2])
			if err := csvpoints.ExportFile(path, m.project); err != nil {
				m.setError(err.Error())
				return nil
			}
			m.message = "exported " + path
		case "geojson":
			path := paths.GeoJSON(fields[2])
			if err := geojson.ExportFile(path, m.project); err != nil {
				m.setError(err.Error())
				return nil
			}
			m.message = "exported " + path
		default:
			m.setError("usage: export dxf|csv|geojson <file>")
		}
	case "import":
		if len(fields) != 3 {
			m.setError("usage: import csv|geojson <file>")
			return nil
		}
		switch fields[1] {
		case "csv":
			path := paths.CSV(fields[2])
			count, err := csvpoints.ImportFile(path, m.project)
			if err != nil {
				m.setError(err.Error())
				return nil
			}
			m.dirty = true
			m.message = fmt.Sprintf("imported %d points from %s", count, path)
		case "geojson":
			path := paths.GeoJSON(fields[2])
			points, lines, err := geojson.ImportFile(path, m.project)
			if err != nil {
				m.setError(err.Error())
				return nil
			}
			m.dirty = true
			m.message = fmt.Sprintf("imported %d points and %d lines from %s", points, lines, path)
		default:
			m.setError("usage: import csv|geojson <file>")
		}
	case "desc":
		description := strings.TrimSpace(strings.TrimPrefix(command, "desc"))
		if description == "" {
			m.setError("usage: desc <project description>")
			return nil
		}
		m.project.Description = description
		m.dirty = true
		m.message = "project description updated"
	case "precision":
		if len(fields) != 2 {
			m.setError("usage: precision <0-6>")
			return nil
		}
		precision, err := strconv.Atoi(fields[1])
		if err != nil || precision < 0 || precision > 6 {
			m.setError("precision must be a number from 0 to 6")
			return nil
		}
		m.project.SetDisplayPrecision(precision)
		m.dirty = true
		m.message = fmt.Sprintf("display precision set to %d", precision)
	case "filter":
		m.filter = strings.TrimSpace(strings.TrimPrefix(command, "filter"))
		m.message = "filter " + quoteBlank(m.filter)
	case "clear":
		if len(fields) == 2 && fields[1] == "filter" {
			m.filter = ""
			m.message = "filter cleared"
			return nil
		}
		m.setError("usage: clear filter")
	case "sort":
		m.handleSort(fields)
	case "help":
		query := ""
		if len(fields) > 1 {
			query = strings.Join(fields[1:], " ")
		}
		m.openHelp(query)
	case "map":
		m.handleMapCommand(fields)
	default:
		result, err := cogo.ExecuteAndRecord(m.project, command)
		if err != nil {
			m.setError(err.Error())
			return nil
		}
		m.dirty = true
		m.message = result.Message
	}
	return nil
}

func (m Model) executeInput() (tea.Model, tea.Cmd) {
	command := m.input.Value()
	m.input.SetValue("")
	m.recordHistory(command)
	cmd := (&m).ExecuteCommand(command)
	return m, cmd
}

func (m *Model) openHelp(query string) {
	m.prior = m.mode
	m.help.SetContent(help.RenderStyled(query))
	m.help.GotoTop()
	m.mode = ModeHelp
	m.message = "help"
}

func (m Model) priorMode() Mode {
	if m.prior == ModeMap {
		return ModeMap
	}
	return ModeMain
}

func (m *Model) toggleMap() {
	if m.mode == ModeMap {
		m.mode = ModeMain
		return
	}
	m.mode = ModeMap
	m.message = "map"
}

func (m *Model) handleMapCommand(fields []string) {
	if len(fields) == 1 {
		m.toggleMap()
		return
	}
	if len(fields) == 2 && fields[1] == "lines" {
		m.mapState.ShowLines = !m.mapState.ShowLines
		m.mode = ModeMap
		m.message = "map lines " + onOff(m.mapState.ShowLines)
		return
	}
	if len(fields) == 2 && fields[1] == "fit" {
		m.mapState.fit()
		m.mode = ModeMap
		m.message = "map fit"
		return
	}
	if len(fields) == 3 && fields[1] == "zoom" {
		switch fields[2] {
		case "in":
			m.mapState.zoomBy(m.project, 1.5)
			m.mode = ModeMap
			m.message = "map zoom in"
		case "out":
			m.mapState.zoomBy(m.project, 1.0/1.5)
			m.mode = ModeMap
			m.message = "map zoom out"
		default:
			m.setError("usage: map [lines|fit|zoom in|zoom out]")
		}
		return
	}
	m.setError("usage: map [lines|fit|zoom in|zoom out]")
}

func (m *Model) handleSort(fields []string) {
	if len(fields) < 2 || len(fields) > 3 {
		m.setError("usage: sort <id|north|east|elev|code|desc> [asc|desc]")
		return
	}
	field, ok := ParseSortField(fields[1])
	if !ok {
		m.setError("unknown sort field " + fields[1])
		return
	}
	m.sort = field
	if len(fields) == 3 {
		switch fields[2] {
		case "asc":
			m.sortAsc = true
		case "desc":
			m.sortAsc = false
		default:
			m.setError("sort direction must be asc or desc")
			return
		}
	}
	m.message = "sort " + string(m.sort) + " " + m.sortDirection()
}

func (m *Model) syncMainViewports() {
	width := max(60, m.width)
	height := max(18, m.height)
	infoHeight := 4
	commandHeight := 6
	lineHeight := max(5, height/4)
	pointHeight := max(6, height-infoHeight-commandHeight-lineHeight)
	contentWidth := max(1, width-4)
	pointContentHeight := max(1, pointHeight-3)
	lineContentHeight := max(1, lineHeight-3)

	m.pointsView.Width = contentWidth
	m.pointsView.Height = pointContentHeight
	m.linesView.Width = contentWidth
	m.linesView.Height = lineContentHeight
	m.pointsView.SetContent(FormatPointRows(FilterAndSortPoints(m.project, m.filter, m.sort, m.sortAsc), m.project.DisplayPrecision()))
	m.linesView.SetContent(FormatLineRows(m.project.SortedLines(), m.project.SortedContourSets(), m.project.DisplayPrecision()))
}

func (m *Model) shouldCycleFocusOnTab() bool {
	return m.focus != focusCommand || strings.TrimSpace(m.input.Value()) == ""
}

func (m *Model) cycleMainFocus(step int) {
	order := []mainFocus{focusCommand, focusPoints, focusLines}
	index := 0
	for i, focus := range order {
		if focus == m.focus {
			index = i
			break
		}
	}
	index = (index + step + len(order)) % len(order)
	m.focus = order[index]
	m.syncInputFocus()
}

func (m *Model) syncInputFocus() {
	if m.focus == focusCommand {
		m.input.Focus()
		return
	}
	m.input.Blur()
}

func (m Model) updateFocusedViewport(msg tea.Msg) (tea.Model, tea.Cmd) {
	m.syncMainViewports()
	var cmd tea.Cmd
	switch m.focus {
	case focusPoints:
		m.pointsView, cmd = m.pointsView.Update(msg)
	case focusLines:
		m.linesView, cmd = m.linesView.Update(msg)
	}
	return m, cmd
}

func (m *Model) handleMainMouse(msg tea.MouseMsg) (bool, tea.Cmd) {
	if !isWheelMouse(msg) && !(msg.Button == tea.MouseButtonLeft && msg.Action == tea.MouseActionPress) {
		return false, nil
	}
	m.syncMainViewports()
	pane, ok := m.mainPaneAt(msg.X, msg.Y)
	if ok {
		m.focus = pane
		m.syncInputFocus()

		if pane == focusCommand || !isWheelMouse(msg) {
			return true, nil
		}
		var cmd tea.Cmd
		switch pane {
		case focusPoints:
			m.pointsView, cmd = m.pointsView.Update(msg)
		case focusLines:
			m.linesView, cmd = m.linesView.Update(msg)
		}
		return true, cmd
	}
	if isWheelMouse(msg) {
		target := m.scrollTargetFocus()
		if target != focusCommand {
			m.focus = target
			m.syncInputFocus()
			updated, cmd := m.updateFocusedViewport(msg)
			*m = updated.(Model)
			return true, cmd
		}
	}
	return false, nil
}

func (m Model) mainPaneAt(x, y int) (mainFocus, bool) {
	width := max(60, m.width)
	height := max(18, m.height)
	infoHeight := 4
	commandHeight := 6
	lineHeight := max(5, height/4)
	pointHeight := max(6, height-infoHeight-commandHeight-lineHeight)

	if x < 0 || x >= width {
		return focusCommand, false
	}
	pointTop := infoHeight
	pointBottom := pointTop + pointHeight
	lineTop := pointBottom
	lineBottom := lineTop + lineHeight
	commandTop := lineBottom
	commandBottom := commandTop + commandHeight

	switch {
	case y >= pointTop && y < pointBottom:
		return focusPoints, true
	case y >= lineTop && y < lineBottom:
		return focusLines, true
	case y >= commandTop && y < commandBottom:
		return focusCommand, true
	default:
		return focusCommand, false
	}
}

func (m Model) mainPaneTitle(title string, focus mainFocus) string {
	if m.focus == focus {
		return title + " [active]"
	}
	return title
}

func (m Model) viewportPaneTitle(title string, focus mainFocus, view viewport.Model) string {
	title = m.mainPaneTitle(title, focus)
	if view.TotalLineCount() <= view.VisibleLineCount() {
		return title
	}
	percent := int(view.ScrollPercent()*100 + 0.5)
	return fmt.Sprintf("%s %d%%", title, percent)
}

func isWheelMouse(msg tea.MouseMsg) bool {
	switch msg.Button {
	case tea.MouseButtonWheelUp, tea.MouseButtonWheelDown, tea.MouseButtonWheelLeft, tea.MouseButtonWheelRight:
		return true
	default:
		return false
	}
}

func (m Model) scrollTargetFocus() mainFocus {
	if m.focus == focusLines {
		return focusLines
	}
	return focusPoints
}

func (m *Model) cycleSort() {
	for i, field := range sortFields {
		if field == m.sort {
			m.sort = sortFields[(i+1)%len(sortFields)]
			m.message = "sort " + string(m.sort) + " " + m.sortDirection()
			return
		}
	}
	m.sort = SortID
}

func (m *Model) refreshCompletions() {
	m.input.SetSuggestions(commandSuggestionsForInput(m.project, m.input.Value()))
}

func (m *Model) recordHistory(command string) {
	command = strings.TrimSpace(command)
	if command == "" {
		m.histIdx = len(m.history)
		return
	}
	if len(m.history) == 0 || m.history[len(m.history)-1] != command {
		m.history = append(m.history, command)
	}
	m.histIdx = len(m.history)
}

func (m *Model) previousHistory() {
	if len(m.history) == 0 {
		return
	}
	if m.histIdx <= 0 || m.histIdx > len(m.history) {
		m.histIdx = len(m.history) - 1
	} else {
		m.histIdx--
	}
	m.input.SetValue(m.history[m.histIdx])
	m.input.CursorEnd()
}

func (m *Model) nextHistory() {
	if len(m.history) == 0 {
		return
	}
	if m.histIdx < 0 {
		m.histIdx = 0
	}
	if m.histIdx >= len(m.history)-1 {
		m.histIdx = len(m.history)
		m.input.SetValue("")
		return
	}
	m.histIdx++
	m.input.SetValue(m.history[m.histIdx])
	m.input.CursorEnd()
}

func (m *Model) setError(value string) {
	m.lastErr = value
	m.message = ""
}

func (m Model) sortDirection() string {
	if m.sortAsc {
		return "asc"
	}
	return "desc"
}

func (m Model) statusLine(visible int) string {
	return titleStyle.Render(m.infoText(visible))
}

func (m Model) infoText(visible int) string {
	path := m.path
	if path == "" {
		path = "unsaved"
	}
	dirty := "saved"
	if m.dirty {
		dirty = "dirty"
	}
	filter := quoteBlank(m.filter)
	label := m.project.Name
	if strings.TrimSpace(m.project.Description) != "" {
		label = m.project.Description
	}
	info := fmt.Sprintf("%s  path=%s  points=%d/%d  lines=%d  contours=%d  precision=%d  filter=%s  sort=%s %s  %s",
		label, path, visible, len(m.project.Points), len(m.project.Lines), len(m.project.ContourSets), m.project.DisplayPrecision(), filter, m.sort, m.sortDirection(), dirty)
	if m.project.Traverse != nil {
		traverse := fmt.Sprintf("  trav current=%s next=%s", m.project.Traverse.Current, m.project.NextPointID())
		if m.project.Traverse.Close != "" {
			traverse += " close=" + m.project.Traverse.Close
		}
		info += traverse
	}
	return info
}

func writeDXF(path string, p *project.Project) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	err = dxf.Write(f, p)
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}
	return err
}

func quoteBlank(value string) string {
	if value == "" {
		return "<none>"
	}
	return value
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

var (
	titleStyle       = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("86"))
	splashTitleStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("220"))
	mutedStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	errorStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	tableStyle       = lipgloss.NewStyle()
	boxTitleStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("86"))
	boxStyle         = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("63")).Padding(0, 1)
)
