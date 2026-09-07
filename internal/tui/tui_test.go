package tui

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"lsurvey/internal/cogo"
	"lsurvey/internal/geom"
	"lsurvey/internal/project"
)

func TestExecuteCommandRoutesCogoAndUpdatesDirtyState(t *testing.T) {
	m := NewModel(project.New("test"), "")
	m.ExecuteCommand("pt add 1 100 200 PEG")
	if _, ok := m.project.Points["1"]; !ok {
		t.Fatal("point not created")
	}
	if !m.dirty {
		t.Fatal("model should be dirty after cogo mutation")
	}
	if m.lastErr != "" {
		t.Fatalf("unexpected error %q", m.lastErr)
	}
}

func TestExecuteCommandLeavesReadOnlyCogoResultClean(t *testing.T) {
	m := NewModel(project.New("test"), "")
	m.ExecuteCommand("pt add 1 0 0")
	m.ExecuteCommand("pt add 2 10 0")
	m.ExecuteCommand("save " + filepath.Join(t.TempDir(), "job"))
	m.ExecuteCommand("inverse 1 2")
	if m.dirty {
		t.Fatal("read-only calculation should not mark model dirty")
	}
}

func TestExecuteCommandUndoRedoAndInfoUseSession(t *testing.T) {
	m := NewModel(project.New("test"), "")
	m.ExecuteCommand("pt add 1 100 200 PEG")
	m.ExecuteCommand("undo")
	if _, exists := m.project.Points["1"]; exists || !strings.Contains(m.message, "undid") {
		t.Fatalf("undo project=%+v message=%q", m.project.Points, m.message)
	}
	m.ExecuteCommand("redo")
	if _, exists := m.project.Points["1"]; !exists || !strings.Contains(m.message, "redid") {
		t.Fatalf("redo project=%+v message=%q", m.project.Points, m.message)
	}
	m.ExecuteCommand("info")
	if !strings.Contains(m.message, "project=test") || !strings.Contains(m.message, "undo=true") {
		t.Fatalf("info message=%q", m.message)
	}
}

func TestNewModelStartsInSplashModeWithVersion(t *testing.T) {
	m := NewStartupModelWithVersion(project.New("test"), "", "1.2.3")
	if m.mode != ModeSplash {
		t.Fatalf("mode=%v want splash", m.mode)
	}
	m.width = 80
	m.height = 24
	view := m.View()
	for _, want := range []string{"| |    ___", "| |___\\__ \\", "Version 1.2.3", "Press any key to continue"} {
		if !strings.Contains(view, want) {
			t.Fatalf("splash missing %q:\n%s", want, view)
		}
	}
}

func TestSplashTitleFallsBackWhenNarrow(t *testing.T) {
	if got := splashTitle(10); got != "Lsurvey" {
		t.Fatalf("title=%q want compact fallback", got)
	}
}

func TestSplashDoneMessageEntersMainMode(t *testing.T) {
	m := NewStartupModelWithVersion(project.New("test"), "", "1.2.3")
	updated, _ := m.Update(splashDoneMsg{})
	got := updated.(Model)
	if got.mode != ModeMain {
		t.Fatalf("mode=%v want main", got.mode)
	}
}

func TestSplashKeyDismissesToMainMode(t *testing.T) {
	m := NewStartupModelWithVersion(project.New("test"), "", "1.2.3")
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	got := updated.(Model)
	if got.mode != ModeMain {
		t.Fatalf("mode=%v want main", got.mode)
	}
}

func TestDismissSplashCommandReturnsSplashDoneMessage(t *testing.T) {
	cmd := dismissSplashAfter(0)
	msg := cmd()
	if _, ok := msg.(splashDoneMsg); !ok {
		t.Fatalf("message=%T want splashDoneMsg", msg)
	}
}

func TestExecuteCommandFilterSortAndHelp(t *testing.T) {
	m := NewModel(project.New("test"), "")
	m.ExecuteCommand("filter PEG")
	if m.filter != "PEG" {
		t.Fatalf("filter=%q want PEG", m.filter)
	}
	m.ExecuteCommand("sort code desc")
	if m.sort != SortCode || m.sortAsc {
		t.Fatalf("sort=%s asc=%v", m.sort, m.sortAsc)
	}
	m.ExecuteCommand("help rad")
	if m.mode != ModeHelp {
		t.Fatal("expected help mode")
	}
	if m.helpPage != helpPageDetail || m.helpReturn {
		t.Fatalf("help page=%v return=%v want direct detail", m.helpPage, m.helpReturn)
	}
	if !strings.Contains(m.help.View(), "rad") {
		t.Fatalf("help view missing rad: %s", m.help.View())
	}
	m.width = 80
	m.height = 24
	view := m.View()
	for _, want := range []string{"Help", "Usage", "╭", "╰"} {
		if !strings.Contains(view, want) {
			t.Fatalf("help view missing %q:\n%s", want, view)
		}
	}
}

func TestMouseWheelScrollsHelpViewport(t *testing.T) {
	m := NewModel(project.New("test"), "")
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 18})
	m = updated.(Model)
	m.ExecuteCommand("help rad")
	if m.mode != ModeHelp {
		t.Fatal("expected help mode")
	}
	m.help.SetContent(strings.Repeat("long detail line\n", 60))

	updated, _ = m.Update(tea.MouseMsg{
		X:      10,
		Y:      8,
		Button: tea.MouseButtonWheelDown,
		Action: tea.MouseActionPress,
	})
	m = updated.(Model)
	if m.help.YOffset == 0 {
		t.Fatal("expected help viewport wheel scroll")
	}
	view := m.View()
	for _, want := range []string{"mouse wheel scroll", "%"} {
		if !strings.Contains(view, want) {
			t.Fatalf("help view missing %q:\n%s", want, view)
		}
	}
}

