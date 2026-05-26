package app

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"

	"lsurvey/internal/cogo"
	"lsurvey/internal/csvpoints"
	"lsurvey/internal/dxf"
	"lsurvey/internal/geojson"
	"lsurvey/internal/landxml"
	"lsurvey/internal/paths"
	"lsurvey/internal/project"
)

type Session struct {
	Project *project.Project
	Path    string
	Version string
	Dirty   bool
	undo    []editFrame
	redo    []editFrame
}

type editFrame struct {
	Command string
	Before  *project.Project
	After   *project.Project
}

type historyAction struct {
	Action        string `json:"action"`
	TargetCommand string `json:"target_command"`
}

type Outcome struct {
	Message         string
	ProjectChanged  bool
	ProjectReplaced bool
	CoordinateLabel string
}

func Fields(command string) ([]string, error) {
	return cogo.Fields(command)
}

func NewSession(p *project.Project, path, version string) *Session {
	return &Session{Project: p, Path: path, Version: version}
}

func (s *Session) Execute(command string) (Outcome, error) {
	command = strings.TrimSpace(command)
	fields, err := cogo.Fields(command)
	if err != nil {
		return Outcome{}, err
	}
	if len(fields) == 0 {
		return Outcome{}, fmt.Errorf("empty command")
	}
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
			return Outcome{}, fmt.Errorf("usage: open <file>")
		}
		path := paths.Project(fields[1])
		loaded, err := project.Load(path)
		if err != nil {
			return Outcome{}, err
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
			return Outcome{}, fmt.Errorf("usage: save [file]")
		}
		if len(fields) == 2 {
			path = paths.Project(fields[1])
		}
		if path == "" {
			return Outcome{}, fmt.Errorf("usage: save <file>")
		}
		path = paths.Project(path)
		s.Project.AppVersion = s.Version
		if err := project.Save(path, s.Project); err != nil {
			return Outcome{}, err
		}
		s.Path = path
		s.Dirty = false
		outcome.Message = "saved " + s.Path
	case "saveas":
		if len(fields) != 2 {
			return Outcome{}, fmt.Errorf("usage: saveas <file>")
		}
		path := paths.Project(fields[1])
		s.Project.AppVersion = s.Version
		if err := project.Save(path, s.Project); err != nil {
			return Outcome{}, err
		}
		s.Path = path
		s.Dirty = false
		outcome.Message = "saved " + s.Path
	case "export":
		if len(fields) != 3 {
			return Outcome{}, fmt.Errorf("usage: export dxf|csv|geojson|landxml <file>")
		}
		path, err := Export(s.Project, fields[1], fields[2])
		if err != nil {
			return Outcome{}, err
		}
		outcome.Message = "exported " + path + coordinateSuffix(s.Project)
	case "undo":
		if len(fields) != 1 {
			return Outcome{}, fmt.Errorf("usage: undo")
		}
		return s.undoEdit()
	case "redo":
		if len(fields) != 1 {
			return Outcome{}, fmt.Errorf("usage: redo")
		}
		return s.redoEdit()
	case "history":
		message, err := s.historyReport(fields)
		if err != nil {
			return Outcome{}, err
		}
		outcome.Message = message
	case "info":
		if len(fields) != 1 {
			return Outcome{}, fmt.Errorf("usage: info")
		}
		outcome.Message = s.projectInfo()
	case "import":
		if len(fields) != 3 {
			return Outcome{}, fmt.Errorf("usage: import csv|geojson <file>")
		}
		before, err := cloneProject(s.Project)
		if err != nil {
			return Outcome{}, err
		}
		switch fields[1] {
		case "csv":
			path := paths.CSV(fields[2])
			count, err := csvpoints.ImportFile(path, s.Project)
			if err != nil {
				return Outcome{}, err
			}
			outcome.Message = fmt.Sprintf("imported %d points from %s", count, path)
		case "geojson":
			path := paths.GeoJSON(fields[2])
			points, lines, err := geojson.ImportFile(path, s.Project)
			if err != nil {
				return Outcome{}, err
			}
			outcome.Message = fmt.Sprintf("imported %d points and %d features from %s", points, lines, path)
		default:
			return Outcome{}, fmt.Errorf("usage: import csv|geojson <file>")
		}
		if err := s.commitMutation(before, command, outcome.Message, nil, nil); err != nil {
			return Outcome{}, err
		}
		outcome.ProjectChanged = true
	case "desc":
		description := strings.TrimSpace(strings.TrimPrefix(command, "desc"))
		if description == "" {
			return Outcome{}, fmt.Errorf("usage: desc <project description>")
		}
		before, err := cloneProject(s.Project)
		if err != nil {
			return Outcome{}, err
		}
		s.Project.Description = description
		outcome.Message = "project description updated"
		if err := s.commitMutation(before, command, outcome.Message, nil, nil); err != nil {
			return Outcome{}, err
		}
		outcome.ProjectChanged = true
	case "precision":
		if len(fields) != 2 {
			return Outcome{}, fmt.Errorf("usage: precision <0-6>")
		}
		precision, err := strconv.Atoi(fields[1])
		if err != nil || precision < 0 || precision > 6 {
			return Outcome{}, fmt.Errorf("precision must be a number from 0 to 6")
		}
		before, err := cloneProject(s.Project)
		if err != nil {
			return Outcome{}, err
		}
		s.Project.SetDisplayPrecision(precision)
		outcome.Message = fmt.Sprintf("display precision set to %d", precision)
		if err := s.commitMutation(before, command, outcome.Message, nil, nil); err != nil {
			return Outcome{}, err
		}
		outcome.ProjectChanged = true
	default:
		before, err := cloneProject(s.Project)
		if err != nil {
			return Outcome{}, err
		}
		result, err := cogo.Execute(s.Project, command)
		if err != nil {
			return Outcome{}, err
		}
		outcome.Message = result.Message
		if result.Changed {
			if err := s.commitMutation(before, command, result.Message, result.Created, result.Updated); err != nil {
				return Outcome{}, err
			}
			outcome.ProjectChanged = true
		}
	}
	outcome.CoordinateLabel = s.Project.CoordinateLabel()
	return outcome, nil
}

