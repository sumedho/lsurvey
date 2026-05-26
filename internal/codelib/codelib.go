package codelib

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"lsurvey/internal/project"
)

const SchemaVersion = 1

type Library struct {
	SchemaVersion   int                      `json:"schema_version"`
	Groups          map[string]project.Group `json:"groups"`
	PointCodeStyles map[string]string        `json:"point_code_styles"`
}

func Write(w io.Writer, p *project.Project) error {
	library := Library{SchemaVersion: SchemaVersion, Groups: p.Groups, PointCodeStyles: p.PointCodeStyles}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(library)
}

func ExportFile(path string, p *project.Project) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	err = Write(f, p)
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}
	return err
}

func ImportFile(path string, p *project.Project) (int, int, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, 0, err
	}
	defer f.Close()
	return Import(f, p)
}

func Import(r io.Reader, p *project.Project) (int, int, error) {
	var library Library
	if err := json.NewDecoder(r).Decode(&library); err != nil {
		return 0, 0, err
	}
	if library.SchemaVersion != SchemaVersion {
		return 0, 0, fmt.Errorf("unsupported code library schema_version %d", library.SchemaVersion)
	}
	groups := cloneGroups(p.Groups)
	mappings := cloneMappings(p.PointCodeStyles)
	for id, incoming := range library.Groups {
		if incoming.ID == "" {
			incoming.ID = id
		}
		if incoming.ID != id || strings.TrimSpace(incoming.Layer) == "" || incoming.Color < 1 || incoming.Color > 255 {
			return 0, 0, fmt.Errorf("invalid group %q in code library", id)
		}
		if existing, ok := groups[id]; ok {
			if existing != incoming {
				return 0, 0, fmt.Errorf("conflicting definition for group %q", id)
			}
			continue
		}
		for otherID, existing := range groups {
			if otherID != id && strings.EqualFold(existing.Layer, incoming.Layer) {
				return 0, 0, fmt.Errorf("group layer %q already exists", incoming.Layer)
			}
		}
		groups[id] = incoming
	}
	for code, groupID := range library.PointCodeStyles {
		if code == "" {
			return 0, 0, fmt.Errorf("point code cannot be empty")
		}
		if _, ok := groups[groupID]; !ok {
			return 0, 0, fmt.Errorf("point code %q references unknown group %q", code, groupID)
		}
		if existing, ok := mappings[code]; ok && existing != groupID {
			return 0, 0, fmt.Errorf("conflicting point code style for %q", code)
		}
		mappings[code] = groupID
	}
	p.Groups = groups
	p.PointCodeStyles = mappings
	return len(library.Groups), len(library.PointCodeStyles), nil
}

func cloneGroups(src map[string]project.Group) map[string]project.Group {
	dst := make(map[string]project.Group, len(src))
	for id, value := range src {
		dst[id] = value
	}
	return dst
}

func cloneMappings(src map[string]string) map[string]string {
	dst := make(map[string]string, len(src))
	for code, groupID := range src {
		dst[code] = groupID
	}
	return dst
}
