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
	p.HorizontalCRS = &HorizontalCRS{Datum: "GDA2020", Projection: "MGA", Zone: 50}
	z := 42.5
	p.Groups["BOUND"] = Group{ID: "BOUND", Layer: "BOUNDARIES", Color: 1}
	p.PointCodeStyles["PEG"] = "BOUND"
	p.Points["1"] = geom.Point{ID: "1", Northing: 100, Easting: 200, Elevation: &z, Code: "PEG", GroupID: "BOUND"}
	p.Points["2"] = geom.Point{ID: "2", Easting: 210, Northing: 110}
	p.Features["L1"] = Feature{ID: "L1", Kind: FeatureLine, PointIDs: []string{"1", "2"}, Code: "BOUNDARY", TerrainRole: "ridge", GroupID: "BOUND"}
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
	p.AddHistoryChange("undo", "undid pt edit 1 code=PEG", nil, []string{"point:1"}, []string{"point:2"}, map[string]string{"action": "undo"}, nil)

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
	if got.HorizontalCRS == nil || got.HorizontalCRS.Label() != "MGA2020_ZONE50" {
		t.Fatalf("horizontal CRS metadata=%+v", got.HorizontalCRS)
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
	if got.History[1].Updated[0] != "point:1" || got.History[1].Deleted[0] != "point:2" || len(got.History[1].Extra) == 0 {
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

func TestProjectCloneDeepCopiesMutableFields(t *testing.T) {
	p := New("test")
	p.SetDisplayPrecision(4)
	z := 42.5
	base := 40.0
	maxEdge := 12.5
	p.Units["distance"] = "m"
	p.Points["1"] = geom.Point{ID: "1", Northing: 100, Easting: 200, Elevation: &z, Code: "PEG"}
	p.Groups["BOUND"] = Group{ID: "BOUND", Layer: "BOUNDARIES", Color: 1}
	p.PointCodeStyles["PEG"] = "BOUND"
	p.Features["L1"] = Feature{ID: "L1", Kind: FeaturePolyline, PointIDs: []string{"1", "2"}, Code: "BOUNDARY"}
	p.LegacyLines = map[string]Line{"OLD": {ID: "OLD", From: "1", To: "2"}}
	p.Traverse = &TraverseState{Start: "1", Current: "2", LegPointIDs: []string{"2"}}
	p.GridGround = &GridGroundConversion{Mode: "local_ground", AnchorEasting: 200, AnchorNorthing: 100, CSF: 0.9996}
	p.HorizontalCRS = &HorizontalCRS{Datum: "GDA2020", Projection: "MGA", Zone: 50}
	p.ContourSets["C1"] = ContourSet{
		ID:             "C1",
		SourcePoints:   []string{"1", "2", "3"},
		Breaklines:     []string{"L1"},
		BreaklineRoles: map[string]string{"L1": "ridge"},
		BoundaryLines:  []string{"B1"},
		ExclusionLines: []string{"X1"},
		Generation: &ContourGenerationSpec{
			Interval:       1,
			Base:           &base,
			BreaklineIDs:   []string{"L1"},
			BoundaryCodes:  []string{"BOUNDARY"},
			ExclusionCodes: []string{"VOID"},
			MaxEdge:        &maxEdge,
		},
		Diagnostics: []ContourDiagnostic{{Code: "long_edge", PointIDs: []string{"1", "2"}, EdgeIDs: []string{"L1"}}},
		RawPolylines: []ContourPolyline{{
			ID:       "raw",
			Vertices: []ContourVertex{{Easting: 200, Northing: 100}},
		}},
		Polylines: []ContourPolyline{{
			ID:       "smooth",
			Vertices: []ContourVertex{{Easting: 205, Northing: 105}},
		}},
	}
	p.History = []HistoryRecord{{
		Command: "pt add 1",
		Created: []string{"point:1"},
		Updated: []string{"project"},
		Deleted: []string{"point:2"},
		Extra:   []byte(`{"source":"test"}`),
	}}

	cloned := p.Clone()

	*cloned.Display.Precision = 2
	cloned.Units["distance"] = "ft"
	pt := cloned.Points["1"]
	*pt.Elevation = 99
	cloned.Points["1"] = pt
	cloned.Groups["BOUND"] = Group{ID: "BOUND", Layer: "CHANGED", Color: 2}
	cloned.PointCodeStyles["PEG"] = "CHANGED"
	feature := cloned.Features["L1"]
	feature.PointIDs[0] = "9"
	cloned.Features["L1"] = feature
	cloned.LegacyLines["OLD"] = Line{ID: "OLD", From: "9", To: "10"}
	cloned.Traverse.LegPointIDs[0] = "9"
	cloned.GridGround.AnchorEasting = 999
	cloned.HorizontalCRS.Zone = 51
	set := cloned.ContourSets["C1"]
	set.SourcePoints[0] = "9"
	set.Breaklines[0] = "L9"
	set.BreaklineRoles["L1"] = "drain"
	set.BoundaryLines[0] = "B9"
	set.ExclusionLines[0] = "X9"
	*set.Generation.Base = 44
	set.Generation.BreaklineIDs[0] = "L9"
	set.Generation.BoundaryCodes[0] = "ROAD"
	set.Generation.ExclusionCodes[0] = "TREE"
	*set.Generation.MaxEdge = 99
	set.Diagnostics[0].PointIDs[0] = "9"
	set.Diagnostics[0].EdgeIDs[0] = "L9"
	set.RawPolylines[0].Vertices[0].Easting = 999
	set.Polylines[0].Vertices[0].Northing = 999
	cloned.ContourSets["C1"] = set
	cloned.History[0].Created[0] = "point:9"
	cloned.History[0].Updated[0] = "changed"
	cloned.History[0].Deleted[0] = "point:10"
	cloned.History[0].Extra[0] = '['

	originalSet := p.ContourSets["C1"]
	if *p.Display.Precision != 4 || p.Units["distance"] != "m" || *p.Points["1"].Elevation != 42.5 {
		t.Fatalf("basic mutable fields shared: precision=%d units=%+v point=%+v", *p.Display.Precision, p.Units, p.Points["1"])
	}
	if p.Groups["BOUND"].Layer != "BOUNDARIES" || p.PointCodeStyles["PEG"] != "BOUND" || p.Features["L1"].PointIDs[0] != "1" {
		t.Fatalf("map values shared: groups=%+v styles=%+v features=%+v", p.Groups, p.PointCodeStyles, p.Features)
	}
	if p.LegacyLines["OLD"].From != "1" || p.Traverse.LegPointIDs[0] != "2" || p.GridGround.AnchorEasting != 200 || p.HorizontalCRS.Zone != 50 {
		t.Fatalf("struct pointers shared: legacy=%+v traverse=%+v grid=%+v crs=%+v", p.LegacyLines, p.Traverse, p.GridGround, p.HorizontalCRS)
	}
	if originalSet.SourcePoints[0] != "1" || originalSet.Breaklines[0] != "L1" || originalSet.BreaklineRoles["L1"] != "ridge" {
		t.Fatalf("contour references shared: %+v", originalSet)
	}
	if originalSet.BoundaryLines[0] != "B1" || originalSet.ExclusionLines[0] != "X1" {
		t.Fatalf("contour clip lines shared: %+v", originalSet)
	}
	if *originalSet.Generation.Base != 40 || originalSet.Generation.BreaklineIDs[0] != "L1" || originalSet.Generation.BoundaryCodes[0] != "BOUNDARY" || originalSet.Generation.ExclusionCodes[0] != "VOID" || *originalSet.Generation.MaxEdge != 12.5 {
		t.Fatalf("contour generation shared: %+v", originalSet.Generation)
	}
	if originalSet.Diagnostics[0].PointIDs[0] != "1" || originalSet.Diagnostics[0].EdgeIDs[0] != "L1" {
		t.Fatalf("contour diagnostics shared: %+v", originalSet.Diagnostics)
	}
	if originalSet.RawPolylines[0].Vertices[0].Easting != 200 || originalSet.Polylines[0].Vertices[0].Northing != 105 {
		t.Fatalf("contour polylines shared: %+v", originalSet)
	}
	if p.History[0].Created[0] != "point:1" || p.History[0].Updated[0] != "project" || p.History[0].Deleted[0] != "point:2" || string(p.History[0].Extra) != `{"source":"test"}` {
		t.Fatalf("history shared: %+v", p.History[0])
	}
}

func TestProjectClonePreservesNilFields(t *testing.T) {
	p := &Project{Name: "nil-heavy"}

	cloned := p.Clone()
	if cloned == nil {
		t.Fatal("clone is nil")
	}
	if cloned.Display.Precision != nil || cloned.Units != nil || cloned.Points != nil || cloned.Groups != nil || cloned.PointCodeStyles != nil || cloned.Features != nil || cloned.LegacyLines != nil || cloned.ContourSets != nil || cloned.History != nil {
		t.Fatalf("nil fields not preserved: %+v", cloned)
	}
	if cloned.Traverse != nil || cloned.GridGround != nil || cloned.HorizontalCRS != nil {
		t.Fatalf("nil pointers not preserved: %+v", cloned)
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
	if got.HorizontalCRS != nil {
		t.Fatalf("old project should have no horizontal CRS: %+v", got.HorizontalCRS)
	}
}

func TestLoadMigratesLegacyLinesToFeatures(t *testing.T) {
	path := filepath.Join(t.TempDir(), "old-lines.srv")
	data := []byte(`{"schema_version":5,"name":"old","points":{"1":{"id":"1"},"2":{"id":"2","easting":10}},"lines":{"L1":{"id":"L1","from":"1","to":"2","code":"BND"}},"history":[]}`)
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
