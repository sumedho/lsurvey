package cogo

import (
	"fmt"
	"strconv"
	"strings"

	"lsurvey/internal/geom"
	"lsurvey/internal/project"
	"lsurvey/internal/terrain"
)

type Result struct {
	Message string
	Created []string
	Updated []string
}

func Execute(p *project.Project, command string) (Result, error) {
	fields, err := tokenize(command)
	if err != nil {
		return Result{}, err
	}
	if len(fields) == 0 {
		return Result{}, fmt.Errorf("empty command")
	}
	switch fields[0] {
	case "pt":
		return execPoint(p, fields)
	case "line":
		return execLine(p, fields)
	case "inverse":
		return execInverse(p, fields)
	case "angle":
		return execAngle(p, fields)
	case "close":
		return execClose(p, fields)
	case "bearing":
		return execBearing(p, fields)
	case "dist":
		return execDistance(p, fields)
	case "rad":
		return execRad(p, fields)
	case "rad3d":
		return execRad3D(p, fields)
	case "midpoint":
		return execMidpoint(p, fields)
	case "offset":
		return execOffset(p, fields)
	case "intersect":
		return execIntersect(p, fields)
	case "resect":
		return execResect(p, fields)
	case "shift":
		return execShift(p, fields)
	case "rotate":
		return execRotate(p, fields)
	case "trav":
		return execTraverse(p, fields)
	case "contour":
		return execContour(p, fields)
	case "units":
		return execUnits(p, fields)
	default:
		return Result{}, fmt.Errorf("unknown command %q", fields[0])
	}
}

func ExecuteAndRecord(p *project.Project, command string) (Result, error) {
	result, err := Execute(p, command)
	p.AddHistory(command, result.Message, result.Created, err)
	return result, err
}

func execPoint(p *project.Project, f []string) (Result, error) {
	if len(f) < 2 {
		return Result{}, fmt.Errorf("pt requires subcommand")
	}
	switch f[1] {
	case "add":
		if len(f) < 5 {
			return Result{}, fmt.Errorf("usage: pt add <id> <east> <north> [elev] [code]")
		}
		id := f[2]
		if _, ok := p.Points[id]; ok {
			return Result{}, fmt.Errorf("point %q already exists", id)
		}
		e, err := parseFloat("easting", f[3])
		if err != nil {
			return Result{}, err
		}
		n, err := parseFloat("northing", f[4])
		if err != nil {
			return Result{}, err
		}
		var z *float64
		code := ""
		if len(f) >= 6 {
			if v, err := strconv.ParseFloat(f[5], 64); err == nil {
				z = &v
				if len(f) >= 7 {
					code = f[6]
				}
			} else {
				code = f[5]
			}
		}
		p.Points[id] = geom.Point{ID: id, Easting: e, Northing: n, Elevation: z, Code: code}
		return Result{Message: "added point " + id, Created: []string{"point:" + id}}, nil
	case "edit":
		if len(f) < 4 {
			return Result{}, fmt.Errorf("usage: pt edit <id> [east=] [north=] [elev=] [code=] [desc=]")
		}
		id := f[2]
		pt, ok := p.Points[id]
		if !ok {
			return Result{}, fmt.Errorf("point %q not found", id)
		}
		for _, arg := range f[3:] {
			k, v, ok := strings.Cut(arg, "=")
			if !ok {
				return Result{}, fmt.Errorf("edit argument %q must be key=value", arg)
			}
			switch k {
			case "north", "n", "northing":
				x, err := parseFloat(k, v)
				if err != nil {
					return Result{}, err
				}
				pt.Northing = x
			case "east", "e", "easting":
				x, err := parseFloat(k, v)
				if err != nil {
					return Result{}, err
				}
				pt.Easting = x
			case "elev", "z":
				x, err := parseFloat(k, v)
				if err != nil {
					return Result{}, err
				}
				pt.Elevation = &x
			case "code":
				pt.Code = v
			case "desc":
				pt.Description = v
			default:
				return Result{}, fmt.Errorf("unknown point field %q", k)
			}
		}
		p.Points[id] = pt
		return Result{Message: "updated point " + id, Updated: []string{"point:" + id}}, nil
	case "del":
		if len(f) != 3 {
			return Result{}, fmt.Errorf("usage: pt del <id>")
		}
		id := f[2]
		if _, ok := p.Points[id]; !ok {
			return Result{}, fmt.Errorf("point %q not found", id)
		}
		for _, line := range p.Lines {
			if line.From == id || line.To == id {
				return Result{}, fmt.Errorf("point %q is used by line %q", id, line.ID)
			}
		}
		delete(p.Points, id)
		return Result{Message: "deleted point " + id, Updated: []string{"point:" + id}}, nil
	case "rename":
		if len(f) != 4 {
			return Result{}, fmt.Errorf("usage: pt rename <old> <new>")
		}
		pt, ok := p.Points[f[2]]
		if !ok {
			return Result{}, fmt.Errorf("point %q not found", f[2])
		}
		if _, ok := p.Points[f[3]]; ok {
			return Result{}, fmt.Errorf("point %q already exists", f[3])
		}
		delete(p.Points, f[2])
		pt.ID = f[3]
		p.Points[f[3]] = pt
		for id, line := range p.Lines {
			if line.From == f[2] {
				line.From = f[3]
			}
			if line.To == f[2] {
				line.To = f[3]
			}
			p.Lines[id] = line
		}
		return Result{Message: "renamed point " + f[2] + " to " + f[3], Updated: []string{"point:" + f[3]}}, nil
	case "list":
		return Result{Message: fmt.Sprintf("%d points", len(p.Points))}, nil
	default:
		return Result{}, fmt.Errorf("unknown pt subcommand %q", f[1])
	}
}

