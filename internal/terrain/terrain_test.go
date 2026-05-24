package terrain

import (
	"math"
	"strings"
	"testing"

	"lsurvey/internal/geom"
	"lsurvey/internal/project"
)

func TestGenerateContoursFromElevatedPoints(t *testing.T) {
	p := project.New("test")
	z0, z10 := 0.0, 10.0
	p.Points["1"] = geom.Point{ID: "1", Northing: 0, Easting: 0, Elevation: &z0}
	p.Points["2"] = geom.Point{ID: "2", Northing: 0, Easting: 10, Elevation: &z10}
	p.Points["3"] = geom.Point{ID: "3", Northing: 10, Easting: 0, Elevation: &z10}
	set, err := Generate(p, Options{ID: "C1", Interval: 5})
	if err != nil {
		t.Fatal(err)
	}
	if set.ID != "C1" || set.Interval != 5 {
		t.Fatalf("set=%+v", set)
	}
	if len(set.Polylines) == 0 {
		t.Fatal("expected contour polylines")
	}
	var found bool
	for _, pl := range set.Polylines {
		if pl.Elevation == 5 && len(pl.Vertices) >= 2 {
			found = true
		}
	}
	if !found {
		t.Fatalf("missing elevation 5 contour: %+v", set.Polylines)
	}
}

func TestGenerateContoursDefaultsWholeNumberLevelsToIndex(t *testing.T) {
	p := project.New("test")
	z0, z2 := 0.0, 2.0
	p.Points["1"] = geom.Point{ID: "1", Northing: 0, Easting: 0, Elevation: &z0}
	p.Points["2"] = geom.Point{ID: "2", Northing: 0, Easting: 10, Elevation: &z2}
	p.Points["3"] = geom.Point{ID: "3", Northing: 10, Easting: 0, Elevation: &z2}
	set, err := Generate(p, Options{ID: "C1", Interval: 0.5})
	if err != nil {
		t.Fatal(err)
	}

	want := map[float64]bool{
		0.5: false,
		1.0: true,
		1.5: false,
	}
	for level, index := range want {
		found := false
		for _, pl := range set.Polylines {
			if pl.Elevation == level {
				found = true
				if pl.Index != index {
					t.Fatalf("level %.1f index=%v want %v", level, pl.Index, index)
				}
			}
		}
		if !found {
			t.Fatalf("missing level %.1f in %+v", level, set.Polylines)
		}
	}
}

func TestGenerateContoursExplicitIndexZeroDisablesIndexLevels(t *testing.T) {
	p := project.New("test")
	z0, z2 := 0.0, 2.0
	p.Points["1"] = geom.Point{ID: "1", Northing: 0, Easting: 0, Elevation: &z0}
	p.Points["2"] = geom.Point{ID: "2", Northing: 0, Easting: 10, Elevation: &z2}
	p.Points["3"] = geom.Point{ID: "3", Northing: 10, Easting: 0, Elevation: &z2}
	set, err := Generate(p, Options{ID: "C1", Interval: 0.5, IndexEverySet: true})
	if err != nil {
		t.Fatal(err)
	}
	for _, pl := range set.Polylines {
		if pl.Index {
			t.Fatalf("level %.1f index=true, want disabled", pl.Elevation)
		}
	}
}

func TestGenerateContoursValidatesBreaklineElevations(t *testing.T) {
	p := project.New("test")
	z0, z10 := 0.0, 10.0
	p.Points["1"] = geom.Point{ID: "1", Northing: 0, Easting: 0, Elevation: &z0}
	p.Points["2"] = geom.Point{ID: "2", Northing: 0, Easting: 10}
	p.Points["3"] = geom.Point{ID: "3", Northing: 10, Easting: 0, Elevation: &z10}
	p.Lines["B1"] = project.Line{ID: "B1", From: "1", To: "2"}
	if _, err := Generate(p, Options{ID: "C1", Interval: 5, UseBreakline: true, BreaklineIDs: []string{"B1"}}); err == nil {
		t.Fatal("expected breakline elevation error")
	}
}

