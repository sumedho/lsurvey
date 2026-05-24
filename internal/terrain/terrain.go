package terrain

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"lsurvey/internal/project"
)

const eps = 1e-9

type Options struct {
	ID             string
	Interval       float64
	Base           *float64
	IndexEvery     int
	IndexEverySet  bool
	BreaklineIDs   []string
	UseBreakline   bool
	BreaklineMode  string
	BoundaryCodes  []string
	ExclusionCodes []string
	MaxEdge        *float64
	Smooth         int
}

type vertex struct {
	ID string
	N  float64
	E  float64
	Z  float64
}

type triangle struct {
	A int
	B int
	C int
}

type edge struct {
	A int
	B int
}

type segment struct {
	Level float64
	Index bool
	A     project.ContourVertex
	B     project.ContourVertex
}

func Generate(p *project.Project, opts Options) (project.ContourSet, error) {
	if strings.TrimSpace(opts.ID) == "" {
		return project.ContourSet{}, fmt.Errorf("contour id is required")
	}
	if opts.Interval <= 0 {
		return project.ContourSet{}, fmt.Errorf("contour interval must be greater than zero")
	}
	if opts.IndexEvery < 0 {
		return project.ContourSet{}, fmt.Errorf("index interval must be zero or greater")
	}
	if opts.MaxEdge != nil && *opts.MaxEdge <= 0 {
		return project.ContourSet{}, fmt.Errorf("maximum edge distance must be greater than zero")
	}
	if opts.Smooth < 0 || opts.Smooth > 3 {
		return project.ContourSet{}, fmt.Errorf("smoothing iterations must be between zero and three")
	}
	points, diagnostics, err := elevatedPoints(p)
	if err != nil {
		return project.ContourSet{}, err
	}
	breaklines, err := validateBreaklines(p, opts, points)
	if err != nil {
		return project.ContourSet{}, err
	}
	tris, err := triangulate(points)
	if err != nil {
		return project.ContourSet{}, err
	}
	if len(breaklines) > 0 {
		tris, err = insertConstraintEdges(points, tris, breaklines)
		if err != nil {
			return project.ContourSet{}, err
		}
	}
	effectiveMaxEdge := maximumEdgeThreshold(points, opts.MaxEdge)
	diagnostics = append(diagnostics, longEdgeDiagnostics(points, tris, effectiveMaxEdge)...)
	clip, err := resolveClipRegions(p, opts)
	if err != nil {
		return project.ContourSet{}, err
	}

	levels := contourLevels(points, opts.Interval, opts.Base)
	segs := sliceTriangles(points, tris, levels, opts)
	segs = clipSegments(segs, clip)
	segs = deduplicateSegments(segs)
	rawPolylines := joinSegments(segs)
	sort.Slice(rawPolylines, func(i, j int) bool {
		if math.Abs(rawPolylines[i].Elevation-rawPolylines[j].Elevation) > eps {
			return rawPolylines[i].Elevation < rawPolylines[j].Elevation
		}
		return rawPolylines[i].ID < rawPolylines[j].ID
	})
	for i := range rawPolylines {
		rawPolylines[i].ID = fmt.Sprintf("%s-%04d", opts.ID, i+1)
	}
	if len(clip.Boundary) == 0 {
		diagnostics = append(diagnostics, hullContactDiagnostics(points, tris, rawPolylines)...)
	}
	polylines := rawPolylines
	var storedRaw []project.ContourPolyline
	if opts.Smooth > 0 {
		storedRaw = clonePolylines(rawPolylines)
		var smoothDiagnostics []project.ContourDiagnostic
		polylines, smoothDiagnostics = smoothPolylines(rawPolylines, opts.Smooth, clip, p, breaklines)
		diagnostics = append(diagnostics, smoothDiagnostics...)
	}

	sourceIDs := make([]string, 0, len(points))
	for _, pt := range points {
		sourceIDs = append(sourceIDs, pt.ID)
	}
	breaklineIDs := make([]string, 0, len(breaklines))
	breaklineRoles := make(map[string]string)
	for _, bl := range breaklines {
		breaklineIDs = append(breaklineIDs, bl.ID)
		if role := p.Lines[bl.ID].TerrainRole; role != "" {
			breaklineRoles[bl.ID] = role
		}
	}
	sort.Strings(breaklineIDs)
	base := levels.Base
	if opts.Base != nil {
		base = *opts.Base
	}
	mode := opts.BreaklineMode
	if mode == "" {
		if opts.UseBreakline {
			if len(opts.BreaklineIDs) > 0 {
				mode = "ids"
			} else {
				mode = "all"
			}
		} else {
			mode = "none"
		}
	}
	spec := &project.ContourGenerationSpec{
		Interval:       opts.Interval,
		Base:           cloneFloat(opts.Base),
		IndexEvery:     opts.IndexEvery,
		IndexEverySet:  opts.IndexEverySet,
		BreaklineMode:  mode,
		BreaklineIDs:   append([]string(nil), opts.BreaklineIDs...),
		BoundaryCodes:  append([]string(nil), opts.BoundaryCodes...),
		ExclusionCodes: append([]string(nil), opts.ExclusionCodes...),
		MaxEdge:        cloneFloat(opts.MaxEdge),
		Smooth:         opts.Smooth,
	}
	return project.ContourSet{
		ID:               opts.ID,
		Interval:         opts.Interval,
		Base:             base,
		IndexEvery:       opts.IndexEvery,
		SourcePoints:     sourceIDs,
		Breaklines:       breaklineIDs,
		BreaklineRoles:   breaklineRoles,
		BoundaryLines:    clip.BoundaryLines,
		ExclusionLines:   clip.ExclusionLines,
		Generation:       spec,
		GeneratedAt:      time.Now().UTC(),
		TriangleCount:    len(tris),
		EffectiveMaxEdge: effectiveMaxEdge,
		Diagnostics:      diagnostics,
		RawPolylines:     storedRaw,
		Polylines:        polylines,
	}, nil
}

