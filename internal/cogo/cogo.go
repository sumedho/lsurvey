package cogo

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"

	"lsurvey/internal/boundary"
	"lsurvey/internal/geom"
	"lsurvey/internal/project"
)

type Result struct {
	Message string
	Created []string
	Updated []string
	Changed bool
}

func Execute(p *project.Project, command string) (Result, error) {
	fields, err := tokenize(command)
	if err != nil {
		return Result{}, err
	}
	if len(fields) == 0 {
		return Result{}, fmt.Errorf("empty command")
	}
	var result Result
	switch fields[0] {
	case "pt":
		result, err = execPoint(p, fields)
	case "line":
		result, err = execLine(p, fields)
	case "polyline":
		result, err = execOrderedFeature(p, fields, project.FeaturePolyline)
	case "polygon":
		result, err = execOrderedFeature(p, fields, project.FeaturePolygon)
	case "group":
		result, err = execGroup(p, fields)
	case "code":
		result, err = execCode(p, fields)
	case "inverse":
		result, err = execInverse(p, fields)
	case "angle":
		result, err = execAngle(p, fields)
	case "close":
		result, err = execClose(p, fields)
	case "bearing":
		result, err = execBearing(p, fields)
	case "dist":
		result, err = execDistance(p, fields)
	case "rad":
		result, err = execRad(p, fields)
	case "rad3d":
		result, err = execRad3D(p, fields)
	case "midpoint":
		result, err = execMidpoint(p, fields)
	case "offset":
		result, err = execOffset(p, fields)
	case "intersect":
		result, err = execIntersect(p, fields)
	case "resect":
		result, err = execResect(p, fields)
	case "shift":
		result, err = execShift(p, fields)
	case "rotate":
		result, err = execRotate(p, fields)
	case "scale":
		result, err = execScale(p, fields)
	case "transform":
		result, err = execTransform(p, fields)
	case "trav":
		result, err = execTraverse(p, fields)
	case "contour":
		result, err = execContour(p, fields)
	case "units":
		result, err = execUnits(p, fields)
	default:
		return Result{}, fmt.Errorf("unknown command %q", fields[0])
	}
	if err == nil {
		result.Changed = commandChangesProject(fields, result)
		markContoursStaleAfterCommand(p, fields, result)
	}
	return result, err
}

func ExecuteAndRecord(p *project.Project, command string) (Result, error) {
	result, err := Execute(p, command)
	if err == nil && result.Changed {
		p.AddHistoryChange(command, result.Message, result.Created, result.Updated, nil, nil)
	}
	return result, err
}

