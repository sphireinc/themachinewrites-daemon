package memory

import (
	"context"
	"errors"
	"io/fs"
	"path/filepath"
	"sort"
	"strings"
	"time"
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
		if isULID(id) && canonicalPostPath(publicRepository, path, id) {
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

func isULID(value string) bool {
	if len(value) != 26 || value[0] < '0' || value[0] > '7' {
		return false
	}
	for _, r := range value[1:] {
		if !((r >= '0' && r <= '9') || (r >= 'A' && r <= 'H') || (r >= 'J' && r <= 'K') ||
			(r >= 'M' && r <= 'N') || (r >= 'P' && r <= 'T') || (r >= 'V' && r <= 'Z')) {
			return false
		}
	}
	return true
}

func canonicalPostPath(publicRepository, path, id string) bool {
	expectedDate := ulidDate(id)
	expected := filepath.Join(publicRepository, "posts", expectedDate[:4], expectedDate[5:7], expectedDate[8:10], id+".md")
	return filepath.Clean(path) == filepath.Clean(expected)
}

func ulidDate(id string) string {
	const alphabet = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"
	var value uint64
	for _, character := range id[:10] {
		value = value*32 + uint64(strings.IndexRune(alphabet, character))
	}
	return time.UnixMilli(int64(value)).UTC().Format("2006-01-02")
}
