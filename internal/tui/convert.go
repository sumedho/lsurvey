package tui

import (
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/filepicker"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"lsurvey/internal/app"
	"lsurvey/internal/geodesy"
	"lsurvey/internal/geom"
	"lsurvey/internal/paths"
	"lsurvey/internal/project"
)

type conversionForm int

const (
	conversionFormNone conversionForm = iota
	conversionFormAdd
	conversionFormImport
	conversionFormGrid
)

type conversionField struct {
	label string
	input textinput.Model
}

type ConversionRow struct {
	ID          string
	Easting     float64
	Northing    float64
	Latitude    float64
	Longitude   float64
	Elevation   *float64
	Code        string
	Description string
	Output      geodesy.Coordinate
	Error       string
}

type ConversionState struct {
	Source     geodesy.System
	Target     geodesy.System
	SourceZone int
	TargetZone int
	Model      string
	GridPath   string
	Rows       []ConversionRow
	Selected   int
	form       conversionForm
	fields     []conversionField
	field      int
	picker     filepicker.Model
	picking    bool
	confirm    bool
	commitAll  bool
}

func newConversionState() ConversionState {
	return ConversionState{
		Source: geodesy.MGA94System, Target: geodesy.MGA2020System,
		SourceZone: 50, TargetZone: 50, Model: geodesy.Conformal,
	}
}

func (m *Model) enterConvert() {
	m.mode = ModeConvert
	m.convert.ensureSelection()
	m.lastErr = ""
	m.message = "coordinate conversion workspace; elevations are carried unchanged"
}

func (m Model) updateConvert(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		m.quitting = true
		return m, tea.Quit
	case "f1":
		m.openHelp("convert")
		return m, nil
	}
	if m.convert.form != conversionFormNone {
		return m.updateConversionForm(msg)
	}
	if m.convert.confirm {
		switch msg.String() {
		case "y", "Y", "enter":
			m.commitConvertedRows(m.convert.commitAll, true)
		case "n", "N", "esc":
			m.convert.confirm = false
			m.message = "conversion commit cancelled"
		}
		return m, nil
	}
	switch msg.String() {
	case "esc", "f4":
		m.mode = ModeMain
	case "s":
		if len(m.convert.Rows) > 0 {
			m.pendingAction = "conversion-source"
			return m, nil
		}
		m.convert.Source = nextConversionSystem(m.convert.Source)
		m.convert.Rows = nil
		m.message = "source changed; staged rows cleared"
	case "t":
		m.convert.Target = nextConversionSystem(m.convert.Target)
		m.convert.clearOutputs()
		m.message = "target changed; recalculate staged rows"
	case "m":
		if m.convert.Model == geodesy.Conformal {
			m.convert.Model = geodesy.ConformalDistortion
		} else {
			m.convert.Model = geodesy.Conformal
		}
		m.convert.clearOutputs()
		m.message = "transformation model set to " + m.convert.Model
	case "[":
		m.convert.SourceZone = max(46, m.convert.SourceZone-1)
		m.convert.clearOutputs()
	case "]":
		m.convert.SourceZone = min(59, m.convert.SourceZone+1)
		m.convert.clearOutputs()
	case ",":
		m.convert.TargetZone = max(46, m.convert.TargetZone-1)
		m.convert.clearOutputs()
	case ".":
		m.convert.TargetZone = min(59, m.convert.TargetZone+1)
		m.convert.clearOutputs()
	case "up":
		m.convert.Selected = max(0, m.convert.Selected-1)
	case "down":
		m.convert.Selected = min(max(0, len(m.convert.Rows)-1), m.convert.Selected+1)
	case "a":
		m.startConversionAdd()
	case "enter":
		if len(m.convert.Rows) > 0 {
			row := m.convert.Rows[m.convert.Selected]
			m.appendResult(commandResult{"Staged point details", fmt.Sprintf("ID: %s\nSource: %s\nResult: %s\nCode: %s\nDescription: %s\nError: %s", row.ID, row.sourceText(m.convert.Source), row.resultText(), row.Code, row.Description, row.Error)})
			m.openResults()
		}
	case "i":
		return m, m.startConversionPicker(conversionFormImport)
	case "g":
		return m, m.startConversionPicker(conversionFormGrid)
	case "x":
		m.removeConversionRow()
	case "r":
		if m.asyncJobs {
			m.queueConversion("")
			return m, nil
		}
		m.calculateConversions()
	case "c":
		m.commitConvertedRows(false, false)
	case "C":
		m.commitConvertedRows(true, false)
	}
	return m, nil
}

