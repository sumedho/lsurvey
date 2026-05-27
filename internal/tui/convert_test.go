package tui

import (
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"lsurvey/internal/geodesy"
	"lsurvey/internal/geom"
	"lsurvey/internal/project"
)

func TestConversionScreenOpensByCommandAndF4(t *testing.T) {
	m := NewModel(project.New("test"), "")
	m.ExecuteCommand("convert")
	if m.mode != ModeConvert {
		t.Fatalf("mode=%v want conversion", m.mode)
	}
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyF4})
	m = updated.(Model)
	if m.mode != ModeMain {
		t.Fatalf("mode=%v want main", m.mode)
	}
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyF4})
	if got := updated.(Model).mode; got != ModeConvert {
		t.Fatalf("F4 mode=%v want conversion", got)
	}
}

func TestConversionScreenCalculatesAndCommitsProjectedRows(t *testing.T) {
	m := NewModel(project.New("test"), "")
	m.enterConvert()
	m.convert.Source = geodesy.MGA2020System
	m.convert.Target = geodesy.MGA2020System
	z := 10.0
	m.convert.Rows = []ConversionRow{{ID: "1", Easting: 500000, Northing: 6500000, Elevation: &z, Code: "PEG"}}
	m.calculateConversions()
	if m.lastErr != "" || m.convert.Rows[0].Output.Grid == nil {
		t.Fatalf("calculation error=%q row=%+v", m.lastErr, m.convert.Rows[0])
	}
	m.commitConvertedRows(true, false)
	if m.lastErr != "" || m.project.Points["1"].Easting != 500000 {
		t.Fatalf("commit error=%q point=%+v", m.lastErr, m.project.Points["1"])
	}
	if m.project.HorizontalCRS == nil || m.project.HorizontalCRS.Label() != "MGA2020_ZONE50" {
		t.Fatalf("CRS=%+v", m.project.HorizontalCRS)
	}
	if !strings.Contains(m.message, "elevations unchanged") {
		t.Fatalf("message=%q", m.message)
	}
}

