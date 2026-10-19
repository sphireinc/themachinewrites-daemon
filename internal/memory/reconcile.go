package memory

import (
	"context"
	"errors"
	"strings"

	"github.com/TheMachineWrites/themachinewrites-daemon/internal/store"
)

func Reconcile(ctx context.Context, runtime *store.Store, postID, remoteCommitSHA string) error {
	if strings.TrimSpace(postID) == "" || strings.TrimSpace(remoteCommitSHA) == "" {
		return errors.New("post ID and confirmed remote commit SHA are required")
	}
	all, err := List(ctx, runtime)
	if err != nil {
		return err
	}
	tx, err := runtime.DB().BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, memory := range all {
		for _, related := range memory.RelatedPostIDs {
			if related == postID {
				if _, err := tx.ExecContext(ctx, `UPDATE memories SET commit_sha = ? WHERE id = ?`, remoteCommitSHA, memory.ID); err != nil {
					return err
				}
				break
			}
		}
	}
	return tx.Commit()
}
