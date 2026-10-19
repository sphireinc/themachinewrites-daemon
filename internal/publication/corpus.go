package publication

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"gopkg.in/yaml.v3"
)

type Corpus struct {
	Root  string
	Posts []PostRecord
}

type PostRecord struct {
	ID              string
	Title           string
	Type            string
	PublishedAt     time.Time
	Topics          []string
	RelatedIDs      []string
	CorrectionOf    string
	Body            string
	Path            string
	Markdown        []byte
	ModelIdentifier string
	PromptVersion   string
}

var ulidPattern = regexp.MustCompile(`^[0-7][0-9A-HJKMNP-TV-Z]{25}$`)

func LoadCorpus(root string) (Corpus, error) {
	var posts []PostRecord
	postsRoot := filepath.Join(root, "posts")
	err := filepath.WalkDir(postsRoot, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			if os.IsNotExist(walkErr) {
				return nil
			}
			return walkErr
		}
		if entry.IsDir() || filepath.Ext(path) != ".md" {
			return nil
		}
		record, err := parseCanonicalPost(root, path)
		if err != nil {
			return err
		}
		posts = append(posts, record)
		return nil
	})
	if err != nil && !os.IsNotExist(err) {
		return Corpus{}, err
	}
	sort.Slice(posts, func(i, j int) bool {
		if !posts[i].PublishedAt.Equal(posts[j].PublishedAt) {
			return posts[i].PublishedAt.After(posts[j].PublishedAt)
		}
		return posts[i].ID > posts[j].ID
	})
	return Corpus{Root: root, Posts: posts}, nil
}

func parseCanonicalPost(root, path string) (PostRecord, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return PostRecord{}, err
	}
	if !utf8.Valid(content) || len(content) > 256*1024 {
		return PostRecord{}, fmt.Errorf("post %s invalid UTF-8 or oversize", path)
	}
	content = bytes.ReplaceAll(content, []byte("\r\n"), []byte("\n"))
	if !bytes.HasPrefix(content, []byte("---\n")) {
		return PostRecord{}, fmt.Errorf("post %s missing frontmatter", path)
	}
	end := bytes.Index(content[4:], []byte("\n---\n"))
	if end < 0 {
		return PostRecord{}, fmt.Errorf("post %s missing frontmatter close", path)
	}
	end += 4
	var metadata struct {
		SchemaVersion   int      `yaml:"schema_version"`
		ID              string   `yaml:"id"`
		Title           string   `yaml:"title"`
		Type            string   `yaml:"type"`
		PublishedAt     string   `yaml:"published_at"`
		Topics          []string `yaml:"topics"`
		RelatedIDs      []string `yaml:"related_ids"`
		CorrectionOf    *string  `yaml:"correction_of"`
		ModelIdentifier string   `yaml:"model_identifier"`
		PromptVersion   string   `yaml:"prompt_version"`
	}
	if err := yaml.Unmarshal(content[4:end], &metadata); err != nil {
		return PostRecord{}, fmt.Errorf("post %s: %w", path, err)
	}
	if err := ValidateSchemaVersion(metadata.SchemaVersion); err != nil {
		return PostRecord{}, err
	}
	if !ulidPattern.MatchString(metadata.ID) {
		return PostRecord{}, fmt.Errorf("post %s invalid ID", path)
	}
	publishedAt, err := time.Parse(time.RFC3339, metadata.PublishedAt)
	if err != nil {
		return PostRecord{}, fmt.Errorf("post %s invalid published_at", path)
	}
	canonical, err := CanonicalPath(metadata.ID, publishedAt)
	if err != nil {
		return PostRecord{}, err
	}
	relative, err := filepath.Rel(root, path)
	if err != nil || filepath.ToSlash(relative) != canonical {
		return PostRecord{}, fmt.Errorf("post %s is not at canonical path %s", path, canonical)
	}
	body := strings.TrimSpace(string(content[end+5:]))
	if body == "" {
		return PostRecord{}, fmt.Errorf("post %s has empty body", path)
	}
	correction := ""
	if metadata.CorrectionOf != nil {
		correction = *metadata.CorrectionOf
	}
	return PostRecord{
		ID: metadata.ID, Title: metadata.Title, Type: metadata.Type, PublishedAt: publishedAt.UTC(),
		Topics: metadata.Topics, RelatedIDs: metadata.RelatedIDs, CorrectionOf: correction,
		Body: body, Path: canonical, Markdown: content, ModelIdentifier: metadata.ModelIdentifier,
		PromptVersion: metadata.PromptVersion,
	}, nil
}
