# lsurvey Tutorial

This tutorial teaches `lsurvey` by building a small project from scratch and
then branching into short focused exercises for the commands that do not fit
naturally into one workflow.

It assumes you are new to this application. It does not assume you already know
how `lsurvey` stores projects, how point IDs work, or how the command line is
used inside the TUI.

## 1. Start The Application

Start a new in-memory project:

```sh
go run ./cmd/lsurvey
```

Or, after building a binary:

```sh
./lsurvey
```

When the TUI opens, the screen has four areas:

- `Info` at the top for project name, file path, precision, filter, and status.
- `Points` for stored points.
- `Lines` for stored lines and contour summaries.
- `Command` at the bottom for entering commands.

Useful keys:

- `F1` opens help.
- `F2` toggles the map.
- `Tab` completes the current command.
- `Ctrl+n` and `Ctrl+p` cycle completion suggestions.
- `Up` and `Down` browse command history.
- `/` starts a `filter` command.
- `Esc` closes help or map.
- `Ctrl+C` exits.

Type commands in the bottom command area and press `Enter`.

## 2. Create Your First Project

Start with a named project:

```text
new Lot 42
desc Boundary survey Lot 42
precision 3
units angle=dd.mmss distance=m
```

What these do:

- `new` resets the in-memory project.
- `desc` sets the label shown in the top bar and saved in the project file.
- `precision` changes display formatting only. Stored coordinates are not
  rounded.
- `units` changes the labels stored with the project.

Save the project:

```text
save lot42
```

`lsurvey` appends `.srv` automatically, so this writes `lot42.srv`.

Save the same project under another name:

```text
saveas lot42_backup
```

Open a saved project later with or without the extension:

```text
open lot42
open lot42.srv
```

## 3. Add And Edit Points

Points are stored by ID. IDs do not have to be numeric, but numeric IDs are
convenient because command completion suggests the next unused number.

Add a few 2D and 3D points:

```text
pt add 1 2000 1000 25.500 PEG
pt add 2 2100 1000 25.650 PEG
pt add 3 2100 1080 25.880 PEG
pt add 4 2000 1080 25.700 PEG
pt add 5 2050 1040 TREE
```

The `pt add` forms are:

- `pt add <id> <east> <north> [code]`
- `pt add <id> <east> <north> <elev> [code]`

Edit point fields:

```text
pt edit 5 desc="large tree near center"
pt edit 5 code=TREE
pt edit 5 elev=25.740
```

Rename a point:

```text
pt rename 5 50
```

List the number of points:

```text
pt list
```

Delete a point that is not referenced by a line:

```text
pt del 50
```

Re-add it so later examples still work:

```text
pt add 50 2050 1040 25.740 TREE
pt edit 50 desc="large tree near center"
```

## 4. Basic Measurements

With points in place, run a few report-only calculations.

Measure between two points:

```text
inverse 1 2
inverse 1 3
```

`inverse` reports azimuth, horizontal distance, easting and northing deltas,
and 3D values when both points have elevations.

Measure an angle at a vertex:

```text
angle 1 2 3
```

Do bearing math:

```text
bearing add 350.0000 20.0000
bearing sub 10.0000 20.0000
```

Do distance math:

```text
dist add 12.5 3.125
dist sub 12.5 15
```

## 5. Create New Points From Calculations

Create a point by direction and distance:

```text
rad 1 90.0000 25 as 6 CALC
rad 1 N 45.0000 E 30 vdiff 0.5 as 7 PEG
```

`rad` uses either an azimuth like `90.0000` or a quadrant bearing like
`N 45.0000 E`. `vdiff` is optional and only affects elevation when the start
point already has one.

Create a 3D point by slope distance and zenith angle:

```text
rad3d 1 90.0000 25 90.0000 as 8 SHOT
```

Zenith angle uses `90°` as horizontal. Smaller angles go up; larger angles go
down.

Create a midpoint:

```text
midpoint 1 3 as 9 MID
```

Create offset points:

```text
offset 1 2 5 25 as 10 OFF
offset 1 2 -5 25 as 11 OFF
```

Positive offset is to the left of the direction from the first point to the
second point.

## 6. Store And Manage Lines

Create stored lines:

```text
line add B1 1 2 BOUNDARY
line add B2 2 3 BOUNDARY
line add B3 3 4 BOUNDARY
line add B4 4 1 BOUNDARY
line add D1 1 3 EASE
```

Edit a line:

```text
line edit D1 code=DIAG desc="lot diagonal"
```

List the number of lines:

```text
line list
```

Generate lines automatically from point code:

```text
pt add 100 2200 1000 PEG
pt add 101 2250 1030 PEG
pt add 102 2300 1010 PEG
line gen PEG
```

Delete one of those generated lines if you no longer need it:

```text
line del L1
```

Point deletion is protected. If you try to delete a point used by a stored
line, `lsurvey` rejects it until the referencing line is edited or deleted.

You can also offset from a stored line:

```text
offset B1 5 25 as 12 OFF
```

## 7. Use The TUI Features While You Work

Try filtering the point list:

```text
filter PEG
clear filter
```

Sort the points:

```text
sort code
sort north desc
```

Open help:

```text
help
help rad
help contour gen
```

Open the map with `F2` or:

```text
map
map lines
map contours
map fit
map zoom in
map zoom out
```

Map keys:

- Arrow keys pan.
- `c` toggles stored contour overlays; `~` marks minor and `=` marks index contours.
- `+` or `=` zooms in.
- `-` zooms out.
- `f` fits to points.
- `l` toggles line overlay.
- `Esc` or `F2` returns to the main view.

## 8. Intersections And Resection

These commands are easier to understand in a clean mini-scenario. Start a fresh
project:

```text
new Intersection Demo
pt add A 0 0
pt add B 100 0
pt add C 0 100
pt add D 100 100
```

Line-line intersection:

```text
line intersect A D B C as X IP
```

Bearing-bearing intersection:

```text
intersect bearing-bearing A 45.0000 B 315.0000 as X1 IP
```

Distance-distance intersection:

```text
intersect distance-distance A 100 B 100 choose left as X2 DD
intersect distance-distance A 100 B 100 choose right as X3 DD
```

Bearing-distance intersection:

```text
intersect bearing-distance A 45.0000 B 70 choose near as X4 BD
intersect bearing-distance A 45.0000 B 70 choose far as X5 BD
```

Resection uses three known points and bearings observed from the unknown point
to those known points:

```text
new Resection Demo
pt add 1 290000.504 6147540.603
pt add 2 289477.373 6147660.737
pt add 3 290192.164 6147882.102
resect 1 150.4157 2 246.4213 3 28.1626 as 4 RS
```

## 9. Traverse Workflow

Start another fresh mini-scenario:

```text
new Traverse Demo
pt add 1 5000 5000 100.000
trav start 1
trav leg 90.0000 50 TRV
trav leg 180.0000 40 vdiff -0.2 TRV
trav show
```

Close the traverse onto a known point:

```text
pt add 99 5050 4960 99.800
trav close 99
```

Adjust it:

```text
trav adjust compass
```

You can also use:

```text
trav adjust transit
```

## 10. Whole-Project Transformations

These commands change stored project coordinates, so work on a copy or a throw
away project when learning them.

```text
new Transform Demo
pt add 1 1000 1000 10
pt add 2 1100 1000 10
pt add 3 1000 1100 11
line add L1 1 2 TEST
```

Shift every point and stored contour vertex by moving point `1` onto target
coordinates:

```text
shift 1 east=1005 north=998 elev=10.5
```

Rotate every point and stored contour vertex around a base point:

```text
rotate 1 15.3000
rotate 1 -15.3000
```

`rotate` accepts signed compact DMS or signed decimal degrees such as `-15.5d`.
It does not accept quadrant bearings.

Convert MGA-style grid coordinates to an anchored local-ground system using a
combined scale factor, then reverse that persisted conversion when needed:

```text
scale apply 1 csf=0.9996 system=MGA2020_ZONE50
scale reverse
```

Only horizontal coordinates and horizontal contour-distance diagnostics are
scaled. Elevations remain unchanged. Export files contain the active
coordinates; the TUI status and export message show the label while the scale
is applied and remove it after reversal. If the project is shifted or rotated
while scaled, the saved scale anchor is transformed with it.

Fit a source-to-target similarity transformation without modifying project
data:

```text
new Transform Fit Demo
pt add A1 0 0 1
pt add A2 100 0 1
pt add A3 0 100 2
pt add B1 500 200 6
pt add B2 500 80 6
pt add B3 620 200 7
transform fit A1 B1 A2 B2 A3 B3
```

`transform fit` reports:

- `dx` and `dy`
- optional `dz`
- signed angle
- floating-point scale
- horizontal RMS residual
- vertical RMS residual when complete 3D pairs exist

## 11. Generate And Manage Contours

Contours require at least three elevated, non-collinear points. This section
uses a small scratch dataset created from scratch.

```text
new Terrain Demo
pt add 1 0 0 100 BRK
pt add 2 50 0 101 BRK
pt add 3 100 0 102 BRK
pt add 4 0 50 100.5
pt add 5 50 50 102
pt add 6 100 50 103
pt add 7 0 100 101
pt add 8 50 100 103
pt add 9 100 100 104
line add R1 1 2 RIDGE
line add R2 2 3 RIDGE
```