func TestMouseWheelScrollsHelpBrowserAndShowsPercent(t *testing.T) {
	m := NewModel(project.New("test"), "")
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 18})
	m = updated.(Model)
	m.ExecuteCommand("help")
	if m.mode != ModeHelp || m.helpPage != helpPageBrowser {
		t.Fatalf("mode=%v page=%v want help browser", m.mode, m.helpPage)
	}

	updated, _ = m.Update(tea.MouseMsg{
		X:      10,
		Y:      8,
		Button: tea.MouseButtonWheelDown,
		Action: tea.MouseActionPress,
	})
	m = updated.(Model)
	if m.helpList.GlobalIndex() == 0 {
		t.Fatal("expected help browser wheel scroll")
	}
	view := m.View()
	for _, want := range []string{"mouse wheel scroll", "%"} {
		if !strings.Contains(view, want) {
			t.Fatalf("help browser missing %q:\n%s", want, view)
		}
	}
}

func TestHelpBrowserFiltersAndOpensSelectedDetail(t *testing.T) {
	m := NewModel(project.New("test"), "")
	m.ExecuteCommand("help")
	if m.mode != ModeHelp || m.helpPage != helpPageBrowser {
		t.Fatalf("mode=%v page=%v want browser", m.mode, m.helpPage)
	}
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	m = updated.(Model)
	if !m.helpList.SettingFilter() {
		t.Fatal("slash should enter help filtering")
	}
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = updated.(Model)
	if m.mode != ModeHelp || m.helpList.SettingFilter() {
		t.Fatal("escape should cancel filtering without closing browser")
	}

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	m = updated.(Model)
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("landxml")})
	m = updated.(Model)
	updated, _ = m.Update(filterMatchesFromCommand(t, cmd))
	m = updated.(Model)
	if count := len(m.helpList.VisibleItems()); count == 0 || count >= len(m.helpList.Items()) {
		t.Fatalf("filtered commands=%d should be a non-empty reduced result set", count)
	}
	found := false
	for _, result := range m.helpList.VisibleItems() {
		item, ok := result.(helpCommandItem)
		if ok && item.command.Name == "export landxml" {
			found = true
		}
	}
	if !found {
		t.Fatal("filtered results do not include export landxml")
	}

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)
	for {
		item, ok := m.helpList.SelectedItem().(helpCommandItem)
		if ok && item.command.Name == "export landxml" {
			break
		}
		updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
		m = updated.(Model)
	}
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)
	if m.helpPage != helpPageDetail || !m.helpReturn || !strings.Contains(m.help.View(), "export landxml") {
		t.Fatalf("detail page=%v return=%v content=%q", m.helpPage, m.helpReturn, m.help.View())
	}
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = updated.(Model)
	if m.helpPage != helpPageBrowser || m.helpList.FilterValue() != "landxml" {
		t.Fatalf("browser return page=%v filter=%q", m.helpPage, m.helpList.FilterValue())
	}
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = updated.(Model)
	if m.mode != ModeHelp || m.helpList.IsFiltered() {
		t.Fatal("first browser escape should clear the applied filter")
	}
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = updated.(Model)
	if m.mode != ModeMain {
		t.Fatalf("mode=%v want closed help", m.mode)
	}
}

func TestHelpBrowserRendersLogicalSections(t *testing.T) {
	browser := newHelpBrowser(120, 240)
	got := browser.View()
	for _, want := range []string{"Project & Session", "Point Data", "COGO Calculations", "Feature Geometry & Styling", "Import & Export", "Interface"} {
		if !strings.Contains(got, want) {
			t.Fatalf("help browser missing section %q:\n%s", want, got)
		}
	}
	lines := strings.Split(got, "\n")
	separate := false
	for i := 0; i+1 < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "Project & Session" && strings.HasPrefix(strings.TrimSpace(lines[i+1]), "> new <name>") {
			separate = true
			break
		}
	}
	if !separate {
		t.Fatalf("first command should render below its section heading:\n%s", got)
	}
}

func TestHelpBrowserFiltersOnlyCommandNames(t *testing.T) {
	browser := newHelpBrowser(120, 120)
	for _, item := range browser.Items() {
		command := item.(helpCommandItem).command
		if got := item.FilterValue(); got != command.Name {
			t.Fatalf("filter value for %q=%q want command name only", command.Name, got)
		}
	}
}

func TestScreenTabsRenderAndSwitchByMouse(t *testing.T) {
	m := NewModel(project.New("test"), "")
	m.width = 90
	m.height = 24
	view := m.View()
	for _, want := range []string{"Project", "Help F1", "Map F2", "Styles F3", "Convert F4"} {
		if !strings.Contains(view, want) {
			t.Fatalf("tab bar missing %q:\n%s", want, view)
		}
	}

	for _, tc := range []struct {
		label string
		mode  Mode
	}{
		{label: "Help F1", mode: ModeHelp},
		{label: "Map F2", mode: ModeMap},
		{label: "Styles F3", mode: ModeStyle},
		{label: "Convert F4", mode: ModeConvert},
		{label: "Project", mode: ModeMain},
	} {
		updated, _ := m.Update(tea.MouseMsg{
			X:      tabClickX(tc.label),
			Y:      0,
			Button: tea.MouseButtonLeft,
			Action: tea.MouseActionPress,
		})
		m = updated.(Model)
		if m.mode != tc.mode {
			t.Fatalf("click %q mode=%v want %v", tc.label, m.mode, tc.mode)
		}
	}
}

