package store

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestBackupToCreatesRestorablePrivateSnapshot(t *testing.T) {
	ctx := context.Background()
	sourcePath := filepath.Join(t.TempDir(), "source.sqlite")
	runtime, err := Open(ctx, sourcePath)
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Close()
	if _, err := runtime.DB().ExecContext(ctx, "INSERT INTO memories(id,kind,summary,updated_at,source_hash) VALUES ('m1','observation','summary','2026-10-18T20:00:00Z','hash')"); err != nil {
		t.Fatal(err)
	}
	destination := filepath.Join(t.TempDir(), "backups", "runtime.sqlite")
	if err := runtime.BackupTo(ctx, destination); err != nil {
		t.Fatal(err)
	}
	if info, err := os.Stat(destination); err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("backup mode = %v, %v", info.Mode().Perm(), err)
	}
	restored, err := Open(ctx, destination)
	if err != nil {
		t.Fatal(err)
	}
	defer restored.Close()
	var count int
	if err := restored.DB().QueryRowContext(ctx, "SELECT count(*) FROM memories").Scan(&count); err != nil || count != 1 {
		t.Fatalf("restored count = %d, %v", count, err)
	}
	if err := runtime.BackupTo(ctx, destination); err == nil {
		t.Fatal("backup overwrote existing destination")
	}
}
