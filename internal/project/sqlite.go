package project

import (
	"context"
	"database/sql"
	_ "embed"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	_ "modernc.org/sqlite"
)

const SQLiteSchemaVersion = 1
const sqliteApplicationID = 0x4c535256 // LSRV

//go:embed sqlite_schema.sql
var sqliteSchema string

// SQLiteStore stores native relational data. Save commits the current in-memory
// document in one database transaction, retaining the persisted audit prefix.
// The hooks are private fault-injection seams used by storage tests.
type SQLiteStore struct {
	replace      func(string, string) error
	beforeCommit func(*sql.Tx) error
}

func openSQLite(path, mode string) (*sql.DB, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	name := filepath.ToSlash(abs)
	if !strings.HasPrefix(name, "/") {
		name = "/" + name
	} // Windows drive-letter URI
	u := url.URL{Scheme: "file", Path: name}
	params := url.Values{"mode": {mode}, "_pragma": {"foreign_keys(1)", "busy_timeout(5000)", "synchronous(FULL)"}}
	if mode != "ro" {
		params.Set("_txlock", "immediate")
	}
	u.RawQuery = params.Encode()
	db, err := sql.Open("sqlite", u.String())
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	if err = db.Ping(); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}

type sqlReader interface {
	Query(string, ...any) (*sql.Rows, error)
	QueryRow(string, ...any) *sql.Row
}

func checkSQLite(q sqlReader) error {
	var appID, version int
	if err := q.QueryRow("PRAGMA application_id").Scan(&appID); err != nil {
		return err
	}
	if appID != sqliteApplicationID {
		return fmt.Errorf("not an lsurvey SQLite project (application_id=%d)", appID)
	}
	if err := q.QueryRow("PRAGMA user_version").Scan(&version); err != nil {
		return err
	}
	if version != SQLiteSchemaVersion {
		return fmt.Errorf("unsupported SQLite schema version %d (supported: %d)", version, SQLiteSchemaVersion)
	}
	var integrity string
	if err := q.QueryRow("PRAGMA quick_check").Scan(&integrity); err != nil {
		return err
	}
	if integrity != "ok" {
		return fmt.Errorf("database integrity: %s", integrity)
	}
	rows, err := q.Query("PRAGMA foreign_key_check")
	if err != nil {
		return err
	}
	defer rows.Close()
	if rows.Next() {
		return fmt.Errorf("database contains broken foreign key references")
	}
	return rows.Err()
}

func (s SQLiteStore) Load(path string) (*Project, error) {
	// Read legacy JSON only after SQLite has declined to open it. mode=ro never
	// creates a missing database. No legacy writer or in-place conversion exists.
	db, err := openSQLite(path, "ro")
	if err != nil {
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return nil, readErr
		}
		if strings.HasPrefix(strings.TrimSpace(string(data)), "{") {
			return decodeProject(data)
		}
		return nil, fmt.Errorf("open %s: %w; recover the previous save if available", path, err)
	}
	defer db.Close()
	tx, err := db.BeginTx(context.Background(), &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	if err = checkSQLite(tx); err != nil {
		return nil, err
	}
	p, err := readSQLiteProject(tx)
	if err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return p, nil
}

func (s SQLiteStore) Recover(path string) (*Project, error) { return s.Load(path + ".bak") }

func (s SQLiteStore) Save(path string, p *Project) error {
	if err := p.Validate(); err != nil {
		return fmt.Errorf("save: %w", err)
	}
	candidate := p.Clone()
	candidate.ensure()
	info, err := regularFile(path)
	if os.IsNotExist(err) {
		return s.create(path, candidate)
	}
	if err != nil {
		return err
	}
	db, err := openSQLite(path, "rw")
	if err != nil {
		return fmt.Errorf("existing file is not a writable SQLite project; preserve it and use saveas to a new path: %w", err)
	}
	defer db.Close()
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err = checkSQLite(tx); err != nil {
		return err
	}
	previous, err := readSQLiteProject(tx)
	if err != nil {
		return fmt.Errorf("existing project is invalid; use saveas to a new path: %w", err)
	}
	if err = checkAuditPrefix(previous.History, candidate.History); err != nil {
		return err
	}
	// BEGIN IMMEDIATE prevents other writers between this snapshot and commit.
	// VACUUM INTO reads a consistent snapshot including any WAL content; copying
	// the main file with os.ReadFile would not provide that guarantee.
	if err = s.backup(path, info.Mode().Perm()); err != nil {
		return fmt.Errorf("backup: %w", err)
	}
	if err = writeSQLiteProject(tx, candidate, len(previous.History)); err != nil {
		return err
	}
	if s.beforeCommit != nil {
		if err = s.beforeCommit(tx); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s SQLiteStore) create(path string, p *Project) error {
	f, err := os.CreateTemp(filepath.Dir(path), ".lsurvey-save-*")
	if err != nil {
		return err
	}
	temp := f.Name()
	if err = f.Close(); err != nil {
		return err
	}
	defer os.Remove(temp)
	db, err := openSQLite(temp, "rw")
	if err != nil {
		return err
	}
	err = func() error {
		tx, err := db.Begin()
		if err != nil {
			return err
		}
		defer tx.Rollback()
		if _, err = tx.Exec(sqliteSchema); err != nil {
			return err
		}
		if _, err = tx.Exec(fmt.Sprintf("PRAGMA application_id=%d; PRAGMA user_version=%d", sqliteApplicationID, SQLiteSchemaVersion)); err != nil {
			return err
		}
		if err = writeSQLiteProject(tx, p, 0); err != nil {
			return err
		}
		if s.beforeCommit != nil {
			if err = s.beforeCommit(tx); err != nil {
				return err
			}
		}
		return tx.Commit()
	}()
	closeErr := db.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	if _, err = os.Lstat(path); !os.IsNotExist(err) {
		return fmt.Errorf("destination appeared during save: %s", path)
	}
	return s.installFile(temp, path, 0600)
}

func (s SQLiteStore) backup(path string, mode os.FileMode) error {
	f, err := os.CreateTemp(filepath.Dir(path), ".lsurvey-save-*")
	if err != nil {
		return err
	}
	temp := f.Name()
	if err = f.Close(); err != nil {
		return err
	}
	defer os.Remove(temp)
	db, err := openSQLite(path, "ro")
	if err != nil {
		return err
	}
	_, err = db.Exec("VACUUM INTO ?", temp)
	closeErr := db.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	return s.installFile(temp, path+".bak", mode)
}