func commandChangesProject(fields []string, result Result) bool {
	switch fields[0] {
	case "pt":
		return len(fields) > 1 && fields[1] != "list"
	case "line":
		if len(fields) > 1 && fields[1] == "list" {
			return false
		}
		if len(fields) > 1 && fields[1] == "gen" {
			return len(result.Created) > 0
		}
		return true
	case "polyline":
		return len(fields) > 1 && fields[1] != "list" && fields[1] != "info"
	case "polygon":
		return len(fields) > 1 && fields[1] != "list" && fields[1] != "info" && fields[1] != "report"
	case "group":
		return len(fields) > 1 && fields[1] != "list" && fields[1] != "info"
	case "code":
		return len(fields) > 2 && fields[2] != "list"
	case "rad", "rad3d", "midpoint", "offset", "intersect", "resect",
		"shift", "rotate", "scale", "units":
		return true
	case "trav":
		return len(fields) > 1 && fields[1] != "show"
	case "contour":
		return len(fields) > 1 && fields[1] != "list" && fields[1] != "info"
	default:
		return false
	}
}

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
		return Result{Message: "added point " + id, Created: []string{"point:" + id}}, nil
	case "edit":
		if len(f) < 4 {
			return Result{}, fmt.Errorf("usage: pt edit <id> [east=] [north=] [elev=] [code=] [desc=] [group=<id>|none]")
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
		return Result{Message: "updated point " + id, Updated: []string{"point:" + id}}, nil
	case "del":
		if len(f) != 3 {
			return Result{}, fmt.Errorf("usage: pt del <id>")
		}
		id := f[2]
		if _, ok := p.Points[id]; !ok {
			return Result{}, fmt.Errorf("point %q not found", id)
		}
		for _, feature := range p.Features {
			for _, pointID := range feature.PointIDs {
				if pointID == id {
					return Result{}, fmt.Errorf("point %q is used by feature %q", id, feature.ID)
				}
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
		for id, feature := range p.Features {
			for i, pointID := range feature.PointIDs {
				if pointID == f[2] {
					feature.PointIDs[i] = f[3]
				}
			}
			p.Features[id] = feature
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
			return Result{}, fmt.Errorf("usage: line add <id> <p1> <p2> [code] [group=<id>] [terrain=standard|ridge|drain]")
		}
		feature := project.Feature{ID: f[2], Kind: project.FeatureLine, PointIDs: []string{f[3], f[4]}}
		for _, arg := range f[5:] {
			k, v, ok := strings.Cut(arg, "=")
			if !ok {
				if feature.Code != "" {
					return Result{}, fmt.Errorf("usage: line add <id> <p1> <p2> [code] [group=<id>] [terrain=standard|ridge|drain]")
				}
				feature.Code = arg
				continue
			}
			switch k {
			case "terrain":
				if !validTerrainRole(v) || v == "none" {
					return Result{}, fmt.Errorf("terrain must be standard, ridge, or drain")
				}
				feature.TerrainRole = v
			case "group":
				if err := requireGroup(p, v); err != nil {
					return Result{}, err
				}
				feature.GroupID = v
			default:
				return Result{}, fmt.Errorf("unknown line field %q", k)
			}
		}
		if err := storeFeature(p, feature); err != nil {
			return Result{}, err
		}
		return Result{Message: "added line " + f[2], Created: []string{"line:" + f[2]}}, nil
	case "gen":
		if len(f) < 3 || len(f) > 4 {
			return Result{}, fmt.Errorf("usage: line gen <code> [group=<id>]")
		}
		groupID := ""
		if len(f) == 4 {
			k, v, ok := strings.Cut(f[3], "=")
			if !ok || k != "group" {
				return Result{}, fmt.Errorf("usage: line gen <code> [group=<id>]")
			}
			if err := requireGroup(p, v); err != nil {
				return Result{}, err
			}
			groupID = v
		}
		return genLinesByCode(p, f[2], groupID)
	case "del":
		if len(f) != 3 {
			return Result{}, fmt.Errorf("usage: line del <id>")
		}
		if feature, ok := p.Features[f[2]]; !ok || feature.Kind != project.FeatureLine {
			return Result{}, fmt.Errorf("line %q not found", f[2])
		}
		delete(p.Features, f[2])
		return Result{Message: "deleted line " + f[2], Updated: []string{"line:" + f[2]}}, nil
	case "edit":
		if len(f) < 4 {
			return Result{}, fmt.Errorf("usage: line edit <id> [from=] [to=] [code=] [desc=] [group=<id>|none] [terrain=none|standard|ridge|drain]")
		}
		id := f[2]
		feature, ok := p.Features[id]
		if !ok || feature.Kind != project.FeatureLine {
			return Result{}, fmt.Errorf("line %q not found", id)
		}
		for _, arg := range f[3:] {
			k, v, ok := strings.Cut(arg, "=")
			if !ok {
				return Result{}, fmt.Errorf("edit argument %q must be key=value", arg)
			}
			switch k {
			case "from":
				feature.PointIDs[0] = v
			case "to":
				feature.PointIDs[1] = v
			case "code":
				feature.Code = v
			case "desc":
				feature.Description = v
			case "group":
				if v == "none" {
					v = ""
				} else if err := requireGroup(p, v); err != nil {
					return Result{}, err
				}
				feature.GroupID = v
			case "terrain":
				if !validTerrainRole(v) {
					return Result{}, fmt.Errorf("terrain must be none, standard, ridge, or drain")
				}
				if v == "none" {
					v = ""
				}
				feature.TerrainRole = v
			default:
				return Result{}, fmt.Errorf("unknown line field %q", k)
			}
		}
		if err := validateFeature(p, feature); err != nil {
			return Result{}, err
		}
		p.Features[id] = feature
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
		if err := storeCreatedPoint(p, pt); err != nil {
			return Result{}, err
		}
		return Result{Message: "created point " + pt.ID, Created: []string{"point:" + pt.ID}}, nil
	case "list":
		return Result{Message: fmt.Sprintf("%d lines", countFeatures(p, project.FeatureLine))}, nil
	default:
		return Result{}, fmt.Errorf("unknown line subcommand %q", f[1])
	}
}

func validTerrainRole(role string) bool {
	return role == "none" || role == "standard" || role == "ridge" || role == "drain"
}

func genLinesByCode(p *project.Project, code, groupID string) (Result, error) {
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
		id := p.NextFeatureID()
		p.Features[id] = project.Feature{ID: id, Kind: project.FeatureLine, PointIDs: []string{from, to}, Code: code, GroupID: groupID}
		created = append(created, "line:"+id)
	}
	return Result{
		Message: fmt.Sprintf("generated %d lines for code %s, skipped %d duplicates", len(created), code, skipped),
		Created: created,
	}, nil
}