func (m Model) updateConversionForm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.convert.picking {
		return m.updateConversionPicker(msg)
	}
	switch msg.String() {
	case "esc":
		m.convert.form = conversionFormNone
		m.convert.fields = nil
		m.message = "conversion form cancelled"
		return m, nil
	case "tab":
		m.cycleConversionField(1)
		return m, nil
	case "shift+tab":
		m.cycleConversionField(-1)
		return m, nil
	case "enter":
		m.submitConversionForm()
		return m, nil
	}
	if len(m.convert.fields) == 0 {
		return m, nil
	}
	var cmd tea.Cmd
	field := &m.convert.fields[m.convert.field]
	field.input, cmd = field.input.Update(msg)
	return m, cmd
}

func (m *Model) startConversionPicker(form conversionForm) tea.Cmd {
	picker := filepicker.New()
	picker.CurrentDirectory = "."
	picker.ShowHidden = true
	picker.ShowPermissions = false
	picker.SetHeight(max(5, m.height-12))
	switch form {
	case conversionFormImport:
		picker.AllowedTypes = []string{".csv", ".CSV"}
		m.message = "select a conversion CSV file"
	case conversionFormGrid:
		picker.AllowedTypes = []string{".gsb", ".GSB"}
		if m.convert.GridPath != "" {
			picker.CurrentDirectory = filepath.Dir(m.convert.GridPath)
		}
		m.message = "select an official NTv2 .gsb grid file"
	}
	m.convert.form = form
	m.convert.picker = picker
	m.convert.picking = true
	m.lastErr = ""
	return m.convert.picker.Init()
}

func (m Model) updateConversionPicker(msg tea.Msg) (tea.Model, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch keyMsg.String() {
		case "ctrl+c":
			m.quitting = true
			return m, tea.Quit
		case "esc":
			m.convert.form = conversionFormNone
			m.convert.picking = false
			m.message = "file selection cancelled"
			return m, nil
		}
	}
	var cmd tea.Cmd
	m.convert.picker, cmd = m.convert.picker.Update(msg)
	if selected, path := m.convert.picker.DidSelectFile(msg); selected {
		switch m.convert.form {
		case conversionFormImport:
			if m.asyncJobs {
				m.convert.form = conversionFormNone
				m.convert.picking = false
				m.queueConversion(path)
				return m, nil
			}
			if err := m.importConversionCSV(path); err != nil {
				m.setError(err.Error())
				return m, cmd
			}
		case conversionFormGrid:
			m.convert.GridPath = path
			m.convert.clearOutputs()
			m.message = "NTv2 grid selected; recalculate staged rows"
		}
		m.convert.form = conversionFormNone
		m.convert.picking = false
		m.lastErr = ""
		return m, cmd
	}
	if selected, path := m.convert.picker.DidSelectDisabledFile(msg); selected {
		m.setError("unsupported file type " + strconv.Quote(filepath.Base(path)))
	}
	return m, cmd
}

func (m *Model) startConversionAdd() {
	labels := []string{"ID", "Easting", "Northing", "Elevation", "Code", "Description"}
	if isGeographicSystem(m.convert.Source) {
		labels[1], labels[2] = "Latitude", "Longitude"
	}
	m.startConversionFields(conversionFormAdd, labels, nil)
}

func (m *Model) startConversionFields(form conversionForm, labels, values []string) {
	m.convert.form = form
	m.convert.fields = nil
	m.convert.field = 0
	for i, label := range labels {
		input := textinput.New()
		if i < len(values) {
			input.SetValue(values[i])
		}
		if i == 0 {
			input.Focus()
		}
		m.convert.fields = append(m.convert.fields, conversionField{label: label, input: input})
	}
}