func execLine(p *project.Project, f []string) (Result, error) {
	if len(f) < 2 {
		return Result{}, fmt.Errorf("line requires subcommand")
	}
	switch f[1] {
	case "add":
		if len(f) < 5 {
			return Result{}, fmt.Errorf("usage: line add <id> <p1> <p2> [code]")
		}
		if _, ok := p.Lines[f[2]]; ok {
			return Result{}, fmt.Errorf("line %q already exists", f[2])
		}
		if _, ok := p.Points[f[3]]; !ok {
			return Result{}, fmt.Errorf("point %q not found", f[3])
		}
		if _, ok := p.Points[f[4]]; !ok {
			return Result{}, fmt.Errorf("point %q not found", f[4])
		}
		code := ""
		if len(f) >= 6 {
			code = f[5]
		}
		p.Lines[f[2]] = project.Line{ID: f[2], From: f[3], To: f[4], Code: code}
		return Result{Message: "added line " + f[2], Created: []string{"line:" + f[2]}}, nil
	case "gen":
		if len(f) != 3 {
			return Result{}, fmt.Errorf("usage: line gen <code>")
		}
		return genLinesByCode(p, f[2])
	case "del":
		if len(f) != 3 {
			return Result{}, fmt.Errorf("usage: line del <id>")
		}
		if _, ok := p.Lines[f[2]]; !ok {
			return Result{}, fmt.Errorf("line %q not found", f[2])
		}
		delete(p.Lines, f[2])
		return Result{Message: "deleted line " + f[2], Updated: []string{"line:" + f[2]}}, nil
	case "edit":
		if len(f) < 4 {
			return Result{}, fmt.Errorf("usage: line edit <id> [from=] [to=] [code=] [desc=]")
		}
		id := f[2]
		line, ok := p.Lines[id]
		if !ok {
			return Result{}, fmt.Errorf("line %q not found", id)
		}
		for _, arg := range f[3:] {
			k, v, ok := strings.Cut(arg, "=")
			if !ok {
				return Result{}, fmt.Errorf("edit argument %q must be key=value", arg)
			}
			switch k {
			case "from":
				if _, ok := p.Points[v]; !ok {
					return Result{}, fmt.Errorf("point %q not found", v)
				}
				line.From = v
			case "to":
				if _, ok := p.Points[v]; !ok {
					return Result{}, fmt.Errorf("point %q not found", v)
				}
				line.To = v
			case "code":
				line.Code = v
			case "desc":
				line.Description = v
			default:
				return Result{}, fmt.Errorf("unknown line field %q", k)
			}
		}
		p.Lines[id] = line
		return Result{Message: "updated line " + id, Updated: []string{"line:" + id}}, nil
	case "intersect":
		if len(f) < 8 || f[6] != "as" {
			return Result{}, fmt.Errorf("usage: line intersect <a1> <a2> <b1> <b2> as <id> [code]")
		}
		code := optional(f, 8)
		a1, err := point(p, f[2])
		if err != nil {
			return Result{}, err
		}
		a2, err := point(p, f[3])
		if err != nil {
			return Result{}, err
		}
		b1, err := point(p, f[4])
		if err != nil {
			return Result{}, err
		}
		b2, err := point(p, f[5])
		if err != nil {
			return Result{}, err
		}
		pt, ok := geom.LineIntersection(a1, a2, b1, b2, f[7], code)
		if !ok {
			return Result{}, fmt.Errorf("lines are parallel")
		}
		p.Points[pt.ID] = pt
		return Result{Message: "created point " + pt.ID, Created: []string{"point:" + pt.ID}}, nil
	case "list":
		return Result{Message: fmt.Sprintf("%d lines", len(p.Lines))}, nil
	default:
		return Result{}, fmt.Errorf("unknown line subcommand %q", f[1])
	}
}

