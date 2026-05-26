package cogo

import (
	"math"
	"strings"
	"testing"

	"lsurvey/internal/geom"
	"lsurvey/internal/project"
)

func TestPointAddStoresCode(t *testing.T) {
	p := project.New("test")
	if _, err := Execute(p, "pt add 1 100 200 42.5 PEG"); err != nil {
		t.Fatal(err)
	}
	got := p.Points["1"]
	close(t, got.Easting, 100)
	close(t, got.Northing, 200)
	if got.Code != "PEG" {
		t.Fatalf("code=%q want PEG", got.Code)
	}
	if got.Elevation == nil || *got.Elevation != 42.5 {
		t.Fatalf("elevation not stored")
	}
}

func TestRadCommand(t *testing.T) {
	p := project.New("test")
	mustExec(t, p, "pt add 1 0 0 PEG")
	if _, err := Execute(p, "rad 1 90.0000 10 as 2 CALC"); err != nil {
		t.Fatal(err)
	}
	got := p.Points["2"]
	close(t, got.Northing, 0)
	close(t, got.Easting, 10)
	if got.Code != "CALC" {
		t.Fatalf("code=%q want CALC", got.Code)
	}
}

func TestRadUsesEastingNorthingPointInput(t *testing.T) {
	p := project.New("test")
	mustExec(t, p, "pt add 1 177413 446111")
	mustExec(t, p, "rad 1 12.3015 6235.42 as 2")
	got := p.Points["2"]
	close(t, got.Easting, 178763.034597876)
	close(t, got.Northing, 452198.517487526)
	if got.Code != "" {
		t.Fatalf("code=%q want empty", got.Code)
	}
}

func TestRad3DCommand(t *testing.T) {
	p := project.New("test")
	mustExec(t, p, "pt add 1 100 200 10")
	mustExec(t, p, "rad3d 1 90.0000 10 90.0000 as 2 CALC")
	got := p.Points["2"]
	close(t, got.Easting, 110)
	close(t, got.Northing, 200)
	if got.Elevation == nil {
		t.Fatal("expected elevation")
	}
	close(t, *got.Elevation, 10)
	if got.Code != "CALC" {
		t.Fatalf("code=%q want CALC", got.Code)
	}
}

func TestRad3DCommandAllowsOmittedCode(t *testing.T) {
	p := project.New("test")
	mustExec(t, p, "pt add 1 100 200 10")
	mustExec(t, p, "rad3d 1 90.0000 10 90.0000 as 2")
	if p.Points["2"].Code != "" {
		t.Fatalf("code=%q want empty", p.Points["2"].Code)
	}
}

func TestRad3DCommandRequiresStartElevation(t *testing.T) {
	p := project.New("test")
	mustExec(t, p, "pt add 1 100 200")
	if _, err := Execute(p, "rad3d 1 90.0000 10 90.0000 as 2"); err == nil {
		t.Fatal("expected elevation error")
	}
}

func TestRadQuadrantBearingCommand(t *testing.T) {
	p := project.New("test")
	mustExec(t, p, "pt add 1 0 0")
	mustExec(t, p, "rad 1 N 45.0000 E 10 as 2")
	close(t, p.Points["2"].Northing, math.Sqrt(50))
	close(t, p.Points["2"].Easting, math.Sqrt(50))
}

func TestInverseCommand(t *testing.T) {
	p := project.New("test")
	mustExec(t, p, "pt add 1 0 0")
	mustExec(t, p, "pt add 2 10 0")
	got, err := Execute(p, "inverse 1 2")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got.Message, "hd=10.000") {
		t.Fatalf("message=%q", got.Message)
	}
	if !strings.Contains(got.Message, "az=90°00′00.00″") {
		t.Fatalf("message=%q", got.Message)
	}
}

func TestAngleCommand(t *testing.T) {
	p := project.New("test")
	mustExec(t, p, "pt add 1 10 0")
	mustExec(t, p, "pt add 2 0 0")
	mustExec(t, p, "pt add 3 0 10")
	got, err := Execute(p, "angle 1 2 3")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got.Message, "inside=90°00′00.00″") {
		t.Fatalf("message=%q", got.Message)
	}
	if !strings.Contains(got.Message, "outside=270°00′00.00″") {
		t.Fatalf("message=%q", got.Message)
	}
}

func TestAngleCommandRejectsZeroLengthLeg(t *testing.T) {
	p := project.New("test")
	mustExec(t, p, "pt add 1 0 0")
	mustExec(t, p, "pt add 2 0 0")
	mustExec(t, p, "pt add 3 0 10")
	if _, err := Execute(p, "angle 1 2 3"); err == nil {
		t.Fatal("expected angle error")
	}
}

func TestCloseCommand(t *testing.T) {
	p := project.New("test")
	p.SetDisplayPrecision(2)
	mustExec(t, p, "pt add 1 0 0")
	mustExec(t, p, "pt add 2 4 0")
	mustExec(t, p, "pt add 3 4 3")
	got, err := Execute(p, "close 1 2 3")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"area=6.00", "misclose az=233°07′48.37″", "hd=5.00", "de=-4.00", "dn=-3.00", "accuracy=1:2"} {
		if !strings.Contains(got.Message, want) {
			t.Fatalf("message=%q missing %q", got.Message, want)
		}
	}
	if len(p.Points) != 3 || len(p.Features) != 0 || p.Traverse != nil {
		t.Fatal("close should not mutate project state")
	}
}

func TestCloseCommandPerfectClosure(t *testing.T) {
	p := project.New("test")
	mustExec(t, p, "pt add 1 0 0")
	mustExec(t, p, "pt add 2 4 0")
	mustExec(t, p, "pt add 3 4 3")
	got, err := Execute(p, "close 1 2 3 1")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"area=6.000", "hd=0.000", "de=0.000", "dn=0.000", "accuracy=perfect"} {
		if !strings.Contains(got.Message, want) {
			t.Fatalf("message=%q missing %q", got.Message, want)
		}
	}
}

