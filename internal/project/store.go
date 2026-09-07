package project

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Store is the document lifecycle boundary. It does not prescribe a schema,
// encoding, backup filename, or transaction implementation. A future relational
// SQLite store can replace JSONStore; incremental editing can extend this boundary.
type Store interface {
	Load(string) (*Project, error)
	Save(string, *Project) error
	Recover(string) (*Project, error)
}

// JSONStore is temporary persistence, not the schema for the future database.
// It keeps one previous valid save. It does not autosave unsaved edits.
type JSONStore struct{ replace func(string, string) error }

func (s JSONStore) Load(path string) (*Project, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	p, err := decodeProject(data)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w (use recover with the original project path if a backup exists)", path, err)
	}
	return p, nil
}

func decodeProject(data []byte) (*Project, error) {
	var p Project
	if err := json.Unmarshal(data, &p); err != nil {
		return nil, err
	}
	if p.SchemaVersion < 1 || p.SchemaVersion > CurrentSchemaVersion {
		return nil, fmt.Errorf("unsupported schema_version %d", p.SchemaVersion)
	}
	// Reject invalid precision before legacy defaults can normalise it.
	if p.Display.Precision != nil && (*p.Display.Precision < 0 || *p.Display.Precision > 6) {
		return nil, fmt.Errorf("invalid display precision")
	}
	p.ensure()
	if err := p.Validate(); err != nil {
		return nil, err
	}
	return &p, nil
}

func (s JSONStore) Recover(path string) (*Project, error) { return s.Load(path + ".bak") }

func regularFile(path string) (os.FileInfo, error) {
	info, err := os.Lstat(path)
	if err == nil && !info.Mode().IsRegular() {
		return nil, fmt.Errorf("refusing to replace non-regular file %s", path)
	}
	return info, err
}

func (s JSONStore) Save(path string, p *Project) error {
	if err := p.Validate(); err != nil {
		return fmt.Errorf("save: %w", err)
	}
	copy := p.Clone()
	copy.ensure()
	data, err := json.MarshalIndent(copy, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	mode := os.FileMode(0600)
	info, err := regularFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if err == nil {
		mode = info.Mode().Perm()
		old, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if _, err = decodeProject(old); err != nil {
			return fmt.Errorf("existing project is invalid; preserve it and use saveas to a new path: %w", err)
		}
		if err = s.writeAtomic(path+".bak", old, mode); err != nil {
			return fmt.Errorf("backup: %w", err)
		}
	}
	return s.writeAtomic(path, data, mode)
}

func (s JSONStore) writeAtomic(path string, data []byte, mode os.FileMode) error {
	if _, err := regularFile(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".lsurvey-save-*")
	if err != nil {
		return err
	}
	temp := f.Name()
	defer os.Remove(temp)
	if err = f.Chmod(mode); err == nil {
		_, err = f.Write(data)
	}
	if err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	replace := s.replace
	if replace == nil {
		replace = replaceFile
	}
	if err = replace(temp, path); err != nil {
		return err
	}
	return syncDirectory(filepath.Dir(path))
}
