package terrain

import (
	"fmt"

	"lsurvey/internal/project"
)

func clonePolylines(in []project.ContourPolyline) []project.ContourPolyline {
	out := make([]project.ContourPolyline, len(in))
	for i, polyline := range in {
		out[i] = polyline
		out[i].Vertices = append([]project.ContourVertex(nil), polyline.Vertices...)
	}
	return out
}

func smoothPolylines(raw []project.ContourPolyline, iterations int, clip clipRegions, p *project.Project, breaklines []breakline) ([]project.ContourPolyline, []project.ContourDiagnostic) {
	out := clonePolylines(raw)
	var diagnostics []project.ContourDiagnostic
	for i, polyline := range raw {
		candidate := polyline
		candidate.Vertices = smoothVertices(polyline.Vertices, iterations, clip, p, breaklines)
		if !smoothedPolylineValid(candidate, clip, p, breaklines) {
			diagnostics = append(diagnostics, project.ContourDiagnostic{
				Code:    "smooth_reverted",
				Message: fmt.Sprintf("smoothed contour %s violated a protected terrain constraint; raw geometry retained", polyline.ID),
			})
			continue
		}
		out[i] = candidate
	}
	return out, diagnostics
}

func smoothVertices(vertices []project.ContourVertex, iterations int, clip clipRegions, p *project.Project, breaklines []breakline) []project.ContourVertex {
	if len(vertices) < 3 || iterations == 0 {
		return append([]project.ContourVertex(nil), vertices...)
	}
	if sameVertex(vertices[0], vertices[len(vertices)-1]) && len(vertices) >= 4 {
		unique := append([]project.ContourVertex(nil), vertices[:len(vertices)-1]...)
		anchor := -1
		for i, v := range unique {
			if protectedContact(v, clip, p, breaklines) {
				anchor = i
				break
			}
		}
		if anchor < 0 {
			for n := 0; n < iterations; n++ {
				unique = chaikinClosed(unique)
			}
			return append(unique, unique[0])
		}
		rotated := append([]project.ContourVertex(nil), unique[anchor:]...)
		rotated = append(rotated, unique[:anchor]...)
		rotated = append(rotated, rotated[0])
		return smoothOpenVertices(rotated, iterations, clip, p, breaklines)
	}
	return smoothOpenVertices(vertices, iterations, clip, p, breaklines)
}

func smoothOpenVertices(vertices []project.ContourVertex, iterations int, clip clipRegions, p *project.Project, breaklines []breakline) []project.ContourVertex {
	protected := make([]bool, len(vertices))
	protected[0], protected[len(vertices)-1] = true, true
	for i := 1; i < len(vertices)-1; i++ {
		protected[i] = protectedContact(vertices[i], clip, p, breaklines)
	}
	var out []project.ContourVertex
	start := 0
	for end := 1; end < len(vertices); end++ {
		if !protected[end] {
			continue
		}
		run := append([]project.ContourVertex(nil), vertices[start:end+1]...)
		for n := 0; n < iterations && len(run) >= 3; n++ {
			run = chaikinOpen(run)
		}
		if len(out) > 0 {
			run = run[1:]
		}
		out = append(out, run...)
		start = end
	}
	return out
}

func chaikinClosed(vertices []project.ContourVertex) []project.ContourVertex {
	out := make([]project.ContourVertex, 0, len(vertices)*2)
	for i := range vertices {
		a, b := vertices[i], vertices[(i+1)%len(vertices)]
		out = append(out, interpolate(a, b, 0.25), interpolate(a, b, 0.75))
	}
	return out
}

func chaikinOpen(vertices []project.ContourVertex) []project.ContourVertex {
	out := []project.ContourVertex{vertices[0]}
	for i := 0; i+1 < len(vertices); i++ {
		out = append(out,
			interpolate(vertices[i], vertices[i+1], 0.25),
			interpolate(vertices[i], vertices[i+1], 0.75),
		)
	}
	return append(out, vertices[len(vertices)-1])
}

func protectedContact(v project.ContourVertex, clip clipRegions, p *project.Project, breaklines []breakline) bool {
	for _, ring := range append([][]project.ContourVertex{clip.Boundary}, clip.Exclusions...) {
		for i := range ring {
			if pointOnContourSegment(v, ring[i], ring[(i+1)%len(ring)]) {
				return true
			}
		}
	}
	for _, line := range breaklines {
		if pointOnContourSegment(v, contourVertex(p, line.From), contourVertex(p, line.To)) {
			return true
		}
	}
	return false
}

func smoothedPolylineValid(polyline project.ContourPolyline, clip clipRegions, p *project.Project, breaklines []breakline) bool {
	for i, v := range polyline.Vertices {
		if len(clip.Boundary) > 0 && !pointInRing(v, clip.Boundary, true) {
			return false
		}
		for _, exclusion := range clip.Exclusions {
			if pointInRing(v, exclusion, false) {
				return false
			}
		}
		if i == 0 {
			continue
		}
		a, b := polyline.Vertices[i-1], v
		mid := interpolate(a, b, 0.5)
		if len(clip.Boundary) > 0 && !pointInRing(mid, clip.Boundary, true) {
			return false
		}
		for _, exclusion := range clip.Exclusions {
			if pointInRing(mid, exclusion, false) {
				return false
			}
		}
		for _, ring := range append([][]project.ContourVertex{clip.Boundary}, clip.Exclusions...) {
			for j := range ring {
				c, d := ring[j], ring[(j+1)%len(ring)]
				if contourSegmentsIntersect(a, b, c, d) &&
					!pointOnContourSegment(a, c, d) && !pointOnContourSegment(b, c, d) {
					return false
				}
			}
		}
		for _, line := range breaklines {
			c, d := contourVertex(p, line.From), contourVertex(p, line.To)
			if contourSegmentsIntersect(a, b, c, d) &&
				!pointOnContourSegment(a, c, d) && !pointOnContourSegment(b, c, d) {
				return false
			}
		}
	}
	return true
}
