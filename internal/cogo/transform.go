package cogo

import (
	"fmt"
	"math"
	"strings"

	"lsurvey/internal/geom"
	"lsurvey/internal/project"
)

func execResect(p *project.Project, f []string) (Result, error) {
	if len(f) < 9 {
		return Result{}, fmt.Errorf("usage: resect <p1> <brg1> <p2> <brg2> <p3> <brg3> as <id> [code]")
	}
	var points [3]geom.Point
	var bearings [3]geom.Angle
	i := 1
	for obs := 0; obs < 3; obs++ {
		if i >= len(f) {
			return Result{}, fmt.Errorf("usage: resect <p1> <brg1> <p2> <brg2> <p3> <brg3> as <id> [code]")
		}
		pt, err := point(p, f[i])
		if err != nil {
			return Result{}, err
		}
		points[obs] = pt
		i++
		az, used, err := parseAngleTokens(f[i:])
		if err != nil {
			return Result{}, err
		}
		bearings[obs] = az
		i += used
	}
	if i >= len(f) || f[i] != "as" || i+1 >= len(f) {
		return Result{}, fmt.Errorf("resect requires as <id>")
	}
	pt, ok := geom.ResectionByBearings(points, bearings, f[i+1], optional(f, i+2))
	if !ok {
		return Result{}, fmt.Errorf("resection bearings are degenerate")
	}
	if err := storeCreatedPoint(p, pt); err != nil {
		return Result{}, err
	}
	return Result{Message: "created point " + pt.ID, Created: []string{"point:" + pt.ID}}, nil
}

func execShift(p *project.Project, f []string) (Result, error) {
	if len(f) < 3 {
		return Result{}, fmt.Errorf("usage: shift <base> [east=<coordinate>] [north=<coordinate>] [elev=<coordinate>]")
	}
	base, err := point(p, f[1])
	if err != nil {
		return Result{}, err
	}
	var (
		targetE  *float64
		targetN  *float64
		targetZ  *float64
		haveAxis bool
	)
	for _, arg := range f[2:] {
		k, v, ok := strings.Cut(arg, "=")
		if !ok {
			return Result{}, fmt.Errorf("shift argument %q must be key=value", arg)
		}
		switch k {
		case "east", "e", "easting", "x":
			x, err := parseFloat(k, v)
			if err != nil {
				return Result{}, err
			}
			targetE = &x
			haveAxis = true
		case "north", "n", "northing", "y":
			x, err := parseFloat(k, v)
			if err != nil {
				return Result{}, err
			}
			targetN = &x
			haveAxis = true
		case "elev", "z":
			x, err := parseFloat(k, v)
			if err != nil {
				return Result{}, err
			}
			targetZ = &x
			haveAxis = true
		default:
			return Result{}, fmt.Errorf("unknown shift field %q", k)
		}
	}
	if !haveAxis {
		return Result{}, fmt.Errorf("shift requires at least one of east=, north=, or elev=")
	}
	deltaE, deltaN := 0.0, 0.0
	if targetE != nil {
		deltaE = *targetE - base.Easting
	}
	if targetN != nil {
		deltaN = *targetN - base.Northing
	}
	var deltaZ *float64
	if targetZ != nil {
		if base.Elevation == nil {
			return Result{}, fmt.Errorf("shift base point %q has no elevation", base.ID)
		}
		value := *targetZ - *base.Elevation
		deltaZ = &value
	}
	updated := make([]string, 0, len(p.Points)+len(p.ContourSets))
	for id, pt := range p.Points {
		p.Points[id] = geom.ShiftPoint(pt, deltaE, deltaN, deltaZ)
		updated = append(updated, "point:"+id)
	}
	for id, set := range p.ContourSets {
		shiftPolylines(set.Polylines, deltaE, deltaN, deltaZ)
		shiftPolylines(set.RawPolylines, deltaE, deltaN, deltaZ)
		if deltaZ != nil {
			set.Base += *deltaZ
			if set.Generation != nil && set.Generation.Base != nil {
				value := *set.Generation.Base + *deltaZ
				set.Generation.Base = &value
			}
		}
		p.ContourSets[id] = set
		updated = append(updated, "contour:"+id)
	}
	if p.GridGround != nil && p.GridGround.Mode == "local_ground" {
		p.GridGround.AnchorEasting += deltaE
		p.GridGround.AnchorNorthing += deltaN
	}
	return Result{Message: fmt.Sprintf("shifted %d points", len(p.Points)), Updated: updated}, nil
}

