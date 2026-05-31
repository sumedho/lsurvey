package cogo

import (
	"fmt"

	"lsurvey/internal/geom"
	"lsurvey/internal/project"
)

func execInverse(p *project.Project, f []string) (Result, error) {
	if len(f) != 3 {
		return Result{}, fmt.Errorf("usage: inverse <from> <to>")
	}
	from, err := point(p, f[1])
	if err != nil {
		return Result{}, err
	}
	to, err := point(p, f[2])
	if err != nil {
		return Result{}, err
	}
	inv := geom.Inverse(from, to)
	precision := p.DisplayPrecision()
	msg := fmt.Sprintf("az=%s hd=%s de=%s dn=%s", inv.Azimuth.FormatDMS(2), formatDistance(inv.HorizontalDistance, precision), formatDistance(inv.DeltaEasting, precision), formatDistance(inv.DeltaNorthing, precision))
	if inv.DeltaElevation != nil {
		msg += fmt.Sprintf(" dz=%s sd=%s", formatDistance(*inv.DeltaElevation, precision), formatDistance(*inv.SlopeDistance, precision))
	}
	return Result{Message: msg}, nil
}

func execAngle(p *project.Project, f []string) (Result, error) {
	if len(f) != 4 {
		return Result{}, fmt.Errorf("usage: angle <back> <vertex> <forward>")
	}
	back, err := point(p, f[1])
	if err != nil {
		return Result{}, err
	}
	vertex, err := point(p, f[2])
	if err != nil {
		return Result{}, err
	}
	forward, err := point(p, f[3])
	if err != nil {
		return Result{}, err
	}
	result, ok := geom.AngleBetween(back, vertex, forward)
	if !ok {
		return Result{}, fmt.Errorf("angle legs must have non-zero length")
	}
	return Result{Message: fmt.Sprintf("inside=%s outside=%s", result.Inside.FormatDMS(2), result.Outside.FormatDMS(2))}, nil
}

func execClose(p *project.Project, f []string) (Result, error) {
	if len(f) < 4 {
		return Result{}, fmt.Errorf("usage: close <p1> <p2> <p3> ...")
	}
	points := make([]geom.Point, 0, len(f)-1)
	for _, id := range f[1:] {
		pt, err := point(p, id)
		if err != nil {
			return Result{}, err
		}
		points = append(points, pt)
	}
	result, ok := geom.Close(points)
	if !ok {
		return Result{}, fmt.Errorf("close requires at least 3 points")
	}
	accuracy := "perfect"
	if result.Misclose.HorizontalDistance > 0 {
		accuracy = fmt.Sprintf("1:%.0f", result.Perimeter/result.Misclose.HorizontalDistance)
	}
	return Result{
		Message: fmt.Sprintf(
			"area=%s misclose az=%s hd=%s de=%s dn=%s accuracy=%s",
			formatDistance(result.Area, p.DisplayPrecision()),
			result.Misclose.Azimuth.FormatDMS(2),
			formatDistance(result.Misclose.HorizontalDistance, p.DisplayPrecision()),
			formatDistance(result.Misclose.DeltaEasting, p.DisplayPrecision()),
			formatDistance(result.Misclose.DeltaNorthing, p.DisplayPrecision()),
			accuracy,
		),
	}, nil
}

func execBearing(_ *project.Project, f []string) (Result, error) {
	if len(f) != 4 {
		return Result{}, fmt.Errorf("usage: bearing add|sub <a> <b>")
	}
	a, err := geom.ParseAngle(f[2])
	if err != nil {
		return Result{}, err
	}
	b, err := geom.ParseAngle(f[3])
	if err != nil {
		return Result{}, err
	}
	var result geom.Angle
	switch f[1] {
	case "add":
		result = geom.AngleFromDegrees(a.Degrees() + b.Degrees())
	case "sub":
		result = geom.AngleFromDegrees(a.Degrees() - b.Degrees())
	default:
		return Result{}, fmt.Errorf("unknown bearing subcommand %q", f[1])
	}
	return Result{Message: "bearing=" + result.FormatDMS(2)}, nil
}

func execDistance(p *project.Project, f []string) (Result, error) {
	if len(f) != 4 {
		return Result{}, fmt.Errorf("usage: dist add|sub <a> <b>")
	}
	a, err := parseFloat("distance", f[2])
	if err != nil {
		return Result{}, err
	}
	b, err := parseFloat("distance", f[3])
	if err != nil {
		return Result{}, err
	}
	var result float64
	switch f[1] {
	case "add":
		result = a + b
	case "sub":
		result = a - b
	default:
		return Result{}, fmt.Errorf("unknown dist subcommand %q", f[1])
	}
	return Result{Message: "dist=" + formatDistance(result, p.DisplayPrecision())}, nil
}