func (s *Session) commitMutation(before *project.Project, command, message string, created, updated []string) error {
	s.Project.AddHistoryChange(command, message, created, updated, nil, nil)
	after, err := cloneProject(s.Project)
	if err != nil {
		return err
	}
	s.undo = append(s.undo, editFrame{Command: command, Before: before, After: after})
	s.redo = nil
	s.Dirty = true
	return nil
}

func (s *Session) undoEdit() (Outcome, error) {
	if len(s.undo) == 0 {
		return Outcome{}, fmt.Errorf("nothing to undo")
	}
	frame := s.undo[len(s.undo)-1]
	if err := s.restoreProject(frame.Before); err != nil {
		return Outcome{}, err
	}
	s.undo = s.undo[:len(s.undo)-1]
	s.redo = append(s.redo, frame)
	message := "undid " + frame.Command
	s.Project.AddHistoryChange("undo", message, nil, nil, historyAction{Action: "undo", TargetCommand: frame.Command}, nil)
	s.Dirty = true
	return Outcome{Message: message, ProjectChanged: true, CoordinateLabel: s.Project.CoordinateLabel()}, nil
}

func (s *Session) redoEdit() (Outcome, error) {
	if len(s.redo) == 0 {
		return Outcome{}, fmt.Errorf("nothing to redo")
	}
	frame := s.redo[len(s.redo)-1]
	if err := s.restoreProject(frame.After); err != nil {
		return Outcome{}, err
	}
	s.redo = s.redo[:len(s.redo)-1]
	s.undo = append(s.undo, frame)
	message := "redid " + frame.Command
	s.Project.AddHistoryChange("redo", message, nil, nil, historyAction{Action: "redo", TargetCommand: frame.Command}, nil)
	s.Dirty = true
	return Outcome{Message: message, ProjectChanged: true, CoordinateLabel: s.Project.CoordinateLabel()}, nil
}

func (s *Session) restoreProject(snapshot *project.Project) error {
	restored, err := cloneProject(snapshot)
	if err != nil {
		return err
	}
	restored.History = s.Project.History
	restored.AppVersion = s.Project.AppVersion
	s.Project = restored
	return nil
}

func (s *Session) clearNavigation() {
	s.undo = nil
	s.redo = nil
}

func cloneProject(p *project.Project) (*project.Project, error) {
	data, err := json.Marshal(p)
	if err != nil {
		return nil, err
	}
	var cloned project.Project
	if err := json.Unmarshal(data, &cloned); err != nil {
		return nil, err
	}
	return &cloned, nil
}

