package project

import (
	"os"
	"path/filepath"
	"testing"

	"lsurvey/internal/geom"
)

func TestProjectJSONRoundTripPreservesPointCode(t *testing.T) {
	p := New("test")
	p.AppVersion = "v1.2.3"
	p.Description = "Boundary survey"
	p.SetDisplayPrecision(4)
	p.GridGround = &GridGroundConversion{
		Mode:           "local_ground",
		GridSystem:     "MGA2020_ZONE50",
		AnchorPointID:  "1",
		AnchorEasting:  200,
		AnchorNorthing: 100,
		CSF:            0.9996,
	}
	z := 42.5
	p.Groups["BOUND"] = Group{ID: "BOUND", Layer: "BOUNDARIES", Color: 1}
	p.PointCodeStyles["PEG"] = "BOUND"
	p.Points["1"] = geom.Point{ID: "1", Northing: 100, Easting: 200, Elevation: &z, Code: "PEG", GroupID: "BOUND"}
	p.Features["L1"] = Feature{ID: "L1", Kind: FeatureLine, PointIDs: []string{"1", "1"}, Code: "BOUNDARY", TerrainRole: "ridge", GroupID: "BOUND"}
	p.ContourSets["C1"] = ContourSet{
		ID:               "C1",
		Interval:         1,
		Base:             0,
		Generation:       &ContourGenerationSpec{Interval: 1, BreaklineMode: "none", BoundaryCodes: []string{"BOUNDARY"}, Smooth: 1},
		Stale:            true,
		TriangleCount:    4,
		EffectiveMaxEdge: 12.5,
		Diagnostics:      []ContourDiagnostic{{Code: "long_edge", Message: "long edge"}},
		RawPolylines: []ContourPolyline{{
			ID:        "C1-0001",
			Elevation: 42,
			Vertices:  []ContourVertex{{Northing: 100, Easting: 200}, {Northing: 105, Easting: 205}},
		}},
		Polylines: []ContourPolyline{{
			ID:        "C1-0001",
			Elevation: 42,
			Vertices:  []ContourVertex{{Northing: 100, Easting: 200}, {Northing: 110, Easting: 210}},
		}},
	}
	p.AddHistory("pt add 1 100 200 42.5 PEG", "added point 1", []string{"point:1"}, nil)
	p.AddHistoryChange("undo", "undid pt edit 1 code=PEG", nil, []string{"point:1"}, map[string]string{"action": "undo"}, nil)

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
	if got.AppVersion != "v1.2.3" {
		t.Fatalf("app version=%q want v1.2.3", got.AppVersion)
	}
	if got.GridGround == nil || got.GridGround.Mode != "local_ground" || got.GridGround.GridSystem != "MGA2020_ZONE50" {
		t.Fatalf("grid ground metadata=%+v", got.GridGround)
	}
	if got.DisplayPrecision() != 4 {
		t.Fatalf("precision=%d want 4", got.DisplayPrecision())
	}
	if got.Features["L1"].Code != "BOUNDARY" {
		t.Fatalf("line code=%q want BOUNDARY", got.Features["L1"].Code)
	}
	if got.Features["L1"].TerrainRole != "ridge" {
		t.Fatalf("line terrain role=%q want ridge", got.Features["L1"].TerrainRole)
	}
	if got.Groups["BOUND"].Layer != "BOUNDARIES" || got.Points["1"].GroupID != "BOUND" || got.Features["L1"].GroupID != "BOUND" {
		t.Fatalf("group styling not preserved: groups=%+v point=%+v feature=%+v", got.Groups, got.Points["1"], got.Features["L1"])
	}
	if got.PointCodeStyles["PEG"] != "BOUND" {
		t.Fatalf("point code styles=%+v", got.PointCodeStyles)
	}
	if len(got.History) != 2 {
		t.Fatalf("history length=%d want 2", len(got.History))
	}
	if got.History[1].Updated[0] != "point:1" || len(got.History[1].Extra) == 0 {
		t.Fatalf("extended history=%+v", got.History[1])
	}
	if len(got.ContourSets["C1"].Polylines) != 1 {
		t.Fatalf("contours=%+v", got.ContourSets)
	}
	if got.ContourSets["C1"].Generation == nil || !got.ContourSets["C1"].Stale {
		t.Fatalf("contour generation metadata=%+v", got.ContourSets["C1"])
	}
	if got.ContourSets["C1"].TriangleCount != 4 || len(got.ContourSets["C1"].Diagnostics) != 1 || len(got.ContourSets["C1"].RawPolylines) != 1 {
		t.Fatalf("contour quality metadata=%+v", got.ContourSets["C1"])
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
	if got.AppVersion != "" {
		t.Fatalf("app version=%q want empty", got.AppVersion)
	}
	if got.GridGround != nil {
		t.Fatalf("old project should have no conversion metadata: %+v", got.GridGround)
	}
	if got.PointCodeStyles == nil {
		t.Fatal("old project should initialize point code styles")
	}
}

func TestLoadMigratesLegacyLinesToFeatures(t *testing.T) {
	path := filepath.Join(t.TempDir(), "old-lines.srv")
	data := []byte(`{"schema_version":5,"name":"old","points":{},"lines":{"L1":{"id":"L1","from":"1","to":"2","code":"BND"}},"history":[]}`)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if feature := got.Features["L1"]; feature.Kind != FeatureLine || feature.PointIDs[0] != "1" || feature.PointIDs[1] != "2" || feature.Code != "BND" {
		t.Fatalf("migrated feature=%+v", feature)
	}
	if got.LegacyLines != nil {
		t.Fatalf("legacy lines retained: %+v", got.LegacyLines)
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

func TestSortedPointsOrdersNumericIDsNaturallyBeforeTextIDs(t *testing.T) {
	p := New("test")
	for _, id := range []string{"10", "A", "2", "1", "02"} {
		p.Points[id] = geom.Point{ID: id}
	}

	points := p.SortedPoints()
	got := make([]string, len(points))
	for i, pt := range points {
		got[i] = pt.ID
	}
	want := []string{"1", "02", "2", "10", "A"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("ids=%v want %v", got, want)
		}
	}
}

func TestNextLineIDUsesHighestNumericLineID(t *testing.T) {
	p := New("test")
	p.Features["L1"] = Feature{ID: "L1"}
	p.Features["BOUND"] = Feature{ID: "BOUND"}
	p.Features["L12"] = Feature{ID: "L12"}

	if got := p.NextFeatureID(); got != "L13" {
		t.Fatalf("next line id=%q want L13", got)
	}
}

func TestSortedLinesOrdersMatchingNumericSuffixesNaturally(t *testing.T) {
	p := New("test")
	for _, id := range []string{"L10", "L2", "L1", "BOUND"} {
		p.Features[id] = Feature{ID: id}
	}

	lines := p.SortedFeatures()
	got := make([]string, len(lines))
	for i, line := range lines {
		got[i] = line.ID
	}
	want := []string{"BOUND", "L1", "L2", "L10"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("ids=%v want %v", got, want)
		}
	}
}
