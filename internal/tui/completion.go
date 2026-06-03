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
	add("convert")

	points := p.SortedPoints()
	codes := map[string]bool{}
	for _, pt := range points {
		if pt.Code != "" && !codes[pt.Code] {
			add("line gen " + pt.Code)
			codes[pt.Code] = true
		}
	}

	lines := p.SortedFeatures()
	lineCodes := map[string]bool{}
	for _, feature := range lines {
		add(feature.Kind + " del " + feature.ID)
		if feature.Kind == project.FeaturePolygon {
			add("polygon report " + feature.ID)
			add("export boundarycsv <file> polygon=" + feature.ID)
		}
		if feature.Kind == project.FeatureLine {
			add("offset " + feature.ID + " <offset> <chainage> as " + nextPointID + " [code]")
		}
		if feature.Kind != project.FeaturePolygon {
			add("contour gen C1 1 breaklines=ids:" + feature.ID)
		}
		if feature.Code != "" && !lineCodes[feature.Code] {
			add("contour gen C1 1 boundary=codes:" + feature.Code)
			add("contour gen C1 1 exclude=codes:" + feature.Code)
			add("contour gen C1 1 boundary=codes:" + feature.Code + " maxedge=<distance> smooth=1")
			lineCodes[feature.Code] = true
		}
	}

	for _, group := range p.SortedGroups() {
		add("code style set <code> group=" + group.ID)
	}

	for _, set := range p.SortedContourSets() {
		add("contour info " + set.ID)
		add("contour regen " + set.ID)
		add("contour del " + set.ID)
	}

	for _, value := range help.Suggestions() {
		add(value)
	}

	return suggestions
}

func commandSuggestionsForInput(p *project.Project, value string) []string {
	return commandSuggestionsForInputTemplates(p, commandSuggestions(p), value)
}

func commandSuggestionsForInputTemplates(p *project.Project, suggestions []string, value string) []string {
	contextual := contextualSuggestion(value, suggestions)
	if pointContextual := contextualPointSuggestion(p, value); pointContextual != "" {
		contextual = pointContextual
	}
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

type pointCompletionTemplate struct {
	prefix []string
	suffix []string
}

func contextualPointSuggestion(p *project.Project, value string) string {
	if p == nil || strings.TrimSpace(value) == "" {
		return ""
	}
	fields := strings.Fields(value)
	if len(fields) == 0 {
		return ""
	}
	trailingSpace := hasTrailingSpace(value)
	nextPointID := p.NextPointID()
	templates := []pointCompletionTemplate{
		{prefix: []string{"close"}, suffix: []string{"<p2>", "<p3>", "..."}},
		{prefix: []string{"inverse"}, suffix: []string{"<to>"}},
		{prefix: []string{"rad"}, suffix: []string{"<azimuth|bearing>", "<distance>", "[vdiff", "<delta>]", "as", nextPointID, "[code]"}},
		{prefix: []string{"rad3d"}, suffix: []string{"<azimuth|bearing>", "<slope_distance>", "<zenith>", "as", nextPointID, "[code]"}},
		{prefix: []string{"midpoint"}, suffix: []string{"<p2>", "as", nextPointID, "[code]"}},
		{prefix: []string{"offset"}, suffix: []string{"<p2>", "<offset>", "<chainage>", "as", nextPointID, "[code]"}},
		{prefix: []string{"shift"}, suffix: []string{"east=<coordinate>", "[north=<coordinate>]", "[elev=<coordinate>]"}},
		{prefix: []string{"rotate"}, suffix: []string{"<bearing>"}},
		{prefix: []string{"scale", "apply"}, suffix: []string{"csf=<factor>", "[system=<label>]"}},
		{prefix: []string{"transform", "fit"}, suffix: []string{"<dst1>", "<src2>", "<dst2>", "[<srcN>", "<dstN>", "...]"}},
		{prefix: []string{"line", "intersect"}, suffix: []string{"<a2>", "<b1>", "<b2>", "as", nextPointID, "[code]"}},
		{prefix: []string{"intersect", "bearing-bearing"}, suffix: []string{"<brg1>", "<p2>", "<brg2>", "as", nextPointID, "[code]"}},
		{prefix: []string{"intersect", "bearing-distance"}, suffix: []string{"<brg>", "<p2>", "<dist>", "choose", "near|far", "as", nextPointID, "[code]"}},
		{prefix: []string{"intersect", "distance-distance"}, suffix: []string{"<dist1>", "<p2>", "<dist2>", "choose", "left|right", "as", nextPointID, "[code]"}},
		{prefix: []string{"resect"}, suffix: []string{"<brg1>", "<p2>", "<brg2>", "<p3>", "<brg3>", "as", nextPointID, "[code]"}},
		{prefix: []string{"pt", "edit"}, suffix: nil},
		{prefix: []string{"pt", "del"}, suffix: nil},
		{prefix: []string{"pt", "rename"}, suffix: []string{nextPointID}},
	}
	for _, template := range templates {
		suggestion := pointSuggestionForTemplate(p, value, fields, trailingSpace, template)
		if suggestion != "" {
			return suggestion
		}
	}
	return ""
}

func pointSuggestionForTemplate(p *project.Project, value string, fields []string, trailingSpace bool, template pointCompletionTemplate) string {
	pointIndex := len(template.prefix)
	if !pointCompletionPrefixMatches(fields, trailingSpace, template.prefix) || !pointCompletionReachedPoint(fields, trailingSpace, pointIndex) {
		return ""
	}
	pointQuery := ""
	if len(fields) > pointIndex {
		pointQuery = fields[pointIndex]
	}
	pointID := matchingPointID(p, pointQuery)
	if pointID == "" {
		return ""
	}
	parts := make([]string, 0, len(template.prefix)+1+len(template.suffix))
	parts = append(parts, template.prefix...)
	parts = append(parts, pointID)
	parts = append(parts, template.suffix...)
	suggestion := strings.Join(parts, " ")
	if len(template.suffix) == 0 {
		suggestion += " "
	}
	if completed, ok := fillTemplatePrefix(value, suggestion); ok {
		return completed
	}
	if strings.HasPrefix(strings.ToLower(suggestion), strings.ToLower(value)) {
		return suggestion
	}
	return ""
}

func pointCompletionPrefixMatches(fields []string, trailingSpace bool, prefix []string) bool {
	for i, want := range prefix {
		if i >= len(fields) {
			return false
		}
		if i == len(fields)-1 && !trailingSpace {
			return strings.HasPrefix(strings.ToLower(want), strings.ToLower(fields[i]))
		}
		if !strings.EqualFold(fields[i], want) {
			return false
		}
	}
	return true
}

func pointCompletionReachedPoint(fields []string, trailingSpace bool, pointIndex int) bool {
	return len(fields) > pointIndex || len(fields) == pointIndex && trailingSpace
}

func matchingPointID(p *project.Project, query string) string {
	if query != "" {
		if _, ok := p.Points[query]; ok {
			return query
		}
	}
	for _, pt := range p.SortedPoints() {
		if query == "" || strings.HasPrefix(strings.ToLower(pt.ID), strings.ToLower(query)) {
			return pt.ID
		}
	}
	return ""
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

func hasTrailingSpace(value string) bool {
	runes := []rune(value)
	return len(runes) > 0 && unicode.IsSpace(runes[len(runes)-1])
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
	trailingSpace := hasTrailingSpace(value)
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