func hasLineBetween(p *project.Project, a, b string) bool {
	for _, feature := range p.Features {
		if feature.Kind == project.FeatureLine && ((feature.PointIDs[0] == a && feature.PointIDs[1] == b) || (feature.PointIDs[0] == b && feature.PointIDs[1] == a)) {
			return true
		}
	}
	return false
}

func execGroup(p *project.Project, f []string) (Result, error) {
	if len(f) < 2 {
		return Result{}, fmt.Errorf("group requires subcommand")
	}
	switch f[1] {
	case "add":
		if len(f) < 5 {
			return Result{}, fmt.Errorf("usage: group add <id> layer=<name> color=<1..255> [desc=<text>]")
		}
		if _, exists := p.Groups[f[2]]; exists {
			return Result{}, fmt.Errorf("group %q already exists", f[2])
		}
		group := project.Group{ID: f[2]}
		if err := applyGroupFields(p, &group, f[3:]); err != nil {
			return Result{}, err
		}
		if group.Layer == "" || group.Color == 0 {
			return Result{}, fmt.Errorf("group requires layer and color")
		}
		p.Groups[group.ID] = group
		return Result{Message: "added group " + group.ID, Created: []string{"group:" + group.ID}}, nil
	case "edit":
		if len(f) < 4 {
			return Result{}, fmt.Errorf("usage: group edit <id> [layer=<name>] [color=<1..255>] [desc=<text>]")
		}
		group, ok := p.Groups[f[2]]
		if !ok {
			return Result{}, fmt.Errorf("group %q not found", f[2])
		}
		if err := applyGroupFields(p, &group, f[3:]); err != nil {
			return Result{}, err
		}
		p.Groups[group.ID] = group
		return Result{Message: "updated group " + group.ID, Updated: []string{"group:" + group.ID}}, nil
	case "del":
		if len(f) != 3 {
			return Result{}, fmt.Errorf("usage: group del <id>")
		}
		if _, ok := p.Groups[f[2]]; !ok {
			return Result{}, fmt.Errorf("group %q not found", f[2])
		}
		for _, pt := range p.Points {
			if pt.GroupID == f[2] {
				return Result{}, fmt.Errorf("group %q is used by point %q", f[2], pt.ID)
			}
		}
		for _, feature := range p.Features {
			if feature.GroupID == f[2] {
				return Result{}, fmt.Errorf("group %q is used by feature %q", f[2], feature.ID)
			}
		}
		for code, groupID := range p.PointCodeStyles {
			if groupID == f[2] {
				return Result{}, fmt.Errorf("group %q is used by point code style %q", f[2], code)
			}
		}
		delete(p.Groups, f[2])
		return Result{Message: "deleted group " + f[2], Updated: []string{"group:" + f[2]}}, nil
	case "list":
		return Result{Message: fmt.Sprintf("%d groups", len(p.Groups))}, nil
	case "info":
		if len(f) != 3 {
			return Result{}, fmt.Errorf("usage: group info <id>")
		}
		group, ok := p.Groups[f[2]]
		if !ok {
			return Result{}, fmt.Errorf("group %q not found", f[2])
		}
		return Result{Message: fmt.Sprintf("%s layer=%s color=%d desc=%s", group.ID, group.Layer, group.Color, group.Description)}, nil
	default:
		return Result{}, fmt.Errorf("unknown group subcommand %q", f[1])
	}
}

