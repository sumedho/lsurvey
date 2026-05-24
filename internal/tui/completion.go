package tui

import (
	"strings"
	"unicode"

	"lsurvey/internal/help"
	"lsurvey/internal/project"
)

func commandSuggestions(p *project.Project) []string {
	seen := map[string]bool{}
	var suggestions []string
	add := func(value string) {
		if value == "" || seen[value] {
			return
		}
		seen[value] = true
		suggestions = append(suggestions, value)
	}

	nextPointID := p.NextPointID()
	add("pt add " + nextPointID + " <east> <north> [elev] [code]")
	add("close <p1> <p2> <p3> ...")
	add("bearing add <a> <b>")
	add("bearing sub <a> <b>")
	add("dist add <a> <b>")
	add("dist sub <a> <b>")
	add("trav leg <azimuth|bearing> <distance> [vdiff <delta>] [code]")

	points := p.SortedPoints()
	codes := map[string]bool{}
	for _, pt := range points {
		if pt.Code != "" && !codes[pt.Code] {
			add("line gen " + pt.Code)
			codes[pt.Code] = true
		}
		add("close " + pt.ID + " <p2> <p3> ...")
		add("inverse " + pt.ID + " <to>")
		add("rad " + pt.ID + " <azimuth|bearing> <distance> [vdiff <delta>] as " + nextPointID + " [code]")
		add("rad3d " + pt.ID + " <azimuth|bearing> <slope_distance> <zenith> as " + nextPointID + " [code]")
		add("midpoint " + pt.ID + " <p2> as " + nextPointID + " [code]")
		add("offset " + pt.ID + " <p2> <offset> <chainage> as " + nextPointID + " [code]")
		add("shift " + pt.ID + " east=<delta> [north=<delta>] [elev=<delta>]")
		add("rotate " + pt.ID + " <bearing>")
		add("line intersect " + pt.ID + " <a2> <b1> <b2> as " + nextPointID + " [code]")
		add("intersect bearing-bearing " + pt.ID + " <brg1> <p2> <brg2> as " + nextPointID + " [code]")
		add("intersect bearing-distance " + pt.ID + " <brg> <p2> <dist> choose near|far as " + nextPointID + " [code]")
		add("intersect distance-distance " + pt.ID + " <dist1> <p2> <dist2> choose left|right as " + nextPointID + " [code]")
		add("resect " + pt.ID + " <brg1> <p2> <brg2> <p3> <brg3> as " + nextPointID + " [code]")
		add("pt edit " + pt.ID + " ")
		add("pt del " + pt.ID)
		add("pt rename " + pt.ID + " " + nextPointID)
	}

	lines := p.SortedLines()
	for _, line := range lines {
		add("line del " + line.ID)
		add("offset " + line.ID + " <offset> <chainage> as " + nextPointID + " [code]")
		add("contour gen C1 1 breaklines=ids:" + line.ID)
	}

	for _, set := range p.SortedContourSets() {
		add("contour info " + set.ID)
		add("contour del " + set.ID)
	}

	for _, value := range help.Suggestions() {
		add(value)
	}

	return suggestions
}

func commandSuggestionsForInput(p *project.Project, value string) []string {
	suggestions := commandSuggestions(p)
	contextual := contextualSuggestion(value, suggestions)
	if contextual == "" {
		return suggestions
	}
	out := []string{contextual}
	for _, suggestion := range suggestions {
		if suggestion != contextual {
			out = append(out, suggestion)
		}
	}
	return out
}

func (m *Model) completeNextInput() bool {
	m.refreshCompletions()
	suggestion := m.input.CurrentSuggestion()
	value := m.input.Value()
	chunk := nextCompletionChunk(value, suggestion)
	if chunk == "" {
		return false
	}
	m.input.SetValue(value + chunk)
	m.input.CursorEnd()
	m.refreshCompletions()
	return true
}

func nextCompletionChunk(value, suggestion string) string {
	if suggestion == "" || len([]rune(value)) >= len([]rune(suggestion)) {
		return ""
	}
	if !strings.HasPrefix(strings.ToLower(suggestion), strings.ToLower(value)) {
		return ""
	}
	valueRunes := []rune(value)
	rest := []rune(suggestion)[len(valueRunes):]
	if len(rest) == 0 {
		return ""
	}
	if len(valueRunes) > 0 && unicode.IsSpace(valueRunes[len(valueRunes)-1]) {
		return string(nextToken(rest))
	}
	if unicode.IsSpace(rest[0]) {
		return string(leadingSpaces(rest))
	}
	return string(nextToken(rest))
}

func nextToken(value []rune) []rune {
	i := 0
	for i < len(value) && unicode.IsSpace(value[i]) {
		i++
	}
	for i < len(value) && !unicode.IsSpace(value[i]) {
		i++
	}
	for i < len(value) && unicode.IsSpace(value[i]) {
		i++
	}
	return value[:i]
}

func leadingSpaces(value []rune) []rune {
	i := 0
	for i < len(value) && unicode.IsSpace(value[i]) {
		i++
	}
	return value[:i]
}

func contextualSuggestion(value string, templates []string) string {
	if strings.TrimSpace(value) == "" {
		return ""
	}
	for _, template := range templates {
		if suggestion, ok := fillTemplatePrefix(value, template); ok {
			return suggestion
		}
	}
	return ""
}

func fillTemplatePrefix(value, template string) (string, bool) {
	valueFields := strings.Fields(value)
	templateFields := strings.Fields(template)
	if len(valueFields) == 0 || len(valueFields) > len(templateFields) {
		return "", false
	}
	trailingSpace := len(value) > 0 && unicode.IsSpace([]rune(value)[len([]rune(value))-1])
	out := make([]string, 0, len(templateFields))
	for i, valueField := range valueFields {
		templateField := templateFields[i]
		last := i == len(valueFields)-1
		if isTemplatePlaceholder(templateField) {
			out = append(out, valueField)
			continue
		}
		if last && !trailingSpace {
			if !strings.HasPrefix(strings.ToLower(templateField), strings.ToLower(valueField)) {
				return "", false
			}
			out = append(out, templateField)
			continue
		}
		if !strings.EqualFold(templateField, valueField) {
			return "", false
		}
		out = append(out, templateField)
	}
	out = append(out, templateFields[len(valueFields):]...)
	suggestion := strings.Join(out, " ")
	if trailingSpace && !strings.HasSuffix(suggestion, " ") {
		// strings.Fields intentionally normalizes whitespace. The next template
		// token remains after the space, which keeps the original value a prefix.
		return suggestion, strings.HasPrefix(suggestion, value)
	}
	return suggestion, strings.HasPrefix(strings.ToLower(suggestion), strings.ToLower(value))
}

func isTemplatePlaceholder(value string) bool {
	return strings.Contains(value, "<") || strings.Contains(value, "[")
}
