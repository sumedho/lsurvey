package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"lsurvey/internal/geom"
	"lsurvey/internal/project"
)

func TestCommandCompletionAdvancesToNextInput(t *testing.T) {
	model := NewModel(project.New("test"), "")
	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	updated, _ = updated.(Model).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	updated, _ = updated.(Model).Update(tea.KeyMsg{Type: tea.KeyTab})
	got := updated.(Model)
	if got.input.Value() != "rad " {
		t.Fatalf("input=%q want rad plus trailing space", got.input.Value())
	}
}

func TestCommandCompletionIncludesProjectPointHints(t *testing.T) {
	m := NewModel(project.New("test"), "")
	m.ExecuteCommand("pt add 1 100 200 PEG")
	m.input.SetValue("inverse 1")
	m.refreshCompletions()
	if len(m.input.MatchedSuggestions()) == 0 {
		t.Fatal("expected matched point suggestions")
	}
}

func TestCommandCompletionKeepsRemainingRadArgumentsAfterPoint(t *testing.T) {
	m := NewModel(project.New("test"), "")
	m.project.Points["1"] = geom.Point{ID: "1", Easting: 100, Northing: 200}
	m.project.Points["7"] = geom.Point{ID: "7", Easting: 110, Northing: 210}
	m.input.SetValue("rad 1")
	m.refreshCompletions()
	matches := m.input.MatchedSuggestions()
	if len(matches) == 0 {
		t.Fatal("expected rad point suggestion")
	}
	if !strings.Contains(matches[0], "<azimuth|bearing> <distance>") {
		t.Fatalf("suggestion=%q missing remaining rad arguments", matches[0])
	}
	if !strings.Contains(matches[0], "as 8") {
		t.Fatalf("suggestion=%q missing next point id", matches[0])
	}
}

func TestCommandCompletionKeepsRemainingIntersectArgumentsAfterPoint(t *testing.T) {
	m := NewModel(project.New("test"), "")
	m.project.Points["1"] = geom.Point{ID: "1", Easting: 100, Northing: 200}
	m.input.SetValue("intersect bearing-bearing 1")
	m.refreshCompletions()
	matches := m.input.MatchedSuggestions()
	if len(matches) == 0 {
		t.Fatal("expected intersect point suggestion")
	}
	if !strings.Contains(matches[0], "<brg1> <p2> <brg2>") {
		t.Fatalf("suggestion=%q missing remaining intersect arguments", matches[0])
	}
	if !strings.Contains(matches[0], "as 2") {
		t.Fatalf("suggestion=%q missing next point id", matches[0])
	}
}

func TestCommandCompletionSuggestsNextPointIDForPointAdd(t *testing.T) {
	m := NewModel(project.New("test"), "")
	m.project.Points["1"] = geom.Point{ID: "1", Easting: 100, Northing: 200}
	m.project.Points["A"] = geom.Point{ID: "A", Easting: 110, Northing: 210}
	m.project.Points["3"] = geom.Point{ID: "3", Easting: 120, Northing: 220}
	m.input.SetValue("pt add")
	m.refreshCompletions()
	matches := m.input.MatchedSuggestions()
	if len(matches) == 0 {
		t.Fatal("expected pt add suggestion")
	}
	if !strings.HasPrefix(matches[0], "pt add 4 ") {
		t.Fatalf("suggestion=%q want next point id 4", matches[0])
	}
}

func TestCommandCompletionSuggestsNextPointIDForRenameAndLineOffset(t *testing.T) {
	m := NewModel(project.New("test"), "")
	m.project.Points["1"] = geom.Point{ID: "1", Easting: 100, Northing: 200}
	m.project.Points["2"] = geom.Point{ID: "2", Easting: 110, Northing: 210}
	m.ExecuteCommand("line add L1 1 2")

	m.input.SetValue("pt rename 1")
	m.refreshCompletions()
	matches := m.input.MatchedSuggestions()
	if len(matches) == 0 || !strings.HasPrefix(matches[0], "pt rename 1 3") {
		t.Fatalf("rename suggestion=%v want next point id 3", matches)
	}

	m.input.SetValue("offset L1")
	m.refreshCompletions()
	matches = m.input.MatchedSuggestions()
	if len(matches) == 0 || !strings.Contains(matches[0], "as 3") {
		t.Fatalf("offset suggestion=%v want next point id 3", matches)
	}
}

func TestCommandCompletionUsesTraverseLegWithoutAsID(t *testing.T) {
	m := NewModel(project.New("test"), "")
	m.input.SetValue("trav leg")
	m.refreshCompletions()
	matches := m.input.MatchedSuggestions()
	if len(matches) == 0 {
		t.Fatal("expected traverse suggestion")
	}
	if !strings.Contains(matches[0], "[vdiff <delta>] [code]") {
		t.Fatalf("suggestion=%q missing traverse leg shape", matches[0])
	}
	if strings.Contains(matches[0], " as ") {
		t.Fatalf("suggestion=%q should not require as <id>", matches[0])
	}
}

func TestCommandCompletionIncludesShiftAndRotate(t *testing.T) {
	m := NewModel(project.New("test"), "")
	m.project.Points["1"] = geom.Point{ID: "1", Easting: 100, Northing: 200}
	m.input.SetValue("shift 1")
	m.refreshCompletions()
	matches := m.input.MatchedSuggestions()
	if len(matches) == 0 || !strings.Contains(matches[0], "east=<delta>") {
		t.Fatalf("shift suggestion=%v", matches)
	}

	m.input.SetValue("rotate 1")
	m.refreshCompletions()
	matches = m.input.MatchedSuggestions()
	if len(matches) == 0 || !strings.Contains(matches[0], "<bearing>") {
		t.Fatalf("rotate suggestion=%v", matches)
	}
}

