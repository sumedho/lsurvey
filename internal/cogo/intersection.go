package cogo

import (
	"fmt"

	"lsurvey/internal/geom"
	"lsurvey/internal/project"
)

func execIntersect(p *project.Project, f []string) (Result, error) {
	if len(f) < 2 {
		return Result{}, fmt.Errorf("intersect requires type")
	}
	switch f[1] {
	case "bearing-bearing":
		if len(f) < 8 {
			return Result{}, fmt.Errorf("usage: intersect bearing-bearing <p1> <brg1> <p2> <brg2> as <id> [code]")
		}
		p1, err := point(p, f[2])
		if err != nil {
			return Result{}, err
		}
		az1, used1, err := parseAngleTokens(f[3:])
		if err != nil {
			return Result{}, err
		}
		p2idx := 3 + used1
		p2, err := point(p, f[p2idx])
		if err != nil {
			return Result{}, err
		}
		az2, used2, err := parseAngleTokens(f[p2idx+1:])
		if err != nil {
			return Result{}, err
		}
		asIdx := p2idx + 1 + used2
		if asIdx >= len(f) || f[asIdx] != "as" || asIdx+1 >= len(f) {
			return Result{}, fmt.Errorf("intersect bearing-bearing requires as <id>")
		}
		pt, ok := geom.BearingBearingIntersection(p1, az1, p2, az2, f[asIdx+1], optional(f, asIdx+2))
		if !ok {
			return Result{}, fmt.Errorf("bearings are parallel")
		}
		if err := storeCreatedPoint(p, pt); err != nil {
			return Result{}, err
		}
		return Result{Message: "created point " + pt.ID, Created: []string{"point:" + pt.ID}}, nil
	case "distance-distance":
		if len(f) < 10 || f[8] != "as" {
			return Result{}, fmt.Errorf("usage: intersect distance-distance <p1> <dist1> <p2> <dist2> choose left|right as <id> [code]")
		}
		d1, err := parseFloat("distance1", f[3])
		if err != nil {
			return Result{}, err
		}
		d2, err := parseFloat("distance2", f[5])
		if err != nil {
			return Result{}, err
		}
		if f[6] != "choose" || (f[7] != "left" && f[7] != "right") {
			return Result{}, fmt.Errorf("distance-distance requires choose left|right")
		}
		p1, err := point(p, f[2])
		if err != nil {
			return Result{}, err
		}
		p2, err := point(p, f[4])
		if err != nil {
			return Result{}, err
		}
		pt, ok := geom.DistanceDistanceIntersection(p1, d1, p2, d2, f[7], f[9], optional(f, 10))
		if !ok {
			return Result{}, fmt.Errorf("circles do not intersect")
		}
		if err := storeCreatedPoint(p, pt); err != nil {
			return Result{}, err
		}
		return Result{Message: "created point " + pt.ID, Created: []string{"point:" + pt.ID}}, nil
	case "bearing-distance":
		if len(f) < 10 {
			return Result{}, fmt.Errorf("usage: intersect bearing-distance <p1> <brg> <p2> <dist> choose near|far as <id> [code]")
		}
		p1, err := point(p, f[2])
		if err != nil {
			return Result{}, err
		}
		az, used, err := parseAngleTokens(f[3:])
		if err != nil {
			return Result{}, err
		}
		p2idx := 3 + used
		p2, err := point(p, f[p2idx])
		if err != nil {
			return Result{}, err
		}
		distIdx := p2idx + 1
		dist, err := parseFloat("distance", f[distIdx])
		if err != nil {
			return Result{}, err
		}
		if distIdx+4 >= len(f) || f[distIdx+1] != "choose" || f[distIdx+3] != "as" {
			return Result{}, fmt.Errorf("bearing-distance requires choose near|far as <id>")
		}
		choice := f[distIdx+2]
		if choice != "near" && choice != "far" && choice != "left" && choice != "right" {
			return Result{}, fmt.Errorf("bearing-distance choice must be near|far")
		}
		pt, ok := geom.BearingDistanceIntersection(p1, az, p2, dist, choice, f[distIdx+4], optional(f, distIdx+5))
		if !ok {
			return Result{}, fmt.Errorf("bearing and distance do not intersect")
		}
		if err := storeCreatedPoint(p, pt); err != nil {
			return Result{}, err
		}
		return Result{Message: "created point " + pt.ID, Created: []string{"point:" + pt.ID}}, nil
	default:
		return Result{}, fmt.Errorf("unknown intersect type %q", f[1])
	}
}
