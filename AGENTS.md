# AGENTS.md

This file is for coding agents working on `lsurvey`.

## Project Overview

`lsurvey` is a Go land-surveying COGO application with a Bubble Tea TUI. The
main design goal is to keep calculation/project logic independent from the TUI
so another UI, such as a web frontend, can be added later.

## Architecture

- `cmd/lsurvey`: CLI entry point, TUI startup, and batch exports.
- `internal/cogo`: command execution and COGO command behavior.
- `internal/geom`: geometry primitives, angle parsing/formatting, point math.
- `internal/project`: project model, save/load, display settings.
- `internal/tui`: Bubble Tea UI, command input, completions, map rendering.
- `internal/help`: command help data and styled help rendering.
- `internal/csvpoints`: point CSV import/export.
- `internal/dxf`: ASCII DXF export.
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
- Preserve point codes and descriptions unless the command intentionally edits
  them.
- Points may be 2D or 3D. A nil elevation means the point is 2D.
- Display precision affects presentation only; do not round stored coordinates.

## Command Behavior Expectations

- `save` and `saveas` append `.srv` when missing.
- `open` accepts project paths with or without `.srv`.
- `export dxf` appends `.dxf` when missing.
- `export csv` and `import csv` append `.csv` when missing.
- CSV import must be atomic: invalid input must not partially mutate the
  project.
- CSV files must use this exact header:

```text
id,easting,northing,elevation,code,description
```

- `pt del` must reject missing points and points referenced by stored lines.
- `pt rename` must update line references.
- `line add` must reject duplicate line IDs.
- `line del` must reject missing line IDs.
- `line edit` and `pt edit` must support quoted descriptions such as
  `desc="this is a point"`.
- `offset` must work from either two point IDs or one stored line ID.
- `resect` takes three known points and bearings observed from the unknown point
  to those points, then stores the best-fit point from the reverse bearing lines.
- `rad3d` must use slope distance and zenith angle, with zenith 90° treated as
  horizontal.
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
- `breaklines=ids:L1,L2` must use only those stored lines.
- Breakline endpoint points must have elevations.
- Selected breaklines must not cross except at shared endpoints.
- Editing points or lines does not auto-regenerate stored contours.
- DXF export must include stored contours as polylines on `CONTOURS` or
  `CONTOURS_INDEX`, using different ACI colors and visibly wider polyline
  width for index contours.
- DXF export must add contour elevation text labels on `CONTOUR_LABELS` or
  `CONTOUR_LABELS_INDEX` at the end of each contour line.
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
- `F1` opens help.
- `F2` toggles the ASCII map.
- Map view supports panning with arrows, zoom in/out, fit-to-points, and line
  overlay.
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