func (s *Session) historyReport(fields []string) (string, error) {
	if len(fields) == 3 && fields[1] == "info" {
		n, err := strconv.Atoi(fields[2])
		if err != nil || n < 1 || n > len(s.Project.History) {
			return "", fmt.Errorf("history entry must be between 1 and %d", len(s.Project.History))
		}
		return historyDetail(n, s.Project.History[n-1]), nil
	}
	limit := 20
	if len(fields) == 2 && strings.HasPrefix(fields[1], "limit=") {
		n, err := strconv.Atoi(strings.TrimPrefix(fields[1], "limit="))
		if err != nil || n <= 0 {
			return "", fmt.Errorf("history limit must be greater than zero")
		}
		limit = n
	} else if len(fields) != 1 {
		return "", fmt.Errorf("usage: history [limit=<n>] OR history info <n>")
	}
	if len(s.Project.History) == 0 {
		return "0 history entries", nil
	}
	start := len(s.Project.History) - limit
	if start < 0 {
		start = 0
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%d history entries (showing %d)", len(s.Project.History), len(s.Project.History)-start)
	for i := len(s.Project.History) - 1; i >= start; i-- {
		record := s.Project.History[i]
		fmt.Fprintf(&b, "\n%d %s %s: %s", i+1, record.At.UTC().Format("2006-01-02T15:04:05Z"), record.Command, record.Result)
	}
	return b.String(), nil
}

func historyDetail(index int, record project.HistoryRecord) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%d %s command=%s result=%s", index, record.At.UTC().Format("2006-01-02T15:04:05Z"), record.Command, record.Result)
	if len(record.Created) > 0 {
		fmt.Fprintf(&b, "\ncreated=%s", strings.Join(record.Created, ","))
	}
	if len(record.Updated) > 0 {
		fmt.Fprintf(&b, "\nupdated=%s", strings.Join(record.Updated, ","))
	}
	if len(record.Extra) > 0 {
		fmt.Fprintf(&b, "\nextra=%s", record.Extra)
	}
	return b.String()
}

func (s *Session) projectInfo() string {
	path := s.Path
	if path == "" {
		path = "unsaved"
	}
	state := "clean"
	if s.Dirty {
		state = "dirty"
	}
	keys := make([]string, 0, len(s.Project.Units))
	for key := range s.Project.Units {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	units := make([]string, 0, len(keys))
	for _, key := range keys {
		units = append(units, key+"="+s.Project.Units[key])
	}
	message := fmt.Sprintf("project=%s path=%s state=%s precision=%d units=%s points=%d features=%d groups=%d contours=%d history=%d undo=%t redo=%t",
		s.Project.Name, path, state, s.Project.DisplayPrecision(), strings.Join(units, ","),
		len(s.Project.Points), len(s.Project.Features), len(s.Project.Groups), len(s.Project.ContourSets), len(s.Project.History), len(s.undo) > 0, len(s.redo) > 0)
	if s.Project.Description != "" {
		message += " description=" + s.Project.Description
	}
	if label := s.Project.CoordinateLabel(); label != "" {
		message += " coords=" + label
	}
	if s.Project.Traverse != nil {
		message += fmt.Sprintf(" traverse=%s->%s", s.Project.Traverse.Start, s.Project.Traverse.Current)
	}
	return message
}

func Export(p *project.Project, format, path string) (string, error) {
	switch format {
	case "dxf":
		path = paths.DXF(path)
		f, err := os.Create(path)
		if err != nil {
			return "", err
		}
		err = dxf.Write(f, p)
		if closeErr := f.Close(); err == nil {
			err = closeErr
		}
		return path, err
	case "csv":
		path = paths.CSV(path)
		return path, csvpoints.ExportFile(path, p)
	case "geojson":
		path = paths.GeoJSON(path)
		return path, geojson.ExportFile(path, p)
	case "landxml":
		path = paths.LandXML(path)
		return path, landxml.ExportFile(path, p)
	default:
		return "", fmt.Errorf("usage: export dxf|csv|geojson|landxml <file>")
	}
}

func ExportProject(format, projectPath, outputPath string) error {
	p, err := project.Load(projectPath)
	if err != nil {
		return err
	}
	_, err = Export(p, format, outputPath)
	return err
}

func coordinateSuffix(p *project.Project) string {
	if label := p.CoordinateLabel(); label != "" {
		return "  coords=" + label
	}
	return ""
}
