package tui

import (
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

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

func TestNewModelStartsInSplashModeWithVersion(t *testing.T) {
	m := NewStartupModelWithVersion(project.New("test"), "", "1.2.3")
	if m.mode != ModeSplash {
		t.Fatalf("mode=%v want splash", m.mode)
	}
	m.width = 80
	m.height = 24
	view := m.View()
	for _, want := range []string{"LSurvey", "Version 1.2.3", "Press any key to continue"} {
		if !strings.Contains(view, want) {
			t.Fatalf("splash missing %q:\n%s", want, view)
		}
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
	m.ExecuteCommand("help")
	if m.mode != ModeHelp {
		t.Fatal("expected help mode")
	}

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
	if !strings.Contains(m.View(), "mouse wheel scroll") {
		t.Fatalf("help instructions should mention mouse scrolling:\n%s", m.View())
	}
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
	if len(loaded.project.Lines) != 1 {
		t.Fatalf("lines=%d want 1", len(loaded.project.Lines))
	}
	if got := loaded.project.Lines["L1"]; got.From != "1" || got.To != "2" {
		t.Fatalf("line=%+v want from=1 to=2", got)
	}
	if !loaded.dirty {
		t.Fatal("import should mark project dirty")
	}
}

func TestSaveWithoutArgumentNormalizesExistingPath(t *testing.T) {
	m := NewModel(project.New("test"), "job")
	m.ExecuteCommand("save")
	if m.path != "job.srv" {
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
	p.Lines["L1"] = project.Line{ID: "L1", From: "1", To: "2"}
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
	if m.mapState.Zoom != 1 || m.mapState.Custom || m.mapState.ShowLines || m.mapState.ShowContours {
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
	if m.mapState.Zoom != 1 || m.mapState.Custom || m.mapState.ShowLines || m.mapState.ShowContours {
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
	p.Lines["L1"] = project.Line{ID: "L1", From: "1", To: "2"}
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
	m.project.ContourSets["C1"] = project.ContourSet{ID: "C1"}
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