func genLinesByCode(p *project.Project, code string) (Result, error) {
	points := p.SortedPoints()
	matching := make([]geom.Point, 0, len(points))
	for _, pt := range points {
		if pt.Code == code {
			matching = append(matching, pt)
		}
	}
	if len(matching) < 2 {
		return Result{}, fmt.Errorf("line gen requires at least 2 points with code %q", code)
	}

	created := make([]string, 0, len(matching)-1)
	skipped := 0
	for i := 0; i < len(matching)-1; i++ {
		from := matching[i].ID
		to := matching[i+1].ID
		if hasLineBetween(p, from, to) {
			skipped++
			continue
		}
		id := p.NextLineID()
		p.Lines[id] = project.Line{ID: id, From: from, To: to, Code: code}
		created = append(created, "line:"+id)
	}
	return Result{
		Message: fmt.Sprintf("generated %d lines for code %s, skipped %d duplicates", len(created), code, skipped),
		Created: created,
	}, nil
}

func hasLineBetween(p *project.Project, a, b string) bool {
	for _, line := range p.Lines {
		if (line.From == a && line.To == b) || (line.From == b && line.To == a) {
			return true
		}
	}
	return false
}

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
			"area=%s misclose az=%s hd=%s accuracy=%s",
			formatDistance(result.Area, p.DisplayPrecision()),
			result.Misclose.Azimuth.FormatDMS(2),
			formatDistance(result.Misclose.HorizontalDistance, p.DisplayPrecision()),
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
	p.Points[id] = pt
	return Result{Message: "created point " + id, Created: []string{"point:" + id}}, nil
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
	p.Points[id] = pt
	return Result{Message: "created point " + id, Created: []string{"point:" + id}}, nil
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
	p.Points[pt.ID] = pt
	return Result{Message: "created point " + pt.ID, Created: []string{"point:" + pt.ID}}, nil
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
	p.Points[pt.ID] = pt
	return Result{Message: "created point " + pt.ID, Created: []string{"point:" + pt.ID}}, nil
}

func execOffsetFromLine(p *project.Project, f []string) (Result, error) {
	line, ok := p.Lines[f[1]]
	if !ok {
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
	p1, err := point(p, line.From)
	if err != nil {
		return Result{}, err
	}
	p2, err := point(p, line.To)
	if err != nil {
		return Result{}, err
	}
	pt := geom.Offset(p1, p2, off, chainage, f[5], optional(f, 6))
	p.Points[pt.ID] = pt
	return Result{Message: "created point " + pt.ID, Created: []string{"point:" + pt.ID}}, nil
}

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
		p.Points[pt.ID] = pt
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
		p.Points[pt.ID] = pt
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
		p.Points[pt.ID] = pt
		return Result{Message: "created point " + pt.ID, Created: []string{"point:" + pt.ID}}, nil
	default:
		return Result{}, fmt.Errorf("unknown intersect type %q", f[1])
	}
}

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
	p.Points[pt.ID] = pt
	return Result{Message: "created point " + pt.ID, Created: []string{"point:" + pt.ID}}, nil
}