func TestKeyboardTabStillCyclesMainFocus(t *testing.T) {
	m := NewModel(project.New("test"), "")
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = updated.(Model)
	if m.mode != ModeMain || m.focus != focusPoints {
		t.Fatalf("mode=%v focus=%v want main points focus", m.mode, m.focus)
	}
}

func tabClickX(label string) int {
	left := 0
	for _, tab := range screenTabs {
		width := lipgloss.Width(" " + tab.Label + " ")
		if tab.Label == label {
			return left + width/2
		}
		left += width
	}
	return -1
}

func filterMatchesFromCommand(t *testing.T, cmd tea.Cmd) list.FilterMatchesMsg {
	t.Helper()
	msg, ok := findFilterMatches(cmd)
	if !ok {
		t.Fatal("filtering did not produce a list.FilterMatchesMsg")
	}
	return msg
}

func findFilterMatches(cmd tea.Cmd) (list.FilterMatchesMsg, bool) {
	if cmd == nil {
		return nil, false
	}
	switch msg := cmd().(type) {
	case list.FilterMatchesMsg:
		return msg, true
	case tea.BatchMsg:
		for _, nested := range msg {
			if result, ok := findFilterMatches(nested); ok {
				return result, true
			}
		}
	}
	return nil, false
}

func TestDescriptionCommandUpdatesStatusAndPersists(t *testing.T) {
	dir := t.TempDir()
	projectPath := filepath.Join(dir, "job")
	m := NewModel(project.New("untitled"), "")
	m.ExecuteCommand("desc Boundary survey")
	if m.project.Description != "Boundary survey" {
		t.Fatalf("description=%q want Boundary survey", m.project.Description)
	}
	if !m.dirty {
		t.Fatal("description command should mark project dirty")
	}
	if !strings.Contains(m.statusLine(0), "Boundary survey") {
		t.Fatalf("status line=%q", m.statusLine(0))
	}
	m.ExecuteCommand("save " + projectPath)
	loaded, err := project.Load(projectPath + ".srv")
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Description != "Boundary survey" {
		t.Fatalf("loaded description=%q want Boundary survey", loaded.Description)
	}
}

func TestPrecisionCommandUpdatesDisplayAndPersists(t *testing.T) {
	dir := t.TempDir()
	projectPath := filepath.Join(dir, "precision")
	m := NewModel(project.New("test"), "")
	m.ExecuteCommand("precision 4")
	if m.project.DisplayPrecision() != 4 {
		t.Fatalf("precision=%d want 4", m.project.DisplayPrecision())
	}
	if !m.dirty {
		t.Fatal("precision command should mark project dirty")
	}
	if !strings.Contains(m.statusLine(0), "precision=4") {
		t.Fatalf("status line=%q", m.statusLine(0))
	}
	m.ExecuteCommand("pt add 1 0 0")
	m.ExecuteCommand("pt add 2 0 1")
	result, err := cogo.Execute(m.project, "inverse 1 2")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result.Message, "hd=1.0000") {
		t.Fatalf("message=%q", result.Message)
	}
	m.ExecuteCommand("save " + projectPath)
	loaded, err := project.Load(projectPath + ".srv")
	if err != nil {
		t.Fatal(err)
	}
	if loaded.DisplayPrecision() != 4 {
		t.Fatalf("loaded precision=%d want 4", loaded.DisplayPrecision())
	}
}

func TestPrecisionCommandRejectsInvalidPrecision(t *testing.T) {
	m := NewModel(project.New("test"), "")
	m.ExecuteCommand("precision 9")
	if m.lastErr == "" {
		t.Fatal("expected precision error")
	}
}

func TestScaleCommandShowsCoordinateModeAndExportWarning(t *testing.T) {
	dir := t.TempDir()
	m := NewModel(project.New("test"), "")
	m.ExecuteCommand("pt add 1 500000 6500000")
	m.ExecuteCommand("scale apply 1 csf=0.9996 system=MGA2020_ZONE50")
	if m.lastErr != "" {
		t.Fatalf("scale error: %s", m.lastErr)
	}
	if !strings.Contains(m.statusLine(1), "coords=MGA2020_ZONE50") {
		t.Fatalf("status line=%q", m.statusLine(1))
	}
	m.ExecuteCommand("export csv " + filepath.Join(dir, "points"))
	if !strings.Contains(m.message, "coords=MGA2020_ZONE50") {
		t.Fatalf("message=%q", m.message)
	}
	m.ExecuteCommand("scale reverse")
	if strings.Contains(m.statusLine(1), "coords=") {
		t.Fatalf("status line should not keep scale label after reverse: %q", m.statusLine(1))
	}
}

func TestScaleWithoutSystemLabelDoesNotExposeInternalMode(t *testing.T) {
	m := NewModel(project.New("test"), "")
	m.ExecuteCommand("pt add 1 500000 6500000")
	m.ExecuteCommand("scale apply 1 csf=0.9996")
	if m.lastErr != "" {
		t.Fatalf("scale error: %s", m.lastErr)
	}
	if strings.Contains(m.statusLine(1), "coords=") || strings.Contains(m.statusLine(1), "local_ground") {
		t.Fatalf("status line should not expose internal scale mode: %q", m.statusLine(1))
	}
}