func execRotate(p *project.Project, f []string) (Result, error) {
	if len(f) != 3 {
		return Result{}, fmt.Errorf("usage: rotate <base> <bearing>")
	}
	base, err := point(p, f[1])
	if err != nil {
		return Result{}, err
	}
	angle, err := geom.ParseAngle(f[2])
	if err != nil {
		return Result{}, err
	}
	updated := make([]string, 0, len(p.Points)+len(p.ContourSets))
	for id, pt := range p.Points {
		p.Points[id] = geom.RotatePoint(pt, base, angle)
		updated = append(updated, "point:"+id)
	}
	for id, set := range p.ContourSets {
		rotatePolylines(set.Polylines, base, angle)
		rotatePolylines(set.RawPolylines, base, angle)
		p.ContourSets[id] = set
		updated = append(updated, "contour:"+id)
	}
	if p.GridGround != nil && p.GridGround.Mode == "local_ground" {
		anchor := geom.RotatePoint(geom.Point{
			Easting:  p.GridGround.AnchorEasting,
			Northing: p.GridGround.AnchorNorthing,
		}, base, angle)
		p.GridGround.AnchorEasting = anchor.Easting
		p.GridGround.AnchorNorthing = anchor.Northing
	}
	return Result{Message: fmt.Sprintf("rotated %d points by %s", len(p.Points), f[2]), Updated: updated}, nil
}

func shiftPolylines(polylines []project.ContourPolyline, east, north float64, elev *float64) {
	for polyIdx := range polylines {
		if elev != nil {
			polylines[polyIdx].Elevation += *elev
		}
		for vertexIdx := range polylines[polyIdx].Vertices {
			polylines[polyIdx].Vertices[vertexIdx].Easting += east
			polylines[polyIdx].Vertices[vertexIdx].Northing += north
		}
	}
}

func rotatePolylines(polylines []project.ContourPolyline, base geom.Point, angle geom.Angle) {
	for polyIdx := range polylines {
		for vertexIdx := range polylines[polyIdx].Vertices {
			pt := geom.Point{
				Easting:  polylines[polyIdx].Vertices[vertexIdx].Easting,
				Northing: polylines[polyIdx].Vertices[vertexIdx].Northing,
			}
			pt = geom.RotatePoint(pt, base, angle)
			polylines[polyIdx].Vertices[vertexIdx].Easting = pt.Easting
			polylines[polyIdx].Vertices[vertexIdx].Northing = pt.Northing
		}
	}
}

func execScale(p *project.Project, f []string) (Result, error) {
	if len(f) < 2 {
		return Result{}, fmt.Errorf("usage: scale apply <base> csf=<factor> [system=<label>] OR scale reverse")
	}
	switch f[1] {
	case "apply":
		return execScaleApply(p, f)
	case "reverse":
		return execScaleReverse(p, f)
	default:
		return Result{}, fmt.Errorf("usage: scale apply <base> csf=<factor> [system=<label>] OR scale reverse")
	}
}

func execScaleApply(p *project.Project, f []string) (Result, error) {
	const usage = "usage: scale apply <base> csf=<factor> [system=<label>]"
	if len(f) < 4 {
		return Result{}, fmt.Errorf(usage)
	}
	if p.GridGround != nil && p.GridGround.Mode == "local_ground" {
		return Result{}, fmt.Errorf("project already has an applied scale; use scale reverse first")
	}
	base, err := point(p, f[2])
	if err != nil {
		return Result{}, err
	}
	var (
		csf       float64
		haveCSF   bool
		gridLabel string
	)
	for _, arg := range f[3:] {
		key, value, ok := strings.Cut(arg, "=")
		if !ok {
			return Result{}, fmt.Errorf(usage)
		}
		switch key {
		case "csf":
			csf, err = parseFloat("csf", value)
			if err != nil {
				return Result{}, err
			}
			haveCSF = true
		case "system":
			gridLabel = value
		default:
			return Result{}, fmt.Errorf("unknown scale field %q", key)
		}
	}
	if !haveCSF || math.IsNaN(csf) || math.IsInf(csf, 0) || csf <= 0 {
		return Result{}, fmt.Errorf("csf must be a finite number greater than zero")
	}
	p.GridGround = &project.GridGroundConversion{
		Mode:           "local_ground",
		GridSystem:     gridLabel,
		AnchorPointID:  base.ID,
		AnchorEasting:  base.Easting,
		AnchorNorthing: base.Northing,
		CSF:            csf,
	}
	updated := scaleHorizontalGeometry(p, base.Easting, base.Northing, 1/csf)
	return Result{Message: fmt.Sprintf("applied scale to %d points", len(p.Points)), Updated: updated}, nil
}

