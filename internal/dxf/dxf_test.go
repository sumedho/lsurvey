package dxf

import (
	"strings"
	"testing"

	"lsurvey/internal/geom"
	"lsurvey/internal/project"
)

func TestWriteIncludesPointsLabelsLinesAndCodeLayers(t *testing.T) {
	p := project.New("test")
	p.SetDisplayPrecision(2)
	p.Points["1"] = geom.Point{ID: "1", Northing: 100, Easting: 200, Code: "PEG"}
	p.Points["2"] = geom.Point{ID: "2", Northing: 110, Easting: 210}
	p.Features["L1"] = project.Feature{ID: "L1", Kind: project.FeatureLine, PointIDs: []string{"1", "2"}, Code: "BOUNDARY"}
	p.ContourSets["C1"] = project.ContourSet{
		ID: "C1",
		Polylines: []project.ContourPolyline{
			{
				ID:        "C1-0001",
				Elevation: 100,
				Vertices:  []project.ContourVertex{{Northing: 90, Easting: 190}, {Northing: 100, Easting: 200}},
			},
			{
				ID:        "C1-0002",
				Elevation: 105,
				Index:     true,
				Vertices:  []project.ContourVertex{{Northing: 100, Easting: 200}, {Northing: 110, Easting: 210}},
			},
		},
	}

	var out strings.Builder
	if err := Write(&out, p); err != nil {
		t.Fatal(err)
	}
	got := out.String()
	for _, want := range []string{
		"9\n$ACADVER\n1\nAC1021\n",
		"0\nPOINT\n",
		"8\nPOINTS_PEG\n",
		"0\nTEXT\n",
		"1\n1 PEG\n",
		"0\nLINE\n",
		"8\nLINES_BOUNDARY\n",
		"62\n6\n",
		"10\n200\n20\n100\n",
		"0\nTABLE\n2\nLAYER\n",
		"0\nLAYER\n2\nCONTOURS\n70\n0\n62\n8\n6\nCONTINUOUS\n370\n5\n",
		"0\nLAYER\n2\nCONTOURS_INDEX\n70\n0\n62\n1\n6\nCONTINUOUS\n370\n13\n",
		"0\nLAYER\n2\nCONTOUR_LABELS\n70\n0\n62\n8\n6\nCONTINUOUS\n370\n5\n",
		"0\nLAYER\n2\nCONTOUR_LABELS_INDEX\n70\n0\n62\n1\n6\nCONTINUOUS\n370\n13\n",
		"0\nLAYER\n2\nLINE_LABELS\n70\n0\n62\n3\n6\nCONTINUOUS\n370\n0\n",
		"0\nLWPOLYLINE\n",
		"8\nCONTOURS\n",
		"62\n8\n",
		"370\n5\n",
		"90\n2\n",
		"38\n100\n",
		"43\n0\n",
		"8\nCONTOUR_LABELS\n",
		"1\n100\n",
		"8\nCONTOURS_INDEX\n",
		"62\n1\n",
		"370\n13\n",
		"38\n105\n",
		"43\n0.1\n",
		"8\nCONTOUR_LABELS_INDEX\n",
		"1\n105\n",
		"8\nLINE_LABELS\n",
		"0\nMTEXT\n",
		"1\n14.14\n",
		"1\n45\\U+00B0" + "00'00.00\"\n",
		"71\n5\n",
		"72\n1\n",
		"11\n0.7071067811865476\n21\n0.7071067811865475\n31\n0\n",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("DXF missing %q:\n%s", want, got)
		}
	}
}

func TestContourLabelUsesPathMidpointAndReadableRotation(t *testing.T) {
	got, _, ok := contourTextLabel(project.ContourPolyline{
		Elevation: 5,
		Vertices: []project.ContourVertex{
			{Northing: 0, Easting: 0},
			{Northing: 0, Easting: 6},
			{Northing: 10, Easting: 6},
		},
	})
	if !ok {
		t.Fatal("expected label point")
	}
	if got.Northing != 2 || got.Easting != 6 || got.Rotation != 90 {
		t.Fatalf("label=%+v want midpoint on vertical segment", got)
	}
	reversed, _, ok := contourTextLabel(project.ContourPolyline{
		Elevation: 5,
		Vertices:  []project.ContourVertex{{Easting: 10, Northing: 0}, {Easting: 0, Northing: -10}},
	})
	if !ok || reversed.Rotation != 45 {
		t.Fatalf("label=%+v want normalized readable rotation", reversed)
	}
}

func TestContourLabelRejectsShortPolyline(t *testing.T) {
	if _, _, ok := contourTextLabel(project.ContourPolyline{
		Elevation: 100,
		Vertices:  []project.ContourVertex{{Easting: 0}, {Easting: 1}},
	}); ok {
		t.Fatal("expected short contour to be unlabeled")
	}
}