func TestScaleWithoutSystemLabelHidesPersistedCRSWhileCoordinatesAreLocal(t *testing.T) {
	p := project.New("test")
	p.HorizontalCRS = &project.HorizontalCRS{Datum: "GDA2020", Projection: "MGA", Zone: 50}
	m := NewModel(p, "")
	m.ExecuteCommand("pt add 1 500000 6500000")
	m.ExecuteCommand("scale apply 1 csf=0.9996")
	if strings.Contains(m.statusLine(1), "MGA2020_ZONE50") {
		t.Fatalf("status line should not label locally scaled coordinates as MGA: %q", m.statusLine(1))
	}
	m.ExecuteCommand("scale reverse")
	if !strings.Contains(m.statusLine(1), "MGA2020_ZONE50") {
		t.Fatalf("status line should restore project CRS after scale reverse: %q", m.statusLine(1))
	}
}

func TestExecuteCommandSaveAndExport(t *testing.T) {
	dir := t.TempDir()
	projectPath := filepath.Join(dir, "job")
	dxfPath := filepath.Join(dir, "job")
	m := NewModel(project.New("test"), "")
	m.ExecuteCommand("pt add 1 100 200 PEG")
	m.ExecuteCommand("save " + projectPath)
	if m.lastErr != "" {
		t.Fatalf("save error: %s", m.lastErr)
	}
	if m.dirty {
		t.Fatal("model should not be dirty after save")
	}
	if m.path != projectPath+".srv" {
		t.Fatalf("path=%q want %q", m.path, projectPath+".srv")
	}
	m.ExecuteCommand("export dxf " + dxfPath)
	if m.lastErr != "" {
		t.Fatalf("export error: %s", m.lastErr)
	}
	if !strings.Contains(m.message, dxfPath+".dxf") {
		t.Fatalf("message=%q", m.message)
	}
}

func TestExecuteCommandImportExportCSV(t *testing.T) {
	dir := t.TempDir()
	csvPath := filepath.Join(dir, "points")
	m := NewModel(project.New("test"), "")
	m.ExecuteCommand("pt add 1 100 200 5.5 PEG")
	m.ExecuteCommand("export csv " + csvPath)
	if m.lastErr != "" {
		t.Fatalf("export csv error: %s", m.lastErr)
	}

	loaded := NewModel(project.New("loaded"), "")
	loaded.ExecuteCommand("import csv " + csvPath)
	if loaded.lastErr != "" {
		t.Fatalf("import csv error: %s", loaded.lastErr)
	}
	if loaded.project.Points["1"].Code != "PEG" {
		t.Fatalf("point=%+v", loaded.project.Points["1"])
	}
	if !loaded.dirty {
		t.Fatal("import should mark project dirty")
	}
}

func TestExecuteCommandImportExportGeoJSON(t *testing.T) {
	dir := t.TempDir()
	geojsonPath := filepath.Join(dir, "survey")
	m := NewModel(project.New("test"), "")
	m.ExecuteCommand("pt add 1 100 200 5.5 PEG")
	m.ExecuteCommand(`pt edit 1 desc="corner"`)
	m.ExecuteCommand("pt add 2 110 210")
	m.ExecuteCommand("line add L1 1 2 BOUNDARY")
	m.ExecuteCommand("export geojson " + geojsonPath)
	if m.lastErr != "" {
		t.Fatalf("export geojson error: %s", m.lastErr)
	}

	loaded := NewModel(project.New("loaded"), "")
	loaded.ExecuteCommand("import geojson " + geojsonPath)
	if loaded.lastErr != "" {
		t.Fatalf("import geojson error: %s", loaded.lastErr)
	}
	if len(loaded.project.Points) != 2 {
		t.Fatalf("points=%d want 2", len(loaded.project.Points))
	}
	if len(loaded.project.Features) != 1 {
		t.Fatalf("lines=%d want 1", len(loaded.project.Features))
	}
	if got := loaded.project.Features["L1"]; got.PointIDs[0] != "1" || got.PointIDs[1] != "2" {
		t.Fatalf("line=%+v want from=1 to=2", got)
	}
	if !loaded.dirty {
		t.Fatal("import should mark project dirty")
	}
}

func TestF5FileBrowserImportsCSVSelection(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "points.csv")
	if err := os.WriteFile(path, []byte("id,easting,northing,elevation,code,description\n1,100,200,,PEG,corner\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	m := NewModel(project.New("test"), "")
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyF5})
	m = updated.(Model)
	if !m.importPicking {
		t.Fatal("expected import picker")
	}
	m.importPicker.CurrentDirectory = dir
	updated, _ = m.Update(m.importPicker.Init()())
	m = updated.(Model)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)
	if m.importPicking || m.lastErr != "" || m.project.Points["1"].Code != "PEG" || !m.dirty {
		t.Fatalf("picker=%v error=%q dirty=%v points=%+v", m.importPicking, m.lastErr, m.dirty, m.project.Points)
	}
}

