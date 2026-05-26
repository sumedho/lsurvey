package cogo

import (
	"fmt"
	"strconv"
	"strings"

	"lsurvey/internal/project"
	"lsurvey/internal/terrain"
)

func execContour(p *project.Project, f []string) (Result, error) {
	if len(f) < 2 {
		return Result{}, fmt.Errorf("contour requires subcommand")
	}
	switch f[1] {
	case "gen":
		if len(f) < 4 {
			return Result{}, fmt.Errorf("usage: contour gen <id> <interval> [base=<elev>] [index=<n>] [breaklines=all|none|ids:L1,L2] [boundary=codes:C1,C2] [exclude=codes:C3,C4] [maxedge=<distance>] [smooth=<0..3>]")
		}
		interval, err := parseFloat("interval", f[3])
		if err != nil {
			return Result{}, err
		}
		opts := terrain.Options{ID: f[2], Interval: interval, UseBreakline: true, BreaklineMode: "all"}
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
					opts.BreaklineMode = "all"
					opts.BreaklineIDs = nil
				case v == "none":
					opts.UseBreakline = false
					opts.BreaklineMode = "none"
					opts.BreaklineIDs = nil
				case strings.HasPrefix(v, "ids:"):
					opts.UseBreakline = true
					opts.BreaklineMode = "ids"
					value := strings.TrimPrefix(v, "ids:")
					if value == "" {
						return Result{}, fmt.Errorf("breaklines ids list is empty")
					}
					opts.BreaklineIDs = strings.Split(value, ",")
				default:
					return Result{}, fmt.Errorf("breaklines must be all, none, or ids:L1,L2")
				}
			case "boundary":
				codes, err := parseCodeSelector("boundary", v)
				if err != nil {
					return Result{}, err
				}
				opts.BoundaryCodes = codes
			case "exclude":
				codes, err := parseCodeSelector("exclude", v)
				if err != nil {
					return Result{}, err
				}
				opts.ExclusionCodes = codes
			case "maxedge":
				x, err := parseFloat("maxedge", v)
				if err != nil || x <= 0 {
					return Result{}, fmt.Errorf("maxedge must be greater than zero")
				}
				opts.MaxEdge = &x
			case "smooth":
				n, err := strconv.Atoi(v)
				if err != nil || n < 0 || n > 3 {
					return Result{}, fmt.Errorf("smooth must be between zero and three")
				}
				opts.Smooth = n
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
			return Result{Message: contourGenerationMessage("replaced", set), Updated: []string{"contour:" + set.ID}}, nil
		}
		return Result{Message: contourGenerationMessage("generated", set), Created: []string{"contour:" + set.ID}}, nil
	case "regen":
		if len(f) != 3 {
			return Result{}, fmt.Errorf("usage: contour regen <id>")
		}
		existing, ok := p.ContourSets[f[2]]
		if !ok {
			return Result{}, fmt.Errorf("contour set %q not found", f[2])
		}
		if existing.Generation == nil {
			return Result{}, fmt.Errorf("contour set %q has no saved generation specification; replace it with contour gen", f[2])
		}
		set, err := terrain.Generate(p, optionsFromSpec(existing.ID, existing.Generation))
		if err != nil {
			return Result{}, err
		}
		p.ContourSets[set.ID] = set
		return Result{Message: contourGenerationMessage("regenerated", set), Updated: []string{"contour:" + set.ID}}, nil
	case "list":
		if len(p.ContourSets) == 0 {
			return Result{Message: "0 contour sets"}, nil
		}
		var b strings.Builder
		fmt.Fprintf(&b, "%d contour sets", len(p.ContourSets))
		for _, set := range p.SortedContourSets() {
			fmt.Fprintf(&b, "\n%s interval=%s base=%s polylines=%d breaklines=%d index=%d smooth=%d warnings=%d%s",
				set.ID, formatDistance(set.Interval, p.DisplayPrecision()), formatDistance(set.Base, p.DisplayPrecision()),
				len(set.Polylines), len(set.Breaklines), set.IndexEvery, contourSmooth(set), len(set.Diagnostics), contourStaleSuffix(set))
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
		return Result{Message: contourInfo(set, p.DisplayPrecision())}, nil
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

func parseCodeSelector(name, value string) ([]string, error) {
	if !strings.HasPrefix(value, "codes:") {
		return nil, fmt.Errorf("%s must be codes:C1,C2", name)
	}
	value = strings.TrimPrefix(value, "codes:")
	if value == "" {
		return nil, fmt.Errorf("%s codes list is empty", name)
	}
	codes := strings.Split(value, ",")
	for _, code := range codes {
		if code == "" {
			return nil, fmt.Errorf("%s codes list contains an empty code", name)
		}
	}
	return codes, nil
}

func optionsFromSpec(id string, spec *project.ContourGenerationSpec) terrain.Options {
	opts := terrain.Options{
		ID: id, Interval: spec.Interval, Base: spec.Base, IndexEvery: spec.IndexEvery, IndexEverySet: spec.IndexEverySet,
		BreaklineMode: spec.BreaklineMode, BreaklineIDs: append([]string(nil), spec.BreaklineIDs...),
		BoundaryCodes: append([]string(nil), spec.BoundaryCodes...), ExclusionCodes: append([]string(nil), spec.ExclusionCodes...),
		MaxEdge: spec.MaxEdge, Smooth: spec.Smooth,
	}
	opts.UseBreakline = spec.BreaklineMode != "none"
	return opts
}

func contourStaleSuffix(set project.ContourSet) string {
	if set.Stale {
		return " stale"
	}
	return ""
}

func contourInfo(set project.ContourSet, precision int) string {
	msg := fmt.Sprintf("%s interval=%s base=%s polylines=%d breaklines=%d triangles=%d maxedge=%s smooth=%d warnings=%d%s",
		set.ID, formatDistance(set.Interval, precision), formatDistance(set.Base, precision), len(set.Polylines), len(set.Breaklines),
		set.TriangleCount, formatDistance(set.EffectiveMaxEdge, precision), contourSmooth(set), len(set.Diagnostics), contourStaleSuffix(set))
	if set.Generation != nil {
		msg += fmt.Sprintf(" breakline_mode=%s boundary_codes=%s exclusion_codes=%s",
			set.Generation.BreaklineMode, strings.Join(set.Generation.BoundaryCodes, ","), strings.Join(set.Generation.ExclusionCodes, ","))
	}
	if set.StaleReason != "" {
		msg += " reason=" + set.StaleReason
	}
	for _, diagnostic := range set.Diagnostics {
		msg += fmt.Sprintf("\nwarning %s: %s", diagnostic.Code, diagnostic.Message)
	}
	return msg
}

func contourSmooth(set project.ContourSet) int {
	if set.Generation == nil {
		return 0
	}
	return set.Generation.Smooth
}

func contourGenerationMessage(action string, set project.ContourSet) string {
	msg := fmt.Sprintf("%s contour set %s with %d polylines", action, set.ID, len(set.Polylines))
	if len(set.Diagnostics) > 0 {
		msg += fmt.Sprintf(" (%d warnings)", len(set.Diagnostics))
	}
	return msg
}
