package store

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	_ "modernc.org/sqlite"
)

const CurrentSchemaVersion = 5

type Row = sql.Row

type Store struct {
	db *sql.DB
}

type Migration struct {
	Version int
	Name    string
	SQL     string
}

//go:embed migrations/*.sql
var migrationFiles embed.FS

var migrations []Migration

func Open(ctx context.Context, path string) (*Store, error) {
	if strings.TrimSpace(path) == "" {
		return nil, errors.New("database path is required")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("create database directory: %w", err)
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	db.SetMaxOpenConns(1)
	store := &Store{db: db}
	cleanup := func(cause error) (*Store, error) {
		_ = db.Close()
		return nil, cause
	}
	for _, pragma := range []string{
		"PRAGMA busy_timeout = 5000",
		"PRAGMA foreign_keys = ON",
		"PRAGMA journal_mode = WAL",
		"PRAGMA synchronous = FULL",
	} {
		if _, err := db.ExecContext(ctx, pragma); err != nil {
			return cleanup(fmt.Errorf("configure sqlite: %w", err))
		}
	}
	if err := store.migrate(ctx, migrations); err != nil {
		return cleanup(err)
	}
	return store, nil
}

func (s *Store) DB() *sql.DB {
	return s.db
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) migrate(ctx context.Context, list []Migration) error {
	ordered := append([]Migration(nil), list...)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].Version < ordered[j].Version })
	for index, migration := range ordered {
		if migration.Version != index+1 || strings.TrimSpace(migration.SQL) == "" {
			return fmt.Errorf("invalid migration sequence at version %d", migration.Version)
		}
	}

	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return fmt.Errorf("begin migration: %w", err)
	}
	defer tx.Rollback()

	var current int
	if err := tx.QueryRowContext(ctx, "PRAGMA user_version").Scan(&current); err != nil {
		return fmt.Errorf("read schema version: %w", err)
	}
	if current > len(ordered) {
		return fmt.Errorf("database schema version %d is newer than supported version %d", current, len(ordered))
	}
	for _, migration := range ordered[current:] {
		if _, err := tx.ExecContext(ctx, migration.SQL); err != nil {
			return fmt.Errorf("migration %d %s: %w", migration.Version, migration.Name, err)
		}
		if _, err := tx.ExecContext(ctx, fmt.Sprintf("PRAGMA user_version = %d", migration.Version)); err != nil {
			return fmt.Errorf("set migration version %d: %w", migration.Version, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit migrations: %w", err)
	}
	return nil
}

func readMigrations(files fs.FS) ([]Migration, error) {
	entries, err := fs.ReadDir(files, "migrations")
	if err != nil {
		return nil, fmt.Errorf("read migration directory: %w", err)
	}
	result := make([]Migration, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}
		prefix, name, found := strings.Cut(entry.Name(), "_")
		if !found {
			return nil, fmt.Errorf("migration filename lacks version: %s", entry.Name())
		}
		var fileVersion int
		if _, err := fmt.Sscanf(prefix, "%d", &fileVersion); err != nil || fileVersion < 0 {
			return nil, fmt.Errorf("invalid migration filename: %s", entry.Name())
		}
		content, err := fs.ReadFile(files, "migrations/"+entry.Name())
		if err != nil {
			return nil, fmt.Errorf("read migration %s: %w", entry.Name(), err)
		}
		result = append(result, Migration{
			Version: fileVersion + 1,
			Name:    strings.TrimSuffix(name, ".sql"),
			SQL:     string(content),
		})
	}
	return result, nil
}

func init() {
	loaded, err := readMigrations(migrationFiles)
	if err != nil {
		panic(err)
	}
	migrations = loaded
}
