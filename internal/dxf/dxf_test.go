package dxf

import (
	"strings"
	"testing"

	"lsurvey/internal/geom"
	"lsurvey/internal/project"
)

func TestWriteIncludesPointsLabelsLinesAndCodeLayers(t *testing.T) {
	p := project.New("test")
	p.Points["1"] = geom.Point{ID: "1", Northing: 100, Easting: 200, Code: "PEG"}
	p.Points["2"] = geom.Point{ID: "2", Northing: 110, Easting: 210}
	p.Lines["L1"] = project.Line{ID: "L1", From: "1", To: "2", Code: "BOUNDARY"}
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
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("DXF missing %q:\n%s", want, got)
		}
	}
}

func TestContourLabelPointUsesPolylineEnd(t *testing.T) {
	got, ok := contourLabelPoint([]project.ContourVertex{
		{Northing: 0, Easting: 0},
		{Northing: 0, Easting: 10},
		{Northing: 10, Easting: 10},
	})
	if !ok {
		t.Fatal("expected label point")
	}
	if got.Northing != 10 || got.Easting != 10 {
		t.Fatalf("point=%+v want final contour vertex", got)
	}
}
