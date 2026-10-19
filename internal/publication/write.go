package publication

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func WriteRenderedPost(root string, post RenderedPost) error {
	if strings.TrimSpace(root) == "" || strings.TrimSpace(post.ID) == "" || len(post.Markdown) == 0 {
		return errors.New("published post root, ID and Markdown are required")
	}
	root, err := filepath.Abs(root)
	if err != nil {
		return errors.New("published post root is invalid")
	}
	expectedPrefix := "posts/"
	if !strings.HasPrefix(post.Path, expectedPrefix) || strings.Contains(post.Path, "\\") || filepath.IsAbs(post.Path) {
		return errors.New("rendered post path is not a repository-relative canonical path")
	}
	clean := filepath.ToSlash(filepath.Clean(post.Path))
	if clean != post.Path {
		return errors.New("rendered post path is not canonical")
	}
	absolute := filepath.Join(root, filepath.FromSlash(clean))
	relative, err := filepath.Rel(root, absolute)
	if err != nil || relative == "." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return errors.New("rendered post path escapes repository")
	}
	if info, err := os.Lstat(absolute); err == nil && info.Mode()&os.ModeSymlink != 0 {
		return errors.New("rendered post path is a symlink")
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("inspect rendered post destination: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(absolute), 0o700); err != nil {
		return fmt.Errorf("create rendered post directory: %w", err)
	}
	file, err := os.OpenFile(absolute, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return fmt.Errorf("create rendered post: %w", err)
	}
	if _, err := file.Write(post.Markdown); err != nil {
		file.Close()
		os.Remove(absolute)
		return fmt.Errorf("write rendered post: %w", err)
	}
	if err := file.Sync(); err != nil {
		file.Close()
		os.Remove(absolute)
		return fmt.Errorf("sync rendered post: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close rendered post: %w", err)
	}
	return nil
}