func TestWriteSuppressesOverlappingContourLabels(t *testing.T) {
	p := project.New("test")
	p.ContourSets["C1"] = project.ContourSet{
		Polylines: []project.ContourPolyline{
			{ID: "1", Elevation: 10, Vertices: []project.ContourVertex{{Easting: 0}, {Easting: 20}}},
			{ID: "2", Elevation: 11, Vertices: []project.ContourVertex{{Easting: 0, Northing: 0.1}, {Easting: 20, Northing: 0.1}}},
		},
	}
	var out strings.Builder
	if err := Write(&out, p); err != nil {
		t.Fatal(err)
	}
	if got := strings.Count(out.String(), "8\nCONTOUR_LABELS\n"); got != 1 {
		t.Fatalf("label entities=%d want one:\n%s", got, out.String())
	}
}

func TestWriteUsesSmoothedPolylineRepresentationWhenRawIsRetained(t *testing.T) {
	p := project.New("test")
	p.ContourSets["C1"] = project.ContourSet{
		RawPolylines: []project.ContourPolyline{{
			Vertices: []project.ContourVertex{{Easting: 1, Northing: 1}, {Easting: 2, Northing: 2}},
		}},
		Polylines: []project.ContourPolyline{{
			Vertices: []project.ContourVertex{{Easting: 10, Northing: 10}, {Easting: 20, Northing: 20}},
		}},
	}
	var out strings.Builder
	if err := Write(&out, p); err != nil {
		t.Fatal(err)
	}
	got := out.String()
	if !strings.Contains(got, "10\n10\n20\n10\n") || strings.Contains(got, "10\n1\n20\n1\n") {
		t.Fatalf("DXF should use presentation polylines:\n%s", got)
	}
}

func TestWriteUsesGroupStyleForPolylineAndPolygon(t *testing.T) {
	p := project.New("test")
	p.Groups["BND"] = project.Group{ID: "BND", Layer: "LOT BOUNDARY", Color: 1}
	for id, xy := range map[string][2]float64{"1": {0, 0}, "2": {10, 0}, "3": {10, 10}} {
		p.Points[id] = geom.Point{ID: id, Easting: xy[0], Northing: xy[1]}
	}
	p.Features["P1"] = project.Feature{ID: "P1", Kind: project.FeaturePolyline, PointIDs: []string{"1", "2", "3"}, GroupID: "BND"}
	p.Features["A1"] = project.Feature{ID: "A1", Kind: project.FeaturePolygon, PointIDs: []string{"1", "2", "3"}, GroupID: "BND"}
	var out strings.Builder
	if err := Write(&out, p); err != nil {
		t.Fatal(err)
	}
	got := out.String()
	if !strings.Contains(got, "2\nLOT_BOUNDARY\n") || strings.Count(got, "8\nLOT_BOUNDARY\n62\n1\n") != 2 || !strings.Contains(got, "70\n1\n") {
		t.Fatalf("styled feature layers missing:\n%s", got)
	}
}

func TestWritePreservesElevatedPolylineVerticesAs3DPolyline(t *testing.T) {
	p := project.New("test")
	z := 4.5
	p.Points["1"] = geom.Point{ID: "1", Easting: 0, Northing: 0, Elevation: &z}
	p.Points["2"] = geom.Point{ID: "2", Easting: 10, Northing: 0}
	p.Features["P1"] = project.Feature{ID: "P1", Kind: project.FeaturePolyline, PointIDs: []string{"1", "2"}}
	var out strings.Builder
	if err := Write(&out, p); err != nil {
		t.Fatal(err)
	}
	got := out.String()
	if !strings.Contains(got, "0\nPOLYLINE\n") || !strings.Contains(got, "0\nVERTEX\n") || !strings.Contains(got, "30\n4.5\n") {
		t.Fatalf("3D polyline missing vertex elevation:\n%s", got)
	}
}

func TestLineAnnotationLabelsPlaceDistanceAndBearingOnOppositeSides(t *testing.T) {
	from := geom.Point{Easting: 0, Northing: 0}
	to := geom.Point{Easting: 10, Northing: 0}
	labels, ok := lineAnnotationLabels(from, to, 0, 0, geom.Inverse(from, to), 3)
	if !ok {
		t.Fatal("expected labels")
	}
	if len(labels) != 2 {
		t.Fatalf("labels=%d want 2", len(labels))
	}
	if labels[0].Value != "10.000" {
		t.Fatalf("distance label=%q want 10.000", labels[0].Value)
	}
	if labels[1].Value != "90°00′00.00″" {
		t.Fatalf("bearing label=%q", labels[1].Value)
	}
	if labels[0].Easting != 5 || labels[1].Easting != 5 {
		t.Fatalf("labels=%+v want midpoint easting", labels)
	}
	if labels[0].Northing != -1 || labels[1].Northing != 1 {
		t.Fatalf("labels=%+v want opposite-side offsets from midpoint", labels)
	}
	if labels[0].Rotation != 0 || labels[1].Rotation != 0 {
		t.Fatalf("labels=%+v want 0 degree rotation", labels)
	}
}

func TestEncodeDXFTextEscapesUnicodeSymbols(t *testing.T) {
	got := encodeDXFText("90°00′00.00″")
	want := "90\\U+00B0" + "00'00.00\""
	if got != want {
		t.Fatalf("encoded=%q want %q", got, want)
	}
}
