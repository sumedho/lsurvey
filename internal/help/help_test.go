package help

import (
	"strings"
	"testing"
)

func TestRenderIncludesAngleRuleAndCommands(t *testing.T) {
	got := Render("")
	for _, want := range []string{"dd.mmsshhhh", "pt add", "rad", "close <p1> <p2> <p3> ...", "bearing add <a> <b>", "dist sub <a> <b>", "shift <base>", "rotate <base>", "transform fit <src1>", "line gen <code>", "export dxf", "export geojson", "filter <text>", "desc <project description>"} {
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
	for _, want := range []string{"rad <from>", "save job", "trav show", "close 1 2 3", "bearing add <a> <b>", "dist sub 12.5 15", "trav leg <azimuth|bearing> <distance> [vdiff <delta>] [code]", "import geojson <file>", "export geojson <file>", "shift <base> [east=<delta>] [north=<delta>] [elev=<delta>]", "rotate 100 -15.3000", "transform fit <src1> <dst1>", "line gen <code>"} {
		if !strings.Contains(got, want) {
			t.Fatalf("suggestions missing %q:\n%s", want, got)
		}
	}
}

func TestRenderStyledIncludesStyledSections(t *testing.T) {
	got := RenderStyled("")
	for _, want := range []string{"lsurvey command help", "Project", "Points", "Keys:"} {
		if !strings.Contains(got, want) {
			t.Fatalf("styled help missing %q:\n%s", want, got)
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
