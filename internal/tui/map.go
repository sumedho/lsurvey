package tui

import (
	"fmt"
	"math"
	"strings"

	"lsurvey/internal/geom"
	"lsurvey/internal/project"
)

const defaultMapZoom = 1.0
const minMapZoom = 0.05
const maxMapZoom = 100.0
const mapPanFraction = 0.20

type MapState struct {
	ShowLines bool
	Zoom      float64
	CenterE   float64
	CenterN   float64
	Custom    bool
}

type mapBounds struct {
	MinE float64
	MaxE float64
	MinN float64
	MaxN float64
}

func newMapState() MapState {
	return MapState{Zoom: defaultMapZoom}
}

func (s *MapState) fit() {
	s.Zoom = defaultMapZoom
	s.Custom = false
}

func (s *MapState) zoomBy(p *project.Project, factor float64) {
	if factor <= 0 {
		return
	}
	if !s.Custom {
		bounds, ok := pointBounds(p.SortedPoints())
		if ok {
			s.CenterE = (bounds.MinE + bounds.MaxE) / 2
			s.CenterN = (bounds.MinN + bounds.MaxN) / 2
		}
		s.Custom = true
	}
	s.Zoom = clampFloat(s.Zoom*factor, minMapZoom, maxMapZoom)
}

func (s *MapState) panBy(p *project.Project, eastFraction, northFraction float64) {
	points := p.SortedPoints()
	bounds, ok := pointBounds(points)
	if !ok {
		return
	}
	if !s.Custom {
		s.CenterE = (bounds.MinE + bounds.MaxE) / 2
		s.CenterN = (bounds.MinN + bounds.MaxN) / 2
		s.Custom = true
	}
	view := visibleBounds(bounds, *s)
	s.CenterE += (view.MaxE - view.MinE) * eastFraction
	s.CenterN += (view.MaxN - view.MinN) * northFraction
}

func renderMap(p *project.Project, state MapState, width, height int, precision int) string {
	width = max(20, width)
	height = max(8, height)
	bodyWidth := max(10, width-4)
	bodyHeight := max(4, height-6)
	points := p.SortedPoints()
	bounds, ok := pointBounds(points)
	if !ok {
		return box("Map", "No points to map\n\nF2/Esc: return  +/-: zoom  f: fit  l: lines  F1: help", width, height)
	}

	view := visibleBounds(bounds, state)
	grid := newRuneGrid(bodyWidth, bodyHeight, ' ')
	if state.ShowLines {
		drawMapLines(grid, p, view)
	}
	for _, pt := range points {
		x, y, ok := mapCell(pt.Easting, pt.Northing, view, bodyWidth, bodyHeight)
		if !ok {
			continue
		}
		grid[y][x] = '*'
		drawLabel(grid, x+1, y, pt.ID)
	}

	status := fmt.Sprintf("points=%d  bounds E:%s..%s N:%s..%s  zoom=%.2fx  lines=%s",
		len(points),
		formatDecimal(view.MinE, precision),
		formatDecimal(view.MaxE, precision),
		formatDecimal(view.MinN, precision),
		formatDecimal(view.MaxN, precision),
		state.Zoom,
		onOff(state.ShowLines),
	)
	body := status + "\n" + strings.Join(gridLines(grid), "\n") + "\n" + "F2/Esc: return  arrows: pan  +/-: zoom  f: fit  l: lines  F1: help"
	return box("Map", body, width, height)
}

func pointBounds(points []geom.Point) (mapBounds, bool) {
	if len(points) == 0 {
		return mapBounds{}, false
	}
	b := mapBounds{MinE: points[0].Easting, MaxE: points[0].Easting, MinN: points[0].Northing, MaxN: points[0].Northing}
	for _, pt := range points[1:] {
		b.MinE = math.Min(b.MinE, pt.Easting)
		b.MaxE = math.Max(b.MaxE, pt.Easting)
		b.MinN = math.Min(b.MinN, pt.Northing)
		b.MaxN = math.Max(b.MaxN, pt.Northing)
	}
	return b, true
}

func visibleBounds(fit mapBounds, state MapState) mapBounds {
	spanE := fit.MaxE - fit.MinE
	spanN := fit.MaxN - fit.MinN
	if spanE == 0 {
		spanE = 1
	}
	if spanN == 0 {
		spanN = 1
	}
	spanE *= 1.1
	spanN *= 1.1
	if state.Zoom <= 0 {
		state.Zoom = defaultMapZoom
	}
	centerE := (fit.MinE + fit.MaxE) / 2
	centerN := (fit.MinN + fit.MaxN) / 2
	if state.Custom {
		centerE = state.CenterE
		centerN = state.CenterN
	}
	spanE /= state.Zoom
	spanN /= state.Zoom
	return mapBounds{
		MinE: centerE - spanE/2,
		MaxE: centerE + spanE/2,
		MinN: centerN - spanN/2,
		MaxN: centerN + spanN/2,
	}
}

