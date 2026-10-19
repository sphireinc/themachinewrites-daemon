package gitpub

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
)

func VerifyChangedFiles(_ context.Context, root string, paths []string) error {
	repo := &Repository{Root: root}
	for _, path := range paths {
		absolute, err := repo.SafePath(path)
		if err != nil {
			return err
		}
		info, err := os.Lstat(absolute)
		if err != nil {
			return fmt.Errorf("verify %s: %w", path, err)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("symlink forbidden: %s", path)
		}
		if info.IsDir() {
			return fmt.Errorf("directory not allowed as file: %s", path)
		}
		if strings.HasSuffix(path, ".json") || strings.HasSuffix(path, ".md") {
			content, err := os.ReadFile(absolute)
			if err != nil || len(content) == 0 {
				return errors.New("empty or unreadable publication artifact")
			}
		}
	}
	return nil
}