func TestF5FileBrowserImportsGeoJSONSelection(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "survey.geojson")
	source := NewModel(project.New("source"), "")
	source.ExecuteCommand("pt add 1 100 200 PEG")
	source.ExecuteCommand("pt add 2 110 210")
	source.ExecuteCommand("line add L1 1 2 BOUNDARY")
	source.ExecuteCommand("export geojson " + path)
	if source.lastErr != "" {
		t.Fatalf("export geojson error: %s", source.lastErr)
	}

	m := NewModel(project.New("test"), "")
	m.startImportPicker()
	m.importPicker.CurrentDirectory = dir
	updated, _ := m.Update(m.importPicker.Init()())
	m = updated.(Model)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)
	if m.importPicking || m.lastErr != "" || len(m.project.Points) != 2 || len(m.project.Features) != 1 || !m.dirty {
		t.Fatalf("picker=%v error=%q dirty=%v points=%d features=%d", m.importPicking, m.lastErr, m.dirty, len(m.project.Points), len(m.project.Features))
	}
}

func TestF5FileBrowserOpensSRVSelection(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "opened.srv")
	p := project.New("opened")
	p.Points["1"] = geom.Point{ID: "1", Easting: 1, Northing: 2}
	if err := project.Save(path, p); err != nil {
		t.Fatal(err)
	}

	m := NewModel(project.New("current"), "")
	m.ExecuteCommand("map lines")
	m.startImportPicker()
	m.importPicker.CurrentDirectory = dir
	updated, _ := m.Update(m.importPicker.Init()())
	m = updated.(Model)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)
	if m.importPicking || m.lastErr != "" || m.path != path || m.project.Name != "opened" {
		t.Fatalf("picker=%v error=%q path=%q project=%q", m.importPicking, m.lastErr, m.path, m.project.Name)
	}
	if m.mapState.ShowLines {
		t.Fatalf("open should reset map state: %+v", m.mapState)
	}
}

func TestF5FileBrowserConfirmsDirtySRVOpen(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "replacement.srv")
	p := project.New("replacement")
	p.Points["2"] = geom.Point{ID: "2", Easting: 5, Northing: 6}
	if err := project.Save(path, p); err != nil {
		t.Fatal(err)
	}

	m := NewModel(project.New("current"), "")
	m.ExecuteCommand("pt add 1 1 1")
	m.startImportPicker()
	m.importPicker.CurrentDirectory = dir
	updated, _ := m.Update(m.importPicker.Init()())
	m = updated.(Model)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)
	if m.importPicking || m.pendingAction == "" || m.project.Name != "current" {
		t.Fatalf("picker=%v confirm=%q project=%q", m.importPicking, m.pendingAction, m.project.Name)
	}
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = updated.(Model)
	if m.importPicking || m.pendingAction != "" || m.project.Name != "current" {
		t.Fatalf("cancel picker=%v confirm=%q project=%q", m.importPicking, m.pendingAction, m.project.Name)
	}
	m.dispatchCommand("open " + quoteStyleField(path))
	if m.pendingAction == "" {
		t.Fatal("expected confirmation to restart")
	}
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("d")})
	m = updated.(Model)
	if m.importPicking || m.lastErr != "" || m.project.Name != "replacement" || m.path != path {
		t.Fatalf("picker=%v error=%q path=%q project=%q", m.importPicking, m.lastErr, m.path, m.project.Name)
	}
}

func TestF5FileBrowserCancelAndUnsupportedSelection(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("not supported"), 0o644); err != nil {
		t.Fatal(err)
	}
	m := NewModel(project.New("test"), "")
	m.startImportPicker()
	m.importPicker.CurrentDirectory = dir
	updated, _ := m.Update(m.importPicker.Init()())
	m = updated.(Model)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)
	if !m.importPicking || !strings.Contains(m.lastErr, "unsupported file type") {
		t.Fatalf("picker=%v error=%q", m.importPicking, m.lastErr)
	}
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = updated.(Model)
	if m.importPicking || m.mode != ModeMain {
		t.Fatalf("picker=%v mode=%v", m.importPicking, m.mode)
	}
}

func TestSaveWithoutArgumentNormalizesExistingPath(t *testing.T) {
	path := filepath.Join(t.TempDir(), "job")
	m := NewModel(project.New("test"), path)
	m.ExecuteCommand("save")
	if m.path != path+".srv" {
		t.Fatalf("path=%q want job.srv", m.path)
	}
}

func TestExecuteCommandInvalidShowsError(t *testing.T) {
	m := NewModel(project.New("test"), "")
	m.ExecuteCommand("sort nope")
	if m.lastErr == "" {
		t.Fatal("expected error")
	}
}

func TestPlainSIsCommandInputNotSortShortcut(t *testing.T) {
	model := NewModel(project.New("test"), "")
	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}, Alt: false})
	got := updated.(Model)
	if got.input.Value() != "s" {
		t.Fatalf("input=%q want s", got.input.Value())
	}
	if got.sort != SortID {
		t.Fatalf("sort changed to %s", got.sort)
	}
}

func TestUpDownBrowseCommandHistory(t *testing.T) {
	model := NewModel(project.New("test"), "")
	model.input.SetValue("pt add 1 0 0")
	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updated.(Model)
	model.input.SetValue("pt add 2 0 1")
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updated.(Model)

	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyUp})
	model = updated.(Model)
	if model.input.Value() != "pt add 2 0 1" {
		t.Fatalf("input=%q want latest command", model.input.Value())
	}
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyUp})
	model = updated.(Model)
	if model.input.Value() != "pt add 1 0 0" {
		t.Fatalf("input=%q want previous command", model.input.Value())
	}
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyDown})
	model = updated.(Model)
	if model.input.Value() != "pt add 2 0 1" {
		t.Fatalf("input=%q want next command", model.input.Value())
	}
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyDown})
	model = updated.(Model)
	if model.input.Value() != "" {
		t.Fatalf("input=%q want empty prompt", model.input.Value())
	}
}