func TestCloseCommandRejectsInvalidInput(t *testing.T) {
	p := project.New("test")
	mustExec(t, p, "pt add 1 0 0")
	mustExec(t, p, "pt add 2 4 0")
	if _, err := Execute(p, "close 1 2"); err == nil {
		t.Fatal("expected usage error")
	}
	if _, err := Execute(p, "close 1 2 9"); err == nil {
		t.Fatal("expected missing point error")
	}
}

func TestBearingAddCommandNormalizesResult(t *testing.T) {
	p := project.New("test")
	got, err := Execute(p, "bearing add 350.0000 20.0000")
	if err != nil {
		t.Fatal(err)
	}
	if got.Message != "bearing=10°00′00.00″" {
		t.Fatalf("message=%q", got.Message)
	}
	if len(p.Points) != 0 || len(p.Features) != 0 {
		t.Fatal("bearing add should not mutate project")
	}
}

func TestBearingSubCommandWrapsNegativeResult(t *testing.T) {
	p := project.New("test")
	got, err := Execute(p, "bearing sub 10.0000 20.0000")
	if err != nil {
		t.Fatal(err)
	}
	if got.Message != "bearing=350°00′00.00″" {
		t.Fatalf("message=%q", got.Message)
	}
}

func TestBearingCommandsAcceptSignedAngles(t *testing.T) {
	p := project.New("test")
	got, err := Execute(p, "bearing add -15.3000 30.0000")
	if err != nil {
		t.Fatal(err)
	}
	if got.Message != "bearing=14°30′00.00″" {
		t.Fatalf("message=%q", got.Message)
	}

	got, err = Execute(p, "bearing sub 10d -15.5d")
	if err != nil {
		t.Fatal(err)
	}
	if got.Message != "bearing=25°30′00.00″" {
		t.Fatalf("message=%q", got.Message)
	}
}

func TestBearingCommandRejectsInvalidInput(t *testing.T) {
	p := project.New("test")
	if _, err := Execute(p, "bearing add 10.0000"); err == nil {
		t.Fatal("expected usage error")
	}
	if _, err := Execute(p, "bearing add N 45.0000"); err == nil {
		t.Fatal("expected invalid angle error")
	}
	if _, err := Execute(p, "bearing mul 10.0000 20.0000"); err == nil {
		t.Fatal("expected subcommand error")
	}
}

func TestDistanceCommandsUseDisplayPrecisionWithoutMutation(t *testing.T) {
	p := project.New("test")
	p.SetDisplayPrecision(2)
	got, err := Execute(p, "dist add 12.5 3.125")
	if err != nil {
		t.Fatal(err)
	}
	if got.Message != "dist=15.62" {
		t.Fatalf("message=%q", got.Message)
	}

	got, err = Execute(p, "dist sub 12.5 15")
	if err != nil {
		t.Fatal(err)
	}
	if got.Message != "dist=-2.50" {
		t.Fatalf("message=%q", got.Message)
	}
	if len(p.Points) != 0 || len(p.Features) != 0 {
		t.Fatal("dist commands should not mutate project")
	}
}

func TestDistanceCommandRejectsInvalidInput(t *testing.T) {
	p := project.New("test")
	if _, err := Execute(p, "dist add 10"); err == nil {
		t.Fatal("expected usage error")
	}
	if _, err := Execute(p, "dist add ten 5"); err == nil {
		t.Fatal("expected invalid distance error")
	}
	if _, err := Execute(p, "dist mul 10 5"); err == nil {
		t.Fatal("expected subcommand error")
	}
}

func TestMidpointCommand(t *testing.T) {
	p := project.New("test")
	mustExec(t, p, "pt add 1 0 0")
	mustExec(t, p, "pt add 2 10 10")
	mustExec(t, p, "midpoint 1 2 as 3 MID")
	close(t, p.Points["3"].Northing, 5)
	close(t, p.Points["3"].Easting, 5)
	if p.Points["3"].Code != "MID" {
		t.Fatalf("code=%q want MID", p.Points["3"].Code)
	}
}

func TestOffsetCommandFromTwoPoints(t *testing.T) {
	p := project.New("test")
	mustExec(t, p, "pt add 1 0 0")
	mustExec(t, p, "pt add 2 10 0")
	mustExec(t, p, "offset 1 2 2 5 as 3 OFF")
	close(t, p.Points["3"].Northing, -2)
	close(t, p.Points["3"].Easting, 5)
	if p.Points["3"].Code != "OFF" {
		t.Fatalf("code=%q want OFF", p.Points["3"].Code)
	}
}

func TestOffsetCommandFromLine(t *testing.T) {
	p := project.New("test")
	mustExec(t, p, "pt add 1 0 0")
	mustExec(t, p, "pt add 2 10 0")
	mustExec(t, p, "line add L1 1 2 BASE")
	mustExec(t, p, "offset L1 2 5 as 3 OFF")
	close(t, p.Points["3"].Northing, -2)
	close(t, p.Points["3"].Easting, 5)
	if p.Points["3"].Code != "OFF" {
		t.Fatalf("code=%q want OFF", p.Points["3"].Code)
	}
}

func TestLineIntersectCommand(t *testing.T) {
	p := project.New("test")
	mustExec(t, p, "pt add 1 0 0")
	mustExec(t, p, "pt add 2 10 10")
	mustExec(t, p, "pt add 3 10 0")
	mustExec(t, p, "pt add 4 0 10")
	mustExec(t, p, "line intersect 1 2 3 4 as 5 IP")
	close(t, p.Points["5"].Northing, 5)
	close(t, p.Points["5"].Easting, 5)
}

