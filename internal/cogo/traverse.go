package cogo

import (
	"fmt"

	"lsurvey/internal/geom"
	"lsurvey/internal/project"
)

func execTraverse(p *project.Project, f []string) (Result, error) {
	if len(f) < 2 {
		return Result{}, fmt.Errorf("trav requires subcommand")
	}
	switch f[1] {
	case "start":
		if len(f) != 3 {
			return Result{}, fmt.Errorf("usage: trav start <point>")
		}
		if _, err := point(p, f[2]); err != nil {
			return Result{}, err
		}
		p.Traverse = &project.TraverseState{Start: f[2], Current: f[2], LegPointIDs: []string{}}
		return Result{Message: "started traverse at " + f[2]}, nil
	case "leg":
		if p.Traverse == nil {
			return Result{}, fmt.Errorf("no active traverse")
		}
		if len(f) < 4 {
			return Result{}, fmt.Errorf("usage: trav leg <azimuth|bearing> <distance> [vdiff <delta>] [code]")
		}
		from, err := point(p, p.Traverse.Current)
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
		code := optional(f, i)
		id := p.NextPointID()
		pt := geom.Radiate(from, az, dist, dz, id, code)
		if err := storeCreatedPoint(p, pt); err != nil {
			return Result{}, err
		}
		p.Traverse.Current = id
		p.Traverse.Close = ""
		p.Traverse.LegPointIDs = append(p.Traverse.LegPointIDs, id)
		return Result{Message: "created traverse point " + id + " current=" + p.Traverse.Current, Created: []string{"point:" + id}}, nil
	case "close":
		if p.Traverse == nil {
			return Result{}, fmt.Errorf("no active traverse")
		}
		if len(f) != 3 {
			return Result{}, fmt.Errorf("usage: trav close <known_point>")
		}
		current, err := point(p, p.Traverse.Current)
		if err != nil {
			return Result{}, err
		}
		known, err := point(p, f[2])
		if err != nil {
			return Result{}, err
		}
		p.Traverse.Close = f[2]
		misclose := geom.Inverse(current, known)
		return Result{Message: fmt.Sprintf("misclose az=%s hd=%s", misclose.Azimuth.FormatDMS(2), formatDistance(misclose.HorizontalDistance, p.DisplayPrecision()))}, nil
	case "show":
		if p.Traverse == nil {
			return Result{}, fmt.Errorf("no active traverse")
		}
		message := fmt.Sprintf("traverse start=%s current=%s legs=%d next=%s", p.Traverse.Start, p.Traverse.Current, len(p.Traverse.LegPointIDs), p.NextPointID())
		if p.Traverse.Close != "" {
			message += " close=" + p.Traverse.Close
		}
		return Result{Message: message}, nil
	case "adjust":
		if p.Traverse == nil || p.Traverse.Close == "" {
			return Result{}, fmt.Errorf("traverse must be closed before adjustment")
		}
		if len(f) != 3 || (f[2] != "compass" && f[2] != "transit") {
			return Result{}, fmt.Errorf("usage: trav adjust compass|transit")
		}
		return adjustTraverse(p)
	default:
		return Result{}, fmt.Errorf("unknown trav subcommand %q", f[1])
	}
}

func adjustTraverse(p *project.Project) (Result, error) {
	current, err := point(p, p.Traverse.Current)
	if err != nil {
		return Result{}, err
	}
	closePt, err := point(p, p.Traverse.Close)
	if err != nil {
		return Result{}, err
	}
	count := len(p.Traverse.LegPointIDs)
	if count == 0 {
		return Result{}, fmt.Errorf("traverse has no legs")
	}
	dn := closePt.Northing - current.Northing
	de := closePt.Easting - current.Easting
	var dz *float64
	if current.Elevation != nil && closePt.Elevation != nil {
		v := *closePt.Elevation - *current.Elevation
		dz = &v
	}
	updated := make([]string, 0, count)
	for i, id := range p.Traverse.LegPointIDs {
		pt := p.Points[id]
		frac := float64(i+1) / float64(count)
		pt.Northing += dn * frac
		pt.Easting += de * frac
		if dz != nil && pt.Elevation != nil {
			v := *pt.Elevation + *dz*frac
			pt.Elevation = &v
		}
		p.Points[id] = pt
		updated = append(updated, "point:"+id)
	}
	p.Traverse.Current = p.Traverse.Close
	return Result{Message: fmt.Sprintf("adjusted %d traverse points", count), Updated: updated}, nil
}