func TestTabCyclesFocusOnlyWhenCommandInputIsBlank(t *testing.T) {
	m := NewModel(project.New("test"), "")

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = updated.(Model)
	if m.focus != focusPoints {
		t.Fatalf("focus=%v want points", m.focus)
	}

	m.focus = focusCommand
	m.syncInputFocus()
	m.input.SetValue("he")
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = updated.(Model)
	if m.focus != focusCommand {
		t.Fatalf("focus=%v want command", m.focus)
	}
	if m.input.Value() == "he" {
		t.Fatalf("tab should still perform completion when command input is not blank")
	}
}

func TestShiftTabCyclesFocusBackward(t *testing.T) {
	m := NewModel(project.New("test"), "")
	m.focus = focusPoints
	m.syncInputFocus()

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyShiftTab})
	m = updated.(Model)
	if m.focus != focusCommand {
		t.Fatalf("focus=%v want command", m.focus)
	}
}

func TestDuplicateConsecutiveHistoryNotRecorded(t *testing.T) {
	model := NewModel(project.New("test"), "")
	model.recordHistory("help")
	model.recordHistory("help")
	if len(model.history) != 1 {
		t.Fatalf("history=%v", model.history)
	}
}

func TestMapCommandAndShortcutToggleMode(t *testing.T) {
	m := NewModel(project.New("test"), "")
	m.ExecuteCommand("map")
	if m.mode != ModeMap {
		t.Fatalf("mode=%v want map", m.mode)
	}
	m.ExecuteCommand("map")
	if m.mode != ModeMain {
		t.Fatalf("mode=%v want main", m.mode)
	}
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyF2})
	m = updated.(Model)
	if m.mode != ModeMap {
		t.Fatalf("mode=%v want map", m.mode)
	}
}

func TestMStartsMidpointCommandInput(t *testing.T) {
	m := NewModel(project.New("test"), "")
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'m'}})
	got := updated.(Model)
	if got.mode != ModeMain {
		t.Fatalf("mode=%v want main", got.mode)
	}
	if got.input.Value() != "m" {
		t.Fatalf("input=%q want m", got.input.Value())
	}
}

func TestMapLineZoomFitAndEscKeys(t *testing.T) {
	p := project.New("test")
	p.Points["1"] = geom.Point{ID: "1", Northing: 0, Easting: 0}
	p.Points["2"] = geom.Point{ID: "2", Northing: 0, Easting: 10}
	p.Features["L1"] = project.Feature{ID: "L1", Kind: project.FeatureLine, PointIDs: []string{"1", "2"}}
	m := NewModel(p, "")
	m.mode = ModeMap

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})
	m = updated.(Model)
	if !m.mapState.ShowLines {
		t.Fatal("expected lines on")
	}
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	m = updated.(Model)
	if !m.mapState.ShowContours {
		t.Fatal("expected contours on")
	}
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'i'}})
	m = updated.(Model)
	if !m.mapState.ShowCodes {
		t.Fatal("expected code labels on")
	}
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'+'}})
	m = updated.(Model)
	if m.mapState.Zoom <= 1 {
		t.Fatalf("zoom=%f want > 1", m.mapState.Zoom)
	}
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRight})
	m = updated.(Model)
	if m.mapState.CenterE <= 5 {
		t.Fatalf("center easting=%f want panned right", m.mapState.CenterE)
	}
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'f'}})
	m = updated.(Model)
	if m.mapState.Zoom != 1 || m.mapState.Custom {
		t.Fatalf("state=%+v want fit", m.mapState)
	}
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = updated.(Model)
	if m.mode != ModeMain {
		t.Fatalf("mode=%v want main", m.mode)
	}
}

func TestMapEntryResetsLabelsAndHelpReturnsToOrigin(t *testing.T) {
	m := NewModel(project.New("test"), "")
	m.ExecuteCommand("map")
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'i'}})
	m = updated.(Model)
	if !m.mapState.ShowCodes {
		t.Fatal("expected code labels on")
	}

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyF1})
	m = updated.(Model)
	if m.helpPage != helpPageBrowser {
		t.Fatalf("F1 help page=%v want browser", m.helpPage)
	}
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = updated.(Model)
	if m.mode != ModeMap || !m.mapState.ShowCodes {
		t.Fatalf("help escape should return to map and retain map state: mode=%v state=%+v", m.mode, m.mapState)
	}

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = updated.(Model)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyF2})
	m = updated.(Model)
	if m.mapState.ShowCodes {
		t.Fatalf("fresh map entry should default to IDs: %+v", m.mapState)
	}

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'i'}})
	m = updated.(Model)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = updated.(Model)
	m.ExecuteCommand("map lines")
	if m.mapState.ShowCodes {
		t.Fatalf("command-driven map entry should default to IDs: %+v", m.mapState)
	}
}

