package help

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

type Command struct {
	Group       string
	Name        string
	Usage       string
	Description string
	Examples    []string
	Notes       []string
}

func All() []Command {
	return []Command{
		{Group: "Project", Name: "new", Usage: "new <name>", Description: "Create a new in-memory project.", Examples: []string{"new Smith Road"}},
		{Group: "Project", Name: "open", Usage: "open <file>", Description: "Load a project file. The .srv extension is appended when missing.", Examples: []string{"open job", "open job.srv"}},
		{Group: "Project", Name: "save", Usage: "save [file]", Description: "Save the current project. The .srv extension is appended when missing.", Examples: []string{"save", "save job"}},
		{Group: "Project", Name: "saveas", Usage: "saveas <file>", Description: "Save the current project to a new .srv path.", Examples: []string{"saveas revised"}},
		{Group: "Project", Name: "desc", Usage: "desc <project description>", Description: "Set the project description displayed in the top status bar and saved in the project file.", Examples: []string{"desc Boundary survey Lot 42"}},
		{Group: "Project", Name: "precision", Usage: "precision <0-6>", Description: "Set display precision for distances, coordinates, and elevations.", Examples: []string{"precision 3", "precision 4"}},
		{Group: "Project", Name: "units", Usage: "units key=value ...", Description: "Set project unit labels.", Examples: []string{"units angle=dd.mmss distance=m"}},
		{Group: "Points", Name: "pt add", Usage: "pt add <id> <east> <north> [elev] [code]", Description: "Create a coded 2D or 3D point.", Examples: []string{"pt add 1 2000 1000 52.4 PEG", "pt add 2 2010 1010 TREE"}},
		{Group: "Points", Name: "pt edit", Usage: "pt edit <id> [east=] [north=] [elev=] [code=] [desc=]", Description: "Edit point fields.", Examples: []string{"pt edit 1 code=PEG desc=corner"}},
		{Group: "Points", Name: "pt del", Usage: "pt del <id>", Description: "Delete a point.", Examples: []string{"pt del 10"}},
		{Group: "Points", Name: "pt rename", Usage: "pt rename <old> <new>", Description: "Rename a point and update line references.", Examples: []string{"pt rename 100 101"}},
		{Group: "Points", Name: "pt list", Usage: "pt list", Description: "Report the number of points.", Examples: []string{"pt list"}},
		{Group: "Calculations", Name: "inverse", Usage: "inverse <from> <to>", Description: "Calculate azimuth, horizontal distance, deltas, and 3D values when elevations exist.", Examples: []string{"inverse 1 2"}},
		{Group: "Calculations", Name: "angle", Usage: "angle <back> <vertex> <forward>", Description: "Calculate inside and outside angles at the vertex point.", Examples: []string{"angle 1 2 3"}},
		{Group: "Calculations", Name: "close", Usage: "close <p1> <p2> <p3> ...", Description: "Report area, misclose bearing, distance, easting/northing deltas, and closure accuracy for ordered point IDs.", Examples: []string{"close 1 2 3", "close 10 11 12 10"}},
		{Group: "Calculations", Name: "bearing add", Usage: "bearing add <a> <b>", Description: "Add two bearings and report the clamped 0-360 result.", Examples: []string{"bearing add 350.0000 20.0000"}},
		{Group: "Calculations", Name: "bearing sub", Usage: "bearing sub <a> <b>", Description: "Subtract one bearing from another and report the clamped 0-360 result.", Examples: []string{"bearing sub 10.0000 20.0000"}},
		{Group: "Calculations", Name: "dist add", Usage: "dist add <a> <b>", Description: "Add two distances and report the result without creating project data.", Examples: []string{"dist add 12.5 3.125"}},
		{Group: "Calculations", Name: "dist sub", Usage: "dist sub <a> <b>", Description: "Subtract one distance from another and report the result without creating project data.", Examples: []string{"dist sub 12.5 15"}},
		{Group: "Calculations", Name: "rad", Usage: "rad <from> <azimuth|bearing> <distance> [vdiff <delta>] as <id> [code]", Description: "Create a point from a start point, direction, and distance.", Examples: []string{"rad 1 90.0000 10 as 2 CALC", "rad 1 N 45.0000 E 50 vdiff 1.2 as 3 PEG"}},
		{Group: "Calculations", Name: "rad3d", Usage: "rad3d <from> <azimuth|bearing> <slope_distance> <zenith> as <id> [code]", Description: "Create a 3D point from a start point, direction, slope distance, and zenith angle.", Examples: []string{"rad3d 1 90.0000 10 90.0000 as 2 CALC", "rad3d 1 N 45.0000 E 25 88.3000 as 3 SHOT"}, Notes: []string{"Zenith angle uses 90° as horizontal; smaller angles go up and larger angles go down."}},
		{Group: "Calculations", Name: "midpoint", Usage: "midpoint <p1> <p2> as <id> [code]", Description: "Create the midpoint between two points.", Examples: []string{"midpoint 1 2 as 10 MID"}},
		{Group: "Calculations", Name: "offset", Usage: "offset <p1> <p2> <offset> <chainage> as <id> [code] OR offset <line_id> <offset> <chainage> as <id> [code]", Description: "Create a point offset from two points or a stored line at a chainage.", Examples: []string{"offset 1 2 5 25 as 20 OFF", "offset L1 5 25 as 20 OFF"}},
		{Group: "Calculations", Name: "shift", Usage: "shift <base> [east=<coordinate>] [north=<coordinate>] [elev=<coordinate>]", Description: "Translate all project points and stored contour geometry so the base point reaches the supplied coordinates.", Examples: []string{"shift 100 east=2000 north=1000", "shift 100 east=2000 north=1000 elev=50"}, Notes: []string{"Omitted coordinates retain the base point's current value. A target elevation requires an elevated base point."}},
		{Group: "Calculations", Name: "rotate", Usage: "rotate <base> <bearing>", Description: "Rotate all project points and stored contour geometry around the base point by a signed relative angle.", Examples: []string{"rotate 100 15.3000", "rotate 100 -15.3000"}, Notes: []string{"Rotate accepts signed compact DMS or signed decimal degrees such as -15.5d. Quadrant bearings are not accepted."}},
		{Group: "Calculations", Name: "scale apply", Usage: "scale apply <base> csf=<factor> [system=<label>]", Description: "Apply an anchored horizontal scale factor and add its active coordinate label.", Examples: []string{"scale apply 100 csf=0.9996 system=MGA2020_ZONE50"}, Notes: []string{"Only horizontal coordinates and horizontal contour-distance metadata are scaled; elevations are unchanged."}},
		{Group: "Calculations", Name: "scale reverse", Usage: "scale reverse", Description: "Reverse the applied scale using its stored anchor and factor, then remove its coordinate label.", Examples: []string{"scale reverse"}},
		{Group: "Calculations", Name: "transform fit", Usage: "transform fit <src1> <dst1> <src2> <dst2> [<srcN> <dstN> ...]", Description: "Calculate a source-to-target similarity transform with shift, signed rotation, scale, and RMS residuals.", Examples: []string{"transform fit A1 B1 A2 B2 A3 B3"}, Notes: []string{"Horizontal parameters use all pairs. Vertical shift and RMS use only pairs where both points have elevations. This command does not modify project points."}},
		{Group: "Intersections", Name: "line intersect", Usage: "line intersect <a1> <a2> <b1> <b2> as <id> [code]", Description: "Create a point at the intersection of two lines.", Examples: []string{"line intersect 1 2 3 4 as 5 IP"}},
		{Group: "Intersections", Name: "intersect bearing-bearing", Usage: "intersect bearing-bearing <p1> <brg1> <p2> <brg2> as <id> [code]", Description: "Intersect two bearings.", Examples: []string{"intersect bearing-bearing 1 45.0000 2 315.0000 as 9 IP"}},
		{Group: "Intersections", Name: "intersect bearing-distance", Usage: "intersect bearing-distance <p1> <brg> <p2> <dist> choose near|far as <id> [code]", Description: "Intersect a bearing with a distance circle.", Examples: []string{"intersect bearing-distance 1 90.0000 2 5 choose far as 3 BD"}},
		{Group: "Intersections", Name: "intersect distance-distance", Usage: "intersect distance-distance <p1> <dist1> <p2> <dist2> choose left|right as <id> [code]", Description: "Intersect two distance circles.", Examples: []string{"intersect distance-distance 1 10 2 10 choose left as 3 DD"}},
		{Group: "Intersections", Name: "resect", Usage: "resect <p1> <brg1> <p2> <brg2> <p3> <brg3> as <id> [code]", Description: "Create a point from three known points and bearings observed from the new point to those points.", Examples: []string{"resect 1 0.0000 2 90.0000 3 270.0000 as 4 RS"}, Notes: []string{"Bearings are from the unknown point to each known point; the calculation uses the reverse bearing lines from the known points."}},
		{Group: "Traverse", Name: "trav start", Usage: "trav start <point>", Description: "Start a traverse at a known point.", Examples: []string{"trav start 1"}},
		{Group: "Traverse", Name: "trav leg", Usage: "trav leg <azimuth|bearing> <distance> [vdiff <delta>] [code]", Description: "Add a traverse leg from the current traverse point using the next available numeric point ID.", Examples: []string{"trav leg 90.0000 25 TRV"}},
		{Group: "Traverse", Name: "trav show", Usage: "trav show", Description: "Show the active traverse start point, current point, close point, and next auto point ID.", Examples: []string{"trav show"}},
		{Group: "Traverse", Name: "trav close", Usage: "trav close <known_point>", Description: "Report traverse misclose to a known point.", Examples: []string{"trav close 99"}},
		{Group: "Traverse", Name: "trav adjust", Usage: "trav adjust compass|transit", Description: "Adjust traverse points after closing.", Examples: []string{"trav adjust compass"}},
		{Group: "Contours", Name: "contour gen", Usage: "contour gen <id> <interval> [base=<elev>] [index=<n>] [breaklines=all|none|ids:L1,L2] [boundary=codes:C1,C2] [exclude=codes:C3,C4] [maxedge=<distance>] [smooth=<0..3>]", Description: "Generate contour polylines with persisted diagnostics and optional constraint-safe smoothing.", Examples: []string{"contour gen C1 1", "contour gen C1 0.5 boundary=codes:SITE exclude=codes:POND maxedge=25 smooth=1"}},
		{Group: "Contours", Name: "contour regen", Usage: "contour regen <id>", Description: "Regenerate a stored contour set from its saved options and current source geometry.", Examples: []string{"contour regen C1"}},
		{Group: "Contours", Name: "contour list", Usage: "contour list", Description: "List stored contour sets with contour counts and generation settings.", Examples: []string{"contour list"}},
		{Group: "Contours", Name: "contour info", Usage: "contour info <id>", Description: "Show settings, triangle count, smoothing, staleness, and stored quality warnings for a contour set.", Examples: []string{"contour info C1"}},
		{Group: "Contours", Name: "contour del", Usage: "contour del <id>", Description: "Delete a stored contour set.", Examples: []string{"contour del C1"}},
		{Group: "Lines and Export", Name: "line add", Usage: "line add <id> <p1> <p2> [code] [terrain=standard|ridge|drain]", Description: "Create a coded line, optionally assigning terrain-breakline intent.", Examples: []string{"line add L1 1 2 BOUNDARY", "line add R1 1 2 FEATURE terrain=ridge"}},
		{Group: "Lines and Export", Name: "line gen", Usage: "line gen <code>", Description: "Generate stored lines between points with the supplied point code in project point order.", Examples: []string{"line gen PEG"}},
		{Group: "Lines and Export", Name: "line edit", Usage: "line edit <id> [from=] [to=] [code=] [desc=] [terrain=none|standard|ridge|drain]", Description: "Edit line data or terrain intent. Quote descriptions containing spaces.", Examples: []string{`line edit L1 code=EASE desc="access easement" terrain=none`}},
		{Group: "Lines and Export", Name: "line del", Usage: "line del <id>", Description: "Delete a line.", Examples: []string{"line del L1"}},
		{Group: "Lines and Export", Name: "line list", Usage: "line list", Description: "Report the number of lines.", Examples: []string{"line list"}},
		{Group: "Import and Export", Name: "import csv", Usage: "import csv <file>", Description: "Import points from CSV with header id,easting,northing,elevation,code,description.", Examples: []string{"import csv points"}},
		{Group: "Import and Export", Name: "export csv", Usage: "export csv <file>", Description: "Export sorted points to CSV with header id,easting,northing,elevation,code,description.", Examples: []string{"export csv points"}},
		{Group: "Import and Export", Name: "import geojson", Usage: "import geojson <file>", Description: "Import GeoJSON point and line features into points and stored lines.", Examples: []string{"import geojson survey"}},
		{Group: "Import and Export", Name: "export geojson", Usage: "export geojson <file>", Description: "Export points and stored lines as GeoJSON features. The .geojson extension is appended when missing.", Examples: []string{"export geojson survey"}},
		{Group: "Lines and Export", Name: "export dxf", Usage: "export dxf <file>", Description: "Export points, point labels, and lines to ASCII DXF. The .dxf extension is appended when missing.", Examples: []string{"export dxf job"}},
		{Group: "TUI", Name: "filter", Usage: "filter <text>", Description: "Filter the point list by ID, code, or description.", Examples: []string{"filter PEG"}},
		{Group: "TUI", Name: "clear filter", Usage: "clear filter", Description: "Clear the point list filter.", Examples: []string{"clear filter"}},
		{Group: "TUI", Name: "sort", Usage: "sort <id|north|east|elev|code|desc> [asc|desc]", Description: "Sort the point list.", Examples: []string{"sort code", "sort north desc"}},
		{Group: "TUI", Name: "map", Usage: "map [lines|contours|fit|zoom in|zoom out]", Description: "Toggle the full-screen ASCII map, line or contour overlays, zoom, or fit-to-points view.", Examples: []string{"map", "map lines", "map contours", "map fit"}},
		{Group: "TUI", Name: "help", Usage: "help [command]", Description: "Open full-screen command help.", Examples: []string{"help", "help rad"}},
		{Group: "TUI", Name: "quit", Usage: "quit", Description: "Exit the application.", Examples: []string{"quit"}},
	}
}

