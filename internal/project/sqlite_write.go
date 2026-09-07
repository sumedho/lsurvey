package project

import (
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// rowWriter reuses prepared statements for large vertex/point collections.
// Table names are internal constants, never user input.
type rowWriter struct {
	tx         *sql.Tx
	statements map[string]*sql.Stmt
	err        error
}

func (w *rowWriter) insert(table string, values ...any) {
	if w.err != nil {
		return
	}
	stmt := w.statements[table]
	if stmt == nil {
		stmt, w.err = w.tx.Prepare("INSERT INTO " + table + " VALUES (" + strings.TrimSuffix(strings.Repeat("?,", len(values)), ",") + ")")
		if w.err != nil {
			return
		}
		w.statements[table] = stmt
	}
	_, w.err = stmt.Exec(values...)
	if w.err != nil {
		w.err = fmt.Errorf("write %s: %w", table, w.err)
	}
}

func (w *rowWriter) close() {
	for _, s := range w.statements {
		s.Close()
	}
}
func nullable(s string) any {
	if s == "" {
		return nil
	}
	return s
}
func timestamp(t time.Time) string { return t.Format(time.RFC3339Nano) }

func writeSQLiteProject(tx *sql.Tx, p *Project, auditStart int) error {
	// Explicit Save synchronises the in-memory document. Geometry is replaced
	// transactionally in dependency order; audit rows are only ever appended.
	for _, table := range []string{"contour_sets", "traverse", "features", "point_code_styles", "points", "style_groups", "horizontal_crs", "grid_ground", "units", "project_metadata"} {
		if _, err := tx.Exec("DELETE FROM " + table); err != nil {
			return err
		}
	}
	w := rowWriter{tx: tx, statements: map[string]*sql.Stmt{}}
	defer w.close()
	w.insert("project_metadata", 1, p.Name, p.Description, p.AppVersion, p.DisplayPrecision())
	for key, value := range p.Units {
		w.insert("units", key, value)
	}
	for _, g := range p.Groups {
		w.insert("style_groups", g.ID, g.Layer, g.Color, g.Description)
	}
	for _, pt := range p.Points {
		w.insert("points", pt.ID, pt.Easting, pt.Northing, pt.Elevation, pt.Code, pt.Description, nullable(pt.GroupID))
	}
	for code, group := range p.PointCodeStyles {
		w.insert("point_code_styles", code, group)
	}
	for _, f := range p.Features {
		w.insert("features", f.ID, f.Kind, f.Code, f.Description, f.TerrainRole, nullable(f.GroupID))
		for i, id := range f.PointIDs {
			w.insert("feature_vertices", f.ID, i, id)
		}
	}
	if c := p.HorizontalCRS; c != nil {
		w.insert("horizontal_crs", 1, c.Datum, c.Projection, c.Zone)
	}
	if g := p.GridGround; g != nil {
		w.insert("grid_ground", 1, g.Mode, g.GridSystem, g.AnchorPointID, g.AnchorEasting, g.AnchorNorthing, g.CSF)
	}
	if t := p.Traverse; t != nil {
		w.insert("traverse", 1, t.Start, t.Current, nullable(t.Close), t.Adjusted)
		for i, id := range t.LegPointIDs {
			w.insert("traverse_legs", 1, i, id)
		}
	}
	for _, c := range p.ContourSets {
		w.insert("contour_sets", c.ID, c.Interval, c.Base, c.IndexEvery, c.Stale, c.StaleReason, timestamp(c.GeneratedAt), c.TriangleCount, c.EffectiveMaxEdge)
		refs := map[string][]string{"point": c.SourcePoints, "breakline": c.Breaklines, "boundary": c.BoundaryLines, "exclusion": c.ExclusionLines}
		if g := c.Generation; g != nil {
			w.insert("contour_generation", c.ID, g.Interval, g.Base, g.IndexEvery, g.IndexEverySet, g.BreaklineMode, g.MaxEdge, g.Smooth)
			refs["generation_breakline"] = g.BreaklineIDs
			refs["boundary_code"] = g.BoundaryCodes
			refs["exclusion_code"] = g.ExclusionCodes
		}
		for kind, ids := range refs {
			for i, id := range ids {
				w.insert("contour_references", c.ID, kind, i, id)
			}
		}
		for id, role := range c.BreaklineRoles {
			w.insert("contour_roles", c.ID, id, role)
		}
		for representation, lines := range map[string][]ContourPolyline{"raw": c.RawPolylines, "display": c.Polylines} {
			for i, line := range lines {
				w.insert("contour_polylines", c.ID, representation, line.ID, i, line.Elevation, line.Index)
				for j, v := range line.Vertices {
					w.insert("contour_vertices", c.ID, representation, line.ID, j, v.Easting, v.Northing)
				}
			}
		}
		for i, d := range c.Diagnostics {
			w.insert("contour_diagnostics", c.ID, i, d.Code, d.Message, d.Measured, d.Limit)
			for kind, ids := range map[string][]string{"point": d.PointIDs, "edge": d.EdgeIDs} {
				for j, id := range ids {
					w.insert("contour_diagnostic_refs", c.ID, i, kind, j, id)
				}
			}
		}
	}
	for i := auditStart; i < len(p.History); i++ {
		h := p.History[i]
		seq := i + 1
		w.insert("audit_events", seq, timestamp(h.At), h.Command, h.Result, h.Error)
		for kind, ids := range map[string][]string{"created": h.Created, "updated": h.Updated, "deleted": h.Deleted} {
			for j, id := range ids {
				w.insert("audit_changes", seq, kind, j, id)
			}
		}
		if err := writeAuditValues(&w, seq, h.Extra); err != nil {
			return err
		}
	}
	return w.err
}