func execCode(p *project.Project, f []string) (Result, error) {
	if len(f) < 2 || f[1] != "style" {
		return Result{}, fmt.Errorf("usage: code style set <code> group=<id> OR code style del <code> OR code style list")
	}
	if len(f) < 3 {
		return Result{}, fmt.Errorf("usage: code style set <code> group=<id> OR code style del <code> OR code style list")
	}
	switch f[2] {
	case "set":
		if len(f) != 5 || !strings.HasPrefix(f[4], "group=") || f[3] == "" {
			return Result{}, fmt.Errorf("usage: code style set <code> group=<id>")
		}
		groupID := strings.TrimPrefix(f[4], "group=")
		if err := requireGroup(p, groupID); err != nil {
			return Result{}, err
		}
		p.PointCodeStyles[f[3]] = groupID
		return Result{Message: fmt.Sprintf("set point code style %s group=%s", f[3], groupID), Updated: []string{"code:" + f[3]}}, nil
	case "del":
		if len(f) != 4 {
			return Result{}, fmt.Errorf("usage: code style del <code>")
		}
		if _, ok := p.PointCodeStyles[f[3]]; !ok {
			return Result{}, fmt.Errorf("point code style %q not found", f[3])
		}
		delete(p.PointCodeStyles, f[3])
		return Result{Message: "deleted point code style " + f[3], Updated: []string{"code:" + f[3]}}, nil
	case "list":
		if len(f) != 3 {
			return Result{}, fmt.Errorf("usage: code style list")
		}
		groups := p.SortedGroups()
		codes := make([]string, 0, len(p.PointCodeStyles))
		for code := range p.PointCodeStyles {
			codes = append(codes, code)
		}
		sort.Strings(codes)
		var b strings.Builder
		fmt.Fprintf(&b, "%d style groups", len(groups))
		for _, group := range groups {
			fmt.Fprintf(&b, "\n%s layer=%s color=%d desc=%s", group.ID, group.Layer, group.Color, group.Description)
		}
		fmt.Fprintf(&b, "\n%d point code styles", len(codes))
		for _, code := range codes {
			fmt.Fprintf(&b, "\n%s group=%s", code, p.PointCodeStyles[code])
		}
		return Result{Message: b.String()}, nil
	default:
		return Result{}, fmt.Errorf("usage: code style set <code> group=<id> OR code style del <code> OR code style list")
	}
}

func applyGroupFields(p *project.Project, group *project.Group, fields []string) error {
	for _, arg := range fields {
		k, v, ok := strings.Cut(arg, "=")
		if !ok {
			return fmt.Errorf("group argument %q must be key=value", arg)
		}
		switch k {
		case "layer":
			if strings.TrimSpace(v) == "" {
				return fmt.Errorf("group layer cannot be empty")
			}
			for id, existing := range p.Groups {
				if id != group.ID && strings.EqualFold(existing.Layer, v) {
					return fmt.Errorf("group layer %q already exists", v)
				}
			}
			group.Layer = v
		case "color":
			color, err := strconv.Atoi(v)
			if err != nil || color < 1 || color > 255 {
				return fmt.Errorf("group color must be between 1 and 255")
			}
			group.Color = color
		case "desc":
			group.Description = v
		default:
			return fmt.Errorf("unknown group field %q", k)
		}
	}
	return nil
}