func Find(query string) (Command, bool) {
	q := strings.ToLower(strings.TrimSpace(query))
	for _, cmd := range All() {
		if strings.ToLower(cmd.Name) == q || strings.HasPrefix(strings.ToLower(cmd.Name), q+" ") {
			return cmd, true
		}
	}
	return Command{}, false
}

func Suggestions() []string {
	seen := map[string]bool{}
	var suggestions []string
	for _, cmd := range All() {
		for _, value := range append([]string{cmd.Usage}, cmd.Examples...) {
			if value == "" || seen[value] {
				continue
			}
			seen[value] = true
			suggestions = append(suggestions, value)
		}
	}
	return suggestions
}

func Render(query string) string {
	if strings.TrimSpace(query) != "" {
		if cmd, ok := Find(query); ok {
			return renderCommand(cmd)
		}
		return "No help found for " + query + "\n\n" + Render("")
	}

	var b strings.Builder
	b.WriteString("lsurvey command help\n\n")
	b.WriteString("Angles use compact HP-style DMS: dd.mmsshhhh. Example: 123.304567 = 123 degrees, 30 minutes, 45.67 seconds.\n")
	b.WriteString("Decimal degrees require a suffix, e.g. 123.5125d. Quadrant bearings use forms like N 45.0000 E.\n\n")
	group := ""
	for _, cmd := range All() {
		if cmd.Group != group {
			group = cmd.Group
			fmt.Fprintf(&b, "%s\n", group)
		}
		fmt.Fprintf(&b, "  %-28s %s\n", cmd.Usage, cmd.Description)
	}
	b.WriteString("\nKeys: F1 opens help, F2 toggles map, / starts a filter command, Tab accepts a command completion, Ctrl+N/Ctrl+P cycle completions, Up/Down browse command history, arrow keys pan map, +/- zoom map, f fits map, l toggles map lines, c toggles map contours, alt+s cycles sort fields, alt+d toggles direction, Esc closes help/map, Ctrl+C quits.\n")
	return b.String()
}

