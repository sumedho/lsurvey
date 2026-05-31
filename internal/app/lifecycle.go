package app

import (
	"fmt"
	"strconv"
	"strings"

	"lsurvey/internal/codelib"
	"lsurvey/internal/csvpoints"
	"lsurvey/internal/geojson"
	"lsurvey/internal/paths"
	"lsurvey/internal/project"
)

type Lifecycle interface {
	TryExecute(command string, fields []string) (Outcome, bool, error)
}

type sessionLifecycle struct {
	s *Session
}

func (l sessionLifecycle) TryExecute(command string, fields []string) (Outcome, bool, error) {
	s := l.s
	var outcome Outcome
	switch fields[0] {
	case "new":
		name := "untitled"
		if len(fields) > 1 {
			name = strings.Join(fields[1:], " ")
		}
		s.Project = project.New(name)
		s.Path = ""
		s.Dirty = false
		s.clearNavigation()
		outcome.Message = "new project: " + name
		outcome.ProjectReplaced = true
	case "open":
		if len(fields) != 2 {
			return Outcome{}, true, fmt.Errorf("usage: open <file>")
		}
		path := paths.Project(fields[1])
		loaded, err := project.Load(path)
		if err != nil {
			return Outcome{}, true, err
		}
		s.Project = loaded
		s.Path = path
		s.Dirty = false
		s.clearNavigation()
		outcome.Message = "opened " + s.Path
		outcome.ProjectReplaced = true
	case "save":
		path := s.Path
		if len(fields) > 2 {
			return Outcome{}, true, fmt.Errorf("usage: save [file]")
		}
		if len(fields) == 2 {
			path = paths.Project(fields[1])
		}
		if path == "" {
			return Outcome{}, true, fmt.Errorf("usage: save <file>")
		}
		path = paths.Project(path)
		s.Project.AppVersion = s.Version
		if err := project.Save(path, s.Project); err != nil {
			return Outcome{}, true, err
		}
		s.Path = path
		s.Dirty = false
		outcome.Message = "saved " + s.Path
	case "saveas":
		if len(fields) != 2 {
			return Outcome{}, true, fmt.Errorf("usage: saveas <file>")
		}
		path := paths.Project(fields[1])
		s.Project.AppVersion = s.Version
		if err := project.Save(path, s.Project); err != nil {
			return Outcome{}, true, err
		}
		s.Path = path
		s.Dirty = false
		outcome.Message = "saved " + s.Path
	case "export":
		if len(fields) < 3 {
			return Outcome{}, true, fmt.Errorf("usage: export dxf|csv|geojson|landxml|boundarycsv|codes <file> [polygon=<id>]")
		}
		path, err := Export(s.Project, fields[1], fields[2], fields[3:]...)
		if err != nil {
			return Outcome{}, true, err
		}
		outcome.Message = "exported " + path + coordinateSuffix(s.Project)
	case "undo":
		if len(fields) != 1 {
			return Outcome{}, true, fmt.Errorf("usage: undo")
		}
		outcome, err := s.undoEdit()
		return outcome, true, err
	case "redo":
		if len(fields) != 1 {
			return Outcome{}, true, fmt.Errorf("usage: redo")
		}
		outcome, err := s.redoEdit()
		return outcome, true, err
	case "history":
		message, err := s.historyReport(fields)
		if err != nil {
			return Outcome{}, true, err
		}
		outcome.Message = message
	case "info":
		if len(fields) != 1 {
			return Outcome{}, true, fmt.Errorf("usage: info")
		}
		outcome.Message = s.projectInfo()
	case "import":
		if len(fields) != 3 {
			return Outcome{}, true, fmt.Errorf("usage: import csv|geojson|codes <file>")
		}
		before, err := cloneProject(s.Project)
		if err != nil {
			return Outcome{}, true, err
		}
		switch fields[1] {
		case "csv":
			path := paths.CSV(fields[2])
			count, err := csvpoints.ImportFile(path, s.Project)
			if err != nil {
				return Outcome{}, true, err
			}
			outcome.Message = fmt.Sprintf("imported %d points from %s", count, path)
		case "geojson":
			path := paths.GeoJSON(fields[2])
			points, lines, err := geojson.ImportFile(path, s.Project)
			if err != nil {
				return Outcome{}, true, err
			}
			outcome.Message = fmt.Sprintf("imported %d points and %d features from %s", points, lines, path)
		case "codes":
			path := paths.Codes(fields[2])
			groups, mappings, err := codelib.ImportFile(path, s.Project)
			if err != nil {
				return Outcome{}, true, err
			}
			outcome.Message = fmt.Sprintf("imported %d groups and %d point code styles from %s", groups, mappings, path)
		default:
			return Outcome{}, true, fmt.Errorf("usage: import csv|geojson|codes <file>")
		}
		if err := s.commitMutation(before, command, outcome.Message, nil, nil); err != nil {
			return Outcome{}, true, err
		}
		outcome.ProjectChanged = true
	case "desc":
		description := strings.TrimSpace(strings.TrimPrefix(command, "desc"))
		if description == "" {
			return Outcome{}, true, fmt.Errorf("usage: desc <project description>")
		}
		before, err := cloneProject(s.Project)
		if err != nil {
			return Outcome{}, true, err
		}
		s.Project.Description = description
		outcome.Message = "project description updated"
		if err := s.commitMutation(before, command, outcome.Message, nil, nil); err != nil {
			return Outcome{}, true, err
		}
		outcome.ProjectChanged = true
	case "precision":
		if len(fields) != 2 {
			return Outcome{}, true, fmt.Errorf("usage: precision <0-6>")
		}
		precision, err := strconv.Atoi(fields[1])
		if err != nil || precision < 0 || precision > 6 {
			return Outcome{}, true, fmt.Errorf("precision must be a number from 0 to 6")
		}
		before, err := cloneProject(s.Project)
		if err != nil {
			return Outcome{}, true, err
		}
		s.Project.SetDisplayPrecision(precision)
		outcome.Message = fmt.Sprintf("display precision set to %d", precision)
		if err := s.commitMutation(before, command, outcome.Message, nil, nil); err != nil {
			return Outcome{}, true, err
		}
		outcome.ProjectChanged = true
	default:
		return Outcome{}, false, nil
	}
	return outcome, true, nil
}
