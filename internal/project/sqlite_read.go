package project

import (
	"database/sql"
	"fmt"
	"time"

	"lsurvey/internal/geom"
)

func eachRow(q sqlReader, query string, fn func(*sql.Rows) error, args ...any) error {
	rows, err := q.Query(query, args...)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		if err = fn(rows); err != nil {
			return err
		}
	}
	return rows.Err()
}

type rowReader struct {
	q   sqlReader
	err error
}

func (r *rowReader) each(query string, fn func(*sql.Rows) error) {
	if r.err == nil {
		r.err = eachRow(r.q, query, fn)
	}
}

func appendOrdered[T any](values *[]T, ordinal int, value T) error {
	if ordinal != len(*values) {
		return fmt.Errorf("invalid ordered data: expected ordinal %d, got %d", len(*values), ordinal)
	}
	*values = append(*values, value)
	return nil
}

func readSQLiteProject(q sqlReader) (*Project, error) {
	p := New("")
	p.Units = map[string]string{}
	var precision int
	if err := q.QueryRow("SELECT name,description,app_version,precision FROM project_metadata WHERE id=1").Scan(&p.Name, &p.Description, &p.AppVersion, &precision); err != nil {
		return nil, err
	}
	p.Display.Precision = &precision
	r := rowReader{q: q}
	r.each("SELECT name,value FROM units", func(rows *sql.Rows) error {
		var key, value string
		if err := rows.Scan(&key, &value); err != nil {
			return err
		}
		p.Units[key] = value
		return nil
	})
	r.each("SELECT id,layer,color,description FROM style_groups", func(rows *sql.Rows) error {
		var g Group
		if err := rows.Scan(&g.ID, &g.Layer, &g.Color, &g.Description); err != nil {
			return err
		}
		p.Groups[g.ID] = g
		return nil
	})
	r.each("SELECT id,easting,northing,elevation,code,description,coalesce(group_id,'') FROM points", func(rows *sql.Rows) error {
		var pt geom.Point
		if err := rows.Scan(&pt.ID, &pt.Easting, &pt.Northing, &pt.Elevation, &pt.Code, &pt.Description, &pt.GroupID); err != nil {
			return err
		}
		p.Points[pt.ID] = pt
		return nil
	})
	r.each("SELECT code,group_id FROM point_code_styles", func(rows *sql.Rows) error {
		var code, group string
		if err := rows.Scan(&code, &group); err != nil {
			return err
		}
		p.PointCodeStyles[code] = group
		return nil
	})
	r.each("SELECT id,kind,code,description,terrain_role,coalesce(group_id,'') FROM features", func(rows *sql.Rows) error {
		var f Feature
		if err := rows.Scan(&f.ID, &f.Kind, &f.Code, &f.Description, &f.TerrainRole, &f.GroupID); err != nil {
			return err
		}
		p.Features[f.ID] = f
		return nil
	})
	r.each("SELECT feature_id,ordinal,point_id FROM feature_vertices ORDER BY feature_id,ordinal", func(rows *sql.Rows) error {
		var id, point string
		var ordinal int
		if err := rows.Scan(&id, &ordinal, &point); err != nil {
			return err
		}
		f, ok := p.Features[id]
		if !ok {
			return fmt.Errorf("orphan feature vertex")
		}
		if err := appendOrdered(&f.PointIDs, ordinal, point); err != nil {
			return err
		}
		p.Features[id] = f
		return nil
	})
	r.each("SELECT datum,projection,zone FROM horizontal_crs", func(rows *sql.Rows) error {
		var c HorizontalCRS
		if err := rows.Scan(&c.Datum, &c.Projection, &c.Zone); err != nil {
			return err
		}
		p.HorizontalCRS = &c
		return nil
	})
	r.each("SELECT mode,grid_system,anchor_point_id,anchor_easting,anchor_northing,csf FROM grid_ground", func(rows *sql.Rows) error {
		var g GridGroundConversion
		if err := rows.Scan(&g.Mode, &g.GridSystem, &g.AnchorPointID, &g.AnchorEasting, &g.AnchorNorthing, &g.CSF); err != nil {
			return err
		}
		p.GridGround = &g
		return nil
	})
	r.each("SELECT start_id,current_id,coalesce(close_id,''),adjusted FROM traverse", func(rows *sql.Rows) error {
		var tr TraverseState
		if err := rows.Scan(&tr.Start, &tr.Current, &tr.Close, &tr.Adjusted); err != nil {
			return err
		}
		p.Traverse = &tr
		return nil
	})
	r.each("SELECT ordinal,point_id FROM traverse_legs ORDER BY ordinal", func(rows *sql.Rows) error {
		var ordinal int
		var id string
		if err := rows.Scan(&ordinal, &id); err != nil {
			return err
		}
		if p.Traverse == nil {
			return fmt.Errorf("orphan traverse leg")
		}
		return appendOrdered(&p.Traverse.LegPointIDs, ordinal, id)
	})
	r.each("SELECT id,interval,base,index_every,stale,stale_reason,generated_at,triangle_count,effective_max_edge FROM contour_sets", func(rows *sql.Rows) error {
		var c ContourSet
		var at string
		if err := rows.Scan(&c.ID, &c.Interval, &c.Base, &c.IndexEvery, &c.Stale, &c.StaleReason, &at, &c.TriangleCount, &c.EffectiveMaxEdge); err != nil {
			return err
		}
		var err error
		c.GeneratedAt, err = time.Parse(time.RFC3339Nano, at)
		if err != nil {
			return err
		}
		p.ContourSets[c.ID] = c
		return nil
	})
	r.each("SELECT set_id,interval,base,index_every,index_every_set,breakline_mode,max_edge,smooth FROM contour_generation", func(rows *sql.Rows) error {
		var id string
		var g ContourGenerationSpec
		if err := rows.Scan(&id, &g.Interval, &g.Base, &g.IndexEvery, &g.IndexEverySet, &g.BreaklineMode, &g.MaxEdge, &g.Smooth); err != nil {
			return err
		}
		c := p.ContourSets[id]
		c.Generation = &g
		p.ContourSets[id] = c
		return nil
	})
	r.each("SELECT set_id,kind,ordinal,value FROM contour_references ORDER BY set_id,kind,ordinal", func(rows *sql.Rows) error {
		var id, kind, value string
		var ordinal int
		if err := rows.Scan(&id, &kind, &ordinal, &value); err != nil {
			return err
		}
		c := p.ContourSets[id]
		var target *[]string
		switch kind {
		case "point":
			target = &c.SourcePoints
		case "breakline":
			target = &c.Breaklines
		case "boundary":
			target = &c.BoundaryLines
		case "exclusion":
			target = &c.ExclusionLines
		default:
			if c.Generation == nil {
				return fmt.Errorf("contour generation reference without settings")
			}
			switch kind {
			case "generation_breakline":
				target = &c.Generation.BreaklineIDs
			case "boundary_code":
				target = &c.Generation.BoundaryCodes
			case "exclusion_code":
				target = &c.Generation.ExclusionCodes
			default:
				return fmt.Errorf("unknown contour reference kind")
			}
		}
		if err := appendOrdered(target, ordinal, value); err != nil {
			return err
		}
		p.ContourSets[id] = c
		return nil
	})
	r.each("SELECT set_id,feature_id,role FROM contour_roles", func(rows *sql.Rows) error {
		var id, feature, role string
		if err := rows.Scan(&id, &feature, &role); err != nil {
			return err
		}
		c := p.ContourSets[id]
		if c.BreaklineRoles == nil {
			c.BreaklineRoles = map[string]string{}
		}
		c.BreaklineRoles[feature] = role
		p.ContourSets[id] = c
		return nil
	})
	lineIndices := map[[3]string]int{}
	r.each("SELECT set_id,representation,id,ordinal,elevation,is_index FROM contour_polylines ORDER BY set_id,representation,ordinal", func(rows *sql.Rows) error {
		var id, representation string
		var ordinal int
		var line ContourPolyline
		if err := rows.Scan(&id, &representation, &line.ID, &ordinal, &line.Elevation, &line.Index); err != nil {
			return err
		}
		c := p.ContourSets[id]
		target := &c.Polylines
		if representation == "raw" {
			target = &c.RawPolylines
		}
		if err := appendOrdered(target, ordinal, line); err != nil {
			return err
		}
		lineIndices[[3]string{id, representation, line.ID}] = ordinal
		p.ContourSets[id] = c
		return nil
	})
	r.each("SELECT set_id,representation,polyline_id,ordinal,easting,northing FROM contour_vertices ORDER BY set_id,representation,polyline_id,ordinal", func(rows *sql.Rows) error {
		var id, representation, lineID string
		var ordinal int
		var v ContourVertex
		if err := rows.Scan(&id, &representation, &lineID, &ordinal, &v.Easting, &v.Northing); err != nil {
			return err
		}
		index, ok := lineIndices[[3]string{id, representation, lineID}]
		if !ok {
			return fmt.Errorf("orphan contour vertex")
		}
		c := p.ContourSets[id]
		lines := c.Polylines
		if representation == "raw" {
			lines = c.RawPolylines
		}
		return appendOrdered(&lines[index].Vertices, ordinal, v)
	})
	r.each("SELECT set_id,ordinal,code,message,measured,limit_value FROM contour_diagnostics ORDER BY set_id,ordinal", func(rows *sql.Rows) error {
		var id string
		var ordinal int
		var d ContourDiagnostic
		if err := rows.Scan(&id, &ordinal, &d.Code, &d.Message, &d.Measured, &d.Limit); err != nil {
			return err
		}
		c := p.ContourSets[id]
		if err := appendOrdered(&c.Diagnostics, ordinal, d); err != nil {
			return err
		}
		p.ContourSets[id] = c
		return nil
	})
	r.each("SELECT set_id,diagnostic,kind,ordinal,value FROM contour_diagnostic_refs ORDER BY set_id,diagnostic,kind,ordinal", func(rows *sql.Rows) error {
		var id, kind, value string
		var diagnostic, ordinal int
		if err := rows.Scan(&id, &diagnostic, &kind, &ordinal, &value); err != nil {
			return err
		}
		c := p.ContourSets[id]
		if diagnostic < 0 || diagnostic >= len(c.Diagnostics) {
			return fmt.Errorf("orphan contour diagnostic reference")
		}
		target := &c.Diagnostics[diagnostic].PointIDs
		if kind == "edge" {
			target = &c.Diagnostics[diagnostic].EdgeIDs
		}
		return appendOrdered(target, ordinal, value)
	})
	r.each("SELECT sequence,recorded_at,command,result,error FROM audit_events ORDER BY sequence", func(rows *sql.Rows) error {
		var seq int
		var at string
		var h HistoryRecord
		if err := rows.Scan(&seq, &at, &h.Command, &h.Result, &h.Error); err != nil {
			return err
		}
		var err error
		h.At, err = time.Parse(time.RFC3339Nano, at)
		if err != nil {
			return err
		}
		return appendOrdered(&p.History, seq-1, h)
	})
	r.each("SELECT event_id,kind,ordinal,object_ref FROM audit_changes ORDER BY event_id,kind,ordinal", func(rows *sql.Rows) error {
		var seq, ordinal int
		var kind, ref string
		if err := rows.Scan(&seq, &kind, &ordinal, &ref); err != nil {
			return err
		}
		if seq < 1 || seq > len(p.History) {
			return fmt.Errorf("orphan audit change")
		}
		h := &p.History[seq-1]
		var target *[]string
		switch kind {
		case "created":
			target = &h.Created
		case "updated":
			target = &h.Updated
		case "deleted":
			target = &h.Deleted
		default:
			return fmt.Errorf("unknown audit change kind")
		}
		return appendOrdered(target, ordinal, ref)
	})
	if r.err != nil {
		return nil, r.err
	}
	for i := range p.History {
		raw, err := readAuditValues(q, i+1)
		if err != nil {
			return nil, err
		}
		p.History[i].Extra = raw
	}
	if err := p.Validate(); err != nil {
		return nil, fmt.Errorf("project integrity: %w", err)
	}
	return p, nil
}
