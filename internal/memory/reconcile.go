package memory

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/TheMachineWrites/themachinewrites-daemon/internal/store"
)

var ErrCommitConflict = errors.New("memory already confirmed at a different commit")

type PostCommitVerifier func(context.Context, string, string) (bool, error)

func Reconcile(ctx context.Context, runtime *store.Store, postID, remoteCommitSHA string, verifier PostCommitVerifier) error {
	if strings.TrimSpace(postID) == "" || strings.TrimSpace(remoteCommitSHA) == "" {
		return errors.New("post ID and confirmed remote commit SHA are required")
	}
	if verifier == nil {
		return errors.New("post-in-commit verifier is required")
	}
	verified, err := verifier(ctx, remoteCommitSHA, postID)
	if err != nil {
		return fmt.Errorf("verify post in remote commit: %w", err)
	}
	if !verified {
		return errors.New("post ID is not present at the confirmed remote commit")
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
				if memory.CommitSHA != "" && memory.CommitSHA != remoteCommitSHA {
					return ErrCommitConflict
				}
				if _, err := tx.ExecContext(ctx, `UPDATE memories SET commit_sha = ? WHERE id = ?`, remoteCommitSHA, memory.ID); err != nil {
					return err
				}
				break
			}
		}
	}
	return tx.Commit()
}