func TestGenerateContoursRejectsCrossingBreaklines(t *testing.T) {
	p := project.New("test")
	z := 1.0
	p.Points["1"] = geom.Point{ID: "1", Northing: 0, Easting: 0, Elevation: &z}
	p.Points["2"] = geom.Point{ID: "2", Northing: 10, Easting: 10, Elevation: &z}
	p.Points["3"] = geom.Point{ID: "3", Northing: 10, Easting: 0, Elevation: &z}
	p.Points["4"] = geom.Point{ID: "4", Northing: 0, Easting: 10, Elevation: &z}
	p.Lines["B1"] = project.Line{ID: "B1", From: "1", To: "2"}
	p.Lines["B2"] = project.Line{ID: "B2", From: "3", To: "4"}
	if _, err := Generate(p, Options{ID: "C1", Interval: 1, UseBreakline: true}); err == nil {
		t.Fatal("expected crossing breakline error")
	}
}

func TestInsertConstraintEdgesRecoversBreakline(t *testing.T) {
	points := []vertex{
		{ID: "1", N: 0, E: 0, Z: 0},
		{ID: "2", N: 0, E: 10, Z: 10},
		{ID: "3", N: 10, E: 0, Z: 10},
		{ID: "4", N: 10, E: 10, Z: 20},
	}
	tris, err := triangulate(points)
	if err != nil {
		t.Fatal(err)
	}
	target := normEdge(0, 3)
	if hasEdge(tris, target) {
		t.Skip("delaunay already picked target diagonal")
	}
	got, err := insertConstraintEdges(points, tris, []breakline{{ID: "B1", A: 0, B: 3}})
	if err != nil {
		t.Fatal(err)
	}
	if !hasEdge(got, target) {
		t.Fatalf("missing recovered breakline edge in %+v", got)
	}
}

func TestGenerateContoursClipsToTwoDimensionalBoundary(t *testing.T) {
	p := planarContourProject()
	addRing(p, "B", "BOUND", []geom.Point{
		{ID: "B1", Easting: 0, Northing: 2},
		{ID: "B2", Easting: 10, Northing: 2},
		{ID: "B3", Easting: 10, Northing: 8},
		{ID: "B4", Easting: 0, Northing: 8},
	})
	set, err := Generate(p, Options{ID: "C1", Interval: 5, BoundaryCodes: []string{"BOUND"}})
	if err != nil {
		t.Fatal(err)
	}
	vertices := levelVertices(set, 5)
	if len(vertices) < 2 {
		t.Fatalf("level 5 vertices=%+v", vertices)
	}
	for _, v := range vertices {
		if v.Northing < 2-1e-7 || v.Northing > 8+1e-7 {
			t.Fatalf("boundary failed to clip vertex %+v", v)
		}
	}
	if len(set.BoundaryLines) != 4 {
		t.Fatalf("boundary lines=%v", set.BoundaryLines)
	}
}

func TestGenerateContoursRemovesExclusionRing(t *testing.T) {
	p := planarContourProject()
	addRing(p, "X", "VOID", []geom.Point{
		{ID: "X1", Easting: 4, Northing: 4},
		{ID: "X2", Easting: 6, Northing: 4},
		{ID: "X3", Easting: 6, Northing: 6},
		{ID: "X4", Easting: 4, Northing: 6},
	})
	set, err := Generate(p, Options{ID: "C1", Interval: 5, ExclusionCodes: []string{"VOID"}})
	if err != nil {
		t.Fatal(err)
	}
	found := 0
	for _, pl := range set.Polylines {
		if pl.Elevation != 5 {
			continue
		}
		found++
		for _, v := range pl.Vertices {
			if v.Northing > 4+1e-7 && v.Northing < 6-1e-7 {
				t.Fatalf("exclusion retained vertex %+v", v)
			}
		}
	}
	if found != 2 {
		t.Fatalf("level 5 polylines=%d want 2: %+v", found, set.Polylines)
	}
}

