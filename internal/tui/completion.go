package tui

import (
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

	for _, value := range help.Suggestions() {
		add(value)
	}

	points := p.SortedPoints()
	for _, pt := range points {
		add("inverse " + pt.ID + " ")
		add("radiate " + pt.ID + " ")
		add("pt edit " + pt.ID + " ")
		add("pt del " + pt.ID)
		add("pt rename " + pt.ID + " ")
	}

	lines := p.SortedLines()
	for _, line := range lines {
		add("line del " + line.ID)
		add("contour gen C1 1 breaklines=ids:" + line.ID)
	}

	for _, set := range p.SortedContourSets() {
		add("contour info " + set.ID)
		add("contour del " + set.ID)
	}

	return suggestions
}