func (m *Model) cycleConversionField(step int) {
	if len(m.convert.fields) == 0 {
		return
	}
	m.convert.fields[m.convert.field].input.Blur()
	m.convert.field = (m.convert.field + step + len(m.convert.fields)) % len(m.convert.fields)
	m.convert.fields[m.convert.field].input.Focus()
}

func (m *Model) submitConversionForm() {
	switch m.convert.form {
	case conversionFormAdd:
		values := m.conversionValues()
		row, err := conversionRowFromFields(values, m.convert.Source)
		if err != nil {
			m.setError(err.Error())
			return
		}
		if m.convert.hasID(row.ID) {
			m.setError("staged point " + strconv.Quote(row.ID) + " already exists")
			return
		}
		m.convert.Rows = append(m.convert.Rows, row)
		m.convert.Selected = len(m.convert.Rows) - 1
		m.message = "staged point " + row.ID
	case conversionFormImport:
	case conversionFormGrid:
	}
	m.lastErr = ""
	m.convert.form = conversionFormNone
	m.convert.fields = nil
}

func (m Model) conversionValues() []string {
	values := make([]string, len(m.convert.fields))
	for i, field := range m.convert.fields {
		values[i] = strings.TrimSpace(field.input.Value())
	}
	return values
}

func conversionRowFromFields(values []string, system geodesy.System) (ConversionRow, error) {
	if len(values) != 6 || values[0] == "" {
		return ConversionRow{}, fmt.Errorf("point ID is required")
	}
	var elevation *float64
	if values[3] != "" {
		value, err := strconv.ParseFloat(values[3], 64)
		if err != nil {
			return ConversionRow{}, fmt.Errorf("invalid elevation %q", values[3])
		}
		elevation = &value
	}
	row := ConversionRow{ID: values[0], Elevation: elevation, Code: values[4], Description: values[5]}
	if isGeographicSystem(system) {
		var err error
		row.Latitude, err = geodesy.ParseLatitude(values[1])
		if err != nil {
			return ConversionRow{}, err
		}
		row.Longitude, err = geodesy.ParseLongitude(values[2])
		if err != nil {
			return ConversionRow{}, err
		}
	} else {
		var err error
		row.Easting, err = strconv.ParseFloat(values[1], 64)
		if err != nil {
			return ConversionRow{}, fmt.Errorf("invalid coordinate %q", values[1])
		}
		row.Northing, err = strconv.ParseFloat(values[2], 64)
		if err != nil {
			return ConversionRow{}, fmt.Errorf("invalid coordinate %q", values[2])
		}
	}
	return row, nil
}

func (m *Model) importConversionCSV(path string) error {
	if path == "" {
		return fmt.Errorf("CSV path is required")
	}
	path = paths.CSV(path)
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	reader := csv.NewReader(f)
	reader.FieldsPerRecord = 6
	header, err := reader.Read()
	if err != nil {
		return err
	}
	expected := []string{"id", "easting", "northing", "elevation", "code", "description"}
	if isGeographicSystem(m.convert.Source) {
		expected[1], expected[2] = "latitude", "longitude"
	}
	if strings.Join(header, ",") != strings.Join(expected, ",") {
		return fmt.Errorf("invalid conversion CSV header: expected %q", strings.Join(expected, ","))
	}
	added := 0
	for line := 2; ; line++ {
		values, err := reader.Read()
		if err != nil {
			if err == io.EOF {
				break
			}
			return err
		}
		row, parseErr := conversionRowFromFields(values, m.convert.Source)
		if parseErr != nil {
			row = ConversionRow{ID: values[0], Error: fmt.Sprintf("row %d: %v", line, parseErr)}
		}
		if row.ID == "" {
			row.ID = fmt.Sprintf("row-%d", line)
		}
		if m.convert.hasID(row.ID) {
			row.Error = "duplicate staged point ID"
		}
		m.convert.Rows = append(m.convert.Rows, row)
		added++
	}
	m.convert.ensureSelection()
	m.message = fmt.Sprintf("staged %d rows from %s", added, path)
	return nil
}