func execScaleReverse(p *project.Project, f []string) (Result, error) {
	if len(f) != 2 {
		return Result{}, fmt.Errorf("usage: scale reverse")
	}
	if p.GridGround == nil || p.GridGround.Mode != "local_ground" {
		return Result{}, fmt.Errorf("project has no applied scale to reverse")
	}
	conversion := p.GridGround
	updated := scaleHorizontalGeometry(p, conversion.AnchorEasting, conversion.AnchorNorthing, conversion.CSF)
	p.GridGround = nil
	return Result{Message: fmt.Sprintf("reversed scale for %d points", len(p.Points)), Updated: updated}, nil
}

func scaleHorizontalGeometry(p *project.Project, anchorE, anchorN, factor float64) []string {
	updated := make([]string, 0, len(p.Points)+len(p.ContourSets))
	for id, pt := range p.Points {
		pt.Easting = anchorE + (pt.Easting-anchorE)*factor
		pt.Northing = anchorN + (pt.Northing-anchorN)*factor
		p.Points[id] = pt
		updated = append(updated, "point:"+id)
	}
	for id, set := range p.ContourSets {
		scalePolylines(set.Polylines, anchorE, anchorN, factor)
		scalePolylines(set.RawPolylines, anchorE, anchorN, factor)
		if set.EffectiveMaxEdge != 0 {
			set.EffectiveMaxEdge *= factor
		}
		if set.Generation != nil && set.Generation.MaxEdge != nil {
			value := *set.Generation.MaxEdge * factor
			set.Generation.MaxEdge = &value
		}
		for i := range set.Diagnostics {
			if set.Diagnostics[i].Code != "long_edge" {
				continue
			}
			set.Diagnostics[i].Measured *= factor
			set.Diagnostics[i].Limit *= factor
			if len(set.Diagnostics[i].EdgeIDs) >= 2 {
				set.Diagnostics[i].Message = fmt.Sprintf("TIN edge %s-%s length %.3f exceeds warning limit %.3f",
					set.Diagnostics[i].EdgeIDs[0], set.Diagnostics[i].EdgeIDs[1],
					set.Diagnostics[i].Measured, set.Diagnostics[i].Limit)
			}
		}
		p.ContourSets[id] = set
		updated = append(updated, "contour:"+id)
	}
	return updated
}

func scalePolylines(polylines []project.ContourPolyline, anchorE, anchorN, factor float64) {
	for polyIdx := range polylines {
		for vertexIdx := range polylines[polyIdx].Vertices {
			v := &polylines[polyIdx].Vertices[vertexIdx]
			v.Easting = anchorE + (v.Easting-anchorE)*factor
			v.Northing = anchorN + (v.Northing-anchorN)*factor
		}
	}
}

func execTransform(p *project.Project, f []string) (Result, error) {
	const usage = "usage: transform fit <src1> <dst1> <src2> <dst2> [<srcN> <dstN> ...]"
	if len(f) < 6 || f[1] != "fit" || (len(f)-2)%2 != 0 {
		return Result{}, fmt.Errorf(usage)
	}
	pairs := make([]geom.PointPair, 0, (len(f)-2)/2)
	for i := 2; i < len(f); i += 2 {
		source, err := point(p, f[i])
		if err != nil {
			return Result{}, err
		}
		target, err := point(p, f[i+1])
		if err != nil {
			return Result{}, err
		}
		pairs = append(pairs, geom.PointPair{Source: source, Target: target})
	}
	fit, ok := geom.FitSimilarityTransform(pairs)
	if !ok {
		return Result{}, fmt.Errorf("transform fit requires distinct source points and a determinate scale and rotation")
	}
	precision := p.DisplayPrecision()
	dz := "n/a"
	zrms := "n/a"
	if fit.DZ != nil {
		dz = formatDistance(*fit.DZ, precision)
		zrms = formatDistance(*fit.VerticalRMS, precision)
	}
	return Result{Message: fmt.Sprintf(
		"dx=%s dy=%s dz=%s angle=%s scale=%.8f hrms=%s zrms=%s pairs=%d zpairs=%d",
		formatDistance(fit.DX, precision),
		formatDistance(fit.DY, precision),
		dz,
		geom.FormatSignedDMS(fit.RotationDegrees, 2),
		fit.Scale,
		formatDistance(fit.HorizontalRMS, precision),
		zrms,
		fit.PairCount,
		fit.VerticalPairs,
	)}, nil
}