func TestGenerateContoursRejectsOpenBoundaryLinework(t *testing.T) {
	p := planarContourProject()
	p.Points["B1"] = geom.Point{ID: "B1", Easting: 0, Northing: 2}
	p.Points["B2"] = geom.Point{ID: "B2", Easting: 10, Northing: 2}
	p.Lines["B1"] = project.Line{ID: "B1", From: "B1", To: "B2", Code: "BOUND"}
	if _, err := Generate(p, Options{ID: "C1", Interval: 5, BoundaryCodes: []string{"BOUND"}}); err == nil {
		t.Fatal("expected open boundary error")
	}
}

func TestGenerateContoursDeduplicatesEqualElevationPointsWithWarning(t *testing.T) {
	p := planarContourProject()
	z0 := 0.0
	p.Points["0"] = geom.Point{ID: "0", Easting: 0, Northing: 0, Elevation: &z0}
	set, err := Generate(p, Options{ID: "C1", Interval: 5})
	if err != nil {
		t.Fatal(err)
	}
	if len(set.SourcePoints) != 4 {
		t.Fatalf("source points=%v want four unique XY points", set.SourcePoints)
	}
	if !hasDiagnostic(set, "duplicate_xy") {
		t.Fatalf("diagnostics=%+v want duplicate_xy", set.Diagnostics)
	}
}

func TestGenerateContoursWarnsForLongEdgesUsingExplicitThreshold(t *testing.T) {
	p := planarContourProject()
	maxEdge := 5.0
	set, err := Generate(p, Options{ID: "C1", Interval: 5, MaxEdge: &maxEdge})
	if err != nil {
		t.Fatal(err)
	}
	if set.EffectiveMaxEdge != 5 || !hasDiagnostic(set, "long_edge") {
		t.Fatalf("maxedge=%f diagnostics=%+v", set.EffectiveMaxEdge, set.Diagnostics)
	}
	if set.Generation == nil || set.Generation.MaxEdge == nil || *set.Generation.MaxEdge != 5 {
		t.Fatalf("generation=%+v", set.Generation)
	}
}

func TestAutomaticMaximumEdgeThresholdAndWarningsAreDeterministic(t *testing.T) {
	points := []vertex{
		{ID: "1", E: 0, N: 0},
		{ID: "2", E: 1, N: 0},
		{ID: "3", E: 10, N: 0},
	}
	if got := maximumEdgeThreshold(points, nil); got != 5 {
		t.Fatalf("threshold=%f want 5", got)
	}
	got := longEdgeDiagnostics(points, []triangle{{A: 0, B: 1, C: 2}}, 5)
	if len(got) != 2 || strings.Join(got[0].EdgeIDs, "-") != "1-3" || strings.Join(got[1].EdgeIDs, "-") != "2-3" {
		t.Fatalf("diagnostics=%+v want stable long-edge order", got)
	}
}

func TestGenerateContoursWarnsAtHullUnlessBoundarySpecified(t *testing.T) {
	without, err := Generate(planarContourProject(), Options{ID: "C1", Interval: 5})
	if err != nil {
		t.Fatal(err)
	}
	if !hasDiagnostic(without, "hull_contact") {
		t.Fatalf("diagnostics=%+v want hull_contact", without.Diagnostics)
	}

	p := planarContourProject()
	addRing(p, "B", "BOUND", []geom.Point{
		{ID: "B1", Easting: 1, Northing: 1},
		{ID: "B2", Easting: 9, Northing: 1},
		{ID: "B3", Easting: 9, Northing: 9},
		{ID: "B4", Easting: 1, Northing: 9},
	})
	withBoundary, err := Generate(p, Options{ID: "C2", Interval: 5, BoundaryCodes: []string{"BOUND"}})
	if err != nil {
		t.Fatal(err)
	}
	if hasDiagnostic(withBoundary, "hull_contact") {
		t.Fatalf("diagnostics=%+v should suppress hull warning", withBoundary.Diagnostics)
	}
}

