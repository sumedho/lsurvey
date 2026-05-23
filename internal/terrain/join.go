package terrain

import (
	"fmt"
	"sort"

	"lsurvey/internal/project"
)

func joinSegments(segments []segment) []project.ContourPolyline {
	byLevel := map[float64][]segment{}
	for _, seg := range segments {
		byLevel[seg.Level] = append(byLevel[seg.Level], seg)
	}
	levels := make([]float64, 0, len(byLevel))
	for level := range byLevel {
		levels = append(levels, level)
	}
	sort.Float64s(levels)

	var out []project.ContourPolyline
	for _, level := range levels {
		segs := append([]segment(nil), byLevel[level]...)
		sort.Slice(segs, func(i, j int) bool {
			if !sameVertex(segs[i].A, segs[j].A) {
				if segs[i].A.Easting != segs[j].A.Easting {
					return segs[i].A.Easting < segs[j].A.Easting
				}
				return segs[i].A.Northing < segs[j].A.Northing
			}
			if segs[i].B.Easting != segs[j].B.Easting {
				return segs[i].B.Easting < segs[j].B.Easting
			}
			return segs[i].B.Northing < segs[j].B.Northing
		})
		used := make([]bool, len(segs))
		for i := range segs {
			if used[i] {
				continue
			}
			used[i] = true
			vertices := []project.ContourVertex{segs[i].A, segs[i].B}
			extended := true
			for extended {
				extended = false
				for j := range segs {
					if used[j] {
						continue
					}
					if appendSegment(&vertices, segs[j]) {
						used[j] = true
						extended = true
					}
				}
			}
			if len(vertices) >= 2 {
				out = append(out, project.ContourPolyline{
					ID:        fmt.Sprintf("tmp-%d", len(out)+1),
					Elevation: level,
					Index:     segs[i].Index,
					Vertices:  vertices,
				})
			}
		}
	}
	return out
}

func appendSegment(vertices *[]project.ContourVertex, seg segment) bool {
	v := *vertices
	first := v[0]
	last := v[len(v)-1]
	switch {
	case sameVertex(last, seg.A):
		*vertices = append(v, seg.B)
		return true
	case sameVertex(last, seg.B):
		*vertices = append(v, seg.A)
		return true
	case sameVertex(first, seg.B):
		*vertices = append([]project.ContourVertex{seg.A}, v...)
		return true
	case sameVertex(first, seg.A):
		*vertices = append([]project.ContourVertex{seg.B}, v...)
		return true
	default:
		return false
	}
}