func (m *Model) calculateConversions() {
	if len(m.convert.Rows) == 0 {
		m.setError("no staged points to calculate")
		return
	}
	var transformer geodesy.Transformer
	sourceDatum, _ := geodesy.DatumForSystem(m.convert.Source)
	targetDatum, _ := geodesy.DatumForSystem(m.convert.Target)
	if sourceDatum != targetDatum {
		if m.convert.GridPath == "" {
			m.setError("select an official NTv2 grid path with g before cross-datum conversion")
			return
		}
		grid, err := geodesy.LoadNTv2(m.convert.GridPath, m.convert.Model)
		if err != nil {
			m.setError(err.Error())
			return
		}
		transformer = grid
	}
	targetZone := m.convert.TargetZone
	if isGeographicSystem(m.convert.Source) && isGridSystem(m.convert.Target) {
		zone, err := conversionTargetZone(m.convert.Rows)
		if err != nil {
			m.setError(err.Error())
			return
		}
		targetZone = zone
		m.convert.TargetZone = zone
	}
	valid := 0
	for i := range m.convert.Rows {
		row := &m.convert.Rows[i]
		if row.Error != "" && row.Output.Grid == nil && row.Output.Geographic == nil {
			continue
		}
		row.Output = geodesy.Coordinate{}
		row.Error = ""
		output, err := geodesy.Convert(geodesy.Request{
			Source: row.source(m.convert.Source, m.convert.SourceZone),
			Target: m.convert.Target, TargetZone: targetZone, Transform: transformer,
		})
		if err != nil {
			row.Error = err.Error()
			continue
		}
		row.Output = output
		valid++
	}
	m.lastErr = ""
	m.message = fmt.Sprintf("calculated %d/%d rows; elevations unchanged", valid, len(m.convert.Rows))
}

func conversionTargetZone(rows []ConversionRow) (int, error) {
	zone := 0
	for _, row := range rows {
		if row.Error != "" {
			continue
		}
		candidate, err := geodesy.MGAZoneForLongitude(row.Longitude)
		if err != nil {
			return 0, err
		}
		if zone == 0 {
			zone = candidate
		} else if zone != candidate {
			return 0, fmt.Errorf("geographic input spans MGA zones %d and %d; calculate each zone as a separate batch", zone, candidate)
		}
	}
	if zone == 0 {
		return 0, fmt.Errorf("no valid geographic rows to derive an MGA zone")
	}
	return zone, nil
}

func (m *Model) commitConvertedRows(all, assumeExisting bool) {
	if !isGridSystem(m.convert.Target) {
		m.setError("only MGA results can be committed to the main project")
		return
	}
	rows := m.convert.Rows
	if !all {
		if len(rows) == 0 {
			m.setError("no staged point selected")
			return
		}
		rows = rows[m.convert.Selected : m.convert.Selected+1]
	}
	points := make([]geom.Point, 0, len(rows))
	for _, row := range rows {
		if row.Error != "" || row.Output.Grid == nil {
			m.setError("calculate valid MGA output before committing selected rows")
			return
		}
		points = append(points, geom.Point{
			ID: row.ID, Easting: row.Output.Grid.Easting, Northing: row.Output.Grid.Northing,
			Elevation: row.Output.Grid.Elevation, Code: row.Code, Description: row.Description,
		})
	}
	datum, _ := geodesy.DatumForSystem(m.convert.Target)
	sourceDatum, _ := geodesy.DatumForSystem(m.convert.Source)
	model := ""
	if sourceDatum != datum {
		model = m.convert.Model
	}
	request := app.ConversionCommit{
		Points: points, TargetCRS: project.HorizontalCRS{Datum: string(datum), Projection: "MGA", Zone: m.convert.TargetZone},
		SourceSystem: string(m.convert.Source), TargetSystem: string(m.convert.Target), Model: model,
		AssumeExisting: assumeExisting,
	}
	if m.asyncJobs {
		m.queueConversionCommit(request, all)
		return
	}
	outcome, err := m.session.CommitConvertedPoints(request)
	if err != nil {
		if strings.Contains(err.Error(), "confirm they are") && !assumeExisting {
			m.convert.confirm = true
			m.convert.commitAll = all
			m.message = err.Error() + "  y: confirm  n: cancel"
			return
		}
		m.setError(err.Error())
		return
	}
	m.syncSessionState()
	m.tableRevision++
	m.refreshCompletions()
	m.syncMainViewports()
	m.convert.confirm = false
	m.lastErr = ""
	m.message = outcome.Message
	if all {
		m.convert.Rows = nil
		m.convert.Selected = 0
		return
	}
	m.removeConversionRow()
	m.message = outcome.Message
}