func TestMapCommandsChangeState(t *testing.T) {
	p := project.New("test")
	p.Points["1"] = geom.Point{ID: "1", Northing: 0, Easting: 0}
	p.Points["2"] = geom.Point{ID: "2", Northing: 0, Easting: 10}
	m := NewModel(p, "")
	m.ExecuteCommand("map lines")
	if m.mode != ModeMap || !m.mapState.ShowLines {
		t.Fatalf("mode=%v state=%+v", m.mode, m.mapState)
	}
	m.ExecuteCommand("map contours")
	if !m.mapState.ShowContours {
		t.Fatalf("state=%+v want contours shown", m.mapState)
	}
	m.ExecuteCommand("map zoom in")
	if m.mapState.Zoom <= 1 {
		t.Fatalf("zoom=%f want > 1", m.mapState.Zoom)
	}
	m.ExecuteCommand("map fit")
	if m.mapState.Zoom != 1 || m.mapState.Custom {
		t.Fatalf("state=%+v want fit", m.mapState)
	}
}

func TestProjectChangesResetMapState(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "job")
	m := NewModel(project.New("test"), "")
	m.project.Points["1"] = geom.Point{ID: "1", Northing: 0, Easting: 0}
	m.project.Points["2"] = geom.Point{ID: "2", Northing: 0, Easting: 10}
	m.ExecuteCommand("map lines")
	m.ExecuteCommand("map contours")
	m.ExecuteCommand("map zoom in")
	m.ExecuteCommand("save " + path)
	if m.lastErr != "" {
		t.Fatalf("save error: %s", m.lastErr)
	}

	m.ExecuteCommand("new next")
	if m.mapState.Zoom != 1 || m.mapState.Custom || m.mapState.ShowLines || m.mapState.ShowContours || m.mapState.ShowCodes {
		t.Fatalf("new should reset map state: %+v", m.mapState)
	}

	m.ExecuteCommand("map lines")
	m.ExecuteCommand("map contours")
	m.ExecuteCommand("map zoom in")
	m.ExecuteCommand("open " + path)
	if m.lastErr != "" {
		t.Fatalf("open error: %s", m.lastErr)
	}
	if m.path != path+".srv" {
		t.Fatalf("path=%q want %q", m.path, path+".srv")
	}
	if m.mapState.Zoom != 1 || m.mapState.Custom || m.mapState.ShowLines || m.mapState.ShowContours || m.mapState.ShowCodes {
		t.Fatalf("open should reset map state: %+v", m.mapState)
	}
}

func TestViewRendersBoxedRegionsWithCommandLast(t *testing.T) {
	m := NewModel(project.New("test"), "")
	m.width = 80
	m.height = 24
	m.ExecuteCommand("pt add 1 100 200 PEG")
	m.ExecuteCommand("pt add 2 110 210 TREE")
	m.ExecuteCommand("line add L1 1 2 BOUNDARY")
	view := m.View()
	for _, want := range []string{"Info", "Points", "Lines", "Command", "╭", "╰"} {
		if !strings.Contains(view, want) {
			t.Fatalf("view missing %q:\n%s", want, view)
		}
	}
	if strings.LastIndex(view, "Command") < strings.LastIndex(view, "Lines") {
		t.Fatalf("command box should be after line box:\n%s", view)
	}
	if !strings.Contains(view, ">") {
		t.Fatalf("command input prompt missing:\n%s", view)
	}
}

func TestViewShowsPointScrollPercentageOnlyWhenScrollable(t *testing.T) {
	p := project.New("test")
	for i := 0; i < 40; i++ {
		id := strconv.Itoa(i + 1)
		p.Points[id] = geom.Point{ID: id, Easting: float64(i), Northing: float64(i)}
	}
	m := NewModel(p, "")
	m.width = 80
	m.height = 24
	m.focus = focusPoints
	m.syncInputFocus()
	m.syncMainViewports()

	if view := m.View(); !strings.Contains(view, "Points [active] 0%") {
		t.Fatalf("top view missing scroll percentage:\n%s", view)
	}

	m.pointsView.ScrollDown(1)
	if title := m.viewportPaneTitle("Points", focusPoints, m.pointsView); !strings.Contains(title, "%") || strings.Contains(title, "0%") || strings.Contains(title, "100%") {
		t.Fatalf("scrolled title=%q want intermediate percentage", title)
	}

	m.pointsView.GotoBottom()
	if view := m.View(); !strings.Contains(view, "Points [active] 100%") {
		t.Fatalf("bottom view missing completed percentage:\n%s", view)
	}

	short := NewModel(project.New("short"), "")
	short.width = 80
	short.height = 24
	if view := short.View(); strings.Contains(view, "Points 100%") || strings.Contains(view, "Lines 100%") {
		t.Fatalf("short panes should not show scroll percentages:\n%s", view)
	}
}

func TestFocusedViewportHandlesArrowAndPageKeys(t *testing.T) {
	p := project.New("test")
	for i := 0; i < 40; i++ {
		id := strconv.Itoa(i + 1)
		p.Points[id] = geom.Point{ID: id, Easting: float64(i), Northing: float64(i)}
	}
	m := NewModel(p, "")
	m.width = 80
	m.height = 24
	m.syncMainViewports()
	m.focus = focusPoints
	m.syncInputFocus()

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = updated.(Model)
	if m.pointsView.YOffset == 0 {
		t.Fatal("expected point viewport to scroll down")
	}

	start := m.pointsView.YOffset
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyPgDown})
	m = updated.(Model)
	if m.pointsView.YOffset <= start {
		t.Fatal("expected page down to advance point viewport")
	}
}

