package app

import (
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"

	"lsurvey/internal/boundary"
	"lsurvey/internal/codelib"
	"lsurvey/internal/cogo"
	"lsurvey/internal/csvpoints"
	"lsurvey/internal/dxf"
	"lsurvey/internal/geojson"
	"lsurvey/internal/geom"
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

type ConversionCommit struct {
	Points         []geom.Point
	TargetCRS      project.HorizontalCRS
	SourceSystem   string
	TargetSystem   string
	Model          string
	AssumeExisting bool
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
	lifecycle := sessionLifecycle{s: s}
	if outcome, handled, err := lifecycle.TryExecute(command, fields); handled || err != nil {
		if err != nil {
			return Outcome{}, err
		}
		outcome.CoordinateLabel = s.Project.CoordinateLabel()
		return outcome, nil
	}
	outcome, err := s.executeCOGOCommand(command)
	if err != nil {
		return Outcome{}, err
	}
	outcome.CoordinateLabel = s.Project.CoordinateLabel()
	return outcome, nil
}

func (s *Session) executeCOGOCommand(command string) (Outcome, error) {
	before, err := cloneProject(s.Project)
	if err != nil {
		return Outcome{}, err
	}
	result, err := cogo.Execute(s.Project, command)
	if err != nil {
		return Outcome{}, err
	}
	outcome := Outcome{Message: result.Message}
	if result.Changed {
		if err := s.commitMutation(before, command, result.Message, result.Created, result.Updated, result.Deleted); err != nil {
			return Outcome{}, err
		}
		outcome.ProjectChanged = true
	}
	return outcome, nil
}

func (s *Session) commitMutation(before *project.Project, command, message string, created, updated, deleted []string) error {
	return s.commitMutationExtra(before, command, message, created, updated, deleted, nil)
}

func (s *Session) commitMutationExtra(before *project.Project, command, message string, created, updated, deleted []string, extra any) error {
	s.Project.AddHistoryChange(command, message, created, updated, deleted, extra, nil)
	after, err := cloneProject(s.Project)
	if err != nil {
		return err
	}
	s.undo = append(s.undo, editFrame{Command: command, Before: before, After: after})
	s.redo = nil
	s.Dirty = true
	return nil
}

func (s *Session) CommitConvertedPoints(request ConversionCommit) (Outcome, error) {
	if len(request.Points) == 0 {
		return Outcome{}, fmt.Errorf("no converted points selected")
	}
	if request.TargetCRS.Projection != "MGA" || (request.TargetCRS.Datum != "GDA94" && request.TargetCRS.Datum != "GDA2020") || request.TargetCRS.Zone < 46 || request.TargetCRS.Zone > 59 {
		return Outcome{}, fmt.Errorf("conversion target must be MGA94 or MGA2020 in zone 46 to 59")
	}
	if s.Project.GridGround != nil {
		return Outcome{}, fmt.Errorf("cannot commit converted coordinates while a grid-to-ground scale is active")
	}
	if s.Project.HorizontalCRS != nil && !s.Project.HorizontalCRS.Equal(request.TargetCRS) {
		return Outcome{}, fmt.Errorf("project coordinates are %s; converted points are %s", s.Project.HorizontalCRS.Label(), request.TargetCRS.Label())
	}
	if s.Project.HorizontalCRS == nil && len(s.Project.Points) > 0 && !request.AssumeExisting {
		return Outcome{}, fmt.Errorf("existing project points have no CRS; confirm they are %s before committing", request.TargetCRS.Label())
	}
	seen := make(map[string]bool, len(request.Points))
	for _, point := range request.Points {
		if point.ID == "" {
			return Outcome{}, fmt.Errorf("converted point ID is required")
		}
		if seen[point.ID] {
			return Outcome{}, fmt.Errorf("converted point %q is duplicated", point.ID)
		}
		seen[point.ID] = true
		if _, exists := s.Project.Points[point.ID]; exists {
			return Outcome{}, fmt.Errorf("point %q already exists", point.ID)
		}
	}
	before, err := cloneProject(s.Project)
	if err != nil {
		return Outcome{}, err
	}
	target := request.TargetCRS
	s.Project.HorizontalCRS = &target
	created := make([]string, 0, len(request.Points))
	for _, point := range request.Points {
		s.Project.Points[point.ID] = s.Project.ApplyPointCodeStyle(point)
		created = append(created, "point:"+point.ID)
	}
	s.Project.MarkContoursStale("converted point geometry changed")
	model := request.Model
	if model == "" {
		model = "projection_only"
	}
	command := fmt.Sprintf("convert commit source=%s target=%s model=%s count=%d", request.SourceSystem, request.TargetSystem, model, len(request.Points))
	message := fmt.Sprintf("committed %d converted points to %s; elevations unchanged", len(request.Points), request.TargetCRS.Label())
	extra := map[string]any{
		"source_system": request.SourceSystem, "target_system": request.TargetSystem,
		"model": model, "count": len(request.Points), "elevation": "unchanged",
	}
	if err := s.commitMutationExtra(before, command, message, created, nil, nil, extra); err != nil {
		return Outcome{}, err
	}
	return Outcome{Message: message, ProjectChanged: true, CoordinateLabel: s.Project.CoordinateLabel()}, nil
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
	s.Project.AddHistoryChange("undo", message, nil, nil, nil, historyAction{Action: "undo", TargetCommand: frame.Command}, nil)
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
	s.Project.AddHistoryChange("redo", message, nil, nil, nil, historyAction{Action: "redo", TargetCommand: frame.Command}, nil)
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
	return p.Clone(), nil
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
	if len(record.Deleted) > 0 {
		fmt.Fprintf(&b, "\ndeleted=%s", strings.Join(record.Deleted, ","))
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

func Export(p *project.Project, format, path string, options ...string) (string, error) {
	switch format {
	case "dxf":
		if len(options) != 0 {
			return "", fmt.Errorf("usage: export dxf <file>")
		}
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
		if len(options) != 0 {
			return "", fmt.Errorf("usage: export csv <file>")
		}
		path = paths.CSV(path)
		return path, csvpoints.ExportFile(path, p)
	case "geojson":
		if len(options) != 0 {
			return "", fmt.Errorf("usage: export geojson <file>")
		}
		path = paths.GeoJSON(path)
		return path, geojson.ExportFile(path, p)
	case "landxml":
		if len(options) != 0 {
			return "", fmt.Errorf("usage: export landxml <file>")
		}
		path = paths.LandXML(path)
		return path, landxml.ExportFile(path, p)
	case "boundarycsv":
		polygonID := ""
		if len(options) > 1 || len(options) == 1 && !strings.HasPrefix(options[0], "polygon=") {
			return "", fmt.Errorf("usage: export boundarycsv <file> [polygon=<id>]")
		}
		if len(options) == 1 {
			polygonID = strings.TrimPrefix(options[0], "polygon=")
			if polygonID == "" {
				return "", fmt.Errorf("polygon ID cannot be empty")
			}
		}
		path = paths.CSV(path)
		return path, boundary.ExportFile(path, p, polygonID)
	case "codes":
		if len(options) != 0 {
			return "", fmt.Errorf("usage: export codes <file>")
		}
		path = paths.Codes(path)
		return path, codelib.ExportFile(path, p)
	default:
		return "", fmt.Errorf("usage: export dxf|csv|geojson|landxml|boundarycsv|codes <file> [polygon=<id>]")
	}
}

func ExportProject(format, projectPath, outputPath string, options ...string) error {
	p, err := project.Load(projectPath)
	if err != nil {
		return err
	}
	_, err = Export(p, format, outputPath, options...)
	return err
}

func coordinateSuffix(p *project.Project) string {
	if label := p.CoordinateLabel(); label != "" {
		return "  coords=" + label
	}
	return ""
}