func TestCommandCompletionIncludesLineGenForKnownCode(t *testing.T) {
	m := NewModel(project.New("test"), "")
	m.project.Points["1"] = geom.Point{ID: "1", Easting: 100, Northing: 200, Code: "PEG"}
	m.input.SetValue("line gen")
	m.refreshCompletions()
	matches := m.input.MatchedSuggestions()
	if len(matches) == 0 || !strings.Contains(matches[0], "line gen PEG") {
		t.Fatalf("line gen suggestion=%v", matches)
	}
}

func TestCommandCompletionIncludesBearingAndDistanceMath(t *testing.T) {
	m := NewModel(project.New("test"), "")

	m.input.SetValue("bearing add")
	m.refreshCompletions()
	matches := m.input.MatchedSuggestions()
	if len(matches) == 0 || !strings.Contains(matches[0], "bearing add <a> <b>") {
		t.Fatalf("bearing suggestion=%v", matches)
	}

	m.input.SetValue("dist sub")
	m.refreshCompletions()
	matches = m.input.MatchedSuggestions()
	if len(matches) == 0 || !strings.Contains(matches[0], "dist sub <a> <b>") {
		t.Fatalf("dist suggestion=%v", matches)
	}
}

func TestCommandCompletionIncludesCloseCommand(t *testing.T) {
	m := NewModel(project.New("test"), "")
	m.project.Points["1"] = geom.Point{ID: "1", Easting: 100, Northing: 200}
	m.input.SetValue("close 1")
	m.refreshCompletions()
	matches := m.input.MatchedSuggestions()
	if len(matches) == 0 || !strings.Contains(matches[0], "close 1 <p2> <p3> ...") {
		t.Fatalf("close suggestion=%v", matches)
	}
}

func TestCommandCompletionTabFillsOneArgumentAtATime(t *testing.T) {
	m := NewModel(project.New("test"), "")
	m.project.Points["1"] = geom.Point{ID: "1", Easting: 100, Northing: 200}
	m.input.SetValue("rad ")
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	got := updated.(Model)
	if got.input.Value() != "rad 1 " {
		t.Fatalf("input=%q want first point filled only", got.input.Value())
	}

	updated, _ = got.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'9', '0', '.', '0', '0', '0', '0'}})
	got = updated.(Model)
	updated, _ = got.Update(tea.KeyMsg{Type: tea.KeyTab})
	got = updated.(Model)
	if got.input.Value() != "rad 1 90.0000 " {
		t.Fatalf("input=%q want space after bearing only", got.input.Value())
	}
}

func TestCommandCompletionTabFillsNextPointIDOnly(t *testing.T) {
	m := NewModel(project.New("test"), "")
	m.project.Points["1"] = geom.Point{ID: "1", Easting: 100, Northing: 200}
	m.project.Points["3"] = geom.Point{ID: "3", Easting: 120, Northing: 220}
	m.input.SetValue("pt add ")
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	got := updated.(Model)
	if got.input.Value() != "pt add 4 " {
		t.Fatalf("input=%q want next point id filled only", got.input.Value())
	}
}

func TestCommandCompletionKeepsPointAddHintAfterCoordinateInput(t *testing.T) {
	m := NewModel(project.New("test"), "")
	m.project.Points["3"] = geom.Point{ID: "3", Easting: 100, Northing: 200}
	m.input.SetValue("pt add 4 123.45")
	m.refreshCompletions()
	matches := m.input.MatchedSuggestions()
	if len(matches) == 0 {
		t.Fatal("expected contextual pt add suggestion")
	}
	if !strings.HasPrefix(matches[0], "pt add 4 123.45 <north>") {
		t.Fatalf("suggestion=%q want northing hint after easting", matches[0])
	}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	got := updated.(Model)
	if got.input.Value() != "pt add 4 123.45 " {
		t.Fatalf("input=%q want tab to move to northing field", got.input.Value())
	}
}

func TestCommandCompletionKeepsRadHintAfterTypedBearing(t *testing.T) {
	m := NewModel(project.New("test"), "")
	m.project.Points["1"] = geom.Point{ID: "1", Easting: 100, Northing: 200}
	m.input.SetValue("rad 1 12.3015")
	m.refreshCompletions()
	matches := m.input.MatchedSuggestions()
	if len(matches) == 0 {
		t.Fatal("expected contextual rad suggestion")
	}
	if !strings.HasPrefix(matches[0], "rad 1 12.3015 <distance>") {
		t.Fatalf("suggestion=%q want distance hint after bearing", matches[0])
	}
}

func TestNextCompletionChunk(t *testing.T) {
	tests := []struct {
		value      string
		suggestion string
		want       string
	}{
		{value: "ra", suggestion: "rad <from> <azimuth|bearing>", want: "d "},
		{value: "rad", suggestion: "rad <from> <azimuth|bearing>", want: " "},
		{value: "rad ", suggestion: "rad 1 <azimuth|bearing>", want: "1 "},
		{value: "rad 1", suggestion: "rad 1 <azimuth|bearing>", want: " "},
	}
	for _, tt := range tests {
		if got := nextCompletionChunk(tt.value, tt.suggestion); got != tt.want {
			t.Fatalf("nextCompletionChunk(%q, %q)=%q want %q", tt.value, tt.suggestion, got, tt.want)
		}
	}
}

func TestFillTemplatePrefixSubstitutesTypedPlaceholderValues(t *testing.T) {
	got, ok := fillTemplatePrefix("pt add 4 123.45", "pt add 4 <east> <north> [elev] [code]")
	if !ok {
		t.Fatal("expected template match")
	}
	want := "pt add 4 123.45 <north> [elev] [code]"
	if got != want {
		t.Fatalf("suggestion=%q want %q", got, want)
	}
}
