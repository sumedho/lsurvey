-- Native lsurvey schema. Versions are independent of the retired JSON schema.
CREATE TABLE project_metadata (
 id INTEGER PRIMARY KEY CHECK(id=1), name TEXT NOT NULL, description TEXT NOT NULL,
 app_version TEXT NOT NULL, precision INTEGER NOT NULL CHECK(precision BETWEEN 0 AND 6)
) STRICT;
CREATE TABLE units (name TEXT PRIMARY KEY, value TEXT NOT NULL) STRICT;
CREATE TABLE style_groups (
 id TEXT PRIMARY KEY, layer TEXT NOT NULL COLLATE NOCASE UNIQUE CHECK(length(layer)>0),
 color INTEGER NOT NULL CHECK(color BETWEEN 1 AND 255), description TEXT NOT NULL
) STRICT;
CREATE TABLE points (
 id TEXT PRIMARY KEY CHECK(length(id)>0), easting REAL NOT NULL, northing REAL NOT NULL,
 elevation REAL, code TEXT NOT NULL, description TEXT NOT NULL,
 group_id TEXT REFERENCES style_groups(id) DEFERRABLE INITIALLY DEFERRED
) STRICT;
CREATE INDEX points_group ON points(group_id);
CREATE TABLE point_code_styles (code TEXT PRIMARY KEY, group_id TEXT NOT NULL REFERENCES style_groups(id)) STRICT;
CREATE TABLE features (
 id TEXT PRIMARY KEY, kind TEXT NOT NULL CHECK(kind IN ('line','polyline','polygon')),
 code TEXT NOT NULL, description TEXT NOT NULL, terrain_role TEXT NOT NULL CHECK(terrain_role IN ('','standard','ridge','drain')),
 group_id TEXT REFERENCES style_groups(id) DEFERRABLE INITIALLY DEFERRED
) STRICT;
CREATE TABLE feature_vertices (
 feature_id TEXT NOT NULL REFERENCES features(id) ON DELETE CASCADE,
 ordinal INTEGER NOT NULL CHECK(ordinal>=0), point_id TEXT NOT NULL REFERENCES points(id) DEFERRABLE INITIALLY DEFERRED,
 PRIMARY KEY(feature_id,ordinal)
) STRICT;
CREATE INDEX feature_vertices_point ON feature_vertices(point_id);
CREATE TABLE horizontal_crs (
 id INTEGER PRIMARY KEY CHECK(id=1), datum TEXT NOT NULL CHECK(datum IN ('GDA94','GDA2020')),
 projection TEXT NOT NULL CHECK(projection='MGA'), zone INTEGER NOT NULL CHECK(zone BETWEEN 46 AND 59)
) STRICT;
-- Anchor coordinates are authoritative; its historical point ID may be deleted.
CREATE TABLE grid_ground (
 id INTEGER PRIMARY KEY CHECK(id=1), mode TEXT NOT NULL CHECK(mode='local_ground'),
 grid_system TEXT NOT NULL, anchor_point_id TEXT NOT NULL,
 anchor_easting REAL NOT NULL, anchor_northing REAL NOT NULL, csf REAL NOT NULL CHECK(csf>0)
) STRICT;
CREATE TABLE traverse (
 id INTEGER PRIMARY KEY CHECK(id=1), start_id TEXT NOT NULL REFERENCES points(id),
 current_id TEXT NOT NULL REFERENCES points(id), close_id TEXT REFERENCES points(id),
 adjusted INTEGER NOT NULL CHECK(adjusted IN (0,1))
) STRICT;
CREATE TABLE traverse_legs (
 traverse_id INTEGER NOT NULL REFERENCES traverse(id) ON DELETE CASCADE,
 ordinal INTEGER NOT NULL CHECK(ordinal>=0), point_id TEXT NOT NULL REFERENCES points(id),
 PRIMARY KEY(traverse_id,ordinal), UNIQUE(traverse_id,point_id)
) STRICT;
CREATE TABLE contour_sets (
 id TEXT PRIMARY KEY, interval REAL NOT NULL CHECK(interval>0), base REAL NOT NULL,
 index_every INTEGER NOT NULL CHECK(index_every>=0), stale INTEGER NOT NULL CHECK(stale IN (0,1)),
 stale_reason TEXT NOT NULL, generated_at TEXT NOT NULL,
 triangle_count INTEGER NOT NULL CHECK(triangle_count>=0), effective_max_edge REAL NOT NULL CHECK(effective_max_edge>=0)
) STRICT;
CREATE TABLE contour_generation (
 set_id TEXT PRIMARY KEY REFERENCES contour_sets(id) ON DELETE CASCADE,
 interval REAL NOT NULL CHECK(interval>0), base REAL, index_every INTEGER NOT NULL CHECK(index_every>=0),
 index_every_set INTEGER NOT NULL CHECK(index_every_set IN (0,1)),
 breakline_mode TEXT NOT NULL CHECK(breakline_mode IN ('','all','none','ids')),
 max_edge REAL CHECK(max_edge>0), smooth INTEGER NOT NULL CHECK(smooth BETWEEN 0 AND 3)
) STRICT;
-- Provenance references intentionally do not FK to current geometry: stale sets
-- must retain source identities after geometry is deleted or renamed.
CREATE TABLE contour_references (
 set_id TEXT NOT NULL REFERENCES contour_sets(id) ON DELETE CASCADE,
 kind TEXT NOT NULL CHECK(kind IN ('point','breakline','boundary','exclusion','generation_breakline','boundary_code','exclusion_code')),
 ordinal INTEGER NOT NULL CHECK(ordinal>=0), value TEXT NOT NULL, PRIMARY KEY(set_id,kind,ordinal)
) STRICT;
CREATE TABLE contour_roles (
 set_id TEXT NOT NULL REFERENCES contour_sets(id) ON DELETE CASCADE,
 feature_id TEXT NOT NULL, role TEXT NOT NULL, PRIMARY KEY(set_id,feature_id)
) STRICT;
CREATE TABLE contour_polylines (
 set_id TEXT NOT NULL REFERENCES contour_sets(id) ON DELETE CASCADE,
 representation TEXT NOT NULL CHECK(representation IN ('raw','display')), id TEXT NOT NULL,
 ordinal INTEGER NOT NULL CHECK(ordinal>=0), elevation REAL NOT NULL,
 is_index INTEGER NOT NULL CHECK(is_index IN (0,1)), PRIMARY KEY(set_id,representation,id), UNIQUE(set_id,representation,ordinal)
) STRICT;
CREATE TABLE contour_vertices (
 set_id TEXT NOT NULL, representation TEXT NOT NULL, polyline_id TEXT NOT NULL,
 ordinal INTEGER NOT NULL CHECK(ordinal>=0), easting REAL NOT NULL, northing REAL NOT NULL,
 PRIMARY KEY(set_id,representation,polyline_id,ordinal),
 FOREIGN KEY(set_id,representation,polyline_id) REFERENCES contour_polylines(set_id,representation,id) ON DELETE CASCADE
) STRICT;
CREATE TABLE contour_diagnostics (
 set_id TEXT NOT NULL REFERENCES contour_sets(id) ON DELETE CASCADE,
 ordinal INTEGER NOT NULL CHECK(ordinal>=0), code TEXT NOT NULL, message TEXT NOT NULL,
 measured REAL NOT NULL, limit_value REAL NOT NULL, PRIMARY KEY(set_id,ordinal)
) STRICT;
CREATE TABLE contour_diagnostic_refs (
 set_id TEXT NOT NULL, diagnostic INTEGER NOT NULL, kind TEXT NOT NULL CHECK(kind IN ('point','edge')),
 ordinal INTEGER NOT NULL CHECK(ordinal>=0), value TEXT NOT NULL, PRIMARY KEY(set_id,diagnostic,kind,ordinal),
 FOREIGN KEY(set_id,diagnostic) REFERENCES contour_diagnostics(set_id,ordinal) ON DELETE CASCADE
) STRICT;
CREATE TABLE audit_events (
 sequence INTEGER PRIMARY KEY CHECK(sequence>0), recorded_at TEXT NOT NULL,
 command TEXT NOT NULL, result TEXT NOT NULL, error TEXT NOT NULL
) STRICT;
CREATE TABLE audit_changes (
 event_id INTEGER NOT NULL REFERENCES audit_events(sequence),
 kind TEXT NOT NULL CHECK(kind IN ('created','updated','deleted')),
 ordinal INTEGER NOT NULL CHECK(ordinal>=0), object_ref TEXT NOT NULL, PRIMARY KEY(event_id,kind,ordinal)
) STRICT;
-- Extensible audit details are typed tree nodes, not JSON documents/blobs.
CREATE TABLE audit_values (
 event_id INTEGER NOT NULL REFERENCES audit_events(sequence), node INTEGER NOT NULL,
 parent INTEGER, ordinal INTEGER NOT NULL CHECK(ordinal>=0), member TEXT NOT NULL,
 kind TEXT NOT NULL CHECK(kind IN ('object','array','string','number','boolean','null')), value TEXT,
 PRIMARY KEY(event_id,node), FOREIGN KEY(event_id,parent) REFERENCES audit_values(event_id,node) DEFERRABLE INITIALLY DEFERRED
) STRICT;
CREATE TRIGGER audit_events_no_update BEFORE UPDATE ON audit_events BEGIN SELECT RAISE(ABORT,'audit is append-only'); END;
CREATE TRIGGER audit_events_no_delete BEFORE DELETE ON audit_events BEGIN SELECT RAISE(ABORT,'audit is append-only'); END;
CREATE TRIGGER audit_changes_no_update BEFORE UPDATE ON audit_changes BEGIN SELECT RAISE(ABORT,'audit is append-only'); END;
CREATE TRIGGER audit_changes_no_delete BEFORE DELETE ON audit_changes BEGIN SELECT RAISE(ABORT,'audit is append-only'); END;
CREATE TRIGGER audit_values_no_update BEFORE UPDATE ON audit_values BEGIN SELECT RAISE(ABORT,'audit is append-only'); END;
CREATE TRIGGER audit_values_no_delete BEFORE DELETE ON audit_values BEGIN SELECT RAISE(ABORT,'audit is append-only'); END;
