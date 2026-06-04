package cogo

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"lsurvey/internal/boundary"
	"lsurvey/internal/geom"
	"lsurvey/internal/project"
	"lsurvey/internal/validate"
)

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
		return staleContours(Result{Message: "added line " + f[2], Created: []string{"line:" + f[2]}}, "feature geometry changed"), nil
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
		return staleContours(Result{Message: "deleted line " + f[2], Deleted: []string{"line:" + f[2]}}, "feature geometry changed"), nil
	case "edit":
		if len(f) < 4 {
			return Result{}, fmt.Errorf("usage: line edit <id> [from=] [to=] [code=] [desc=] [group=<id>|none] [terrain=none|standard|ridge|drain]")
		}
		id := f[2]
		feature, ok := p.Features[id]
		if !ok || feature.Kind != project.FeatureLine {
			return Result{}, fmt.Errorf("line %q not found", id)
		}
		terrainInputChanged := false
		for _, arg := range f[3:] {
			k, v, ok := strings.Cut(arg, "=")
			if !ok {
				return Result{}, fmt.Errorf("edit argument %q must be key=value", arg)
			}
			switch k {
			case "from":
				terrainInputChanged = true
				feature.PointIDs[0] = v
			case "to":
				terrainInputChanged = true
				feature.PointIDs[1] = v
			case "code":
				terrainInputChanged = true
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
				terrainInputChanged = true
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
		return staleContoursIf(Result{Message: "updated line " + id, Updated: []string{"line:" + id}}, "feature terrain input changed", terrainInputChanged), nil
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
		return staleContours(Result{Message: "created point " + pt.ID, Created: []string{"point:" + pt.ID}}, "point geometry changed"), nil
	case "list":
		return Result{Message: fmt.Sprintf("%d lines", countFeatures(p, project.FeatureLine))}, nil
	default:
		return Result{}, fmt.Errorf("unknown line subcommand %q", f[1])
	}
}

func validTerrainRole(role string) bool {
	return validate.ValidTerrainRole(role)
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

	linePairs := existingLinePairs(p)
	created := make([]string, 0, len(matching)-1)
	skipped := 0
	for i := 0; i < len(matching)-1; i++ {
		from := matching[i].ID
		to := matching[i+1].ID
		pair := linePairKey(from, to)
		if linePairs[pair] {
			skipped++
			continue
		}
		id := p.NextFeatureID()
		p.Features[id] = project.Feature{ID: id, Kind: project.FeatureLine, PointIDs: []string{from, to}, Code: code, GroupID: groupID}
		linePairs[pair] = true
		created = append(created, "line:"+id)
	}
	result := Result{
		Message: fmt.Sprintf("generated %d lines for code %s, skipped %d duplicates", len(created), code, skipped),
		Created: created,
	}
	if len(created) == 0 {
		return result, nil
	}
	return staleContours(result, "line geometry changed"), nil
}

func existingLinePairs(p *project.Project) map[linePair]bool {
	pairs := make(map[linePair]bool)
	for _, feature := range p.Features {
		if feature.Kind == project.FeatureLine && len(feature.PointIDs) >= 2 {
			pairs[linePairKey(feature.PointIDs[0], feature.PointIDs[1])] = true
		}
	}
	return pairs
}

type linePair struct {
	A string
	B string
}

func linePairKey(a, b string) linePair {
	if a > b {
		a, b = b, a
	}
	return linePair{A: a, B: b}
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
		return changed(Result{Message: "added group " + group.ID, Created: []string{"group:" + group.ID}}), nil
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
		return changed(Result{Message: "updated group " + group.ID, Updated: []string{"group:" + group.ID}}), nil
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
		return changed(Result{Message: "deleted group " + f[2], Deleted: []string{"group:" + f[2]}}), nil
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
		return changed(Result{Message: fmt.Sprintf("set point code style %s group=%s", f[3], groupID), Updated: []string{"code:" + f[3]}}), nil
	case "del":
		if len(f) != 4 {
			return Result{}, fmt.Errorf("usage: code style del <code>")
		}
		if _, ok := p.PointCodeStyles[f[3]]; !ok {
			return Result{}, fmt.Errorf("point code style %q not found", f[3])
		}
		delete(p.PointCodeStyles, f[3])
		return changed(Result{Message: "deleted point code style " + f[3], Deleted: []string{"code:" + f[3]}}), nil
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
		return staleContours(Result{Message: "added " + name + " " + feature.ID, Created: []string{name + ":" + feature.ID}}, "feature geometry changed"), nil
	case "edit":
		if len(f) < 4 {
			return Result{}, fmt.Errorf("usage: %s edit <id> [points=<p1,p2,...>] [code=] [desc=] [group=<id>|none] [terrain=none|standard|ridge|drain]", name)
		}
		feature, ok := p.Features[f[2]]
		if !ok || feature.Kind != kind {
			return Result{}, fmt.Errorf("%s %q not found", name, f[2])
		}
		terrainInputChanged := featureFieldsAffectTerrainInput(f[3:])
		if err := applyFeatureFields(p, &feature, f[3:], kind != project.FeaturePolygon); err != nil {
			return Result{}, err
		}
		if err := validateFeature(p, feature); err != nil {
			return Result{}, err
		}
		p.Features[feature.ID] = feature
		return staleContoursIf(Result{Message: "updated " + name + " " + feature.ID, Updated: []string{name + ":" + feature.ID}}, "feature terrain input changed", terrainInputChanged), nil
	case "del":
		if len(f) != 3 {
			return Result{}, fmt.Errorf("usage: %s del <id>", name)
		}
		feature, ok := p.Features[f[2]]
		if !ok || feature.Kind != kind {
			return Result{}, fmt.Errorf("%s %q not found", name, f[2])
		}
		delete(p.Features, f[2])
		return staleContours(Result{Message: "deleted " + name + " " + f[2], Deleted: []string{name + ":" + f[2]}}, "feature geometry changed"), nil
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

func featureFieldsAffectTerrainInput(fields []string) bool {
	for _, arg := range fields {
		k, _, ok := strings.Cut(arg, "=")
		if !ok {
			continue
		}
		if k == "points" || k == "code" || k == "terrain" {
			return true
		}
	}
	return false
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
	return validate.Feature(
		feature,
		func(id string) (geom.Point, bool) {
			pt, ok := p.Points[id]
			return pt, ok
		},
		func(id string) bool {
			_, ok := p.Groups[id]
			return ok
		},
	)
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