func TestIntersectionCommandsAllowOmittedCode(t *testing.T) {
	t.Run("line", func(t *testing.T) {
		p := project.New("test")
		mustExec(t, p, "pt add 1 0 0")
		mustExec(t, p, "pt add 2 10 10")
		mustExec(t, p, "pt add 3 10 0")
		mustExec(t, p, "pt add 4 0 10")
		mustExec(t, p, "line intersect 1 2 3 4 as 5")
		if p.Points["5"].Code != "" {
			t.Fatalf("code=%q want empty", p.Points["5"].Code)
		}
	})

	t.Run("bearing-bearing", func(t *testing.T) {
		p := project.New("test")
		mustExec(t, p, "pt add 1 0 0")
		mustExec(t, p, "pt add 2 10 0")
		mustExec(t, p, "intersect bearing-bearing 1 45.0000 2 315.0000 as 3")
		if p.Points["3"].Code != "" {
			t.Fatalf("code=%q want empty", p.Points["3"].Code)
		}
	})

	t.Run("bearing-distance", func(t *testing.T) {
		p := project.New("test")
		mustExec(t, p, "pt add 1 0 0")
		mustExec(t, p, "pt add 2 5 0")
		mustExec(t, p, "intersect bearing-distance 1 90.0000 2 5 choose far as 3")
		if p.Points["3"].Code != "" {
			t.Fatalf("code=%q want empty", p.Points["3"].Code)
		}
	})

	t.Run("distance-distance", func(t *testing.T) {
		p := project.New("test")
		mustExec(t, p, "pt add 1 0 0")
		mustExec(t, p, "pt add 2 10 0")
		mustExec(t, p, "intersect distance-distance 1 10 2 10 choose left as 3")
		if p.Points["3"].Code != "" {
			t.Fatalf("code=%q want empty", p.Points["3"].Code)
		}
	})
}

func TestResectCommand(t *testing.T) {
	p := project.New("test")
	mustExec(t, p, "pt add N 0 10")
	mustExec(t, p, "pt add E 10 0")
	mustExec(t, p, "pt add W -10 0")
	mustExec(t, p, "resect N 0.0000 E 90.0000 W 270.0000 as X RS")
	got := p.Points["X"]
	close(t, got.Northing, 0)
	close(t, got.Easting, 0)
	if got.Code != "RS" {
		t.Fatalf("code=%q want RS", got.Code)
	}
}

func TestResectCommandAllowsOmittedCode(t *testing.T) {
	p := project.New("test")
	mustExec(t, p, "pt add N 0 10")
	mustExec(t, p, "pt add E 10 0")
	mustExec(t, p, "pt add W -10 0")
	mustExec(t, p, "resect N 0.0000 E 90.0000 W 270.0000 as X")
	got := p.Points["X"]
	close(t, got.Northing, 0)
	close(t, got.Easting, 0)
	if got.Code != "" {
		t.Fatalf("code=%q want empty", got.Code)
	}
}

func TestCalculatedPointCommandsRejectExistingDestinationID(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(*project.Project)
		command string
	}{
		{"rad", func(p *project.Project) { mustExec(t, p, "pt add 1 0 0 1") }, "rad 1 90.0000 10 as X"},
		{"rad3d", func(p *project.Project) { mustExec(t, p, "pt add 1 0 0 1") }, "rad3d 1 90.0000 10 90.0000 as X"},
		{"midpoint", func(p *project.Project) {
			mustExec(t, p, "pt add 1 0 0")
			mustExec(t, p, "pt add 2 10 10")
		}, "midpoint 1 2 as X TEST"},
		{"offset points", func(p *project.Project) {
			mustExec(t, p, "pt add 1 0 0")
			mustExec(t, p, "pt add 2 10 0")
		}, "offset 1 2 2 5 as X"},
		{"offset line", func(p *project.Project) {
			mustExec(t, p, "pt add 1 0 0")
			mustExec(t, p, "pt add 2 10 0")
			mustExec(t, p, "line add L1 1 2")
		}, "offset L1 2 5 as X"},
		{"line intersect", func(p *project.Project) {
			mustExec(t, p, "pt add 1 0 0")
			mustExec(t, p, "pt add 2 10 10")
			mustExec(t, p, "pt add 3 0 10")
			mustExec(t, p, "pt add 4 10 0")
		}, "line intersect 1 2 3 4 as X"},
		{"bearing-bearing", func(p *project.Project) {
			mustExec(t, p, "pt add 1 0 0")
			mustExec(t, p, "pt add 2 10 0")
		}, "intersect bearing-bearing 1 45.0000 2 315.0000 as X"},
		{"bearing-distance", func(p *project.Project) {
			mustExec(t, p, "pt add 1 0 0")
			mustExec(t, p, "pt add 2 5 0")
		}, "intersect bearing-distance 1 90.0000 2 5 choose far as X"},
		{"distance-distance", func(p *project.Project) {
			mustExec(t, p, "pt add 1 0 0")
			mustExec(t, p, "pt add 2 10 0")
		}, "intersect distance-distance 1 10 2 10 choose left as X"},
		{"resect", func(p *project.Project) {
			mustExec(t, p, "pt add N 0 10")
			mustExec(t, p, "pt add E 10 0")
			mustExec(t, p, "pt add W -10 0")
		}, "resect N 0.0000 E 90.0000 W 270.0000 as X"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p := project.New("test")
			tc.setup(p)
			mustExec(t, p, "pt add X 123 456 KEEP")
			before := p.Points["X"]
			if _, err := Execute(p, tc.command); err == nil || !strings.Contains(err.Error(), "already exists") {
				t.Fatalf("error=%v want existing point rejection", err)
			}
			if got := p.Points["X"]; got != before {
				t.Fatalf("point X overwritten: got=%+v before=%+v", got, before)
			}
		})
	}
}

func TestShiftCommand(t *testing.T) {
	p := project.New("test")
	z := 5.0
	p.Points["1"] = geom.Point{ID: "1", Easting: 100, Northing: 200, Elevation: &z}
	p.Points["2"] = geom.Point{ID: "2", Easting: 110, Northing: 210}
	p.ContourSets["C1"] = project.ContourSet{
		ID: "C1",
		Polylines: []project.ContourPolyline{{
			ID:        "C1-1",
			Elevation: 100,
			Vertices:  []project.ContourVertex{{Easting: 1, Northing: 2}, {Easting: 3, Northing: 4}},
		}},
	}
	mustExec(t, p, "shift 1 east=2 north=-3 elev=1.5")
	close(t, p.Points["1"].Easting, 2)
	close(t, p.Points["1"].Northing, -3)
	close(t, *p.Points["1"].Elevation, 1.5)
	close(t, p.Points["2"].Easting, 12)
	close(t, p.Points["2"].Northing, 7)
	if p.Points["2"].Elevation != nil {
		t.Fatalf("2D point gained elevation: %+v", p.Points["2"])
	}
	close(t, p.ContourSets["C1"].Polylines[0].Vertices[0].Easting, -97)
	close(t, p.ContourSets["C1"].Polylines[0].Vertices[0].Northing, -201)
	close(t, p.ContourSets["C1"].Polylines[0].Elevation, 96.5)
}