func execOrderedFeature(p *project.Project, f []string, kind string) (Result, error) {
	name := kind
	if len(f) < 2 {
		return Result{}, fmt.Errorf("%s requires subcommand", name)
	}
	switch f[1] {
	case "add":
		min := 2
		if kind == project.FeaturePolygon {
			min = 3
		}
		if len(f) < 3+min {
			return Result{}, fmt.Errorf("usage: %s add <id> <p1> <%s> [<pN> ...] [code=<code>] [desc=<text>] [group=<id>] [terrain=standard|ridge|drain]", name, map[bool]string{true: "p3", false: "p2"}[kind == project.FeaturePolygon])
		}
		feature := project.Feature{ID: f[2], Kind: kind}
		i := 3
		for i < len(f) && !strings.Contains(f[i], "=") {
			feature.PointIDs = append(feature.PointIDs, f[i])
			i++
		}
		if err := applyFeatureFields(p, &feature, f[i:], kind != project.FeaturePolygon); err != nil {
			return Result{}, err
		}
		if err := storeFeature(p, feature); err != nil {
			return Result{}, err
		}
		return Result{Message: "added " + name + " " + feature.ID, Created: []string{name + ":" + feature.ID}}, nil
	case "edit":
		if len(f) < 4 {
			return Result{}, fmt.Errorf("usage: %s edit <id> [points=<p1,p2,...>] [code=] [desc=] [group=<id>|none] [terrain=none|standard|ridge|drain]", name)
		}
		feature, ok := p.Features[f[2]]
		if !ok || feature.Kind != kind {
			return Result{}, fmt.Errorf("%s %q not found", name, f[2])
		}
		if err := applyFeatureFields(p, &feature, f[3:], kind != project.FeaturePolygon); err != nil {
			return Result{}, err
		}
		if err := validateFeature(p, feature); err != nil {
			return Result{}, err
		}
		p.Features[feature.ID] = feature
		return Result{Message: "updated " + name + " " + feature.ID, Updated: []string{name + ":" + feature.ID}}, nil
	case "del":
		if len(f) != 3 {
			return Result{}, fmt.Errorf("usage: %s del <id>", name)
		}
		feature, ok := p.Features[f[2]]
		if !ok || feature.Kind != kind {
			return Result{}, fmt.Errorf("%s %q not found", name, f[2])
		}
		delete(p.Features, f[2])
		return Result{Message: "deleted " + name + " " + f[2], Updated: []string{name + ":" + f[2]}}, nil
	case "list":
		return Result{Message: fmt.Sprintf("%d %ss", countFeatures(p, kind), name)}, nil
	case "info":
		if len(f) != 3 {
			return Result{}, fmt.Errorf("usage: %s info <id>", name)
		}
		feature, ok := p.Features[f[2]]
		if !ok || feature.Kind != kind {
			return Result{}, fmt.Errorf("%s %q not found", name, f[2])
		}
		return Result{Message: fmt.Sprintf("%s points=%s code=%s group=%s", feature.ID, strings.Join(feature.PointIDs, ","), feature.Code, feature.GroupID)}, nil
	case "report":
		if kind != project.FeaturePolygon || len(f) != 3 {
			return Result{}, fmt.Errorf("usage: polygon report <id|all>")
		}
		return polygonReport(p, f[2])
	default:
		return Result{}, fmt.Errorf("unknown %s subcommand %q", name, f[1])
	}
}

func polygonReport(p *project.Project, selection string) (Result, error) {
	var schedules []boundary.Schedule
	if selection == "all" {
		schedules = boundary.All(p)
	} else {
		schedule, err := boundary.ForPolygon(p, selection)
		if err != nil {
			return Result{}, err
		}
		schedules = []boundary.Schedule{schedule}
	}
	if len(schedules) == 0 {
		return Result{Message: "0 polygons"}, nil
	}
	var b strings.Builder
	precision := p.DisplayPrecision()
	for index, schedule := range schedules {
		if index > 0 {
			b.WriteByte('\n')
		}
		fmt.Fprintf(&b, "%s code=%s desc=%s area=%s %s^2 perimeter=%s %s",
			schedule.Feature.ID, schedule.Feature.Code, schedule.Feature.Description,
			formatDistance(schedule.Area, precision), p.Units["distance"],
			formatDistance(schedule.Perimeter, precision), p.Units["distance"])
		for _, leg := range schedule.Legs {
			fmt.Fprintf(&b, "\nleg=%d from=%s to=%s bearing=%s distance=%s",
				leg.Number, leg.From, leg.To, leg.Bearing.FormatDMS(2), formatDistance(leg.Distance, precision))
		}
	}
	return Result{Message: b.String()}, nil
}

