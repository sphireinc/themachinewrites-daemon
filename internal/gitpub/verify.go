package gitpub

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/TheMachineWrites/themachinewrites-daemon/internal/publication"
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
		if strings.HasPrefix(path, "posts/") {
			if _, err := publication.LoadCorpus(root); err != nil {
				return fmt.Errorf("verify post contract: %w", err)
			}
		}
		if strings.HasPrefix(path, "feed/") {
			var decoded map[string]any
			if err := json.Unmarshal(contentForVerify(absolute), &decoded); err != nil {
				return fmt.Errorf("verify feed JSON %s: %w", path, err)
			}
			if decoded["schema_version"] != float64(1) {
				return fmt.Errorf("unsupported feed schema in %s", path)
			}
		}
	}
	return nil
}

func contentForVerify(path string) []byte { content, _ := os.ReadFile(path); return content }