func TestShiftCommandRejectsMissingAxes(t *testing.T) {
	p := project.New("test")
	mustExec(t, p, "pt add 1 0 0")
	if _, err := Execute(p, "shift 1"); err == nil {
		t.Fatal("expected shift usage error")
	}
}

func TestShiftCommandLeavesOmittedCoordinateUnchangedAtBase(t *testing.T) {
	p := project.New("test")
	mustExec(t, p, "pt add 1 100 200")
	mustExec(t, p, "pt add 2 110 210")
	mustExec(t, p, "shift 1 east=50")
	close(t, p.Points["1"].Easting, 50)
	close(t, p.Points["1"].Northing, 200)
	close(t, p.Points["2"].Easting, 60)
	close(t, p.Points["2"].Northing, 210)
}

func TestShiftCommandRejectsElevationTargetForTwoDimensionalBase(t *testing.T) {
	p := project.New("test")
	mustExec(t, p, "pt add 1 100 200")
	if _, err := Execute(p, "shift 1 elev=10"); err == nil {
		t.Fatal("expected 2D base elevation shift error")
	}
}

func TestRotateCommand(t *testing.T) {
	p := project.New("test")
	z := 5.0
	p.Points["1"] = geom.Point{ID: "1", Easting: 0, Northing: 0, Elevation: &z}
	p.Points["2"] = geom.Point{ID: "2", Easting: 10, Northing: 0}
	p.ContourSets["C1"] = project.ContourSet{
		ID: "C1",
		Polylines: []project.ContourPolyline{{
			ID:        "C1-1",
			Elevation: 100,
			Vertices:  []project.ContourVertex{{Easting: 10, Northing: 0}},
		}},
	}
	mustExec(t, p, "rotate 1 90.0000")
	close(t, p.Points["1"].Easting, 0)
	close(t, p.Points["1"].Northing, 0)
	close(t, p.Points["2"].Easting, 0)
	close(t, p.Points["2"].Northing, -10)
	close(t, *p.Points["1"].Elevation, 5)
	close(t, p.ContourSets["C1"].Polylines[0].Vertices[0].Easting, 0)
	close(t, p.ContourSets["C1"].Polylines[0].Vertices[0].Northing, -10)
	close(t, p.ContourSets["C1"].Polylines[0].Elevation, 100)
}

func TestRotateCommandSupportsNegativeAngle(t *testing.T) {
	p := project.New("test")
	mustExec(t, p, "pt add 1 0 0")
	mustExec(t, p, "pt add 2 10 0")
	mustExec(t, p, "rotate 1 -90.0000")
	close(t, p.Points["2"].Easting, 0)
	close(t, p.Points["2"].Northing, 10)
}

func TestRotateCommandRejectsQuadrantBearing(t *testing.T) {
	p := project.New("test")
	mustExec(t, p, "pt add 1 0 0")
	if _, err := Execute(p, "rotate 1 N 45.0000 E"); err == nil {
		t.Fatal("expected rotate angle error")
	}
}

func TestScaleApplyAndReversePreservesElevationAndContourMetadata(t *testing.T) {
	p := project.New("test")
	z := 7.0
	maxEdge := 50.0
	p.Points["A"] = geom.Point{ID: "A", Easting: 500000, Northing: 6500000, Elevation: &z}
	p.Points["B"] = geom.Point{ID: "B", Easting: 500100, Northing: 6500200}
	p.ContourSets["C1"] = project.ContourSet{
		ID:               "C1",
		Base:             5,
		EffectiveMaxEdge: 50,
		Generation:       &project.ContourGenerationSpec{Interval: 1, MaxEdge: &maxEdge},
		Diagnostics: []project.ContourDiagnostic{{
			Code: "long_edge", Message: "old", EdgeIDs: []string{"A", "B"}, Measured: 60, Limit: 50,
		}},
		Polylines: []project.ContourPolyline{{
			Elevation: 6, Vertices: []project.ContourVertex{{Easting: 500100, Northing: 6500200}},
		}},
		RawPolylines: []project.ContourPolyline{{
			Elevation: 6, Vertices: []project.ContourVertex{{Easting: 500100, Northing: 6500200}},
		}},
	}

	result, err := Execute(p, "scale apply A csf=0.9996 system=MGA2020_ZONE50")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result.Message, "applied scale") {
		t.Fatalf("message=%q", result.Message)
	}
	if p.GridGround == nil || p.GridGround.Mode != "local_ground" || p.GridGround.GridSystem != "MGA2020_ZONE50" {
		t.Fatalf("conversion=%+v", p.GridGround)
	}
	close(t, p.Points["A"].Easting, 500000)
	close(t, p.Points["B"].Easting, 500000+100/0.9996)
	close(t, p.Points["B"].Northing, 6500000+200/0.9996)
	close(t, *p.Points["A"].Elevation, 7)
	close(t, p.ContourSets["C1"].Base, 5)
	close(t, p.ContourSets["C1"].EffectiveMaxEdge, 50/0.9996)
	close(t, *p.ContourSets["C1"].Generation.MaxEdge, 50/0.9996)
	close(t, p.ContourSets["C1"].Diagnostics[0].Measured, 60/0.9996)
	if !strings.Contains(p.ContourSets["C1"].Diagnostics[0].Message, "60.024") {
		t.Fatalf("diagnostic=%q", p.ContourSets["C1"].Diagnostics[0].Message)
	}

	mustExec(t, p, "pt add C 500050 6500050")
	mustExec(t, p, "scale reverse")
	if p.GridGround != nil {
		t.Fatalf("conversion label should be removed after reverse: %+v", p.GridGround)
	}
	close(t, p.Points["B"].Easting, 500100)
	close(t, p.Points["B"].Northing, 6500200)
	close(t, p.Points["C"].Easting, 500000+(50*0.9996))
	close(t, p.ContourSets["C1"].EffectiveMaxEdge, 50)
	close(t, *p.ContourSets["C1"].Generation.MaxEdge, 50)
}

