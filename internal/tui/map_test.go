package tui

import (
	"strings"
	"testing"

	"lsurvey/internal/geom"
	"lsurvey/internal/project"
)

func TestRenderMapNoPoints(t *testing.T) {
	got := renderMap(project.New("test"), newMapState(), 60, 18, 3)
	if !strings.Contains(got, "No points to map") {
		t.Fatalf("map missing empty state:\n%s", got)
	}
}

func TestRenderMapOnePointCenteredAndLabeled(t *testing.T) {
	p := project.New("test")
	p.Points["1"] = geom.Point{ID: "1", Northing: 100, Easting: 200}
	got := renderMap(p, newMapState(), 60, 18, 3)
	for _, want := range []string{"*1", "labels=id"} {
		if !strings.Contains(got, want) {
			t.Fatalf("map missing %q:\n%s", want, got)
		}
	}
}

func TestRenderMapCodeLabelsFallBackToID(t *testing.T) {
	p := project.New("test")
	p.Points["1"] = geom.Point{ID: "1", Code: "PEG", Northing: 0, Easting: 0}
	p.Points["2"] = geom.Point{ID: "2", Northing: 10, Easting: 10}
	got := renderMap(p, MapState{ShowCodes: true, Zoom: 1}, 60, 18, 3)
	for _, want := range []string{"*PEG", "*2", "labels=code"} {
		if !strings.Contains(got, want) {
			t.Fatalf("map missing %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "*1") {
		t.Fatalf("coded point should display its code:\n%s", got)
	}
}

func TestMapLabelsUseLeftSideWhenRightSideIsBlocked(t *testing.T) {
	grid := newRuneGrid(8, 1, ' ')
	grid[0][7] = '*'
	if !drawMapLabel(grid, 7, 0, "EDGE") {
		t.Fatal("expected label to fit to the left")
	}
	if got := string(grid[0]); got != "   EDGE*" {
		t.Fatalf("grid=%q want left-placed label", got)
	}
}

func TestMapLabelsDoNotPartiallyRenderOrOverwritePointMarkers(t *testing.T) {
	grid := newRuneGrid(8, 1, ' ')
	grid[0][2] = '*'
	grid[0][5] = '*'
	if drawMapLabel(grid, 2, 0, "LONG") {
		t.Fatal("expected blocked label not to render")
	}
	if got := string(grid[0]); got != "  *  *  " {
		t.Fatalf("grid=%q want unchanged markers", got)
	}
}

func TestMapPointMarkersArePlacedBeforeLabels(t *testing.T) {
	p := project.New("test")
	p.Points["AAA"] = geom.Point{ID: "AAA", Northing: 0, Easting: 0}
	p.Points["B"] = geom.Point{ID: "B", Northing: 0, Easting: 1}
	got := renderMap(p, newMapState(), 60, 18, 3)
	if !strings.Contains(got, "*AAA") && !strings.Contains(got, "AAA*") {
		t.Fatalf("map missing point label:\n%s", got)
	}
	if !strings.Contains(got, "*B") && !strings.Contains(got, "B*") {
		t.Fatalf("second point marker/label should remain readable:\n%s", got)
	}
}

func TestVisibleBoundsZoom(t *testing.T) {
	fit := mapBounds{MinE: 0, MaxE: 100, MinN: 0, MaxN: 100}
	base := visibleBounds(fit, MapState{Zoom: 1}, 80, 20)
	zoomed := visibleBounds(fit, MapState{Zoom: 2, CenterE: 50, CenterN: 50, Custom: true}, 80, 20)
	if zoomed.MaxE-zoomed.MinE >= base.MaxE-base.MinE {
		t.Fatalf("zoomed span=%f base span=%f", zoomed.MaxE-zoomed.MinE, base.MaxE-base.MinE)
	}
}

func TestVisibleBoundsPreserveGroundScaleInTerminalCells(t *testing.T) {
	view := visibleBounds(mapBounds{MinE: 0, MaxE: 10, MinN: 0, MaxN: 10}, newMapState(), 80, 20)
	x0, y0, _ := mapCell(0, 0, view, 80, 20)
	x1, _, _ := mapCell(10, 0, view, 80, 20)
	_, y1, _ := mapCell(0, 10, view, 80, 20)
	eastCells := absInt(x1 - x0)
	northCells := absInt(y1 - y0)
	if absInt(eastCells-int(mapCellHeightRatio*float64(northCells))) > 2 {
		t.Fatalf("east cells=%d north cells=%d want equal displayed ground scale", eastCells, northCells)
	}
}

func TestMapLineOverlay(t *testing.T) {
	p := project.New("test")
	p.Points["1"] = geom.Point{ID: "1", Northing: 0, Easting: 0}
	p.Points["2"] = geom.Point{ID: "2", Northing: 0, Easting: 10}
	p.Features["L1"] = project.Feature{ID: "L1", Kind: project.FeatureLine, PointIDs: []string{"1", "2"}}
	got := renderMap(p, MapState{ShowLines: true, Zoom: 1}, 60, 18, 3)
	if !strings.Contains(got, ".") {
		t.Fatalf("map missing line overlay:\n%s", got)
	}
}

func TestMapContourOverlayIsOptInAndReportsStaleSets(t *testing.T) {
	p := project.New("test")
	p.Points["1"] = geom.Point{ID: "1", Northing: 0, Easting: 0}
	p.Points["2"] = geom.Point{ID: "2", Northing: 10, Easting: 10}
	p.ContourSets["C1"] = project.ContourSet{
		ID:    "C1",
		Stale: true,
		Polylines: []project.ContourPolyline{
			{Elevation: 1, Vertices: []project.ContourVertex{{Easting: 1, Northing: 2}, {Easting: 9, Northing: 2}}},
			{Elevation: 2, Index: true, Vertices: []project.ContourVertex{{Easting: 1, Northing: 8}, {Easting: 9, Northing: 8}}},
		},
	}
	hidden := renderMap(p, newMapState(), 120, 20, 3)
	if strings.Contains(hidden, "~") || strings.Contains(hidden, "contours=2") || strings.Contains(hidden, "stale=1") {
		t.Fatalf("map should hide contours by default:\n%s", hidden)
	}
	got := renderMap(p, MapState{ShowContours: true, Zoom: 1}, 120, 20, 3)
	for _, want := range []string{"~", "=", "contours=2", "stale=1", "c: contours"} {
		if !strings.Contains(got, want) {
			t.Fatalf("map missing %q:\n%s", want, got)
		}
	}
}

func TestMapDrawPriorityPreservesIndexContoursAndPoints(t *testing.T) {
	grid := newRuneGrid(3, 1, ' ')
	drawLineGlyph(grid, 0, 0, 2, 0, '.')
	drawLineGlyph(grid, 0, 0, 2, 0, '~')
	drawLineGlyph(grid, 0, 0, 2, 0, '=')
	grid[0][1] = '*'
	if got := string(grid[0]); got != "=*=" {
		t.Fatalf("grid=%q want index contour below point", got)
	}
}

func TestMapPointLabelOverridesContourOverlay(t *testing.T) {
	p := project.New("test")
	p.Points["1"] = geom.Point{ID: "1", Easting: 2, Northing: 5}
	p.Points["2"] = geom.Point{ID: "2", Easting: 10, Northing: 10}
	p.Points["3"] = geom.Point{ID: "3", Easting: 0, Northing: 0}
	p.ContourSets["C1"] = project.ContourSet{
		Polylines: []project.ContourPolyline{{
			Vertices: []project.ContourVertex{{Easting: 0, Northing: 5}, {Easting: 10, Northing: 5}},
		}},
	}
	got := renderMap(p, MapState{ShowContours: true, Zoom: 1}, 80, 20, 3)
	if !strings.Contains(got, "*1") {
		t.Fatalf("point label should remain readable above contour overlay:\n%s", got)
	}
}

func TestRenderMapSkipsPointsOutsideCustomView(t *testing.T) {
	p := project.New("test")
	p.Points["1"] = geom.Point{ID: "1", Northing: 0, Easting: 0}
	p.Points["2"] = geom.Point{ID: "2", Northing: 100, Easting: 100}
	state := MapState{Zoom: 10, CenterE: 0, CenterN: 0, Custom: true}
	got := renderMap(p, state, 60, 18, 3)
	if !strings.Contains(got, "*1") {
		t.Fatalf("map missing visible point:\n%s", got)
	}
	if strings.Contains(got, "*2") {
		t.Fatalf("map should not show off-screen point:\n%s", got)
	}
}

func TestRenderMapClipsLinesToCustomView(t *testing.T) {
	p := project.New("test")
	p.Points["1"] = geom.Point{ID: "1", Northing: 0, Easting: -100}
	p.Points["2"] = geom.Point{ID: "2", Northing: 0, Easting: 100}
	p.Features["L1"] = project.Feature{ID: "L1", Kind: project.FeatureLine, PointIDs: []string{"1", "2"}}
	state := MapState{ShowLines: true, Zoom: 10, CenterE: 0, CenterN: 0, Custom: true}
	got := renderMap(p, state, 60, 18, 3)
	if !strings.Contains(got, ".") {
		t.Fatalf("map should show clipped crossing line:\n%s", got)
	}
	if strings.Contains(got, "*1") || strings.Contains(got, "*2") {
		t.Fatalf("map should not clamp off-screen endpoints:\n%s", got)
	}
}

func TestMapStateFitAndZoom(t *testing.T) {
	p := project.New("test")
	p.Points["1"] = geom.Point{ID: "1", Northing: 0, Easting: 0}
	p.Points["2"] = geom.Point{ID: "2", Northing: 10, Easting: 10}
	state := newMapState()
	state.zoomBy(p, 1.5)
	if state.Zoom != 1.5 || !state.Custom {
		t.Fatalf("state=%+v", state)
	}
	state.fit()
	if state.Zoom != 1 || state.Custom {
		t.Fatalf("state=%+v", state)
	}
}

func TestMapStatePan(t *testing.T) {
	p := project.New("test")
	p.Points["1"] = geom.Point{ID: "1", Northing: 0, Easting: 0}
	p.Points["2"] = geom.Point{ID: "2", Northing: 100, Easting: 100}
	state := newMapState()
	state.panBy(p, 80, 20, 0.2, -0.2)
	if !state.Custom {
		t.Fatal("pan should create custom map view")
	}
	if state.CenterE <= 50 {
		t.Fatalf("center easting=%f want > 50", state.CenterE)
	}
	if state.CenterN >= 50 {
		t.Fatalf("center northing=%f want < 50", state.CenterN)
	}
}