func execShift(p *project.Project, f []string) (Result, error) {
	if len(f) < 3 {
		return Result{}, fmt.Errorf("usage: shift <base> [east=<delta>] [north=<delta>] [elev=<delta>]")
	}
	if _, err := point(p, f[1]); err != nil {
		return Result{}, err
	}
	var (
		deltaE   float64
		deltaN   float64
		deltaZ   *float64
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
			deltaE = x
			haveAxis = true
		case "north", "n", "northing", "y":
			x, err := parseFloat(k, v)
			if err != nil {
				return Result{}, err
			}
			deltaN = x
			haveAxis = true
		case "elev", "z":
			x, err := parseFloat(k, v)
			if err != nil {
				return Result{}, err
			}
			deltaZ = &x
			haveAxis = true
		default:
			return Result{}, fmt.Errorf("unknown shift field %q", k)
		}
	}
	if !haveAxis {
		return Result{}, fmt.Errorf("shift requires at least one of east=, north=, or elev=")
	}
	updated := make([]string, 0, len(p.Points)+len(p.ContourSets))
	for id, pt := range p.Points {
		p.Points[id] = geom.ShiftPoint(pt, deltaE, deltaN, deltaZ)
		updated = append(updated, "point:"+id)
	}
	for id, set := range p.ContourSets {
		for polyIdx := range set.Polylines {
			if deltaZ != nil {
				set.Polylines[polyIdx].Elevation += *deltaZ
			}
			for vertexIdx := range set.Polylines[polyIdx].Vertices {
				set.Polylines[polyIdx].Vertices[vertexIdx].Easting += deltaE
				set.Polylines[polyIdx].Vertices[vertexIdx].Northing += deltaN
			}
		}
		p.ContourSets[id] = set
		updated = append(updated, "contour:"+id)
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
		for polyIdx := range set.Polylines {
			for vertexIdx := range set.Polylines[polyIdx].Vertices {
				pt := geom.Point{
					Easting:  set.Polylines[polyIdx].Vertices[vertexIdx].Easting,
					Northing: set.Polylines[polyIdx].Vertices[vertexIdx].Northing,
				}
				pt = geom.RotatePoint(pt, base, angle)
				set.Polylines[polyIdx].Vertices[vertexIdx].Easting = pt.Easting
				set.Polylines[polyIdx].Vertices[vertexIdx].Northing = pt.Northing
			}
		}
		p.ContourSets[id] = set
		updated = append(updated, "contour:"+id)
	}
	return Result{Message: fmt.Sprintf("rotated %d points by %s", len(p.Points), f[2]), Updated: updated}, nil
}

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
		p.Points[id] = pt
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

func execUnits(p *project.Project, f []string) (Result, error) {
	if len(f) < 2 {
		return Result{}, fmt.Errorf("usage: units key=value ...")
	}
	for _, arg := range f[1:] {
		k, v, ok := strings.Cut(arg, "=")
		if !ok {
			return Result{}, fmt.Errorf("unit argument %q must be key=value", arg)
		}
		p.Units[k] = v
	}
	return Result{Message: "updated units"}, nil
}