func TestScaleApplyValidation(t *testing.T) {
	p := project.New("test")
	mustExec(t, p, "pt add A 500000 6500000")
	for _, command := range []string{
		"scale apply A csf=0",
		"scale apply A csf=NaN",
		"scale reverse",
	} {
		if _, err := Execute(p, command); err == nil {
			t.Fatalf("command %q should fail", command)
		}
	}
	mustExec(t, p, "scale apply A csf=0.9996")
	if p.GridGround.GridSystem != "" {
		t.Fatalf("system=%q want empty", p.GridGround.GridSystem)
	}
	if _, err := Execute(p, "scale apply A csf=0.9996"); err == nil {
		t.Fatal("second conversion to local ground should fail")
	}
}

func TestShiftUpdatesActiveScaleAnchorBeforeReverse(t *testing.T) {
	p := project.New("test")
	mustExec(t, p, "pt add A 100 200")
	mustExec(t, p, "pt add B 200 200")
	mustExec(t, p, "scale apply A csf=0.5 system=GROUND")
	mustExec(t, p, "shift A east=1000 north=2000")

	close(t, p.GridGround.AnchorEasting, 1000)
	close(t, p.GridGround.AnchorNorthing, 2000)
	mustExec(t, p, "scale reverse")
	close(t, p.Points["A"].Easting, 1000)
	close(t, p.Points["A"].Northing, 2000)
	close(t, p.Points["B"].Easting, 1100)
	close(t, p.Points["B"].Northing, 2000)
}

func TestRotateUpdatesActiveScaleAnchorBeforeReverse(t *testing.T) {
	p := project.New("test")
	mustExec(t, p, "pt add O 0 0")
	mustExec(t, p, "pt add A 100 0")
	mustExec(t, p, "pt add B 200 0")
	mustExec(t, p, "scale apply A csf=0.5 system=GROUND")
	mustExec(t, p, "rotate O 90.0000")

	close(t, p.GridGround.AnchorEasting, -100)
	close(t, p.GridGround.AnchorNorthing, -200)
	mustExec(t, p, "scale reverse")
	close(t, p.Points["A"].Easting, -100)
	close(t, p.Points["A"].Northing, -200)
	close(t, p.Points["B"].Easting, -100)
	close(t, p.Points["B"].Northing, -300)
}

func TestTransformFitCommandReportsSimilarityParametersWithoutMutation(t *testing.T) {
	p := project.New("test")
	mustExec(t, p, "pt add S1 0 0 1")
	mustExec(t, p, "pt add T1 100 200 4")
	mustExec(t, p, "pt add S2 10 0 2")
	mustExec(t, p, "pt add T2 100 180 5")
	mustExec(t, p, "pt add S3 0 10")
	mustExec(t, p, "pt add T3 120 200")

	got, err := Execute(p, "transform fit S1 T1 S2 T2 S3 T3")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"dx=100.000", "dy=200.000", "dz=3.000",
		"angle=+90°00′00.00″", "scale=2.00000000",
		"hrms=0.000", "zrms=0.000", "pairs=3", "zpairs=2",
	} {
		if !strings.Contains(got.Message, want) {
			t.Fatalf("message=%q missing %q", got.Message, want)
		}
	}
	close(t, p.Points["S2"].Easting, 10)
	close(t, p.Points["S2"].Northing, 0)
}

func TestTransformFitCommandReportsUnavailableVerticalFit(t *testing.T) {
	p := project.New("test")
	mustExec(t, p, "pt add S1 0 0")
	mustExec(t, p, "pt add T1 10 20")
	mustExec(t, p, "pt add S2 10 0")
	mustExec(t, p, "pt add T2 20 20")

	got, err := Execute(p, "transform fit S1 T1 S2 T2")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got.Message, "dz=n/a") || !strings.Contains(got.Message, "zrms=n/a") || !strings.Contains(got.Message, "zpairs=0") {
		t.Fatalf("message=%q", got.Message)
	}
}

func TestTransformFitCommandRejectsInvalidPairs(t *testing.T) {
	p := project.New("test")
	mustExec(t, p, "pt add S1 0 0")
	mustExec(t, p, "pt add T1 10 20")
	mustExec(t, p, "pt add S2 0 0")
	mustExec(t, p, "pt add T2 20 20")
	for _, command := range []string{
		"transform fit S1 T1",
		"transform fit S1 T1 S2",
		"transform fit S1 T1 MISSING T2",
		"transform fit S1 T1 S2 T2",
	} {
		if _, err := Execute(p, command); err == nil {
			t.Fatalf("%q: expected error", command)
		}
	}
}

func TestLineEditCommandSupportsQuotedDescription(t *testing.T) {
	p := project.New("test")
	mustExec(t, p, "pt add 1 0 0")
	mustExec(t, p, "pt add 2 10 0")
	mustExec(t, p, "pt add 3 20 0")
	mustExec(t, p, "line add L1 1 2 BOUNDARY")
	mustExec(t, p, `line edit L1 from=2 to=3 code=EASE desc="this is a line"`)
	got := p.Features["L1"]
	if got.PointIDs[0] != "2" || got.PointIDs[1] != "3" || got.Code != "EASE" || got.Description != "this is a line" {
		t.Fatalf("line=%+v", got)
	}
}

