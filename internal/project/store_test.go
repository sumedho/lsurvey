package project

import (
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"
)

func TestSaveBackupAndExplicitRecovery(t *testing.T) {
	path := filepath.Join(t.TempDir(), "job.srv")
	p := New("original")
	if err := Save(path, p); err != nil {
		t.Fatal(err)
	}
	p.Name = "revised"
	if err := Save(path, p); err != nil {
		t.Fatal(err)
	}
	got, err := (SQLiteStore{}).Recover(path)
	if err != nil || got.Name != "original" {
		t.Fatalf("backup=%+v err=%v", got, err)
	}
	if err = os.WriteFile(path, []byte("corrupt"), 0600); err != nil {
		t.Fatal(err)
	}
	got, err = (SQLiteStore{}).Recover(path)
	if err != nil || got.Name != "original" {
		t.Fatalf("recovery=%+v err=%v", got, err)
	}
	if err = Save(path, p); err == nil {
		t.Fatal("overwrote corrupt original")
	}
	got, err = (SQLiteStore{}).Recover(path)
	if err != nil || got.Name != "original" {
		t.Fatal("lost recovery copy")
	}
}

func TestSaveReplacementFailurePreservesOriginalAndCleansTemps(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "job.srv")
	if err := Save(path, New("original")); err != nil {
		t.Fatal(err)
	}
	store := SQLiteStore{beforeCommit: func(*sql.Tx) error { return errors.New("injected transaction failure") }}
	if err := store.Save(path, New("revised")); err == nil {
		t.Fatal("expected failure")
	}
	got, err := Load(path)
	if err != nil || got.Name != "original" {
		t.Fatal("original lost")
	}
	files, _ := os.ReadDir(dir)
	for _, f := range files {
		if f.Name() != "job.srv" && f.Name() != "job.srv.bak" {
			t.Fatalf("temp left behind: %s", f.Name())
		}
	}
}

func TestFailedSaveDoesNotMutateProjectOrFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "job.srv")
	p := New("valid")
	if err := Save(path, p); err != nil {
		t.Fatal(err)
	}
	original, _ := os.ReadFile(path)
	p.Features["bad"] = Feature{ID: "bad", Kind: FeatureLine, PointIDs: []string{"x", "y"}}
	before := p.Clone()
	if err := Save(path, p); err == nil {
		t.Fatal("saved invalid geometry")
	}
	after, _ := os.ReadFile(path)
	if string(original) != string(after) || !reflect.DeepEqual(before, p) {
		t.Fatal("failed save mutated state")
	}
}

func TestSaveRejectsSymlinks(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target.srv")
	link := filepath.Join(dir, "link.srv")
	if err := Save(target, New("original")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, link); err != nil {
		t.Skip(err)
	}
	if err := Save(link, New("changed")); err == nil {
		t.Fatal("followed symlink")
	}
	p, err := Load(target)
	if err != nil || p.Name != "original" {
		t.Fatal("target changed")
	}
}

func TestBackupFailureDoesNotReplaceCurrentSave(t *testing.T) {
	path := filepath.Join(t.TempDir(), "job.srv")
	if err := Save(path, New("original")); err != nil {
		t.Fatal(err)
	}
	store := SQLiteStore{replace: func(string, string) error { return errors.New("backup disk failure") }}
	if err := store.Save(path, New("revised")); err == nil {
		t.Fatal("expected error")
	}
	p, err := Load(path)
	if err != nil || p.Name != "original" {
		t.Fatal("backup failure lost original")
	}
}

func TestReplacementPreservesPermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX permission semantics")
	}
	path := filepath.Join(t.TempDir(), "job.srv")
	if err := Save(path, New("original")); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0640); err != nil {
		t.Fatal(err)
	}
	if err := Save(path, New("revised")); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{path, path + ".bak"} {
		info, err := os.Stat(p)
		if err != nil || info.Mode().Perm() != 0640 {
			t.Fatalf("permissions changed: %v %v", info, err)
		}
	}
}