func mapCell(easting, northing float64, bounds mapBounds, width, height int) (int, int, bool) {
	if easting < bounds.MinE || easting > bounds.MaxE || northing < bounds.MinN || northing > bounds.MaxN {
		return 0, 0, false
	}
	x := 0
	if bounds.MaxE != bounds.MinE {
		x = int(math.Round((easting - bounds.MinE) / (bounds.MaxE - bounds.MinE) * float64(width-1)))
	}
	y := height / 2
	if bounds.MaxN != bounds.MinN {
		y = int(math.Round((bounds.MaxN - northing) / (bounds.MaxN - bounds.MinN) * float64(height-1)))
	}
	return clampInt(x, 0, width-1), clampInt(y, 0, height-1), true
}

func drawMapLines(grid [][]rune, p *project.Project, bounds mapBounds) {
	height := len(grid)
	if height == 0 {
		return
	}
	width := len(grid[0])
	for _, line := range p.SortedLines() {
		from, ok1 := p.Points[line.From]
		to, ok2 := p.Points[line.To]
		if !ok1 || !ok2 {
			continue
		}
		fromClipped, toClipped, ok := clipLineToBounds(from.Easting, from.Northing, to.Easting, to.Northing, bounds)
		if !ok {
			continue
		}
		x0, y0, ok0 := mapCell(fromClipped.Easting, fromClipped.Northing, bounds, width, height)
		x1, y1, ok1 := mapCell(toClipped.Easting, toClipped.Northing, bounds, width, height)
		if !ok0 || !ok1 {
			continue
		}
		drawLine(grid, x0, y0, x1, y1)
	}
}

func clipLineToBounds(e0, n0, e1, n1 float64, bounds mapBounds) (geom.Point, geom.Point, bool) {
	de := e1 - e0
	dn := n1 - n0
	u1 := 0.0
	u2 := 1.0
	for _, edge := range []struct {
		p float64
		q float64
	}{
		{-de, e0 - bounds.MinE},
		{de, bounds.MaxE - e0},
		{-dn, n0 - bounds.MinN},
		{dn, bounds.MaxN - n0},
	} {
		if edge.p == 0 {
			if edge.q < 0 {
				return geom.Point{}, geom.Point{}, false
			}
			continue
		}
		r := edge.q / edge.p
		if edge.p < 0 {
			if r > u2 {
				return geom.Point{}, geom.Point{}, false
			}
			if r > u1 {
				u1 = r
			}
		} else {
			if r < u1 {
				return geom.Point{}, geom.Point{}, false
			}
			if r < u2 {
				u2 = r
			}
		}
	}
	return geom.Point{Easting: e0 + u1*de, Northing: n0 + u1*dn}, geom.Point{Easting: e0 + u2*de, Northing: n0 + u2*dn}, true
}

func drawLine(grid [][]rune, x0, y0, x1, y1 int) {
	dx := absInt(x1 - x0)
	sx := -1
	if x0 < x1 {
		sx = 1
	}
	dy := -absInt(y1 - y0)
	sy := -1
	if y0 < y1 {
		sy = 1
	}
	err := dx + dy
	for {
		if grid[y0][x0] == ' ' {
			grid[y0][x0] = '.'
		}
		if x0 == x1 && y0 == y1 {
			return
		}
		e2 := 2 * err
		if e2 >= dy {
			err += dy
			x0 += sx
		}
		if e2 <= dx {
			err += dx
			y0 += sy
		}
	}
}

func drawLabel(grid [][]rune, x, y int, label string) {
	if y < 0 || y >= len(grid) {
		return
	}
	for _, r := range label {
		if x < 0 || x >= len(grid[y]) {
			return
		}
		if grid[y][x] == ' ' {
			grid[y][x] = r
		}
		x++
	}
}

func newRuneGrid(width, height int, fill rune) [][]rune {
	grid := make([][]rune, height)
	for y := range grid {
		grid[y] = make([]rune, width)
		for x := range grid[y] {
			grid[y][x] = fill
		}
	}
	return grid
}

func gridLines(grid [][]rune) []string {
	lines := make([]string, len(grid))
	for i := range grid {
		lines[i] = string(grid[i])
	}
	return lines
}

func onOff(value bool) string {
	if value {
		return "on"
	}
	return "off"
}

func clampFloat(value, min, max float64) float64 {
	return math.Max(min, math.Min(max, value))
}

func clampInt(value, min, max int) int {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

func absInt(value int) int {
	if value < 0 {
		return -value
	}
	return value
}