func TestPointDeleteRejectsMissingAndReferencedPoint(t *testing.T) {
	p := project.New("test")
	mustExec(t, p, "pt add 1 0 0")
	mustExec(t, p, "pt add 2 10 0")
	mustExec(t, p, "line add L1 1 2 BOUNDARY")
	if _, err := Execute(p, "pt del 9"); err == nil {
		t.Fatal("expected missing point error")
	}
	if _, err := Execute(p, "pt del 1"); err == nil {
		t.Fatal("expected referenced point error")
	}
	if _, ok := p.Points["1"]; !ok {
		t.Fatal("referenced point should not be deleted")
	}
}

func TestLineAddAndDeleteValidateExistingIDs(t *testing.T) {
	p := project.New("test")
	mustExec(t, p, "pt add 1 0 0")
	mustExec(t, p, "pt add 2 10 0")
	mustExec(t, p, "line add L1 1 2 BOUNDARY")
	if _, err := Execute(p, "line add L1 1 2 DUP"); err == nil {
		t.Fatal("expected duplicate line error")
	}
	if p.Features["L1"].Code != "BOUNDARY" {
		t.Fatalf("line was overwritten: %+v", p.Features["L1"])
	}
	if _, err := Execute(p, "line del MISSING"); err == nil {
		t.Fatal("expected missing line error")
	}
}

func TestLineGenCreatesSequentialLinesForCode(t *testing.T) {
	p := project.New("test")
	mustExec(t, p, "pt add 1 0 0 PEG")
	mustExec(t, p, "pt add 2 10 0 TREE")
	mustExec(t, p, "pt add 3 20 0 PEG")
	mustExec(t, p, "pt add 4 30 0 PEG")

	got, err := Execute(p, "line gen PEG")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got.Message, "generated 2 lines for code PEG") {
		t.Fatalf("message=%q", got.Message)
	}
	if len(got.Created) != 2 {
		t.Fatalf("created=%v want 2 lines", got.Created)
	}
	if line := p.Features["L1"]; line.PointIDs[0] != "1" || line.PointIDs[1] != "3" || line.Code != "PEG" || line.Description != "" {
		t.Fatalf("line L1=%+v", line)
	}
	if line := p.Features["L2"]; line.PointIDs[0] != "3" || line.PointIDs[1] != "4" || line.Code != "PEG" || line.Description != "" {
		t.Fatalf("line L2=%+v", line)
	}
}

func TestLineGenSkipsExistingUndirectedPairs(t *testing.T) {
	p := project.New("test")
	mustExec(t, p, "pt add 1 0 0 PEG")
	mustExec(t, p, "pt add 2 10 0 PEG")
	mustExec(t, p, "pt add 3 20 0 PEG")
	mustExec(t, p, "line add L9 2 1 PEG")

	got, err := Execute(p, "line gen PEG")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got.Message, "generated 1 lines") || !strings.Contains(got.Message, "skipped 1 duplicates") {
		t.Fatalf("message=%q", got.Message)
	}
	if _, ok := p.Features["L10"]; !ok {
		t.Fatalf("lines=%+v want generated L10", p.Features)
	}
	if line := p.Features["L10"]; line.PointIDs[0] != "2" || line.PointIDs[1] != "3" {
		t.Fatalf("line L10=%+v", line)
	}
}

func TestLineGenRequiresAtLeastTwoMatchingPoints(t *testing.T) {
	p := project.New("test")
	mustExec(t, p, "pt add 1 0 0 PEG")
	if _, err := Execute(p, "line gen PEG"); err == nil {
		t.Fatal("expected insufficient points error")
	}
}

func TestPointEditCommandSupportsQuotedDescription(t *testing.T) {
	p := project.New("test")
	mustExec(t, p, "pt add 1 0 0 PEG")
	mustExec(t, p, `pt edit 1 desc="this is a point"`)
	if p.Points["1"].Description != "this is a point" {
		t.Fatalf("description=%q", p.Points["1"].Description)
	}
}

func TestBearingDistanceCommand(t *testing.T) {
	p := project.New("test")
	mustExec(t, p, "pt add 1 0 0")
	mustExec(t, p, "pt add 2 5 0")
	mustExec(t, p, "intersect bearing-distance 1 90.0000 2 5 choose far as 3 BD")
	close(t, p.Points["3"].Northing, 0)
	close(t, p.Points["3"].Easting, 10)
	if p.Points["3"].Code != "BD" {
		t.Fatalf("code=%q want BD", p.Points["3"].Code)
	}
}

func TestTraverseCommands(t *testing.T) {
	p := project.New("test")
	mustExec(t, p, "pt add 1 0 0")
	mustExec(t, p, "pt add 99 20 0")
	mustExec(t, p, "trav start 1")
	mustExec(t, p, "trav leg 90.0000 9 TRV")
	mustExec(t, p, "trav leg 90.0000 9 TRV")
	result, err := Execute(p, "trav close 99")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result.Message, "hd=2.000") {
		t.Fatalf("message=%q", result.Message)
	}
	mustExec(t, p, "trav adjust compass")
	close(t, p.Points["100"].Easting, 10)
	close(t, p.Points["101"].Easting, 20)
}

func TestTraverseShowAndRestart(t *testing.T) {
	p := project.New("test")
	mustExec(t, p, "pt add 10 0 0")
	mustExec(t, p, "pt add 12 20 0")
	mustExec(t, p, "trav start 10")
	result, err := Execute(p, "trav show")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"start=10", "current=10", "legs=0", "next=13"} {
		if !strings.Contains(result.Message, want) {
			t.Fatalf("message=%q missing %q", result.Message, want)
		}
	}
	mustExec(t, p, "trav leg 90.0000 5")
	if _, ok := p.Points["13"]; !ok {
		t.Fatal("expected auto-created traverse point 13")
	}
	if p.Traverse.Current != "13" {
		t.Fatalf("current=%q want 13", p.Traverse.Current)
	}
	mustExec(t, p, "trav close 12")
	result, err = Execute(p, "trav show")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result.Message, "close=12") {
		t.Fatalf("message=%q missing close point", result.Message)
	}
	mustExec(t, p, "trav start 12")
	if p.Traverse.Current != "12" || p.Traverse.Close != "" || len(p.Traverse.LegPointIDs) != 0 {
		t.Fatalf("traverse=%+v want reset state", p.Traverse)
	}
}