Generate contours using all lines as breaklines:

```text
contour gen C1 1
```

Generate contours from points only:

```text
contour gen C2 0.5 breaklines=none
```

Generate contours using selected breaklines only:

```text
contour gen C3 0.5 base=100 index=5 breaklines=ids:R1,R2
```

Clip contours to closed linework selected by code, with optional exclusions:

```text
contour gen C4 0.5 boundary=codes:SITE exclude=codes:POND
contour regen C4
```

Request quality warnings for long TIN edges and optional presentation
smoothing:

```text
contour gen C5 0.5 boundary=codes:SITE maxedge=30 smooth=1
contour info C5
```

Inspect and manage contour sets:

```text
contour list
contour info C1
contour del C2
```

Important contour rules:

- Points without elevations are ignored.
- Breakline endpoint points must have elevations.
- Selected breaklines must not cross except at shared endpoints.
- `boundary=codes:` and `exclude=codes:` use closed stored-line rings; their
  points may be 2D because they clip contours horizontally.
- Relevant point or line changes mark stored contour sets stale; run
  `contour regen <id>` to rebuild them from the saved generation settings.
- Contour generation records warnings for duplicate elevated positions, long
  TIN edges, and unconstrained hull contact. Omitting `maxedge=` uses an
  automatic threshold based on point spacing.
- `smooth=1` through `smooth=3` stores smoothed output while retaining raw
  contour geometry; DXF uses the smoothed output.

## 12. Import And Export

Save the current project before exporting:

```text
save terrain_demo
```

CSV import and export:

```text
export csv terrain_points
import csv terrain_points
```

CSV files must use this exact header:

```text
id,easting,northing,elevation,code,description
```

Export GeoJSON:

```text
export geojson terrain
```

Import GeoJSON:

```text
import geojson terrain
```

Export DXF:

```text
export dxf terrain
```

DXF includes points, point labels, stored lines, and stored contours. Contour
labels are placed along readable contour paths and omitted when a path is too
short or would overlap an existing contour label.

## 13. Batch Export Without Opening The TUI

Once a project has been saved, you can export it from the shell:

```sh
go run ./cmd/lsurvey export csv ./terrain_demo.srv ./terrain_points
go run ./cmd/lsurvey export dxf ./terrain_demo.srv ./terrain
go run ./cmd/lsurvey export geojson ./terrain_demo.srv ./terrain
```

These commands append the output extension when missing.

## 14. Completion, History, And Practical Workflow

As you work:

- Use `Tab` to advance through a completion one argument at a time.
- Use `Ctrl+n` and `Ctrl+p` to cycle suggestions.
- Use `Up` and `Down` to revisit earlier commands.
- Use `help <command>` whenever you forget the exact syntax.

A practical day-to-day flow often looks like this:

```text
new Job Name
desc Boundary survey
precision 3
pt add ...
line add ...
inverse ...
offset ...
map lines
map contours
save job_name
export dxf job_name
export csv job_name_points
```

## 15. Full Command Checklist

This section is a quick index so you can confirm what the application supports.

Project:

- `new`
- `open`
- `save`
- `saveas`
- `desc`
- `precision`
- `units`
- `quit`
- `exit`

Points:

- `pt add`
- `pt edit`
- `pt del`
- `pt rename`
- `pt list`

Calculations:

- `inverse`
- `angle`
- `close`
- `bearing add`
- `bearing sub`
- `dist add`
- `dist sub`
- `rad`
- `rad3d`
- `midpoint`
- `offset`
- `shift`
- `rotate`
- `scale apply`
- `scale reverse`
- `transform fit`

Intersections:

- `line intersect`
- `intersect bearing-bearing`
- `intersect bearing-distance`
- `intersect distance-distance`
- `resect`

Traverse:

- `trav start`
- `trav leg`
- `trav show`
- `trav close`
- `trav adjust`

Contours:

- `contour gen`
- `contour list`
- `contour info`
- `contour del`

Lines and stored geometry:

- `line add`
- `line gen`
- `line edit`
- `line del`
- `line list`

Import and export:

- `import csv`
- `export csv`
- `import geojson`
- `export geojson`
- `export dxf`

TUI helpers:

- `filter`
- `clear filter`
- `sort`
- `map`
- `map lines`
- `map contours`
- `map fit`
- `map zoom in`
- `map zoom out`
- `help`
- `help <command>`

If you forget the exact shape of a command, use `help` inside the app. That is
the fastest way to confirm the current syntax.