func cloneFloat(v *float64) *float64 {
	if v == nil {
		return nil
	}
	out := *v
	return &out
}

type levelSet struct {
	Values []float64
	Base   float64
}

func elevatedPoints(p *project.Project) ([]vertex, []project.ContourDiagnostic, error) {
	points := p.SortedPoints()
	out := make([]vertex, 0, len(points))
	seen := map[string]vertex{}
	var diagnostics []project.ContourDiagnostic
	for _, pt := range points {
		if pt.Elevation == nil {
			continue
		}
		key := coordKey(pt.Easting, pt.Northing)
		if prev, ok := seen[key]; ok && math.Abs(prev.Z-*pt.Elevation) > eps {
			return nil, nil, fmt.Errorf("points %q and %q share coordinates with different elevations", prev.ID, pt.ID)
		} else if ok {
			diagnostics = append(diagnostics, project.ContourDiagnostic{
				Code:     "duplicate_xy",
				Message:  fmt.Sprintf("ignored coincident elevated point %s; using %s", pt.ID, prev.ID),
				PointIDs: []string{prev.ID, pt.ID},
			})
			continue
		}
		v := vertex{ID: pt.ID, N: pt.Northing, E: pt.Easting, Z: *pt.Elevation}
		seen[key] = v
		out = append(out, v)
	}
	if len(out) < 3 {
		return nil, nil, fmt.Errorf("at least 3 elevated points are required")
	}
	return out, diagnostics, nil
}

func coordKey(e, n float64) string {
	return strconv.FormatFloat(e, 'f', 9, 64) + "," + strconv.FormatFloat(n, 'f', 9, 64)
}

func contourLevels(points []vertex, interval float64, base *float64) levelSet {
	minZ, maxZ := points[0].Z, points[0].Z
	for _, pt := range points[1:] {
		minZ = math.Min(minZ, pt.Z)
		maxZ = math.Max(maxZ, pt.Z)
	}
	b := 0.0
	if base != nil {
		b = *base
	}
	first := b + math.Ceil((minZ-b)/interval)*interval
	var values []float64
	for z := first; z <= maxZ+eps; z += interval {
		if z > minZ+eps && z < maxZ-eps || math.Abs(z-minZ) <= eps || math.Abs(z-maxZ) <= eps {
			values = append(values, roundLevel(z))
		}
	}
	return levelSet{Values: values, Base: b}
}