func TestCommandHistoryStillWorksWhenCommandFocused(t *testing.T) {
	m := NewModel(project.New("test"), "")
	m.recordHistory("pt add 1 0 0")
	m.recordHistory("pt add 2 0 1")
	m.focus = focusCommand
	m.syncInputFocus()

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyUp})
	m = updated.(Model)
	if m.input.Value() != "pt add 2 0 1" {
		t.Fatalf("input=%q want latest history entry", m.input.Value())
	}
}

func TestMouseWheelScrollsViewportUnderCursor(t *testing.T) {
	p := project.New("test")
	for i := 0; i < 40; i++ {
		id := strconv.Itoa(i + 1)
		p.Points[id] = geom.Point{ID: id, Easting: float64(i), Northing: float64(i)}
	}
	p.Features["L1"] = project.Feature{ID: "L1", Kind: project.FeatureLine, PointIDs: []string{"1", "2"}}
	m := NewModel(p, "")
	m.width = 80
	m.height = 24
	m.syncMainViewports()

	updated, _ := m.Update(tea.MouseMsg{
		X:      10,
		Y:      6,
		Button: tea.MouseButtonWheelDown,
		Action: tea.MouseActionPress,
	})
	m = updated.(Model)
	if m.focus != focusPoints {
		t.Fatalf("focus=%v want points", m.focus)
	}
	if m.pointsView.YOffset == 0 {
		t.Fatal("expected point viewport wheel scroll")
	}
}

func TestMouseWheelScrollsPointsAddedAfterModelCreation(t *testing.T) {
	m := NewModel(project.New("test"), "")
	m.width = 80
	m.height = 24
	for i := 0; i < 40; i++ {
		id := strconv.Itoa(i + 1)
		m.ExecuteCommand("pt add " + id + " " + id + " " + id)
	}

	updated, _ := m.Update(tea.MouseMsg{
		X:      10,
		Y:      6,
		Button: tea.MouseButtonWheelDown,
		Action: tea.MouseActionPress,
	})
	m = updated.(Model)
	if m.pointsView.YOffset == 0 {
		t.Fatal("expected newly populated point viewport to scroll")
	}
}

func TestMouseWheelFallsBackToActiveViewportWhenHitTestingMisses(t *testing.T) {
	p := project.New("test")
	for i := 0; i < 40; i++ {
		id := strconv.Itoa(i + 1)
		p.Points[id] = geom.Point{ID: id, Easting: float64(i), Northing: float64(i)}
	}
	m := NewModel(p, "")
	m.width = 80
	m.height = 24
	m.syncMainViewports()
	m.focus = focusPoints
	m.syncInputFocus()

	updated, _ := m.Update(tea.MouseMsg{
		X:      -1,
		Y:      -1,
		Button: tea.MouseButtonWheelDown,
		Action: tea.MouseActionPress,
	})
	m = updated.(Model)
	if m.pointsView.YOffset == 0 {
		t.Fatal("expected active point viewport to scroll on wheel fallback")
	}
}

func TestMouseWheelDefaultsToPointsWhenCommandIsFocused(t *testing.T) {
	p := project.New("test")
	for i := 0; i < 40; i++ {
		id := strconv.Itoa(i + 1)
		p.Points[id] = geom.Point{ID: id, Easting: float64(i), Northing: float64(i)}
	}
	m := NewModel(p, "")
	m.width = 80
	m.height = 24
	m.syncMainViewports()
	m.focus = focusCommand
	m.syncInputFocus()

	updated, _ := m.Update(tea.MouseMsg{
		X:      -1,
		Y:      -1,
		Button: tea.MouseButtonWheelDown,
		Action: tea.MouseActionPress,
	})
	m = updated.(Model)
	if m.focus != focusPoints {
		t.Fatalf("focus=%v want points", m.focus)
	}
	if m.pointsView.YOffset == 0 {
		t.Fatal("expected wheel fallback to scroll points when command is focused")
	}
}

func TestClickingCommandPaneRestoresInputFocus(t *testing.T) {
	m := NewModel(project.New("test"), "")
	m.width = 80
	m.height = 24
	m.focus = focusPoints
	m.syncInputFocus()

	updated, _ := m.Update(tea.MouseMsg{
		X:      10,
		Y:      20,
		Button: tea.MouseButtonLeft,
		Action: tea.MouseActionPress,
	})
	m = updated.(Model)
	if m.focus != focusCommand || !m.input.Focused() {
		t.Fatalf("focus=%v input focused=%v want command focus", m.focus, m.input.Focused())
	}
}

func TestStatusLineShowsActiveTraverseContext(t *testing.T) {
	m := NewModel(project.New("test"), "")
	m.project.Points["1"] = geom.Point{ID: "1", Easting: 0, Northing: 0}
	m.project.Points["3"] = geom.Point{ID: "3", Easting: 10, Northing: 0}
	m.project.ContourSets["C1"] = project.ContourSet{ID: "C1", Interval: 1}
	m.ExecuteCommand("trav start 1")

	status := m.infoText(2)
	for _, want := range []string{"contours=1", "trav current=1", "next=4"} {
		if !strings.Contains(status, want) {
			t.Fatalf("status=%q missing %q", status, want)
		}
	}

	m.ExecuteCommand("trav close 3")
	status = m.infoText(2)
	if !strings.Contains(status, "close=3") {
		t.Fatalf("status=%q missing close point", status)
	}
}
