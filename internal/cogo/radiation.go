package cogo

import (
	"fmt"

	"lsurvey/internal/geom"
	"lsurvey/internal/project"
)

func execRad(p *project.Project, f []string) (Result, error) {
	if len(f) < 6 {
		return Result{}, fmt.Errorf("usage: rad <from> <azimuth|bearing> <distance> [vdiff <delta>] as <id> [code]")
	}
	from, err := point(p, f[1])
	if err != nil {
		return Result{}, err
	}
	az, used, err := parseAngleTokens(f[2:])
	if err != nil {
		return Result{}, err
	}
	i := 2 + used
	dist, err := parseFloat("distance", f[i])
	if err != nil {
		return Result{}, err
	}
	i++
	var dz *float64
	if i < len(f) && (f[i] == "vdiff" || f[i] == "dz") {
		if i+1 >= len(f) {
			return Result{}, fmt.Errorf("%s requires value", f[i])
		}
		v, err := parseFloat(f[i], f[i+1])
		if err != nil {
			return Result{}, err
		}
		dz = &v
		i += 2
	}
	if i >= len(f) || f[i] != "as" || i+1 >= len(f) {
		return Result{}, fmt.Errorf("rad requires as <id>")
	}
	id := f[i+1]
	code := optional(f, i+2)
	pt := geom.Radiate(from, az, dist, dz, id, code)
	if err := storeCreatedPoint(p, pt); err != nil {
		return Result{}, err
	}
	return staleContours(Result{Message: "created point " + id, Created: []string{"point:" + id}}, "point geometry changed"), nil
}

func execRad3D(p *project.Project, f []string) (Result, error) {
	if len(f) < 7 {
		return Result{}, fmt.Errorf("usage: rad3d <from> <azimuth|bearing> <slope_distance> <zenith> as <id> [code]")
	}
	from, err := point(p, f[1])
	if err != nil {
		return Result{}, err
	}
	if from.Elevation == nil {
		return Result{}, fmt.Errorf("rad3d requires start point elevation")
	}
	az, used, err := parseAngleTokens(f[2:])
	if err != nil {
		return Result{}, err
	}
	i := 2 + used
	if i >= len(f) {
		return Result{}, fmt.Errorf("rad3d requires slope distance")
	}
	slopeDist, err := parseFloat("slope distance", f[i])
	if err != nil {
		return Result{}, err
	}
	i++
	if i >= len(f) {
		return Result{}, fmt.Errorf("rad3d requires zenith angle")
	}
	zenith, err := geom.ParseAngle(f[i])
	if err != nil {
		return Result{}, err
	}
	i++
	if i >= len(f) || f[i] != "as" || i+1 >= len(f) {
		return Result{}, fmt.Errorf("rad3d requires as <id>")
	}
	id := f[i+1]
	code := optional(f, i+2)
	pt := geom.Radiate3D(from, az, slopeDist, zenith, id, code)
	if err := storeCreatedPoint(p, pt); err != nil {
		return Result{}, err
	}
	return staleContours(Result{Message: "created point " + id, Created: []string{"point:" + id}}, "point geometry changed"), nil
}

func execMidpoint(p *project.Project, f []string) (Result, error) {
	if len(f) < 6 || f[3] != "as" {
		return Result{}, fmt.Errorf("usage: midpoint <p1> <p2> as <id> [code]")
	}
	p1, err := point(p, f[1])
	if err != nil {
		return Result{}, err
	}
	p2, err := point(p, f[2])
	if err != nil {
		return Result{}, err
	}
	pt := geom.Midpoint(p1, p2, f[4], optional(f, 5))
	if err := storeCreatedPoint(p, pt); err != nil {
		return Result{}, err
	}
	return staleContours(Result{Message: "created point " + pt.ID, Created: []string{"point:" + pt.ID}}, "point geometry changed"), nil
}

func execOffset(p *project.Project, f []string) (Result, error) {
	if len(f) >= 6 && f[4] == "as" {
		return execOffsetFromLine(p, f)
	}
	if len(f) >= 7 && f[5] == "as" {
		return execOffsetFromPoints(p, f)
	}
	return Result{}, fmt.Errorf("usage: offset <p1> <p2> <offset> <chainage> as <id> [code] OR offset <line_id> <offset> <chainage> as <id> [code]")
}

func execOffsetFromPoints(p *project.Project, f []string) (Result, error) {
	off, err := parseFloat("offset", f[3])
	if err != nil {
		return Result{}, err
	}
	chainage, err := parseFloat("chainage", f[4])
	if err != nil {
		return Result{}, err
	}
	p1, err := point(p, f[1])
	if err != nil {
		return Result{}, err
	}
	p2, err := point(p, f[2])
	if err != nil {
		return Result{}, err
	}
	pt := geom.Offset(p1, p2, off, chainage, f[6], optional(f, 7))
	if err := storeCreatedPoint(p, pt); err != nil {
		return Result{}, err
	}
	return staleContours(Result{Message: "created point " + pt.ID, Created: []string{"point:" + pt.ID}}, "point geometry changed"), nil
}

func execOffsetFromLine(p *project.Project, f []string) (Result, error) {
	line, ok := p.Features[f[1]]
	if !ok || line.Kind != project.FeatureLine {
		return Result{}, fmt.Errorf("line %q not found", f[1])
	}
	off, err := parseFloat("offset", f[2])
	if err != nil {
		return Result{}, err
	}
	chainage, err := parseFloat("chainage", f[3])
	if err != nil {
		return Result{}, err
	}
	p1, err := point(p, line.PointIDs[0])
	if err != nil {
		return Result{}, err
	}
	p2, err := point(p, line.PointIDs[1])
	if err != nil {
		return Result{}, err
	}
	pt := geom.Offset(p1, p2, off, chainage, f[5], optional(f, 6))
	if err := storeCreatedPoint(p, pt); err != nil {
		return Result{}, err
	}
	return staleContours(Result{Message: "created point " + pt.ID, Created: []string{"point:" + pt.ID}}, "point geometry changed"), nil
}
