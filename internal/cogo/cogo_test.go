package cogo

import (
	"math"
	"strings"
	"testing"

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

func TestLineEditCommandSupportsQuotedDescription(t *testing.T) {
	p := project.New("test")
	mustExec(t, p, "pt add 1 0 0")
	mustExec(t, p, "pt add 2 10 0")
	mustExec(t, p, "pt add 3 20 0")
	mustExec(t, p, "line add L1 1 2 BOUNDARY")
	mustExec(t, p, `line edit L1 from=2 to=3 code=EASE desc="this is a line"`)
	got := p.Lines["L1"]
	if got.From != "2" || got.To != "3" || got.Code != "EASE" || got.Description != "this is a line" {
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
	if p.Lines["L1"].Code != "BOUNDARY" {
		t.Fatalf("line was overwritten: %+v", p.Lines["L1"])
	}
	if _, err := Execute(p, "line del MISSING"); err == nil {
		t.Fatal("expected missing line error")
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
	mustExec(t, p, "trav leg 90.0000 9 as 2 TRV")
	mustExec(t, p, "trav leg 90.0000 9 as 3 TRV")
	result, err := Execute(p, "trav close 99")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result.Message, "hd=2.000") {
		t.Fatalf("message=%q", result.Message)
	}
	mustExec(t, p, "trav adjust compass")
	close(t, p.Points["2"].Easting, 10)
	close(t, p.Points["3"].Easting, 20)
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
