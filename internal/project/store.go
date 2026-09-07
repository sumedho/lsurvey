package project

import (
	"fmt"
	"os"
	"path/filepath"
)

// Store is the UI-independent project document lifecycle boundary.
type Store interface {
	Load(string) (*Project, error)
	Save(string, *Project) error
	Recover(string) (*Project, error)
}

func regularFile(path string) (os.FileInfo, error) {
	info, err := os.Lstat(path)
	if err == nil && !info.Mode().IsRegular() {
		return nil, fmt.Errorf("refusing to replace non-regular file %s", path)
	}
	return info, err
}

// installFile syncs and publishes a closed database/backup, on the same volume.
func (s SQLiteStore) installFile(temp, path string, mode os.FileMode) error {
	if _, err := regularFile(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	f, err := os.OpenFile(temp, os.O_RDWR, mode)
	if err != nil {
		return err
	}
	if err = f.Chmod(mode); err == nil {
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
