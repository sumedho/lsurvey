# AGENTS.md

This file is for coding agents working on `lsurvey`.

## Project Overview

`lsurvey` is a Go land-surveying COGO application with a Bubble Tea TUI. The
main design goal is to keep calculation/project logic independent from the TUI
so another UI, such as a web frontend, can be added later.

## Architecture

- `cmd/lsurvey`: CLI entry point, TUI startup, and batch exports.
- `internal/app`: UI-independent project session, command routing, and file
  import/export orchestration.
- `internal/cogo`: command execution and COGO command behavior.
- `internal/geom`: geometry primitives, angle parsing/formatting, point math.
- `internal/project`: project model, save/load, display settings.
- `internal/tui`: Bubble Tea UI, command input, completions, map rendering.
- `internal/help`: command help data and styled help rendering.
- `internal/csvpoints`: point CSV import/export.
- `internal/dxf`: ASCII DXF export.
- `internal/landxml`: LandXML geometry export.
- `internal/boundary`: polygon area/boundary schedules and CSV export.
- `internal/codelib`: reusable point-code style library import/export.
- `internal/paths`: extension handling for `.srv`, `.dxf`, and `.csv`.
- `internal/terrain`: TIN construction, breakline insertion, contour slicing,
  and contour polyline joining.

## Development Rules

- Use TDD for all COGO calculations and command behavior.
- Keep COGO and geometry logic UI-independent.
- Prefer standard library code unless a dependency clearly improves the TUI or
  another established boundary.
- Keep project persistence backward-compatible when practical.
- Add tests for save/load changes.
- Add tests for import/export format changes.
- Add tests for TUI command handling when command behavior changes.
- Do not silently overwrite user data unless the command explicitly says so.
- Point-producing calculations must reject an existing destination point ID.
- Preserve point codes and descriptions unless the command intentionally edits
  them.
- Points may be 2D or 3D. A nil elevation means the point is 2D.
- Display precision affects presentation only; do not round stored coordinates.
- `undo` and `redo` cover successful edits in the current project session,
  survive saves, and reset on `new` or `open`.
- Persisted audit history is append-only; successful `undo` and `redo` actions
  must be recorded rather than deleting earlier audit entries.

## Command Behavior Expectations

- `save` and `saveas` append `.srv` when missing.
- `open` accepts project paths with or without `.srv`.
- `export dxf` appends `.dxf` when missing.
- `export csv` and `import csv` append `.csv` when missing.
- `export landxml` appends `.xml` when missing and currently requires metric
  distance units (`distance=m`).
- CSV import must be atomic: invalid input must not partially mutate the
  project.
- CSV files must use this exact header:

```text
id,easting,northing,elevation,code,description
```

- `pt del` must reject missing points and points referenced by stored features.
- `pt rename` must update feature references.
- `line add` must reject duplicate line IDs.
- `line del` must reject missing line IDs.
- `line edit` and `pt edit` must support quoted descriptions such as
  `desc="this is a point"`.
- Stored user geometry is represented by `line`, `polyline`, and `polygon`
  features; polygons are implicitly closed and must not self-intersect.
- Style groups are flat records with unique case-insensitive DXF layer names
  and ACI colors from 1 through 255; each point or user feature may belong to
  at most one group.
- Point-code style defaults map exact point codes to style groups, apply only
  to newly created/imported points without explicit groups, and persist in
  `.srv`; reusable `.codes.json` library imports must be atomic on conflicts.
- `code style list` reports both style groups and point-code defaults, including
  style groups that are not yet mapped to a point code.
- `polygon report <id|all>` reports stored-polygon area, perimeter, and
  ordered boundary legs; `export boundarycsv` writes schedule rows.
- `offset` must work from either two point IDs or one stored line ID.
- `resect` takes three known points and bearings observed from the unknown point
  to those points, then stores the best-fit point from the reverse bearing lines.
- `rad3d` must use slope distance and zenith angle, with zenith 90° treated as
  horizontal.
- `transform fit` takes alternating source/target point IDs, reports a
  non-mutating similarity fit, and uses only complete 3D pairs for `dz`.
- `shift <base> east=<coordinate> north=<coordinate> elev=<coordinate>` translates
  project geometry so the base reaches entered coordinates; omitted axes are
  unchanged and elevation requires an elevated base.
- `scale apply <base> csf=<factor> [system=<label>]` applies a reversible
  anchored horizontal conversion and displays its active coordinate label.
- `scale reverse` must use the persisted anchor and factor, then remove the
  active scale label; horizontal contour distances scale and elevations do not.
- While a scale is active, whole-project `shift` and `rotate` operations must
  transform its persisted anchor so `scale reverse` remains consistent.