func TestContourCommands(t *testing.T) {
	p := project.New("test")
	mustExec(t, p, "pt add 1 0 0 0")
	mustExec(t, p, "pt add 2 0 10 10")
	mustExec(t, p, "pt add 3 10 0 10")
	got, err := Execute(p, "contour gen C1 5 base=0 index=2 breaklines=none")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got.Message, "generated contour set C1") {
		t.Fatalf("message=%q", got.Message)
	}
	if len(p.ContourSets["C1"].Polylines) == 0 {
		t.Fatal("expected stored contours")
	}
	got, err = Execute(p, "contour gen C1 5 base=0 index=2 breaklines=none")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got.Message, "replaced contour set C1") {
		t.Fatalf("message=%q", got.Message)
	}
	got, err = Execute(p, "contour info C1")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got.Message, "polylines=") {
		t.Fatalf("message=%q", got.Message)
	}
	got, err = Execute(p, "contour list")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"1 contour sets", "C1", "interval=5.000", "base=0.000", "polylines=", "breaklines=0", "index=2"} {
		if !strings.Contains(got.Message, want) {
			t.Fatalf("message=%q missing %q", got.Message, want)
		}
	}
	mustExec(t, p, "contour del C1")
	if len(p.ContourSets) != 0 {
		t.Fatalf("contour sets=%d want 0", len(p.ContourSets))
	}
}

func TestContourGenDefaultsWholeNumberLevelsToIndex(t *testing.T) {
	p := project.New("test")
	mustExec(t, p, "pt add 1 0 0 0")
	mustExec(t, p, "pt add 2 0 10 2")
	mustExec(t, p, "pt add 3 10 0 2")
	if _, err := Execute(p, "contour gen C1 0.5 breaklines=none"); err != nil {
		t.Fatal(err)
	}
	var foundMajor, foundMinor bool
	for _, pl := range p.ContourSets["C1"].Polylines {
		switch pl.Elevation {
		case 1:
			if pl.Index {
				foundMajor = true
			}
		case 1.5:
			if !pl.Index {
				foundMinor = true
			}
		}
	}
	if !foundMajor || !foundMinor {
		t.Fatalf("contours=%+v want level 1 major and level 1.5 minor", p.ContourSets["C1"].Polylines)
	}
}

func TestContourBoundaryRegenerationAndStaleState(t *testing.T) {
	p := project.New("test")
	for _, command := range []string{
		"pt add 1 0 0 0",
		"pt add 2 10 0 10",
		"pt add 3 0 10 0",
		"pt add 4 10 10 10",
		"pt add B1 0 2",
		"pt add B2 10 2",
		"pt add B3 10 8",
		"pt add B4 0 8",
		"line add E1 B1 B2 CLIP",
		"line add E2 B2 B3 CLIP",
		"line add E3 B3 B4 CLIP",
		"line add E4 B4 B1 CLIP",
	} {
		mustExec(t, p, command)
	}
	mustExec(t, p, "contour gen C1 5 boundary=codes:CLIP")
	set := p.ContourSets["C1"]
	if set.Generation == nil || len(set.BoundaryLines) != 4 || set.Generation.BreaklineMode != "all" {
		t.Fatalf("contour metadata=%+v", set)
	}
	if set.Stale {
		t.Fatal("freshly generated contour is stale")
	}
	mustExec(t, p, "pt edit 2 elev=12")
	if !p.ContourSets["C1"].Stale {
		t.Fatal("terrain geometry edit should mark contour stale")
	}
	got, err := Execute(p, "contour info C1")
	if err != nil || !strings.Contains(got.Message, "stale") || !strings.Contains(got.Message, "boundary_codes=CLIP") {
		t.Fatalf("info=%q err=%v", got.Message, err)
	}
	mustExec(t, p, "contour regen C1")
	if p.ContourSets["C1"].Stale {
		t.Fatal("regeneration should clear stale state")
	}
}

func TestLineTerrainRoleIsStoredAndDoesNotRequireReservedCode(t *testing.T) {
	p := project.New("test")
	mustExec(t, p, "pt add 1 0 0 0")
	mustExec(t, p, "pt add 2 10 0 10")
	mustExec(t, p, "line add R1 1 2 FEATURE terrain=ridge")
	if got := p.Features["R1"]; got.Code != "FEATURE" || got.TerrainRole != "ridge" {
		t.Fatalf("line=%+v", got)
	}
	mustExec(t, p, "line edit R1 terrain=drain")
	if p.Features["R1"].TerrainRole != "drain" {
		t.Fatalf("terrain role=%q", p.Features["R1"].TerrainRole)
	}
	mustExec(t, p, "line edit R1 terrain=none")
	if p.Features["R1"].TerrainRole != "" {
		t.Fatalf("terrain role=%q want empty", p.Features["R1"].TerrainRole)
	}
}

func TestGroupPolylineAndPolygonCommandsStoreStyledFeatures(t *testing.T) {
	p := project.New("test")
	for _, command := range []string{
		"pt add 1 0 0", "pt add 2 10 0", "pt add 3 10 10", "pt add 4 0 10",
		"group add BND layer=BOUNDARIES color=1",
		"polyline add K1 1 2 3 code=KERB group=BND terrain=standard",
		"polygon add LOT1 1 2 3 4 code=LOT group=BND",
		"pt edit 1 group=BND",
	} {
		mustExec(t, p, command)
	}
	if got := p.Features["K1"]; got.Kind != project.FeaturePolyline || len(got.PointIDs) != 3 || got.GroupID != "BND" {
		t.Fatalf("polyline=%+v", got)
	}
	if got := p.Features["LOT1"]; got.Kind != project.FeaturePolygon || got.GroupID != "BND" {
		t.Fatalf("polygon=%+v", got)
	}
	if p.Points["1"].GroupID != "BND" {
		t.Fatalf("point group=%q", p.Points["1"].GroupID)
	}
	if _, err := Execute(p, "group del BND"); err == nil {
		t.Fatal("expected assigned group deletion to fail")
	}
}

