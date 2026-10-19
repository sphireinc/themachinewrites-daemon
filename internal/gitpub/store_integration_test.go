package gitpub

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/TheMachineWrites/themachinewrites-daemon/internal/store"
)

func TestSQLiteOutboxPersistsPendingAndAcknowledgement(t *testing.T) {
	ctx := context.Background()
	runtime, err := store.Open(ctx, filepath.Join(t.TempDir(), "runtime.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Close()
	if err := runtime.SavePending(ctx, "post-one"); err != nil {
		t.Fatal(err)
	}
	if err := runtime.Acknowledge(ctx, "post-one", "remote-sha"); err != nil {
		t.Fatal(err)
	}
	if err := runtime.Acknowledge(ctx, "post-one", "remote-sha"); err != nil {
		t.Fatal(err)
	}
}