- `map` should not conflict with `midpoint`; the map shortcut key is `F2`.

## Contour Rules

- Contours are stored in `Project.ContourSets` and persisted to `.srv`.
- Contour generation must use only points with elevations.
- Require at least 3 elevated, non-collinear points.
- If `index=<n>` is omitted, whole-number contour elevations are major/index
  contours and intermediate elevations are minor contours.
- `index=0` disables major/index contours.
- Default contour breakline behavior is `breaklines=all`.
- `breaklines=none` must generate from points only.
- `breaklines=ids:L1,L2` must use only those stored line or polyline features.
- Breakline endpoint points must have elevations.
- Selected breaklines must not cross except at shared endpoints.
- `boundary=codes:C1,C2` clips contours to one closed stored-line ring selected
  by line codes; `exclude=codes:C3,C4` clips out closed rings.
- Boundary and exclusion ring points may be 2D and are not implicit breaklines.
- Lines may store terrain roles `standard`, `ridge`, or `drain`; these roles
  currently share constrained-edge interpolation.
- Editing relevant points or lines marks contour sets stale; `contour regen`
  explicitly rebuilds from persisted generation settings.
- Coincident elevated points with equal elevations are deduplicated with a
  diagnostic; coincident points with conflicting elevations are rejected.
- `maxedge=<distance>` sets the long-TIN-edge warning threshold; otherwise use
  five times median nearest-point spacing. Diagnostics are persisted warnings.
- `smooth=<0..3>` enables constraint-safe Chaikin smoothing; default is `0`.
  Preserve raw contour polylines when smoothing is enabled and export the
  smoothed `Polylines` representation to DXF.
- DXF export must include stored contours as polylines on `CONTOURS` or
  `CONTOURS_INDEX`, using different ACI colors and visibly wider polyline
  width for index contours.
- DXF export must add at most one readable contour elevation label per suitable
  contour line on `CONTOUR_LABELS` or `CONTOUR_LABELS_INDEX`, suppressing short
  or colliding labels.
- DXF export must apply a feature or point group's layer and ACI color while
  retaining the fixed contour layers and styles.
- DXF export must place one derived area label inside every stored polygon,
  using its group label layer/color where assigned.
- LandXML export writes points and line/polyline/polygon plan-feature geometry;
  it does not currently write contours or surfaces.
- Keep contour geometry tests in `internal/terrain`; keep command behavior tests
  in `internal/cogo`.

## Angle Rules

- Default angle input is HP-style DMS: `dd.mmsshhhh`.
- Digits after seconds are fractional seconds. For example:
  `123.304567` means 123 degrees, 30 minutes, 45.67 seconds.
- Decimal degrees require a `d` suffix, for example `123.5125d`.
- Quadrant bearings use separated tokens, for example `N 45.0000 E`.
- Angle display should use DMS symbols and hundredths of seconds, for example
  `90°00′00.00″`.

## TUI Expectations

- The command input belongs at the bottom of the TUI.
- The main view should show a top info area, point list area, line list area,
  and command input area.
- Use Lip Gloss for borders and styled regions.
- `F1` opens searchable command help; `/` filters commands and `Enter` opens
  detail for the selected command.
- `F2` toggles the ASCII map.
- `F3` opens the code styling editor for style groups, point-code defaults, and
  reusable code libraries; it shows nominal AutoCAD ACI color previews.
- In the code styling editor, `g` creates a style group and `c` creates a
  point-code default without requiring pane focus.
- Style-group and point-code panes show table headers and scroll using arrows,
  page keys, or the mouse wheel while active.
- Code styling screen saves must submit the existing group, code-style, and
  import/export commands so undo and audit behavior remains consistent.
- Map view supports panning with arrows, zoom in/out, fit-to-points, and line
  overlay.
- Map view starts with point ID labels on each entry; `i` toggles ID/code
  labels and uncoded points fall back to their IDs.
- Map view supports an opt-in stored contour overlay, distinguishing minor and
  index contours and reporting stale displayed contour sets.
- Map rendering should skip off-screen points and clip lines to the viewport.
- Opening or creating a project should reset map state.

## Verification

Run before handing work back:

```sh
gofmt -w <changed-go-files>
go test ./...
go vet ./...
```

If `go vet ./...` cannot access the Go build cache inside a sandbox, rerun it
with the appropriate approval/escalation instead of skipping it.

## Repository Hygiene

- Do not commit local generated files such as ad hoc `.srv`, `.dxf`, or `.csv`
  test artifacts unless they are intentional fixtures.
- `internal/tui/job.srv` is currently a fixture-like file in the repository.
- Keep README command examples aligned with `internal/help/help.go`.
- Keep this file updated when command behavior or agent expectations change.