func (m *Model) removeConversionRow() {
	if len(m.convert.Rows) == 0 {
		return
	}
	index := m.convert.Selected
	m.convert.Rows = append(m.convert.Rows[:index], m.convert.Rows[index+1:]...)
	m.convert.ensureSelection()
}

func (m Model) renderConvert(width, height int) string {
	if m.convert.form != conversionFormNone && !m.convert.picking {
		return m.renderConversionForm(width, height)
	}
	if m.convert.picking {
		title := "Select conversion CSV"
		if m.convert.form == conversionFormGrid {
			title = "Select official NTv2 grid"
		}
		body := fmt.Sprintf("Directory: %s\n\n%s\nEnter/right: open/select  arrows/j/k: move  left/backspace: parent  Esc: cancel",
			m.convert.picker.CurrentDirectory, m.convert.picker.View())
		if m.lastErr != "" {
			body += "\n" + errorStyle.Render(m.lastErr)
		}
		return box(title, body, width, height)
	}
	settings := fmt.Sprintf("Source: %-18s zone=%d   Target: %-18s zone=%s\nModel: %-25s  NTv2: %s\nAngles: dd.mmsshhhh or decimal d; S=-/S E=E; MGA zone auto from longitude; heights unchanged",
		m.convert.Source, m.convert.SourceZone, m.convert.Target,
		m.convert.targetZoneText(), m.convert.Model, quoteBlank(m.convert.GridPath))
	if height < 18 {
		return m.renderCompactConversion(width, height)
	}
	tableHeight := max(6, height-13)
	start := max(0, m.convert.Selected-(tableHeight-5))
	end := min(len(m.convert.Rows), start+tableHeight-4)
	rows := m.conversionTableRows(start, end)
	if width < 110 {
		rows = m.compactConversionRows(start, end, width-4)
	}
	if len(m.convert.Rows) == 0 {
		rows = append(rows, mutedStyle.Render("  No staged rows. Press a to add or i to import CSV."))
	}
	zoneActions := "[ ]: source zone  , .: target zone"
	if isGeographicSystem(m.convert.Source) && isGridSystem(m.convert.Target) {
		zoneActions = "target MGA zone: auto from longitude"
	}
	actions := "a: add  i: import CSV  g: grid file  s/t: source/target  m: model  " + zoneActions + "\nr: calculate  c: commit selected  C: commit all  x: remove  F4/Esc: close  F1: help"
	if m.convert.confirm {
		actions = "Existing project has no CRS. Confirm all existing coordinates use the target CRS?  y/Enter: confirm  n/Esc: cancel"
	}
	if m.convert.form != conversionFormNone {
		form := []string{"Enter: submit  Tab: next field  Esc: cancel"}
		for i, field := range m.convert.fields {
			marker := " "
			if i == m.convert.field {
				marker = ">"
			}
			form = append(form, fmt.Sprintf("%s %s: %s", marker, field.label, field.input.View()))
		}
		actions = strings.Join(form, "\n")
	}
	if m.lastErr != "" {
		actions += "\n" + errorStyle.Render(m.lastErr)
	} else if m.message != "" {
		actions += "\n" + m.message
	}
	return lipgloss.JoinVertical(lipgloss.Left,
		box("Coordinate Conversion", settings, width, 6),
		box("Staged Points", strings.Join(rows, "\n"), width, tableHeight),
		box("Actions", actions, width, max(5, height-6-tableHeight)),
	)
}

