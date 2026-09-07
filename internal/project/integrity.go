package project

import (
	"encoding/json"
	"fmt"
	"strings"

	"lsurvey/internal/geom"
)

// Validate checks domain integrity without changing the project or depending on
// a storage format. Stale contour source IDs are retained as provenance.
func (p *Project) Validate() error {
	if p == nil {
		return fmt.Errorf("project is nil")
	}
	if p.Display.Precision != nil && (*p.Display.Precision < 0 || *p.Display.Precision > 6) {
		return fmt.Errorf("display precision must be 0 through 6")
	}
	groupExists := func(id string) bool { _, ok := p.Groups[id]; return ok }
	layers := map[string]bool{}
	for key, g := range p.Groups {
		layer := strings.ToLower(g.Layer)
		if strings.TrimSpace(key) == "" || key != g.ID || strings.TrimSpace(layer) == "" || layers[layer] || g.Color < 1 || g.Color > 255 {
			return fmt.Errorf("group %q has invalid identity or duplicate/invalid layer/color", key)
		}
		layers[layer] = true
	}
	for key, pt := range p.Points {
		if strings.TrimSpace(key) == "" || key != pt.ID || !geom.FinitePoint(pt) {
			return fmt.Errorf("point %q has invalid identity or coordinates", key)
		}
		if pt.GroupID != "" && !groupExists(pt.GroupID) {
			return fmt.Errorf("point %q references missing group %q", key, pt.GroupID)
		}
	}
	for code, group := range p.PointCodeStyles {
		if code == "" || !groupExists(group) {
			return fmt.Errorf("point code %q references missing group %q", code, group)
		}
	}
	for key, f := range p.Features {
		if strings.TrimSpace(key) == "" || key != f.ID {
			return fmt.Errorf("feature %q has invalid identity", key)
		}
		if err := ValidateFeature(f, func(id string) (geom.Point, bool) { pt, ok := p.Points[id]; return pt, ok }, groupExists); err != nil {
			return fmt.Errorf("feature %q: %w", key, err)
		}
	}
	if c := p.HorizontalCRS; c != nil {
		if c.Projection != "MGA" || (c.Datum != "GDA94" && c.Datum != "GDA2020") || c.Zone < 46 || c.Zone > 59 {
			return fmt.Errorf("invalid horizontal CRS")
		}
	}
	if g := p.GridGround; g != nil {
		if g.Mode != "local_ground" || !geom.Finite(g.CSF) || g.CSF <= 0 || !geom.Finite(g.AnchorEasting) || !geom.Finite(g.AnchorNorthing) {
			return fmt.Errorf("invalid grid-to-ground conversion")
		}
		// Anchor coordinates remain authoritative if the anchor point is deleted.
	}
	if tr := p.Traverse; tr != nil {
		ids := append([]string{tr.Start, tr.Current}, tr.LegPointIDs...)
		if tr.Close != "" {
			ids = append(ids, tr.Close)
		}
		for _, id := range ids {
			if _, ok := p.Points[id]; !ok {
				return fmt.Errorf("traverse references missing point %q", id)
			}
		}
		seen := map[string]bool{tr.Start: true}
		for _, id := range tr.LegPointIDs {
			if seen[id] || id == tr.Close {
				return fmt.Errorf("invalid repeated traverse/control point %q", id)
			}
			seen[id] = true
		}
		if tr.Adjusted && tr.Close == "" {
			return fmt.Errorf("adjusted traverse has no closing control")
		}
	}
	for key, c := range p.ContourSets {
		if key == "" || key != c.ID || !geom.Finite(c.Interval) || c.Interval <= 0 || !geom.Finite(c.Base) || c.IndexEvery < 0 || c.TriangleCount < 0 || !geom.Finite(c.EffectiveMaxEdge) || c.EffectiveMaxEdge < 0 {
			return fmt.Errorf("contour %q has invalid metadata", key)
		}
		if g := c.Generation; g != nil {
			if !geom.Finite(g.Interval) || g.Interval <= 0 || g.IndexEvery < 0 || g.Smooth < 0 || g.Smooth > 3 || g.Base != nil && !geom.Finite(*g.Base) || g.MaxEdge != nil && (!geom.Finite(*g.MaxEdge) || *g.MaxEdge <= 0) {
				return fmt.Errorf("contour %q has invalid generation settings", key)
			}
			if g.BreaklineMode != "" && g.BreaklineMode != "all" && g.BreaklineMode != "none" && g.BreaklineMode != "ids" {
				return fmt.Errorf("contour %q has invalid breakline mode", key)
			}
		}
		for _, lines := range [][]ContourPolyline{c.RawPolylines, c.Polylines} {
			seen := map[string]bool{}
			for _, line := range lines {
				if line.ID == "" || seen[line.ID] || !geom.Finite(line.Elevation) || len(line.Vertices) < 2 {
					return fmt.Errorf("contour %q has invalid polyline", key)
				}
				seen[line.ID] = true
				for _, v := range line.Vertices {
					if !geom.Finite(v.Easting) || !geom.Finite(v.Northing) {
						return fmt.Errorf("contour %q has non-finite vertices", key)
					}
				}
			}
		}
		for _, d := range c.Diagnostics {
			if !geom.Finite(d.Measured) || !geom.Finite(d.Limit) {
				return fmt.Errorf("contour %q has invalid diagnostic", key)
			}
		}
		if !c.Stale {
			for _, id := range c.SourcePoints {
				pt, ok := p.Points[id]
				if !ok || pt.Elevation == nil {
					return fmt.Errorf("contour %q references missing/2D source point %q", key, id)
				}
			}
			for _, ids := range [][]string{c.Breaklines, c.BoundaryLines, c.ExclusionLines} {
				for _, id := range ids {
					if _, ok := p.Features[id]; !ok {
						return fmt.Errorf("contour %q references missing feature %q", key, id)
					}
				}
			}
		}
	}
	for i, h := range p.History {
		if len(h.Extra) > 0 && !json.Valid(h.Extra) {
			return fmt.Errorf("history entry %d has invalid extra data", i+1)
		}
	}
	return nil
}