func TestGenerateContoursStoresRawGeometryWhenSmoothingEnabled(t *testing.T) {
	p := project.New("test")
	z0, z5, z10 := 0.0, 5.0, 10.0
	p.Points["1"] = geom.Point{ID: "1", Easting: 0, Northing: 0, Elevation: &z0}
	p.Points["2"] = geom.Point{ID: "2", Easting: 10, Northing: 0, Elevation: &z10}
	p.Points["3"] = geom.Point{ID: "3", Easting: 0, Northing: 10, Elevation: &z10}
	p.Points["4"] = geom.Point{ID: "4", Easting: 5, Northing: 5, Elevation: &z5}
	set, err := Generate(p, Options{ID: "C1", Interval: 2.5, Smooth: 1})
	if err != nil {
		t.Fatal(err)
	}
	if set.Generation == nil || set.Generation.Smooth != 1 || len(set.RawPolylines) == 0 {
		t.Fatalf("smoothed set=%+v", set)
	}
	foundExpanded := false
	for i := range set.Polylines {
		if len(set.Polylines[i].Vertices) > len(set.RawPolylines[i].Vertices) {
			foundExpanded = true
		}
	}
	if !foundExpanded {
		t.Fatalf("polylines=%+v raw=%+v want smoothed vertex expansion", set.Polylines, set.RawPolylines)
	}
}

func TestClipSegmentsHonorsConcaveBoundaryAndExclusionEdgePolicy(t *testing.T) {
	boundary := []project.ContourVertex{
		{Easting: 0, Northing: 0}, {Easting: 10, Northing: 0}, {Easting: 10, Northing: 10},
		{Easting: 6, Northing: 10}, {Easting: 6, Northing: 4}, {Easting: 4, Northing: 4},
		{Easting: 4, Northing: 10}, {Easting: 0, Northing: 10},
	}
	parts := clipSegments([]segment{{
		Level: 5, A: project.ContourVertex{Easting: 0, Northing: 5}, B: project.ContourVertex{Easting: 10, Northing: 5},
	}}, clipRegions{Boundary: boundary})
	if len(parts) != 2 || parts[0].B.Easting != 4 || parts[1].A.Easting != 6 {
		t.Fatalf("concave boundary parts=%+v want gap from 4 to 6", parts)
	}
	edge := clipSegments([]segment{{
		Level: 5, A: project.ContourVertex{Easting: 0, Northing: 0}, B: project.ContourVertex{Easting: 10, Northing: 0},
	}}, clipRegions{Boundary: boundary})
	if len(edge) != 1 {
		t.Fatalf("outer boundary edge parts=%+v want retained", edge)
	}
	exclusion := []project.ContourVertex{{Easting: 2, Northing: 0}, {Easting: 8, Northing: 0}, {Easting: 8, Northing: 2}, {Easting: 2, Northing: 2}}
	removed := clipSegments([]segment{{
		Level: 5, A: project.ContourVertex{Easting: 2, Northing: 0}, B: project.ContourVertex{Easting: 8, Northing: 0},
	}}, clipRegions{Exclusions: [][]project.ContourVertex{exclusion}})
	if len(removed) != 0 {
		t.Fatalf("exclusion edge parts=%+v want removed", removed)
	}
}

func TestClipSegmentsTreatsMultipleAndNestedExclusionsAsUnion(t *testing.T) {
	outer := []project.ContourVertex{{Easting: 2, Northing: -1}, {Easting: 8, Northing: -1}, {Easting: 8, Northing: 1}, {Easting: 2, Northing: 1}}
	inner := []project.ContourVertex{{Easting: 4, Northing: -0.5}, {Easting: 6, Northing: -0.5}, {Easting: 6, Northing: 0.5}, {Easting: 4, Northing: 0.5}}
	separate := []project.ContourVertex{{Easting: 12, Northing: -1}, {Easting: 14, Northing: -1}, {Easting: 14, Northing: 1}, {Easting: 12, Northing: 1}}
	got := clipSegments([]segment{{
		Level: 5, A: project.ContourVertex{Easting: 0}, B: project.ContourVertex{Easting: 16},
	}}, clipRegions{Exclusions: [][]project.ContourVertex{outer, inner, separate}})
	if len(got) != 3 || got[0].B.Easting != 2 || got[1].A.Easting != 8 || got[1].B.Easting != 12 || got[2].A.Easting != 14 {
		t.Fatalf("parts=%+v want union exclusion gaps", got)
	}
}

