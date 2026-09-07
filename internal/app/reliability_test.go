package app

import (
	"encoding/json"
	"errors"
	"lsurvey/internal/project"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"lsurvey/internal/geom"
)

type failingStore struct{ project.JSONStore }

func (failingStore) Save(string, *project.Project) error { return errors.New("disk full") }

func TestFailedSavePreservesSession(t *testing.T) {
	s := NewSession(project.New("test"), "", "v2")
	s.Store = failingStore{}
	if _, err := s.Execute("pt add A 1 2"); err != nil {
		t.Fatal(err)
	}
	before := s.Project.Clone()
	if _, err := s.Execute("save " + filepath.Join(t.TempDir(), "job")); err == nil {
		t.Fatal("expected failure")
	}
	if !s.Dirty || s.Path != "" || !reflect.DeepEqual(before, s.Project) {
		t.Fatal("failed save changed session")
	}
	if _, err := s.Execute("undo"); err != nil {
		t.Fatal(err)
	}
}

func TestRecoverLoadsBackupWithoutOverwritingOriginal(t *testing.T) {
	path := filepath.Join(t.TempDir(), "job")
	s := NewSession(project.New("test"), "", "v2")
	for _, cmd := range []string{"pt add A 1 2", "save " + path, "pt add B 3 4", "save"} {
		if _, err := s.Execute(cmd); err != nil {
			t.Fatal(err)
		}
	}
	out, err := s.Execute("recover " + path)
	if err != nil {
		t.Fatal(err)
	}
	if !out.ProjectReplaced || !s.Dirty || s.Path != "" || len(s.Project.Points) != 1 {
		t.Fatalf("recovered session: %+v", s)
	}
	p, err := project.Load(path + ".srv")
	if err != nil || len(p.Points) != 2 {
		t.Fatal("original modified")
	}
	if _, err = s.Execute("undo"); err == nil {
		t.Fatal("recovery retained old undo")
	}
}

func TestCheckAndTraverseQAHistory(t *testing.T) {
	s := NewSession(project.New("test"), "", "")
	for _, cmd := range []string{"pt add S 0 0", "pt add T 20 1", "trav start S", "trav leg 90 9", "trav leg 90 9", "trav close T"} {
		if _, err := s.Execute(cmd); err != nil {
			t.Fatal(err)
		}
	}
	n := len(s.Project.History)
	for _, cmd := range []string{"check", "trav report compass"} {
		out, err := s.Execute(cmd)
		if err != nil || out.ProjectChanged {
			t.Fatalf("%s: %+v %v", cmd, out, err)
		}
	}
	if len(s.Project.History) != n {
		t.Fatal("QA modified audit")
	}
	if _, err := s.Execute("trav adjust compass"); err != nil {
		t.Fatal(err)
	}
	h := s.Project.History[len(s.Project.History)-1]
	if !strings.Contains(string(h.Extra), "CorrectionE") {
		t.Fatalf("missing structured report: %s", h.Extra)
	}
	if _, err := s.Execute("undo"); err != nil {
		t.Fatal(err)
	}
	if s.Project.Traverse.Adjusted {
		t.Fatal("undo did not restore traverse")
	}
	if _, err := s.Execute("redo"); err != nil {
		t.Fatal(err)
	}
	if !s.Project.Traverse.Adjusted || len(s.Project.History) != n+3 {
		t.Fatal("redo/audit failed")
	}
	path := filepath.Join(t.TempDir(), "adjusted")
	if _, err := s.Execute("save " + path); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Execute("open " + path); err != nil {
		t.Fatal(err)
	}
	var saved, original geom.TraverseAdjustment
	if err := json.Unmarshal(s.Project.History[n].Extra, &saved); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(h.Extra, &original); err != nil {
		t.Fatal(err)
	}
	if !s.Project.Traverse.Adjusted || !reflect.DeepEqual(saved, original) {
		t.Fatal("saved adjustment lost QA or adjusted state")
	}
}

func TestInvalidEditAndLoadLeaveSessionIntact(t *testing.T) {
	s := NewSession(project.New("test"), "", "")
	for _, cmd := range []string{"pt add A 0 0", "pt add B 10 0", "pt add C 10 10", "pt add D 0 10", "polygon add P A B C D"} {
		if _, err := s.Execute(cmd); err != nil {
			t.Fatal(err)
		}
	}
	before := s.Project.Clone()
	path := filepath.Join(t.TempDir(), "bad.srv")
	if err := os.WriteFile(path, []byte("broken"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, cmd := range []string{"pt edit B east=-10 north=10", "open " + path, "recover " + path} {
		if _, err := s.Execute(cmd); err == nil {
			t.Fatalf("accepted %s", cmd)
		}
		if !reflect.DeepEqual(before, s.Project) || !s.Dirty || s.Path != "" {
			t.Fatalf("failure changed session: %s", cmd)
		}
	}
	if _, err := s.Execute("undo"); err != nil || len(s.Project.Features) != 0 {
		t.Fatal("failed edits changed undo stack")
	}
}
