package codelib

import (
	"strings"
	"testing"

	"lsurvey/internal/project"
)

func TestWriteAndImportMergeStyleDefinitions(t *testing.T) {
	source := project.New("source")
	source.Groups["BND"] = project.Group{ID: "BND", Layer: "BOUNDARIES", Color: 1}
	source.PointCodeStyles["PEG"] = "BND"
	var exported strings.Builder
	if err := Write(&exported, source); err != nil {
		t.Fatal(err)
	}

	dest := project.New("dest")
	groups, mappings, err := Import(strings.NewReader(exported.String()), dest)
	if err != nil || groups != 1 || mappings != 1 || dest.PointCodeStyles["PEG"] != "BND" {
		t.Fatalf("groups=%d mappings=%d project=%+v err=%v", groups, mappings, dest, err)
	}
}

func TestImportRejectsConflictsAtomically(t *testing.T) {
	p := project.New("test")
	p.Groups["EXIST"] = project.Group{ID: "EXIST", Layer: "BOUNDARY", Color: 1}
	p.PointCodeStyles["PEG"] = "EXIST"
	input := `{"schema_version":1,"groups":{"NEW":{"id":"NEW","layer":"BOUNDARY","color":2}},"point_code_styles":{"TREE":"NEW"}}`
	if _, _, err := Import(strings.NewReader(input), p); err == nil {
		t.Fatal("expected conflicting layer import error")
	}
	if _, ok := p.Groups["NEW"]; ok || p.PointCodeStyles["TREE"] != "" {
		t.Fatalf("failed import mutated project: %+v", p)
	}
}