func TestPolygonReportIncludesAreaPerimeterAndLegsWithoutMutation(t *testing.T) {
	p := project.New("test")
	for _, command := range []string{
		"pt add 1 0 0", "pt add 2 4 0", "pt add 3 4 3",
		"polygon add LOT1 1 2 3 code=LOT desc=parcel",
	} {
		mustExec(t, p, command)
	}
	got, err := Execute(p, "polygon report LOT1")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"LOT1", "area=6.000", "perimeter=12.000", "leg=1", "from=1", "to=2", "bearing=90°00′00.00″"} {
		if !strings.Contains(got.Message, want) {
			t.Fatalf("report=%q missing %q", got.Message, want)
		}
	}
	if got.Changed {
		t.Fatalf("polygon report should be read-only: %+v", got)
	}
}

func TestPointCodeStyleDefaultsApplyOnlyToUngroupedNewPoints(t *testing.T) {
	p := project.New("test")
	for _, command := range []string{
		"group add PEGS layer=PEG_MARKS color=1",
		"group add OTHER layer=OTHER_MARKS color=2",
		"code style set PEG group=PEGS",
		"pt add 1 0 0 PEG",
		"pt add 2 10 0 PEG group=OTHER",
		"rad 1 90.0000 5 as 3 PEG",
	} {
		mustExec(t, p, command)
	}
	if p.Points["1"].GroupID != "PEGS" || p.Points["2"].GroupID != "OTHER" || p.Points["3"].GroupID != "PEGS" {
		t.Fatalf("styled points=%+v", p.Points)
	}
	list, err := Execute(p, "code style list")
	if err != nil || list.Changed || !strings.Contains(list.Message, "PEG group=PEGS") {
		t.Fatalf("style list=%+v err=%v", list, err)
	}
	if _, err := Execute(p, "group del PEGS"); err == nil {
		t.Fatal("group referenced by point code style should not be deletable")
	}
	mustExec(t, p, "code style del PEG")
	if p.Points["1"].GroupID != "PEGS" {
		t.Fatal("removing a default must not restyle existing points")
	}
}

func TestPolygonRejectsSelfIntersectionAndTerrainRole(t *testing.T) {
	p := project.New("test")
	for _, command := range []string{"pt add 1 0 0", "pt add 2 10 10", "pt add 3 0 10", "pt add 4 10 0"} {
		mustExec(t, p, command)
	}
	if _, err := Execute(p, "polygon add BAD 1 2 3 4"); err == nil {
		t.Fatal("expected crossing polygon error")
	}
	if _, err := Execute(p, "polygon add BAD 1 3 2 terrain=ridge"); err == nil {
		t.Fatal("expected polygon terrain role error")
	}
}

func TestContourQualityAndSmoothingOptionsAreStoredAndReported(t *testing.T) {
	p := project.New("test")
	mustExec(t, p, "pt add 1 0 0 0")
	mustExec(t, p, "pt add 2 10 0 10")
	mustExec(t, p, "pt add 3 0 10 10")
	got, err := Execute(p, "contour gen C1 5 breaklines=none maxedge=2 smooth=1")
	if err != nil {
		t.Fatal(err)
	}
	set := p.ContourSets["C1"]
	if set.Generation == nil || set.Generation.MaxEdge == nil || *set.Generation.MaxEdge != 2 || set.Generation.Smooth != 1 {
		t.Fatalf("generation=%+v", set.Generation)
	}
	if !strings.Contains(got.Message, "warnings") {
		t.Fatalf("message=%q want warning summary", got.Message)
	}
	info, err := Execute(p, "contour info C1")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"smooth=1", "warnings=", "triangles=", "maxedge=2.000", "warning long_edge"} {
		if !strings.Contains(info.Message, want) {
			t.Fatalf("info=%q missing %q", info.Message, want)
		}
	}
}

func TestContourRejectsInvalidQualityAndSmoothingOptions(t *testing.T) {
	p := project.New("test")
	mustExec(t, p, "pt add 1 0 0 0")
	mustExec(t, p, "pt add 2 10 0 10")
	mustExec(t, p, "pt add 3 0 10 10")
	for _, command := range []string{
		"contour gen C1 5 maxedge=0",
		"contour gen C1 5 smooth=4",
		"contour gen C1 5 smooth=nope",
	} {
		if _, err := Execute(p, command); err == nil {
			t.Fatalf("%q should fail", command)
		}
	}
}

func TestExecuteAndRecord(t *testing.T) {
	p := project.New("test")
	if _, err := ExecuteAndRecord(p, "pt add 1 0 0 PEG"); err != nil {
		t.Fatal(err)
	}
	if len(p.History) != 1 {
		t.Fatalf("history length=%d want 1", len(p.History))
	}
	if p.History[0].Created[0] != "point:1" {
		t.Fatalf("created=%v", p.History[0].Created)
	}
}

func TestExecuteChangedAndPersistedHistoryTrackMutationsOnly(t *testing.T) {
	p := project.New("test")
	created, err := ExecuteAndRecord(p, "pt add 1 0 0")
	if err != nil {
		t.Fatal(err)
	}
	if !created.Changed || len(p.History) != 1 {
		t.Fatalf("created=%+v history=%d", created, len(p.History))
	}
	mustExec(t, p, "pt add 2 10 0")
	historyCount := len(p.History)
	report, err := ExecuteAndRecord(p, "inverse 1 2")
	if err != nil {
		t.Fatal(err)
	}
	if report.Changed || len(p.History) != historyCount {
		t.Fatalf("report=%+v history=%d want %d", report, len(p.History), historyCount)
	}
}

func mustExec(t *testing.T, p *project.Project, command string) {
	t.Helper()
	if _, err := Execute(p, command); err != nil {
		t.Fatalf("%s: %v", command, err)
	}
}

func close(t *testing.T, got, want float64) {
	t.Helper()
	if math.Abs(got-want) > 1e-8 {
		t.Fatalf("got %f want %f", got, want)
	}
}