func roundLevel(v float64) float64 {
	return math.Round(v*1e9) / 1e9
}

func sliceTriangles(points []vertex, tris []triangle, levels levelSet, opts Options) []segment {
	var out []segment
	for _, tri := range tris {
		vs := []vertex{points[tri.A], points[tri.B], points[tri.C]}
		minZ := math.Min(vs[0].Z, math.Min(vs[1].Z, vs[2].Z))
		maxZ := math.Max(vs[0].Z, math.Max(vs[1].Z, vs[2].Z))
		for _, level := range levels.Values {
			if level < minZ-eps || level > maxZ+eps {
				continue
			}
			pts := triangleContourPoints(vs, level)
			if len(pts) != 2 || sameVertex(pts[0], pts[1]) {
				continue
			}
			out = append(out, segment{
				Level: level,
				Index: isIndexLevel(level, levels.Base, opts.Interval, opts.IndexEvery, opts.IndexEverySet),
				A:     pts[0],
				B:     pts[1],
			})
		}
	}
	return out
}

func deduplicateSegments(segments []segment) []segment {
	seen := map[string]bool{}
	out := make([]segment, 0, len(segments))
	for _, seg := range segments {
		a, b := seg.A, seg.B
		if vertexLess(b, a) {
			a, b = b, a
		}
		key := fmt.Sprintf("%.9f:%.9f:%.9f:%.9f:%.9f", seg.Level, a.Easting, a.Northing, b.Easting, b.Northing)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, seg)
	}
	return out
}

func vertexLess(a, b project.ContourVertex) bool {
	if math.Abs(a.Easting-b.Easting) > eps {
		return a.Easting < b.Easting
	}
	return a.Northing < b.Northing
}

func triangleContourPoints(vs []vertex, level float64) []project.ContourVertex {
	type pair struct{ I, J int }
	edges := []pair{{0, 1}, {1, 2}, {2, 0}}
	var out []project.ContourVertex
	for _, e := range edges {
		a := vs[e.I]
		b := vs[e.J]
		da := a.Z - level
		db := b.Z - level
		if math.Abs(da) <= eps && math.Abs(db) <= eps {
			continue
		}
		if math.Abs(da) <= eps {
			out = addUniqueVertex(out, project.ContourVertex{Northing: a.N, Easting: a.E})
			continue
		}
		if math.Abs(db) <= eps {
			out = addUniqueVertex(out, project.ContourVertex{Northing: b.N, Easting: b.E})
			continue
		}
		if da*db > 0 {
			continue
		}
		t := (level - a.Z) / (b.Z - a.Z)
		out = addUniqueVertex(out, project.ContourVertex{
			Northing: a.N + t*(b.N-a.N),
			Easting:  a.E + t*(b.E-a.E),
		})
	}
	if len(out) > 2 {
		sort.Slice(out, func(i, j int) bool {
			if math.Abs(out[i].Easting-out[j].Easting) > eps {
				return out[i].Easting < out[j].Easting
			}
			return out[i].Northing < out[j].Northing
		})
		return out[:2]
	}
	return out
}

func addUniqueVertex(vertices []project.ContourVertex, v project.ContourVertex) []project.ContourVertex {
	for _, existing := range vertices {
		if sameVertex(existing, v) {
			return vertices
		}
	}
	return append(vertices, v)
}

func sameVertex(a, b project.ContourVertex) bool {
	return math.Abs(a.Easting-b.Easting) <= 1e-7 && math.Abs(a.Northing-b.Northing) <= 1e-7
}

func isIndexLevel(level, base, interval float64, every int, everySet bool) bool {
	if !everySet {
		return isWholeNumberLevel(level)
	}
	if every <= 0 {
		return false
	}
	step := interval * float64(every)
	k := math.Round((level - base) / step)
	return math.Abs(level-(base+k*step)) <= 1e-7
}

func isWholeNumberLevel(level float64) bool {
	return math.Abs(level-math.Round(level)) <= 1e-7
}