func TestDeduplicateSegmentsEmitsCoincidentContourEdgeOnce(t *testing.T) {
	segments := []segment{
		{Level: 5, A: project.ContourVertex{Easting: 0}, B: project.ContourVertex{Easting: 10}},
		{Level: 5, A: project.ContourVertex{Easting: 10}, B: project.ContourVertex{Easting: 0}},
	}
	if got := deduplicateSegments(segments); len(got) != 1 {
		t.Fatalf("segments=%+v want one", got)
	}
}

func TestFlatTriangleDoesNotCreateArtificialContourPerimeter(t *testing.T) {
	flat := []vertex{
		{E: 0, N: 0, Z: 5},
		{E: 10, N: 0, Z: 5},
		{E: 0, N: 10, Z: 5},
	}
	if got := triangleContourPoints(flat, 5); len(got) != 0 {
		t.Fatalf("flat triangle points=%+v want none", got)
	}
}

func TestJoinSegmentsPreservesClosedLoopAndDisjointContour(t *testing.T) {
	segments := []segment{
		{Level: 5, A: project.ContourVertex{Easting: 0, Northing: 0}, B: project.ContourVertex{Easting: 1, Northing: 0}},
		{Level: 5, A: project.ContourVertex{Easting: 1, Northing: 0}, B: project.ContourVertex{Easting: 1, Northing: 1}},
		{Level: 5, A: project.ContourVertex{Easting: 1, Northing: 1}, B: project.ContourVertex{Easting: 0, Northing: 1}},
		{Level: 5, A: project.ContourVertex{Easting: 0, Northing: 1}, B: project.ContourVertex{Easting: 0, Northing: 0}},
		{Level: 5, A: project.ContourVertex{Easting: 10, Northing: 0}, B: project.ContourVertex{Easting: 11, Northing: 0}},
	}
	got := joinSegments(segments)
	if len(got) != 2 {
		t.Fatalf("polylines=%+v want two", got)
	}
	if !sameVertex(got[0].Vertices[0], got[0].Vertices[len(got[0].Vertices)-1]) &&
		!sameVertex(got[1].Vertices[0], got[1].Vertices[len(got[1].Vertices)-1]) {
		t.Fatalf("polylines=%+v want one closed loop", got)
	}
}

func TestSmoothingLeavesBreaklineContactFixed(t *testing.T) {
	p := project.New("test")
	p.Points["A"] = geom.Point{ID: "A", Easting: 5, Northing: -5}
	p.Points["B"] = geom.Point{ID: "B", Easting: 5, Northing: 5}
	p.Lines["R1"] = project.Line{ID: "R1", From: "A", To: "B"}
	vertices := []project.ContourVertex{
		{Easting: 0, Northing: 0},
		{Easting: 5, Northing: 0},
		{Easting: 10, Northing: 0},
	}
	got := smoothVertices(vertices, 2, clipRegions{}, p, []breakline{{ID: "R1"}})
	if len(got) != len(vertices) || !sameVertex(got[1], vertices[1]) {
		t.Fatalf("smoothed=%+v want protected contact unchanged", got)
	}
}

func TestSmoothingClosedLoopHasNoFixedSeamAndStaysClosed(t *testing.T) {
	raw := []project.ContourVertex{
		{Easting: 0, Northing: 0}, {Easting: 10, Northing: 0},
		{Easting: 10, Northing: 10}, {Easting: 0, Northing: 10},
		{Easting: 0, Northing: 0},
	}
	got := smoothVertices(raw, 1, clipRegions{}, project.New("test"), nil)
	if !sameVertex(got[0], got[len(got)-1]) {
		t.Fatalf("smoothed=%+v want closed loop", got)
	}
	if sameVertex(got[0], raw[0]) || len(got) <= len(raw) {
		t.Fatalf("smoothed=%+v want cyclic smoothing without fixed seam", got)
	}
}

