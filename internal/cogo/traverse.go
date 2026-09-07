package cogo

import (
	"fmt"
	"strings"

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
		return changed(Result{Message: "started traverse at " + f[2]}), nil
	case "leg":
		if p.Traverse == nil {
			return Result{}, fmt.Errorf("no active traverse")
		}
		if p.Traverse.Adjusted {
			return Result{}, fmt.Errorf("traverse already adjusted; start a new traverse")
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
		if i >= len(f) {
			return Result{}, fmt.Errorf("traverse leg requires distance")
		}
		dist, err := parseFloat("distance", f[i])
		if err != nil {
			return Result{}, err
		}
		if dist <= 0 {
			return Result{}, fmt.Errorf("traverse leg distance must be positive")
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
		if i+1 < len(f) {
			return Result{}, fmt.Errorf("unexpected traverse leg arguments")
		}
		id := p.NextPointID()
		pt := geom.Radiate(from, az, dist, dz, id, code)
		if err := storeCreatedPoint(p, pt); err != nil {
			return Result{}, err
		}
		p.Traverse.Current = id
		p.Traverse.Close = ""
		p.Traverse.LegPointIDs = append(p.Traverse.LegPointIDs, id)
		return staleContours(Result{Message: "created traverse point " + id + " current=" + p.Traverse.Current, Created: []string{"point:" + id}}, "point geometry changed"), nil
	case "close":
		if p.Traverse == nil {
			return Result{}, fmt.Errorf("no active traverse")
		}
		if len(f) != 3 {
			return Result{}, fmt.Errorf("usage: trav close <known_point>")
		}
		if p.Traverse.Adjusted {
			return Result{}, fmt.Errorf("traverse already adjusted; start a new traverse")
		}
		for _, id := range p.Traverse.LegPointIDs {
			if id == f[2] {
				return Result{}, fmt.Errorf("closing control must not be a traverse leg point")
			}
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
		return changed(Result{Message: fmt.Sprintf("misclose az=%s hd=%s", misclose.Azimuth.FormatDMS(2), formatDistance(misclose.HorizontalDistance, p.DisplayPrecision()))}), nil
	case "show":
		if p.Traverse == nil {
			return Result{}, fmt.Errorf("no active traverse")
		}
		message := fmt.Sprintf("traverse start=%s current=%s legs=%d next=%s", p.Traverse.Start, p.Traverse.Current, len(p.Traverse.LegPointIDs), p.NextPointID())
		if p.Traverse.Close != "" {
			message += " close=" + p.Traverse.Close
		}
		return Result{Message: message}, nil
	case "report", "adjust":
		if p.Traverse == nil || p.Traverse.Close == "" {
			return Result{}, fmt.Errorf("traverse must be closed before adjustment")
		}
		method := "compass"
		if len(f) == 3 {
			method = f[2]
		} else if len(f) != 2 || f[1] == "adjust" {
			return Result{}, fmt.Errorf("usage: trav %s compass|transit", f[1])
		}
		return adjustTraverse(p, method, f[1] == "report")
	default:
		return Result{}, fmt.Errorf("unknown trav subcommand %q", f[1])
	}
}

func adjustTraverse(p *project.Project, method string, preview bool) (Result, error) {
	if p.Traverse.Adjusted {
		return Result{}, fmt.Errorf("traverse already adjusted; original QA is in history; start a new traverse")
	}
	ids := append([]string{p.Traverse.Start}, p.Traverse.LegPointIDs...)
	if len(ids) < 2 || p.Traverse.Current != ids[len(ids)-1] {
		return Result{}, fmt.Errorf("invalid traverse endpoint or no legs")
	}
	seen := map[string]bool{}
	points := make([]geom.Point, 0, len(ids))
	for i, id := range ids {
		if seen[id] || i > 0 && id == p.Traverse.Close {
			return Result{}, fmt.Errorf("traverse repeats a point or includes closing control")
		}
		seen[id] = true
		pt, err := point(p, id)
		if err != nil {
			return Result{}, err
		}
		points = append(points, pt)
	}
	target, err := point(p, p.Traverse.Close)
	if err != nil {
		return Result{}, err
	}
	r, err := geom.AdjustTraverse(points, target, method)
	if err != nil {
		return Result{}, err
	}
	var b strings.Builder
	precision := p.DisplayPrecision()
	display := func(v float64) string { return formatDistance(v, precision) }
	fmt.Fprintf(&b, "traverse %s QA: start=%s target=%s units=%s length=%s misclose hd=%s correction east=%s north=%s", method, r.Start.ID, r.Target.ID, p.Units["distance"], display(r.Length), display(r.Misclosure), display(r.CorrectionE), display(r.CorrectionN))
	if r.RelativePrecision != nil {
		fmt.Fprintf(&b, " relative precision=1:%.0f", *r.RelativePrecision)
	} else {
		b.WriteString(" relative precision=undefined (zero misclosure or numeric range)")
	}
	b.WriteString("\nHorizontal coordinate adjustment only; elevations unchanged; no angular closure or uncertainty estimate.\nfrom to length correction_e correction_n adjusted_e adjusted_n")
	for i, leg := range r.Legs {
		fmt.Fprintf(&b, "\n%s %s %s %s %s %s %s", leg.From, leg.To, display(leg.Length), display(leg.CorrectionE), display(leg.CorrectionN), display(r.Points[i].Easting), display(r.Points[i].Northing))
	}
	if preview {
		return Result{Message: b.String(), Extra: r}, nil
	}
	updated := make([]string, 0, len(r.Points))
	for _, pt := range r.Points {
		p.Points[pt.ID] = pt
		updated = append(updated, "point:"+pt.ID)
	}
	p.Traverse.Adjusted = true
	return staleContours(Result{Message: fmt.Sprintf("adjusted %d traverse points\n%s", len(r.Points), b.String()), Updated: updated, Extra: r}, "point geometry changed"), nil
}