func execContour(p *project.Project, f []string) (Result, error) {
	if len(f) < 2 {
		return Result{}, fmt.Errorf("contour requires subcommand")
	}
	switch f[1] {
	case "gen":
		if len(f) < 4 {
			return Result{}, fmt.Errorf("usage: contour gen <id> <interval> [base=<elev>] [index=<n>] [breaklines=all|none|ids:L1,L2]")
		}
		interval, err := parseFloat("interval", f[3])
		if err != nil {
			return Result{}, err
		}
		opts := terrain.Options{ID: f[2], Interval: interval, UseBreakline: true}
		for _, arg := range f[4:] {
			k, v, ok := strings.Cut(arg, "=")
			if !ok {
				return Result{}, fmt.Errorf("contour option %q must be key=value", arg)
			}
			switch k {
			case "base":
				x, err := parseFloat("base", v)
				if err != nil {
					return Result{}, err
				}
				opts.Base = &x
			case "index":
				n, err := strconv.Atoi(v)
				if err != nil || n < 0 {
					return Result{}, fmt.Errorf("index must be zero or greater")
				}
				opts.IndexEvery = n
				opts.IndexEverySet = true
			case "breaklines":
				switch {
				case v == "all":
					opts.UseBreakline = true
					opts.BreaklineIDs = nil
				case v == "none":
					opts.UseBreakline = false
					opts.BreaklineIDs = nil
				case strings.HasPrefix(v, "ids:"):
					opts.UseBreakline = true
					value := strings.TrimPrefix(v, "ids:")
					if value == "" {
						return Result{}, fmt.Errorf("breaklines ids list is empty")
					}
					opts.BreaklineIDs = strings.Split(value, ",")
				default:
					return Result{}, fmt.Errorf("breaklines must be all, none, or ids:L1,L2")
				}
			default:
				return Result{}, fmt.Errorf("unknown contour option %q", k)
			}
		}
		set, err := terrain.Generate(p, opts)
		if err != nil {
			return Result{}, err
		}
		if p.ContourSets == nil {
			p.ContourSets = map[string]project.ContourSet{}
		}
		_, replaced := p.ContourSets[set.ID]
		p.ContourSets[set.ID] = set
		if replaced {
			return Result{Message: fmt.Sprintf("replaced contour set %s with %d polylines", set.ID, len(set.Polylines)), Updated: []string{"contour:" + set.ID}}, nil
		}
		return Result{Message: fmt.Sprintf("generated contour set %s with %d polylines", set.ID, len(set.Polylines)), Created: []string{"contour:" + set.ID}}, nil
	case "list":
		if len(p.ContourSets) == 0 {
			return Result{Message: "0 contour sets"}, nil
		}
		var b strings.Builder
		fmt.Fprintf(&b, "%d contour sets", len(p.ContourSets))
		for _, set := range p.SortedContourSets() {
			fmt.Fprintf(&b, "\n%s interval=%s base=%s polylines=%d breaklines=%d index=%d",
				set.ID,
				formatDistance(set.Interval, p.DisplayPrecision()),
				formatDistance(set.Base, p.DisplayPrecision()),
				len(set.Polylines),
				len(set.Breaklines),
				set.IndexEvery,
			)
		}
		return Result{Message: b.String()}, nil
	case "info":
		if len(f) != 3 {
			return Result{}, fmt.Errorf("usage: contour info <id>")
		}
		set, ok := p.ContourSets[f[2]]
		if !ok {
			return Result{}, fmt.Errorf("contour set %q not found", f[2])
		}
		return Result{Message: fmt.Sprintf("%s interval=%s base=%s polylines=%d breaklines=%d", set.ID, formatDistance(set.Interval, p.DisplayPrecision()), formatDistance(set.Base, p.DisplayPrecision()), len(set.Polylines), len(set.Breaklines))}, nil
	case "del":
		if len(f) != 3 {
			return Result{}, fmt.Errorf("usage: contour del <id>")
		}
		if _, ok := p.ContourSets[f[2]]; !ok {
			return Result{}, fmt.Errorf("contour set %q not found", f[2])
		}
		delete(p.ContourSets, f[2])
		return Result{Message: "deleted contour set " + f[2], Updated: []string{"contour:" + f[2]}}, nil
	default:
		return Result{}, fmt.Errorf("unknown contour subcommand %q", f[1])
	}
}

func parseAngleTokens(tokens []string) (geom.Angle, int, error) {
	if len(tokens) >= 3 {
		if a, used, err := geom.ParseQuadrantBearing(tokens[:3]); err == nil {
			return a, used, nil
		}
	}
	if len(tokens) == 0 {
		return geom.Angle{}, 0, fmt.Errorf("missing angle")
	}
	a, err := geom.ParseAngle(tokens[0])
	return a, 1, err
}

func point(p *project.Project, id string) (geom.Point, error) {
	pt, ok := p.Points[id]
	if !ok {
		return geom.Point{}, fmt.Errorf("point %q not found", id)
	}
	return pt, nil
}

func parseFloat(name, value string) (float64, error) {
	f, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid %s %q", name, value)
	}
	return f, nil
}

func formatDistance(value float64, precision int) string {
	return strconv.FormatFloat(value, 'f', precision, 64)
}

func optional(fields []string, i int) string {
	if i < len(fields) {
		return fields[i]
	}
	return ""
}