func (m Model) conversionTableRows(start, end int) []string {
	type cells struct {
		prefix, id, source, result, status string
	}
	table := []cells{{prefix: " ", id: "ID", source: "SOURCE", result: "RESULT", status: "STATUS"}}
	for i := start; i < end; i++ {
		row := m.convert.Rows[i]
		prefix := " "
		if i == m.convert.Selected {
			prefix = ">"
		}
		status := "staged"
		if row.Error != "" {
			status = row.Error
		} else if row.Output.Grid != nil || row.Output.Geographic != nil {
			status = "computed"
		}
		table = append(table, cells{
			prefix: prefix, id: row.ID, source: row.sourceText(m.convert.Source),
			result: row.resultText(), status: status,
		})
	}
	idWidth, sourceWidth, resultWidth := 2, lipgloss.Width("SOURCE"), lipgloss.Width("RESULT")
	for _, row := range table[1:] {
		idWidth = max(idWidth, lipgloss.Width(row.id))
		sourceWidth = max(sourceWidth, lipgloss.Width(row.source))
		resultWidth = max(resultWidth, lipgloss.Width(row.result))
	}
	idWidth = max(idWidth, 12)
	sourceWidth = max(sourceWidth, 30)
	resultWidth = max(resultWidth, 30)
	lines := make([]string, 0, len(table))
	for _, row := range table {
		lines = append(lines, row.prefix+" "+padCell(row.id, idWidth)+" "+padCell(row.source, sourceWidth)+" "+padCell(row.result, resultWidth)+" "+row.status)
	}
	return lines
}

func padCell(value string, width int) string {
	padding := width - lipgloss.Width(value)
	if padding <= 0 {
		return value
	}
	return value + strings.Repeat(" ", padding)
}

func (r ConversionRow) source(system geodesy.System, zone int) geodesy.Coordinate {
	datum, _ := geodesy.DatumForSystem(system)
	if isGeographicSystem(system) {
		return geodesy.Coordinate{Geographic: &geodesy.Geographic{Datum: datum, Latitude: r.Latitude, Longitude: r.Longitude, Elevation: r.Elevation}}
	}
	return geodesy.Coordinate{Grid: &geodesy.Grid{Datum: datum, Zone: zone, Easting: r.Easting, Northing: r.Northing, Elevation: r.Elevation}}
}

func (r ConversionRow) sourceText(system geodesy.System) string {
	if isGeographicSystem(system) {
		return geodesy.FormatLatitude(r.Latitude) + ", " + geodesy.FormatLongitude(r.Longitude)
	}
	return fmt.Sprintf("%.3f, %.3f", r.Easting, r.Northing)
}

func (r ConversionRow) resultText() string {
	if r.Output.Grid != nil {
		return fmt.Sprintf("%.3f, %.3f Z%d", r.Output.Grid.Easting, r.Output.Grid.Northing, r.Output.Grid.Zone)
	}
	if r.Output.Geographic != nil {
		return geodesy.FormatLatitude(r.Output.Geographic.Latitude) + ", " + geodesy.FormatLongitude(r.Output.Geographic.Longitude)
	}
	return "-"
}

func (s *ConversionState) ensureSelection() {
	if len(s.Rows) == 0 {
		s.Selected = 0
	} else {
		s.Selected = min(max(0, s.Selected), len(s.Rows)-1)
	}
}

func (s *ConversionState) clearOutputs() {
	for i := range s.Rows {
		s.Rows[i].Output = geodesy.Coordinate{}
		if !strings.HasPrefix(s.Rows[i].Error, "row ") && s.Rows[i].Error != "duplicate staged point ID" {
			s.Rows[i].Error = ""
		}
	}
}

func (s ConversionState) hasID(id string) bool {
	for _, row := range s.Rows {
		if row.ID == id {
			return true
		}
	}
	return false
}

func (s ConversionState) targetZoneText() string {
	if isGeographicSystem(s.Source) && isGridSystem(s.Target) {
		return "auto"
	}
	return strconv.Itoa(s.TargetZone)
}

func nextConversionSystem(system geodesy.System) geodesy.System {
	systems := []geodesy.System{geodesy.MGA94System, geodesy.MGA2020System, geodesy.GDA94Geographic, geodesy.GDA2020Geographic}
	for i, item := range systems {
		if item == system {
			return systems[(i+1)%len(systems)]
		}
	}
	return systems[0]
}

func isGeographicSystem(system geodesy.System) bool {
	return system == geodesy.GDA94Geographic || system == geodesy.GDA2020Geographic
}

func isGridSystem(system geodesy.System) bool {
	return system == geodesy.MGA94System || system == geodesy.MGA2020System
}
