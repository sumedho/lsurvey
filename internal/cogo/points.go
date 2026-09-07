package cogo

import (
	"fmt"
	"strconv"
	"strings"

	"lsurvey/internal/geom"
	"lsurvey/internal/project"
)

func execPoint(p *project.Project, f []string) (Result, error) {
	if len(f) < 2 {
		return Result{}, fmt.Errorf("pt requires subcommand")
	}
	switch f[1] {
	case "add":
		if len(f) < 5 {
			return Result{}, fmt.Errorf("usage: pt add <id> <east> <north> [elev] [code] [group=<id>]")
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
		groupID := ""
		i := 5
		if i < len(f) {
			if v, err := strconv.ParseFloat(f[i], 64); err == nil {
				if !geom.Finite(v) {
					return Result{}, fmt.Errorf("elevation must be finite")
				}
				z = &v
				i++
			}
		}
		if i < len(f) && !strings.Contains(f[i], "=") {
			code = f[i]
			i++
		}
		for ; i < len(f); i++ {
			k, v, ok := strings.Cut(f[i], "=")
			if !ok || k != "group" {
				return Result{}, fmt.Errorf("usage: pt add <id> <east> <north> [elev] [code] [group=<id>]")
			}
			if err := requireGroup(p, v); err != nil {
				return Result{}, err
			}
			groupID = v
		}
		p.Points[id] = p.ApplyPointCodeStyle(geom.Point{ID: id, Easting: e, Northing: n, Elevation: z, Code: code, GroupID: groupID})
		return staleContours(Result{Message: "added point " + id, Created: []string{"point:" + id}}, "point geometry changed"), nil
	case "edit":
		if len(f) < 4 {
			return Result{}, fmt.Errorf("usage: pt edit <id> [east=] [north=] [elev=] [code=] [desc=] [group=<id>|none]")
		}
		id := f[2]
		pt, ok := p.Points[id]
		if !ok {
			return Result{}, fmt.Errorf("point %q not found", id)
		}
		geometryChanged := false
		for _, arg := range f[3:] {
			k, v, ok := strings.Cut(arg, "=")
			if !ok {
				return Result{}, fmt.Errorf("edit argument %q must be key=value", arg)
			}
			switch k {
			case "north", "n", "northing":
				geometryChanged = true
				x, err := parseFloat(k, v)
				if err != nil {
					return Result{}, err
				}
				pt.Northing = x
			case "east", "e", "easting":
				geometryChanged = true
				x, err := parseFloat(k, v)
				if err != nil {
					return Result{}, err
				}
				pt.Easting = x
			case "elev", "z":
				geometryChanged = true
				x, err := parseFloat(k, v)
				if err != nil {
					return Result{}, err
				}
				pt.Elevation = &x
			case "code":
				pt.Code = v
			case "desc":
				pt.Description = v
			case "group":
				if v == "none" {
					v = ""
				} else if err := requireGroup(p, v); err != nil {
					return Result{}, err
				}
				pt.GroupID = v
			default:
				return Result{}, fmt.Errorf("unknown point field %q", k)
			}
		}
		p.Points[id] = pt
		return staleContoursIf(Result{Message: "updated point " + id, Updated: []string{"point:" + id}}, "point geometry changed", geometryChanged), nil
	case "del":
		if len(f) != 3 {
			return Result{}, fmt.Errorf("usage: pt del <id>")
		}
		id := f[2]
		if _, ok := p.Points[id]; !ok {
			return Result{}, fmt.Errorf("point %q not found", id)
		}
		if tr := p.Traverse; tr != nil {
			for _, used := range append([]string{tr.Start, tr.Current, tr.Close}, tr.LegPointIDs...) {
				if used == id {
					return Result{}, fmt.Errorf("point %q is used by the current traverse; start a new traverse before deleting", id)
				}
			}
		}
		for _, feature := range p.Features {
			for _, pointID := range feature.PointIDs {
				if pointID == id {
					return Result{}, fmt.Errorf("point %q is used by feature %q", id, feature.ID)
				}
			}
		}
		delete(p.Points, id)
		return staleContours(Result{Message: "deleted point " + id, Deleted: []string{"point:" + id}}, "point geometry changed"), nil
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
		if tr := p.Traverse; tr != nil {
			if tr.Start == f[2] {
				tr.Start = f[3]
			}
			if tr.Current == f[2] {
				tr.Current = f[3]
			}
			if tr.Close == f[2] {
				tr.Close = f[3]
			}
			for i, id := range tr.LegPointIDs {
				if id == f[2] {
					tr.LegPointIDs[i] = f[3]
				}
			}
		}
		if g := p.GridGround; g != nil && g.AnchorPointID == f[2] {
			g.AnchorPointID = f[3]
		}
		for id, feature := range p.Features {
			for i, pointID := range feature.PointIDs {
				if pointID == f[2] {
					feature.PointIDs[i] = f[3]
				}
			}
			p.Features[id] = feature
		}
		return staleContours(Result{Message: "renamed point " + f[2] + " to " + f[3], Updated: []string{"point:" + f[3]}}, "point geometry changed"), nil
	case "list":
		return Result{Message: fmt.Sprintf("%d points", len(p.Points))}, nil
	default:
		return Result{}, fmt.Errorf("unknown pt subcommand %q", f[1])
	}
}
