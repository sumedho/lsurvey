package project

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"testing"

	"lsurvey/internal/geom"
)

func validIntegrityProject() *Project {
	p := New("integrity")
	p.Points["A"] = geom.Point{ID: "A"}
	p.Points["B"] = geom.Point{ID: "B", Easting: 10}
	p.Features["L"] = Feature{ID: "L", Kind: FeatureLine, PointIDs: []string{"A", "B"}}
	return p
}

func TestIntegrityRejectsInvalidProjects(t *testing.T) {
	for name, damage := range map[string]func(*Project){
		"point identity":   func(p *Project) { p.Points["X"] = geom.Point{ID: "Y"} },
		"nonfinite":        func(p *Project) { p.Points["X"] = geom.Point{ID: "X", Easting: math.Inf(1)} },
		"dangling feature": func(p *Project) { delete(p.Points, "B") },
		"unknown kind":     func(p *Project) { p.Features["X"] = Feature{ID: "X", Kind: "arc", PointIDs: []string{"A", "B"}} },
		"duplicate vertex": func(p *Project) { p.Features["X"] = Feature{ID: "X", Kind: FeatureLine, PointIDs: []string{"A", "A"}} },
		"missing group":    func(p *Project) { p.PointCodeStyles["PEG"] = "missing" },
		"duplicate layer": func(p *Project) {
			p.Groups["a"] = Group{ID: "a", Layer: "PEG", Color: 1}
			p.Groups["b"] = Group{ID: "b", Layer: "peg", Color: 2}
		},
		"color":     func(p *Project) { p.Groups["a"] = Group{ID: "a", Layer: "PEG", Color: 256} },
		"CRS":       func(p *Project) { p.HorizontalCRS = &HorizontalCRS{Datum: "GDA2020", Projection: "MGA", Zone: 0} },
		"scale":     func(p *Project) { p.GridGround = &GridGroundConversion{Mode: "grid_to_ground", CSF: 0} },
		"traverse":  func(p *Project) { p.Traverse = &TraverseState{Start: "A", Current: "missing"} },
		"contour":   func(p *Project) { p.ContourSets["C"] = ContourSet{ID: "C", Interval: -1} },
		"precision": func(p *Project) { v := 9; p.Display.Precision = &v },
	} {
		t.Run(name, func(t *testing.T) {
			p := validIntegrityProject()
			damage(p)
			if err := p.Validate(); err == nil {
				t.Fatal("accepted invalid project")
			}
		})
	}
}

func TestLoadValidatesBeforeAccepting(t *testing.T) {
	p := validIntegrityProject()
	delete(p.Points, "A")
	data, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "bad.srv")
	if err = os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err = Load(path); err == nil {
		t.Fatal("accepted dangling feature")
	}
}

func TestStaleContourMayRetainDeletedSourceReferences(t *testing.T) {
	p := validIntegrityProject()
	p.ContourSets["C"] = ContourSet{ID: "C", Interval: 1, Stale: true, SourcePoints: []string{"deleted"}, Breaklines: []string{"deleted"}}
	if err := p.Validate(); err != nil {
		t.Fatal(err)
	}
}
