package memory

import (
	"context"
	"errors"
	"io/fs"
	"path/filepath"
	"sort"
	"strings"
)

type RebuiltContext struct {
	PostIDs []string
}

func RebuildFromPublic(ctx context.Context, publicRepository string) (RebuiltContext, error) {
	if strings.TrimSpace(publicRepository) == "" {
		return RebuiltContext{}, errors.New("public repository path is required")
	}
	var ids []string
	err := filepath.WalkDir(filepath.Join(publicRepository, "posts"), func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || filepath.Ext(path) != ".md" {
			return nil
		}
		id := strings.TrimSuffix(entry.Name(), ".md")
		if len(id) == 26 {
			ids = append(ids, id)
		}
		return nil
	})
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return RebuiltContext{}, err
	}
	sort.Strings(ids)
	return RebuiltContext{PostIDs: ids}, nil
}
