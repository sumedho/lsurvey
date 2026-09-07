package cogo

import (
	"lsurvey/internal/geom"
	"lsurvey/internal/project"
	"reflect"
	"strings"
	"testing"
)

func qualityTraverse() *project.Project {
	p := project.New("qa")
	p.Points = map[string]geom.Point{"S": {ID: "S"}, "A": {ID: "A", Easting: 30, Northing: 40}, "B": {ID: "B", Easting: -60, Northing: 40}, "C": {ID: "C", Easting: -60}, "T": {ID: "T", Easting: -59, Northing: 2}}
	p.Traverse = &project.TraverseState{Start: "S", Current: "C", Close: "T", LegPointIDs: []string{"A", "B", "C"}}
	return p
}

func TestTraverseQualityReportAndAdjustment(t *testing.T) {
	p := qualityTraverse()
	before := p.Clone()
	r, err := Execute(p, "trav report transit")
	if err != nil {
		t.Fatal(err)
	}
	if r.Changed || !reflect.DeepEqual(p, before) {
		t.Fatal("report mutated project")
	}
	for _, s := range []string{"transit", "180.000", "2.236", "elevations unchanged", "A"} {
		if !strings.Contains(r.Message, s) {
			t.Fatalf("missing %q: %s", s, r.Message)
		}
	}
	r, err = Execute(p, "trav adjust transit")
	if err != nil {
		t.Fatal(err)
	}
	close(t, p.Points["A"].Easting, 30.25)
	close(t, p.Points["A"].Northing, 41)
	if r.Extra == nil {
		t.Fatal("missing structured audit report")
	}
	before = p.Clone()
	if _, err = Execute(p, "trav adjust transit"); err == nil || !reflect.DeepEqual(p, before) {
		t.Fatal("repeated adjustment accepted")
	}
}

func TestTraverseInvalidCommandsAreAtomic(t *testing.T) {
	for _, cmd := range []string{"trav leg N 45 E", "trav leg 90 -2", "trav report invalid", "trav close A"} {
		p := qualityTraverse()
		before := p.Clone()
		if _, err := Execute(p, cmd); err == nil {
			t.Fatalf("accepted %s", cmd)
		}
		if !reflect.DeepEqual(p, before) {
			t.Fatalf("mutated on %s", cmd)
		}
	}
	p := qualityTraverse()
	delete(p.Points, "B")
	before := p.Clone()
	if _, err := Execute(p, "trav adjust compass"); err == nil || !reflect.DeepEqual(p, before) {
		t.Fatal("missing leg not rejected atomically")
	}
}

func TestTraversePointReferences(t *testing.T) {
	p := qualityTraverse()
	if _, err := Execute(p, "pt del B"); err == nil {
		t.Fatal("deleted traverse point")
	}
	if _, err := Execute(p, "pt rename B BB"); err != nil {
		t.Fatal(err)
	}
	if p.Traverse.LegPointIDs[1] != "BB" {
		t.Fatal("rename left dangling traverse")
	}
	if _, err := Execute(p, "trav report compass"); err != nil {
		t.Fatal(err)
	}
}

func TestNonFiniteInputDoesNotCreatePoints(t *testing.T) {
	for _, cmd := range []string{"pt add X NaN 0", "pt add X 0 0 NaN", "rad S NaNd 10 as X", "trav leg NaNd 10"} {
		p := qualityTraverse()
		before := p.Clone()
		if _, err := Execute(p, cmd); err == nil || !reflect.DeepEqual(before, p) {
			t.Fatalf("accepted/mutated on %s", cmd)
		}
	}
}

func TestTransitRejectsAxisWithoutMeasuredComponent(t *testing.T) {
	p := project.New("axis")
	for _, cmd := range []string{"pt add S 0 0", "pt add T 11 1", "trav start S", "trav leg 90 10", "trav close T"} {
		mustExec(t, p, cmd)
	}
	before := p.Clone()
	if _, err := Execute(p, "trav adjust transit"); err == nil || !reflect.DeepEqual(before, p) {
		t.Fatal("floating point cos(90) treated as a measured northing component")
	}
}