func RenderStyled(query string) string {
	if strings.TrimSpace(query) != "" {
		if cmd, ok := Find(query); ok {
			return renderStyledCommand(cmd)
		}
		return helpErrorStyle.Render("No help found for "+query) + "\n\n" + RenderStyled("")
	}

	var b strings.Builder
	b.WriteString(helpTitleStyle.Render("lsurvey command help"))
	b.WriteString("\n\n")
	b.WriteString(helpNoteStyle.Render("Angles: dd.mmsshhhh, e.g. 123.304567 = 123 degrees, 30 minutes, 45.67 seconds. Decimal degrees use 123.5125d."))
	b.WriteString("\n\n")
	group := ""
	for _, cmd := range All() {
		if cmd.Group != group {
			group = cmd.Group
			if b.Len() > 0 {
				b.WriteByte('\n')
			}
			b.WriteString(helpSectionStyle.Render(group))
			b.WriteByte('\n')
		}
		b.WriteString("  ")
		b.WriteString(helpUsageStyle.Render(cmd.Usage))
		b.WriteString("  ")
		b.WriteString(helpDescriptionStyle.Render(cmd.Description))
		b.WriteByte('\n')
	}
	b.WriteByte('\n')
	b.WriteString(helpKeyStyle.Render("Keys: F1 help  Esc close  Tab complete  Ctrl+N/Ctrl+P completions  Up/Down history  F2 map  arrows pan map  +/- zoom  f fit  l lines  c contours"))
	return b.String()
}