func applyFeatureFields(p *project.Project, feature *project.Feature, fields []string, allowTerrain bool) error {
	for _, arg := range fields {
		k, v, ok := strings.Cut(arg, "=")
		if !ok {
			return fmt.Errorf("feature argument %q must be key=value", arg)
		}
		switch k {
		case "points":
			feature.PointIDs = strings.Split(v, ",")
		case "code":
			feature.Code = v
		case "desc":
			feature.Description = v
		case "group":
			if v == "none" {
				v = ""
			} else if err := requireGroup(p, v); err != nil {
				return err
			}
			feature.GroupID = v
		case "terrain":
			if !allowTerrain {
				return fmt.Errorf("polygons do not support terrain roles")
			}
			if !validTerrainRole(v) {
				return fmt.Errorf("terrain must be none, standard, ridge, or drain")
			}
			if v == "none" {
				v = ""
			}
			feature.TerrainRole = v
		default:
			return fmt.Errorf("unknown feature field %q", k)
		}
	}
	return nil
}

func storeFeature(p *project.Project, feature project.Feature) error {
	if _, ok := p.Features[feature.ID]; ok {
		return fmt.Errorf("feature %q already exists", feature.ID)
	}
	if err := validateFeature(p, feature); err != nil {
		return err
	}
	p.Features[feature.ID] = feature
	return nil
}

func validateFeature(p *project.Project, feature project.Feature) error {
	min := 2
	if feature.Kind == project.FeaturePolygon {
		min = 3
	}
	if len(feature.PointIDs) < min || feature.Kind == project.FeatureLine && len(feature.PointIDs) != 2 {
		return fmt.Errorf("%s %q has invalid point count", feature.Kind, feature.ID)
	}
	for i, id := range feature.PointIDs {
		if _, ok := p.Points[id]; !ok {
			return fmt.Errorf("point %q not found", id)
		}
		if i > 0 && id == feature.PointIDs[i-1] {
			return fmt.Errorf("%s %q has duplicate consecutive points", feature.Kind, feature.ID)
		}
	}
	if feature.Kind == project.FeaturePolygon {
		if feature.PointIDs[0] == feature.PointIDs[len(feature.PointIDs)-1] {
			return fmt.Errorf("polygon closure is implicit; do not repeat the first point")
		}
		if polygonInvalid(p, feature.PointIDs) {
			return fmt.Errorf("polygon %q is zero-area or self-intersecting", feature.ID)
		}
	}
	if feature.GroupID != "" {
		return requireGroup(p, feature.GroupID)
	}
	return nil
}

func polygonInvalid(p *project.Project, ids []string) bool {
	area := 0.0
	for i := range ids {
		a, b := p.Points[ids[i]], p.Points[ids[(i+1)%len(ids)]]
		area += a.Easting*b.Northing - b.Easting*a.Northing
	}
	if math.Abs(area) < 1e-9 {
		return true
	}
	for i := range ids {
		a, b := p.Points[ids[i]], p.Points[ids[(i+1)%len(ids)]]
		for j := i + 1; j < len(ids); j++ {
			if j == i+1 || i == 0 && j == len(ids)-1 {
				continue
			}
			c, d := p.Points[ids[j]], p.Points[ids[(j+1)%len(ids)]]
			if segmentsCross(a, b, c, d) {
				return true
			}
		}
	}
	return false
}

