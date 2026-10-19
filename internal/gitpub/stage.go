package gitpub

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
)

func (r *Repository) StageExact(ctx context.Context, paths []string) error {
	if len(paths) == 0 {
		return errors.New("no publication paths supplied")
	}
	args := []string{"add", "--"}
	seen := make(map[string]bool)
	for _, path := range paths {
		if seen[path] {
			continue
		}
		seen[path] = true
		absolute, err := r.SafePath(path)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(r.Root, absolute)
		if err != nil || strings.HasPrefix(rel, "..") {
			return errors.New("path outside worktree")
		}
		args = append(args, filepath.ToSlash(rel))
	}
	_, err := r.run(ctx, args...)
	return err
}
