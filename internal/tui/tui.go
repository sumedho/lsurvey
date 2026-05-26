package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"lsurvey/internal/app"
	"lsurvey/internal/help"
	"lsurvey/internal/project"
)

type Mode int

const (
	ModeSplash Mode = iota
	ModeMain
	ModeHelp
	ModeMap
	ModeStyle
)

const splashDuration = 1500 * time.Millisecond

type splashDoneMsg struct{}

type mainFocus int

const (
	focusCommand mainFocus = iota
	focusPoints
	focusLines
)

type helpPage int

const (
	helpPageBrowser helpPage = iota
	helpPageDetail
)

type Model struct {
	session *app.Session
	project *project.Project
	path    string
	version string
	dirty   bool

	input      textinput.Model
	help       viewport.Model
	helpList   list.Model
	helpPage   helpPage
	helpReturn bool
	pointsView viewport.Model
	linesView  viewport.Model
	mode       Mode
	prior      Mode
	mapState   MapState
	style      StyleState
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
	helpBrowser := newHelpBrowser(76, 16)
	pointsView := viewport.New(80, 10)
	linesView := viewport.New(80, 6)

	session := app.NewSession(p, path, version)
	m := Model{
		session:    session,
		project:    p,
		path:       path,
		version:    version,
		input:      input,
		help:       helpView,
		helpList:   helpBrowser,
		pointsView: pointsView,
		linesView:  linesView,
		mode:       ModeMain,
		focus:      focusCommand,
		sort:       SortID,
		sortAsc:    true,
		histIdx:    -1,
		mapState:   newMapState(),
		style:      newStyleState(),
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
		m.helpList.SetSize(max(20, msg.Width-4), max(5, msg.Height-4))
		m.syncMainViewports()
		return m, nil
	case splashDoneMsg:
		if m.mode == ModeSplash {
			m.mode = ModeMain
		}
		return m, nil
	case list.FilterMatchesMsg:
		if m.mode == ModeHelp && m.helpPage == helpPageBrowser {
			var cmd tea.Cmd
			m.helpList, cmd = m.helpList.Update(msg)
			return m, cmd
		}
	case tea.MouseMsg:
		if m.mode == ModeHelp && m.helpPage == helpPageDetail && isWheelMouse(msg) {
			var cmd tea.Cmd
			m.help, cmd = m.help.Update(msg)
			return m, cmd
		}
		if m.mode == ModeMain {
			if handled, cmd := m.handleMainMouse(msg); handled {
				return m, cmd
			}
		}
		if m.mode == ModeStyle && isWheelMouse(msg) {
			return m.handleStyleMouse(msg)
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
			if msg.String() == "ctrl+c" {
				m.quitting = true
				return m, tea.Quit
			}
			if msg.String() == "f1" {
				m.mode = m.priorMode()
				return m, nil
			}
			if m.helpPage == helpPageBrowser {
				if msg.String() == "enter" && !m.helpList.SettingFilter() {
					if item, ok := m.helpList.SelectedItem().(helpCommandItem); ok {
						m.showHelpDetail(item.command.Name, true)
					}
					return m, nil
				}
				if msg.String() == "esc" && !m.helpList.SettingFilter() && !m.helpList.IsFiltered() {
					m.mode = m.priorMode()
					return m, nil
				}
				var cmd tea.Cmd
				m.helpList, cmd = m.helpList.Update(msg)
				return m, cmd
			}
			switch msg.String() {
			case "esc":
				if m.helpReturn {
					m.helpPage = helpPageBrowser
					m.helpReturn = false
				} else {
					m.mode = m.priorMode()
				}
				return m, nil
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
			case "c":
				m.mapState.ShowContours = !m.mapState.ShowContours
				return m, nil
			case "i":
				m.mapState.ShowCodes = !m.mapState.ShowCodes
				return m, nil
			case "+", "=":
				m.mapState.zoomBy(m.project, 1.5)
				return m, nil
			case "-":
				m.mapState.zoomBy(m.project, 1.0/1.5)
				return m, nil
			case "left":
				width, height := mapGridSize(max(60, m.width), max(18, m.height))
				m.mapState.panBy(m.project, width, height, -mapPanFraction, 0)
				return m, nil
			case "right":
				width, height := mapGridSize(max(60, m.width), max(18, m.height))
				m.mapState.panBy(m.project, width, height, mapPanFraction, 0)
				return m, nil
			case "up":
				width, height := mapGridSize(max(60, m.width), max(18, m.height))
				m.mapState.panBy(m.project, width, height, 0, mapPanFraction)
				return m, nil
			case "down":
				width, height := mapGridSize(max(60, m.width), max(18, m.height))
				m.mapState.panBy(m.project, width, height, 0, -mapPanFraction)
				return m, nil
			case "f":
				m.mapState.fit()
				return m, nil
			}
			return m, nil
		}
		if m.mode == ModeStyle {
			return m.updateStyle(msg)
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
		case "f3":
			if m.input.Value() == "" {
				m.enterStyle()
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
		if m.helpPage == helpPageBrowser {
			body := mutedStyle.Render("/: filter  Enter: details  Esc: clear filter/close  F1: close") + "\n" + m.helpList.View()
			return box("Help", body, width, height)
		}
		back := "Esc closes"
		if m.helpReturn {
			back = "Esc returns to results"
		}
		body := mutedStyle.Render(back+", arrows/page keys or mouse wheel scroll  F1: close") + "\n" + m.help.View()
		return box("Help", body, width, height)
	}
	if m.mode == ModeMap {
		return renderMap(m.project, m.mapState, max(60, m.width), max(18, m.height), m.project.DisplayPrecision())
	}
	if m.mode == ModeStyle {
		return m.renderStyle(max(60, m.width), max(18, m.height))
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
	fields, err := app.Fields(command)
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
	case "style":
		if len(fields) != 1 {
			m.setError("usage: style")
			return nil
		}
		m.enterStyle()
	default:
		outcome, err := m.session.Execute(command)
		if err != nil {
			m.setError(err.Error())
			return nil
		}
		m.syncSessionState()
		if outcome.ProjectReplaced {
			m.mapState = newMapState()
			m.style = newStyleState()
		}
		m.message = outcome.Message
	}
	return nil
}

func (m *Model) syncSessionState() {
	m.project = m.session.Project
	m.path = m.session.Path
	m.version = m.session.Version
	m.dirty = m.session.Dirty
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
	if strings.TrimSpace(query) == "" {
		m.helpList.ResetFilter()
		m.helpList.ResetSelected()
		m.helpPage = helpPageBrowser
		m.helpReturn = false
	} else {
		m.showHelpDetail(query, false)
	}
	m.mode = ModeHelp
	m.message = "help"
}

func (m *Model) showHelpDetail(query string, returnToBrowser bool) {
	m.help.SetContent(help.RenderStyled(query))
	m.help.GotoTop()
	m.helpPage = helpPageDetail
	m.helpReturn = returnToBrowser
}

func (m Model) priorMode() Mode {
	if m.prior == ModeMap || m.prior == ModeStyle {
		return m.prior
	}
	return ModeMain
}

func (m *Model) toggleMap() {
	if m.mode == ModeMap {
		m.mode = ModeMain
		return
	}
	m.enterMap()
	m.message = "map"
}

func (m *Model) enterMap() {
	if m.mode != ModeMap {
		m.mapState.ShowCodes = false
	}
	m.mode = ModeMap
}

func (m *Model) handleMapCommand(fields []string) {
	if len(fields) == 1 {
		m.toggleMap()
		return
	}
	if len(fields) == 2 && fields[1] == "lines" {
		m.mapState.ShowLines = !m.mapState.ShowLines
		m.enterMap()
		m.message = "map lines " + onOff(m.mapState.ShowLines)
		return
	}
	if len(fields) == 2 && fields[1] == "contours" {
		m.mapState.ShowContours = !m.mapState.ShowContours
		m.enterMap()
		m.message = "map contours " + onOff(m.mapState.ShowContours)
		return
	}
	if len(fields) == 2 && fields[1] == "fit" {
		m.mapState.fit()
		m.enterMap()
		m.message = "map fit"
		return
	}
	if len(fields) == 3 && fields[1] == "zoom" {
		switch fields[2] {
		case "in":
			m.mapState.zoomBy(m.project, 1.5)
			m.enterMap()
			m.message = "map zoom in"
		case "out":
			m.mapState.zoomBy(m.project, 1.0/1.5)
			m.enterMap()
			m.message = "map zoom out"
		default:
			m.setError("usage: map [lines|contours|fit|zoom in|zoom out]")
		}
		return
	}
	m.setError("usage: map [lines|contours|fit|zoom in|zoom out]")
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
	m.linesView.SetContent(FormatLineRows(m.project.SortedFeatures(), m.project.SortedContourSets(), m.project.DisplayPrecision()))
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
	info := fmt.Sprintf("%s  path=%s  points=%d/%d  features=%d  contours=%d  precision=%d%s  filter=%s  sort=%s %s  %s",
		label, path, visible, len(m.project.Points), len(m.project.Features), len(m.project.ContourSets), m.project.DisplayPrecision(), m.coordinateSuffix(), filter, m.sort, m.sortDirection(), dirty)
	if m.project.Traverse != nil {
		traverse := fmt.Sprintf("  trav current=%s next=%s", m.project.Traverse.Current, m.project.NextPointID())
		if m.project.Traverse.Close != "" {
			traverse += " close=" + m.project.Traverse.Close
		}
		info += traverse
	}
	return info
}

func (m Model) coordinateSuffix() string {
	label := m.project.CoordinateLabel()
	if label == "" {
		return ""
	}
	return "  coords=" + label
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