func TestSmoothingRevertsCandidateThatCrossesExclusion(t *testing.T) {
	raw := []project.ContourPolyline{{
		ID: "C1-0001",
		Vertices: []project.ContourVertex{
			{Easting: 2, Northing: 4}, {Easting: 2, Northing: 0}, {Easting: 8, Northing: 0},
		},
	}}
	exclusion := []project.ContourVertex{{Easting: 2.5, Northing: 0.2}, {Easting: 5.5, Northing: 0.2}, {Easting: 5.5, Northing: 2.5}, {Easting: 2.5, Northing: 2.5}}
	got, diagnostics := smoothPolylines(raw, 1, clipRegions{Exclusions: [][]project.ContourVertex{exclusion}}, project.New("test"), nil)
	if len(diagnostics) != 1 || diagnostics[0].Code != "smooth_reverted" || !sameVertex(got[0].Vertices[1], raw[0].Vertices[1]) {
		t.Fatalf("polylines=%+v diagnostics=%+v want raw fallback", got, diagnostics)
	}
}

func TestGenerateContoursHandlesDenseNonCrossingBreaklineNetworkDeterministically(t *testing.T) {
	p := project.New("test")
	for row := 0; row < 3; row++ {
		for col := 0; col < 3; col++ {
			id := string(rune('A' + row*3 + col))
			z := float64(col * 5)
			p.Points[id] = geom.Point{ID: id, Easting: float64(col * 5), Northing: float64(row * 5), Elevation: &z}
		}
	}
	for row := 0; row < 3; row++ {
		for col := 0; col < 2; col++ {
			from := string(rune('A' + row*3 + col))
			to := string(rune('A' + row*3 + col + 1))
			id := from + to
			p.Lines[id] = project.Line{ID: id, From: from, To: to}
		}
	}
	first, err := Generate(p, Options{ID: "C1", Interval: 2.5, UseBreakline: true})
	if err != nil {
		t.Fatal(err)
	}
	second, err := Generate(p, Options{ID: "C1", Interval: 2.5, UseBreakline: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Polylines) == 0 || len(first.Polylines) != len(second.Polylines) ||
		math.Abs(first.Polylines[0].Elevation-second.Polylines[0].Elevation) > eps {
		t.Fatalf("first=%+v second=%+v want deterministic contours", first.Polylines, second.Polylines)
	}
	if len(first.Breaklines) != len(p.Lines) {
		t.Fatalf("breaklines=%v want every selected constraint", first.Breaklines)
	}
	for id, line := range p.Lines {
		a, b := contourVertex(p, line.From), contourVertex(p, line.To)
		found := false
		for _, polyline := range first.Polylines {
			for _, v := range polyline.Vertices {
				if pointOnContourSegment(v, a, b) {
					found = true
					break
				}
			}
		}
		if !found {
			t.Fatalf("breakline %s has no retained contour contact in %+v", id, first.Polylines)
		}
	}
}

func planarContourProject() *project.Project {
	p := project.New("test")
	z0, z10 := 0.0, 10.0
	p.Points["1"] = geom.Point{ID: "1", Easting: 0, Northing: 0, Elevation: &z0}
	p.Points["2"] = geom.Point{ID: "2", Easting: 10, Northing: 0, Elevation: &z10}
	p.Points["3"] = geom.Point{ID: "3", Easting: 0, Northing: 10, Elevation: &z0}
	p.Points["4"] = geom.Point{ID: "4", Easting: 10, Northing: 10, Elevation: &z10}
	return p
}

func addRing(p *project.Project, prefix, code string, points []geom.Point) {
	for _, pt := range points {
		p.Points[pt.ID] = pt
	}
	for i := range points {
		id := prefix + string(rune('1'+i))
		p.Lines[id] = project.Line{ID: id, From: points[i].ID, To: points[(i+1)%len(points)].ID, Code: code}
	}
}

func levelVertices(set project.ContourSet, level float64) []project.ContourVertex {
	var out []project.ContourVertex
	for _, pl := range set.Polylines {
		if pl.Elevation == level {
			out = append(out, pl.Vertices...)
		}
	}
	return out
}

func hasDiagnostic(set project.ContourSet, code string) bool {
	for _, diagnostic := range set.Diagnostics {
		if diagnostic.Code == code || strings.Contains(diagnostic.Message, code) {
			return true
		}
	}
	return false
}
