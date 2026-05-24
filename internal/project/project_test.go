package project

import (
	"os"
	"path/filepath"
	"testing"

	"lsurvey/internal/geom"
)

func TestProjectJSONRoundTripPreservesPointCode(t *testing.T) {
	p := New("test")
	p.Description = "Boundary survey"
	p.SetDisplayPrecision(4)
	z := 42.5
	p.Points["1"] = geom.Point{ID: "1", Northing: 100, Easting: 200, Elevation: &z, Code: "PEG"}
	p.Lines["L1"] = Line{ID: "L1", From: "1", To: "1", Code: "BOUNDARY"}
	p.ContourSets["C1"] = ContourSet{
		ID:       "C1",
		Interval: 1,
		Base:     0,
		Polylines: []ContourPolyline{{
			ID:        "C1-0001",
			Elevation: 42,
			Vertices:  []ContourVertex{{Northing: 100, Easting: 200}, {Northing: 110, Easting: 210}},
		}},
	}
	p.AddHistory("pt add 1 100 200 42.5 PEG", "added point 1", []string{"point:1"}, nil)

	path := filepath.Join(t.TempDir(), "project.lsurvey.json")
	if err := Save(path, p); err != nil {
		t.Fatal(err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.Points["1"].Code != "PEG" {
		t.Fatalf("code=%q want PEG", got.Points["1"].Code)
	}
	if got.Description != "Boundary survey" {
		t.Fatalf("description=%q want Boundary survey", got.Description)
	}
	if got.DisplayPrecision() != 4 {
		t.Fatalf("precision=%d want 4", got.DisplayPrecision())
	}
	if got.Lines["L1"].Code != "BOUNDARY" {
		t.Fatalf("line code=%q want BOUNDARY", got.Lines["L1"].Code)
	}
	if len(got.History) != 1 {
		t.Fatalf("history length=%d want 1", len(got.History))
	}
	if len(got.ContourSets["C1"].Polylines) != 1 {
		t.Fatalf("contours=%+v", got.ContourSets)
	}
}

func TestLoadMigratesSchemaOneProject(t *testing.T) {
	path := filepath.Join(t.TempDir(), "old.srv")
	data := []byte(`{
  "schema_version": 1,
  "name": "old",
  "display": {"precision": 3},
  "units": {"angle": "dd.mmss", "distance": "m"},
  "points": {},
  "lines": {},
  "history": []
}`)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.SchemaVersion != CurrentSchemaVersion {
		t.Fatalf("schema=%d want %d", got.SchemaVersion, CurrentSchemaVersion)
	}
	if got.ContourSets == nil {
		t.Fatal("contour sets should be initialized")
	}
}

func TestNextPointIDUsesHighestNumericPointID(t *testing.T) {
	p := New("test")
	p.Points["1"] = geom.Point{ID: "1"}
	p.Points["A"] = geom.Point{ID: "A"}
	p.Points["12"] = geom.Point{ID: "12"}

	if got := p.NextPointID(); got != "13" {
		t.Fatalf("next point id=%q want 13", got)
	}
}

func TestNextLineIDUsesHighestNumericLineID(t *testing.T) {
	p := New("test")
	p.Lines["L1"] = Line{ID: "L1"}
	p.Lines["BOUND"] = Line{ID: "BOUND"}
	p.Lines["L12"] = Line{ID: "L12"}

	if got := p.NextLineID(); got != "L13" {
		t.Fatalf("next line id=%q want L13", got)
	}
}