func segmentsCross(a, b, c, d geom.Point) bool {
	orient := func(p, q, r geom.Point) float64 {
		return (q.Easting-p.Easting)*(r.Northing-p.Northing) - (q.Northing-p.Northing)*(r.Easting-p.Easting)
	}
	onSegment := func(p, q, r geom.Point) bool {
		return q.Easting >= math.Min(p.Easting, r.Easting)-1e-9 && q.Easting <= math.Max(p.Easting, r.Easting)+1e-9 &&
			q.Northing >= math.Min(p.Northing, r.Northing)-1e-9 && q.Northing <= math.Max(p.Northing, r.Northing)+1e-9
	}
	o1, o2, o3, o4 := orient(a, b, c), orient(a, b, d), orient(c, d, a), orient(c, d, b)
	if math.Abs(o1) <= 1e-9 && onSegment(a, c, b) || math.Abs(o2) <= 1e-9 && onSegment(a, d, b) ||
		math.Abs(o3) <= 1e-9 && onSegment(c, a, d) || math.Abs(o4) <= 1e-9 && onSegment(c, b, d) {
		return true
	}
	return (o1 > 0) != (o2 > 0) && (o3 > 0) != (o4 > 0)
}

func requireGroup(p *project.Project, id string) error {
	if _, ok := p.Groups[id]; !ok {
		return fmt.Errorf("group %q not found", id)
	}
	return nil
}

func countFeatures(p *project.Project, kind string) int {
	count := 0
	for _, feature := range p.Features {
		if feature.Kind == kind {
			count++
		}
	}
	return count
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
	if err := storeCreatedPoint(p, pt); err != nil {
		return Result{}, err
	}
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
	if err := storeCreatedPoint(p, pt); err != nil {
		return Result{}, err
	}
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
	if err := storeCreatedPoint(p, pt); err != nil {
		return Result{}, err
	}
	return Result{Message: "created point " + pt.ID, Created: []string{"point:" + pt.ID}}, nil
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

func markContoursStaleAfterCommand(p *project.Project, fields []string, result Result) {
	if len(fields) == 0 || len(p.ContourSets) == 0 {
		return
	}
	reason := ""
	switch fields[0] {
	case "pt":
		switch fields[1] {
		case "add", "del", "rename":
			reason = "point geometry changed"
		case "edit":
			for _, arg := range fields[3:] {
				if strings.HasPrefix(arg, "east=") || strings.HasPrefix(arg, "easting=") ||
					strings.HasPrefix(arg, "e=") || strings.HasPrefix(arg, "north=") ||
					strings.HasPrefix(arg, "northing=") || strings.HasPrefix(arg, "n=") ||
					strings.HasPrefix(arg, "elev=") || strings.HasPrefix(arg, "z=") {
					reason = "point geometry changed"
				}
			}
		}
	case "line", "polyline", "polygon":
		switch fields[1] {
		case "add", "del":
			reason = "feature geometry changed"
		case "gen":
			if len(result.Created) > 0 {
				reason = "line geometry changed"
			}
		case "edit":
			for _, arg := range fields[3:] {
				if strings.HasPrefix(arg, "from=") || strings.HasPrefix(arg, "to=") ||
					strings.HasPrefix(arg, "points=") || strings.HasPrefix(arg, "code=") ||
					strings.HasPrefix(arg, "terrain=") {
					reason = "feature terrain input changed"
				}
			}
		}
	default:
		for _, item := range append(append([]string(nil), result.Created...), result.Updated...) {
			if strings.HasPrefix(item, "point:") || strings.HasPrefix(item, "line:") ||
				strings.HasPrefix(item, "polyline:") || strings.HasPrefix(item, "polygon:") {
				reason = "point geometry changed"
				break
			}
		}
	}
	if reason != "" {
		p.MarkContoursStale(reason)
	}
}

func point(p *project.Project, id string) (geom.Point, error) {
	pt, ok := p.Points[id]
	if !ok {
		return geom.Point{}, fmt.Errorf("point %q not found", id)
	}
	return pt, nil
}

func storeCreatedPoint(p *project.Project, pt geom.Point) error {
	if _, exists := p.Points[pt.ID]; exists {
		return fmt.Errorf("point %q already exists", pt.ID)
	}
	p.Points[pt.ID] = p.ApplyPointCodeStyle(pt)
	return nil
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
