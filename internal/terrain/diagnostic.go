package terrain

import (
	"fmt"
	"math"
	"sort"

	"lsurvey/internal/project"
)

func maximumEdgeThreshold(points []vertex, requested *float64) float64 {
	if requested != nil {
		return *requested
	}
	nearest := make([]float64, 0, len(points))
	for i, point := range points {
		best := math.Inf(1)
		for j, other := range points {
			if i == j {
				continue
			}
			best = math.Min(best, vertexDistance(point, other))
		}
		if !math.IsInf(best, 1) {
			nearest = append(nearest, best)
		}
	}
	sort.Float64s(nearest)
	if len(nearest) == 0 {
		return 0
	}
	middle := len(nearest) / 2
	median := nearest[middle]
	if len(nearest)%2 == 0 {
		median = (nearest[middle-1] + nearest[middle]) / 2
	}
	return median * 5
}

func longEdgeDiagnostics(points []vertex, tris []triangle, limit float64) []project.ContourDiagnostic {
	if limit <= 0 {
		return nil
	}
	var out []project.ContourDiagnostic
	for _, e := range uniqueTriangleEdges(tris) {
		distance := vertexDistance(points[e.A], points[e.B])
		if distance <= limit+eps {
			continue
		}
		ids := []string{points[e.A].ID, points[e.B].ID}
		sort.Strings(ids)
		out = append(out, project.ContourDiagnostic{
			Code:     "long_edge",
			Message:  fmt.Sprintf("TIN edge %s-%s length %.3f exceeds warning limit %.3f", ids[0], ids[1], distance, limit),
			EdgeIDs:  ids,
			Measured: distance,
			Limit:    limit,
		})
	}
	return out
}

func hullContactDiagnostics(points []vertex, tris []triangle, polylines []project.ContourPolyline) []project.ContourDiagnostic {
	counts := map[edge]int{}
	for _, tri := range tris {
		for _, e := range []edge{normEdge(tri.A, tri.B), normEdge(tri.B, tri.C), normEdge(tri.C, tri.A)} {
			counts[e]++
		}
	}
	var hull []edge
	for e, count := range counts {
		if count == 1 {
			hull = append(hull, e)
		}
	}
	var out []project.ContourDiagnostic
	for _, polyline := range polylines {
		if len(polyline.Vertices) < 2 || sameVertex(polyline.Vertices[0], polyline.Vertices[len(polyline.Vertices)-1]) {
			continue
		}
		if vertexOnEdges(polyline.Vertices[0], hull, points) || vertexOnEdges(polyline.Vertices[len(polyline.Vertices)-1], hull, points) {
			out = append(out, project.ContourDiagnostic{
				Code:    "hull_contact",
				Message: fmt.Sprintf("contour %s reaches the unconstrained terrain hull", polyline.ID),
			})
		}
	}
	return out
}

func uniqueTriangleEdges(tris []triangle) []edge {
	seen := map[edge]bool{}
	var out []edge
	for _, tri := range tris {
		for _, e := range []edge{normEdge(tri.A, tri.B), normEdge(tri.B, tri.C), normEdge(tri.C, tri.A)} {
			if !seen[e] {
				seen[e] = true
				out = append(out, e)
			}
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].A != out[j].A {
			return out[i].A < out[j].A
		}
		return out[i].B < out[j].B
	})
	return out
}

func vertexOnEdges(v project.ContourVertex, edges []edge, points []vertex) bool {
	for _, e := range edges {
		if pointOnContourSegment(v,
			project.ContourVertex{Easting: points[e.A].E, Northing: points[e.A].N},
			project.ContourVertex{Easting: points[e.B].E, Northing: points[e.B].N}) {
			return true
		}
	}
	return false
}

func vertexDistance(a, b vertex) float64 {
	return math.Hypot(a.E-b.E, a.N-b.N)
}
