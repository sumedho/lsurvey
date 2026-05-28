package help

import (
	"strings"
	"testing"
)

func TestRenderIncludesAngleRuleAndCommands(t *testing.T) {
	got := Render("")
	for _, want := range []string{"dd.mmsshhhh", "pt add", "rad", "close <p1> <p2> <p3> ...", "bearing add <a> <b>", "dist sub <a> <b>", "shift <base>", "rotate <base>", "scale apply <base>", "transform fit <src1>", "line gen <code>", "polyline add", "polygon add", "polygon report", "group add", "code style set", "export dxf", "export geojson", "export landxml", "export boundarycsv", "import codes", "filter <text>", "style", "convert", "F4 opens", "F5 browses", "F3 opens code styling", "desc <project description>", "undo", "history info <n>", "info", "i toggles map point ID/code labels"} {
		if !strings.Contains(got, want) {
			t.Fatalf("help missing %q:\n%s", want, got)
		}
	}
}

func TestFindCommand(t *testing.T) {
	got, ok := Find("rad")
	if !ok {
		t.Fatal("rad not found")
	}
	if got.Usage == "" || !strings.Contains(got.Description, "Create a point") {
		t.Fatalf("unexpected command: %+v", got)
	}
}

func TestSuggestionsIncludeUsageAndExamples(t *testing.T) {
	got := strings.Join(Suggestions(), "\n")
	for _, want := range []string{"rad <from>", "save job", "trav show", "close 1 2 3", "bearing add <a> <b>", "dist sub 12.5 15", "trav leg <azimuth|bearing> <distance> [vdiff <delta>] [code]", "import geojson <file>", "export geojson <file>", "export landxml <file>", "export boundarycsv <file>", "import codes <file>", "shift <base> [east=<coordinate>] [north=<coordinate>] [elev=<coordinate>]", "rotate 100 -15.3000", "scale apply <base> csf=<factor> [system=<label>]", "scale reverse", "transform fit <src1> <dst1>", "line gen <code>", "polygon report <id|all>", "code style set <code>", "style", "undo", "redo", "history limit=10", "info"} {
		if !strings.Contains(got, want) {
			t.Fatalf("suggestions missing %q:\n%s", want, got)
		}
	}
}

func TestSearchTextIncludesSearchableCommandMetadata(t *testing.T) {
	got := SearchText(Command{
		Group: "Contours", Name: "contour info", Usage: "contour info <id>",
		Description: "Show warnings.", Examples: []string{"contour info C1"}, Notes: []string{"Reports stale data."},
	})
	for _, want := range []string{"Contours", "contour info", "warnings", "C1", "stale"} {
		if !strings.Contains(got, want) {
			t.Fatalf("search text=%q missing %q", got, want)
		}
	}
}

func TestRenderStyledIncludesStyledSections(t *testing.T) {
	got := RenderStyled("")
	for _, want := range []string{"lsurvey command help", "Project & Session", "Point Data", "Keys:"} {
		if !strings.Contains(got, want) {
			t.Fatalf("styled help missing %q:\n%s", want, got)
		}
	}
}

func TestCommandsHaveLogicalGroups(t *testing.T) {
	for name, want := range map[string]string{
		"new":            "Project & Session",
		"pt add":         "Point Data",
		"rad":            "COGO Calculations",
		"contour gen":    "Terrain & Contours",
		"polygon add":    "Feature Geometry & Styling",
		"export landxml": "Import & Export",
		"help":           "Interface",
		"style":          "Interface",
	} {
		command, ok := Find(name)
		if !ok {
			t.Fatalf("command %q not found", name)
		}
		if command.Group != want {
			t.Fatalf("command %q group=%q want %q", name, command.Group, want)
		}
	}
}

func TestRenderStyledCommand(t *testing.T) {
	got := RenderStyled("rad")
	for _, want := range []string{"rad", "Usage", "Examples"} {
		if !strings.Contains(got, want) {
			t.Fatalf("styled command help missing %q:\n%s", want, got)
		}
	}
}

func TestRenderStyledUnknownCommandDoesNotRenderEntireIndex(t *testing.T) {
	got := RenderStyled("missing")
	if !strings.Contains(got, "No help found") || !strings.Contains(got, "use / to search") {
		t.Fatalf("unknown help=%q", got)
	}
	if strings.Contains(got, "Project") || strings.Contains(got, "pt add") {
		t.Fatalf("unknown command should not render full help index:\n%s", got)
	}
}