func TestConversionScreenStagesGeographicCSVAndRequiresGridForDatumChange(t *testing.T) {
	path := filepath.Join(t.TempDir(), "points.csv")
	if err := os.WriteFile(path, []byte("id,latitude,longitude,elevation,code,description\n1,31.5700 S,115.513620 E,,PEG,corner\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	m := NewModel(project.New("test"), "")
	m.enterConvert()
	m.convert.Source = geodesy.GDA94Geographic
	m.convert.Target = geodesy.MGA2020System
	if err := m.importConversionCSV(path); err != nil {
		t.Fatal(err)
	}
	if len(m.convert.Rows) != 1 || math.Abs(m.convert.Rows[0].Latitude+31.95) > 1e-12 {
		t.Fatalf("rows=%+v", m.convert.Rows)
	}
	m.calculateConversions()
	if !strings.Contains(m.lastErr, "NTv2 grid") {
		t.Fatalf("error=%q", m.lastErr)
	}
}

func TestConversionGeographicEntryUsesCompactDMSAndExplicitDecimalDegrees(t *testing.T) {
	row, err := conversionRowFromFields([]string{"1", "-31.5700", "115.513620 E", "", "", ""}, geodesy.GDA2020Geographic)
	if err != nil {
		t.Fatal(err)
	}
	if math.Abs(row.Latitude+31.95) > 1e-12 || !strings.Contains(row.sourceText(geodesy.GDA2020Geographic), "31°57′00.00000″ S") {
		t.Fatalf("DMS row=%+v text=%q", row, row.sourceText(geodesy.GDA2020Geographic))
	}
	row, err = conversionRowFromFields([]string{"2", "-31.95d", "115.86d", "", "", ""}, geodesy.GDA2020Geographic)
	if err != nil || math.Abs(row.Latitude+31.95) > 1e-12 || math.Abs(row.Longitude-115.86) > 1e-12 {
		t.Fatalf("decimal row=%+v err=%v", row, err)
	}
}

func TestConversionGeographicToMGA2020DerivesZoneAndMatchesReference(t *testing.T) {
	m := NewModel(project.New("test"), "")
	m.enterConvert()
	m.convert.Source = geodesy.GDA2020Geographic
	m.convert.Target = geodesy.MGA2020System
	m.convert.TargetZone = 55
	row, err := conversionRowFromFields([]string{"1", "-31.5701", "115.5101", "", "", ""}, m.convert.Source)
	if err != nil {
		t.Fatal(err)
	}
	m.convert.Rows = []ConversionRow{row}
	m.calculateConversions()
	got := m.convert.Rows[0].Output.Grid
	if m.lastErr != "" || got == nil {
		t.Fatalf("error=%q row=%+v", m.lastErr, m.convert.Rows[0])
	}
	if got.Zone != 50 || math.Abs(got.Easting-391340.794) > 0.001 || math.Abs(got.Northing-6464498.654) > 0.001 {
		t.Fatalf("grid=%+v want zone 50 391340.794 E 6464498.654 N", got)
	}
}

func TestConversionGeographicBatchRejectsMultipleMGAZones(t *testing.T) {
	m := NewModel(project.New("test"), "")
	m.enterConvert()
	m.convert.Source = geodesy.GDA2020Geographic
	m.convert.Target = geodesy.MGA2020System
	m.convert.Rows = []ConversionRow{
		{ID: "west", Latitude: -31.95, Longitude: 115.85},
		{ID: "east", Latitude: -31.95, Longitude: 121.85},
	}
	m.calculateConversions()
	if !strings.Contains(m.lastErr, "spans MGA zones") {
		t.Fatalf("error=%q", m.lastErr)
	}
}

func TestConversionCommitConfirmsUnassignedExistingProjectCRS(t *testing.T) {
	p := project.New("test")
	p.Points["old"] = geom.Point{ID: "old", Easting: 1, Northing: 2}
	m := NewModel(p, "")
	m.enterConvert()
	m.convert.Source = geodesy.MGA2020System
	m.convert.Target = geodesy.MGA2020System
	m.convert.Rows = []ConversionRow{{ID: "new", Easting: 500000, Northing: 6500000}}
	m.calculateConversions()
	m.commitConvertedRows(true, false)
	if !m.convert.confirm {
		t.Fatal("expected CRS confirmation")
	}
	m.commitConvertedRows(true, true)
	if _, exists := m.project.Points["new"]; !exists || m.project.HorizontalCRS == nil {
		t.Fatalf("project=%+v", m.project)
	}
}

func TestConversionScreenRendersMethodAndHorizontalWarning(t *testing.T) {
	m := NewModel(project.New("test"), "")
	m.enterConvert()
	m.width, m.height = 120, 30
	view := m.View()
	for _, value := range []string{"Coordinate Conversion", "MGA94", "MGA2020", "conformal", "dd.mmsshhhh", "MGA zone auto", "heights unchanged"} {
		if !strings.Contains(view, value) {
			t.Fatalf("view missing %q:\n%s", value, view)
		}
	}
}

func TestConversionStagedTableAlignsCalculatedGeographicColumns(t *testing.T) {
	m := NewModel(project.New("test"), "")
	m.convert.Source = geodesy.MGA2020System
	m.convert.Target = geodesy.GDA2020Geographic
	row, err := conversionRowFromFields([]string{"1", "391340.794", "6464498.654", "", "", ""}, m.convert.Source)
	if err != nil {
		t.Fatal(err)
	}
	m.convert.Rows = []ConversionRow{row}
	m.calculateConversions()
	lines := m.conversionTableRows(0, 1)
	if len(lines) != 2 {
		t.Fatalf("rows=%q", lines)
	}
	for _, marker := range []string{"RESULT", "STATUS"} {
		headerColumn := lipgloss.Width(lines[0][:strings.Index(lines[0], marker)])
		value := "computed"
		if marker == "RESULT" {
			value = m.convert.Rows[0].resultText()
		}
		rowColumn := lipgloss.Width(lines[1][:strings.Index(lines[1], value)])
		if headerColumn != rowColumn {
			t.Fatalf("%s column header=%d row=%d\n%s\n%s", marker, headerColumn, rowColumn, lines[0], lines[1])
		}
	}
}

func TestConversionFileBrowserImportsCSVSelection(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "points.csv")
	if err := os.WriteFile(path, []byte("id,latitude,longitude,elevation,code,description\n1,31.5701 S,115.5101 E,,PEG,corner\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	m := NewModel(project.New("test"), "")
	m.enterConvert()
	m.convert.Source = geodesy.GDA2020Geographic
	m.startConversionPicker(conversionFormImport)
	m.convert.picker.CurrentDirectory = dir
	updated, _ := m.Update(m.convert.picker.Init()())
	m = updated.(Model)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)
	if m.convert.picking || len(m.convert.Rows) != 1 || m.convert.Rows[0].ID != "1" {
		t.Fatalf("picker=%v rows=%+v error=%q", m.convert.picking, m.convert.Rows, m.lastErr)
	}
}

func TestConversionFileBrowserSelectsGSBPath(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "GDA94_GDA2020_conformal.gsb")
	if err := os.WriteFile(path, []byte("grid"), 0o644); err != nil {
		t.Fatal(err)
	}
	m := NewModel(project.New("test"), "")
	m.enterConvert()
	m.startConversionPicker(conversionFormGrid)
	m.convert.picker.CurrentDirectory = dir
	updated, _ := m.Update(m.convert.picker.Init()())
	m = updated.(Model)
	if view := m.View(); !strings.Contains(view, "Select official NTv2 grid") || !strings.Contains(view, "GDA94_GDA2020_conformal.gsb") {
		t.Fatalf("view=%q", view)
	}
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)
	if m.convert.picking || m.convert.GridPath != path {
		t.Fatalf("picker=%v path=%q error=%q", m.convert.picking, m.convert.GridPath, m.lastErr)
	}
}