func renderCommand(cmd Command) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s\n\nUsage: %s\n\n%s\n", cmd.Name, cmd.Usage, cmd.Description)
	if len(cmd.Examples) > 0 {
		b.WriteString("\nExamples:\n")
		for _, ex := range cmd.Examples {
			fmt.Fprintf(&b, "  %s\n", ex)
		}
	}
	for _, note := range cmd.Notes {
		fmt.Fprintf(&b, "\n%s\n", note)
	}
	return b.String()
}

func renderStyledCommand(cmd Command) string {
	var b strings.Builder
	b.WriteString(helpTitleStyle.Render(cmd.Name))
	b.WriteString("\n\n")
	b.WriteString(helpSectionStyle.Render("Usage"))
	b.WriteByte('\n')
	b.WriteString("  ")
	b.WriteString(helpUsageStyle.Render(cmd.Usage))
	b.WriteString("\n\n")
	b.WriteString(helpSectionStyle.Render("Description"))
	b.WriteByte('\n')
	b.WriteString("  ")
	b.WriteString(helpDescriptionStyle.Render(cmd.Description))
	b.WriteByte('\n')
	if len(cmd.Examples) > 0 {
		b.WriteByte('\n')
		b.WriteString(helpSectionStyle.Render("Examples"))
		b.WriteByte('\n')
		for _, ex := range cmd.Examples {
			b.WriteString("  ")
			b.WriteString(helpExampleStyle.Render(ex))
			b.WriteByte('\n')
		}
	}
	for _, note := range cmd.Notes {
		b.WriteByte('\n')
		b.WriteString(helpNoteStyle.Render(note))
		b.WriteByte('\n')
	}
	return b.String()
}

var (
	helpTitleStyle       = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("86"))
	helpSectionStyle     = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("220")).Underline(true)
	helpUsageStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("81"))
	helpDescriptionStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
	helpExampleStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("150"))
	helpNoteStyle        = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	helpKeyStyle         = lipgloss.NewStyle().Foreground(lipgloss.Color("213"))
	helpErrorStyle       = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("196"))
)
