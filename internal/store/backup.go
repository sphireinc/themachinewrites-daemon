package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	sqlite "modernc.org/sqlite"
)

type onlineBackuper interface {
	NewBackup(string) (*sqlite.Backup, error)
}

func (s *Store) BackupTo(ctx context.Context, destination string) error {
	if s == nil || s.db == nil {
		return errors.New("store is not open")
	}
	if strings.TrimSpace(destination) == "" {
		return errors.New("backup destination is required")
	}
	destination, err := filepath.Abs(destination)
	if err != nil {
		return errors.New("backup destination is invalid")
	}
	if _, err := os.Lstat(destination); err == nil {
		return errors.New("backup destination already exists")
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(destination), 0o700); err != nil {
		return fmt.Errorf("create backup directory: %w", err)
	}
	temporary, err := os.CreateTemp(filepath.Dir(destination), ".backup-*.sqlite")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	if err := temporary.Close(); err != nil {
		os.Remove(temporaryPath)
		return err
	}
	os.Remove(temporaryPath)
	defer os.Remove(temporaryPath)

	conn, err := s.db.Conn(ctx)
	if err != nil {
		return fmt.Errorf("reserve database connection: %w", err)
	}
	defer conn.Close()
	if err := conn.Raw(func(driverConn any) error {
		backuper, ok := driverConn.(onlineBackuper)
		if !ok {
			return errors.New("sqlite driver does not support online backup")
		}
		backup, err := backuper.NewBackup(temporaryPath)
		if err != nil {
			return err
		}
		for more := true; more; {
			more, err = backup.Step(-1)
			if err != nil {
				_ = backup.Finish()
				return err
			}
		}
		return backup.Finish()
	}); err != nil {
		return fmt.Errorf("create SQLite online backup: %w", err)
	}
	if err := os.Chmod(temporaryPath, 0o600); err != nil {
		return err
	}
	file, err := os.Open(temporaryPath)
	if err != nil {
		return err
	}
	if err := file.Sync(); err != nil {
		file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	if err := verifyBackup(ctx, temporaryPath); err != nil {
		return err
	}
	if err := os.Rename(temporaryPath, destination); err != nil {
		return fmt.Errorf("publish backup: %w", err)
	}
	dir, err := os.Open(filepath.Dir(destination))
	if err == nil {
		err = dir.Sync()
		dir.Close()
	}
	return err
}

func verifyBackup(ctx context.Context, path string) error {
	db, err := sql.Open("sqlite", "file:"+filepath.ToSlash(path)+"?mode=ro")
	if err != nil {
		return fmt.Errorf("open backup for verification: %w", err)
	}
	defer db.Close()
	var result string
	if err := db.QueryRowContext(ctx, "PRAGMA integrity_check").Scan(&result); err != nil {
		return fmt.Errorf("verify backup integrity: %w", err)
	}
	if result != "ok" {
		return fmt.Errorf("backup integrity check failed: %s", result)
	}
	return nil
}
