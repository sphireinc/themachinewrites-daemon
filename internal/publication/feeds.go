package publication

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode/utf8"
)

type FeedCard struct {
	ID             string    `json:"id"`
	Title          string    `json:"title"`
	Type           string    `json:"type"`
	Excerpt        string    `json:"excerpt"`
	PublishedAt    time.Time `json:"published_at"`
	Topics         []string  `json:"topics"`
	EditorialMode  string    `json:"editorial_mode,omitempty"`
	PrimaryTopic   string    `json:"primary_topic,omitempty"`
	ReadingMinutes int       `json:"reading_minutes"`
	RelatedCount   int       `json:"related_count"`
}

type LatestFeed struct {
	SchemaVersion int        `json:"schema_version"`
	GeneratedAt   time.Time  `json:"generated_at"`
	Posts         []FeedCard `json:"posts"`
}

type MonthFeed struct {
	SchemaVersion int        `json:"schema_version"`
	GeneratedAt   time.Time  `json:"generated_at"`
	Month         string     `json:"month"`
	Total         int        `json:"total"`
	Posts         []FeedCard `json:"posts"`
}

type MonthCount struct {
	Month string `json:"month"`
	Count int    `json:"count"`
}

type Catalog struct {
	SchemaVersion int          `json:"schema_version"`
	GeneratedAt   time.Time    `json:"generated_at"`
	Months        []MonthCount `json:"months"`
}

type TypeCounts struct {
	Observation int `json:"observation"`
	Analysis    int `json:"analysis"`
	Question    int `json:"question"`
	Prediction  int `json:"prediction"`
	Reflection  int `json:"reflection"`
	Correction  int `json:"correction"`
}

type Stats struct {
	SchemaVersion   int        `json:"schema_version"`
	GeneratedAt     time.Time  `json:"generated_at"`
	TotalPosts      int        `json:"total_posts"`
	LastPublishedAt *time.Time `json:"last_published_at"`
	Last7Days       int        `json:"last_7_days"`
	Last30Days      int        `json:"last_30_days"`
	ByType          TypeCounts `json:"by_type"`
}

type Relations struct {
	SchemaVersion int                 `json:"schema_version"`
	GeneratedAt   time.Time           `json:"generated_at"`
	Relations     map[string][]string `json:"relations"`
}

func BuildLatest(posts []PostRecord, generatedAt time.Time) LatestFeed {
	ordered := append([]PostRecord(nil), posts...)
	sortPosts(ordered)
	if len(ordered) > 50 {
		ordered = ordered[:50]
	}
	return LatestFeed{SchemaVersion: 1, GeneratedAt: generatedAt.UTC(), Posts: cards(ordered)}
}

func BuildMonth(posts []PostRecord, month string, generatedAt time.Time) (MonthFeed, error) {
	if _, err := time.Parse("2006-01", month); err != nil {
		return MonthFeed{}, errors.New("month must be YYYY-MM")
	}
	var selected []PostRecord
	for _, post := range posts {
		if post.PublishedAt.UTC().Format("2006-01") == month {
			selected = append(selected, post)
		}
	}
	sortPosts(selected)
	return MonthFeed{SchemaVersion: 1, GeneratedAt: generatedAt.UTC(), Month: month, Total: len(selected), Posts: cards(selected)}, nil
}

func BuildCatalogAndStats(posts []PostRecord, now time.Time) (Catalog, Stats, error) {
	sortPosts(posts)
	counts := make(map[string]int)
	stats := Stats{SchemaVersion: 1, GeneratedAt: now.UTC(), TotalPosts: len(posts)}
	for _, post := range posts {
		counts[post.PublishedAt.UTC().Format("2006-01")]++
		delta := now.Sub(post.PublishedAt)
		if delta <= 7*24*time.Hour && delta >= 0 {
			stats.Last7Days++
		}
		if delta <= 30*24*time.Hour && delta >= 0 {
			stats.Last30Days++
		}
		switch post.Type {
		case "observation":
			stats.ByType.Observation++
		case "analysis":
			stats.ByType.Analysis++
		case "question":
			stats.ByType.Question++
		case "prediction":
			stats.ByType.Prediction++
		case "reflection":
			stats.ByType.Reflection++
		case "correction":
			stats.ByType.Correction++
		default:
			return Catalog{}, Stats{}, fmt.Errorf("unsupported post type %q", post.Type)
		}
	}
	if len(posts) > 0 {
		last := posts[0].PublishedAt.UTC()
		stats.LastPublishedAt = &last
	}
	months := make([]string, 0, len(counts))
	for month := range counts {
		months = append(months, month)
	}
	sort.Sort(sort.Reverse(sort.StringSlice(months)))
	catalog := Catalog{SchemaVersion: 1, GeneratedAt: now.UTC(), Months: make([]MonthCount, 0, len(months))}
	for _, month := range months {
		catalog.Months = append(catalog.Months, MonthCount{Month: month, Count: counts[month]})
	}
	return catalog, stats, nil
}

func BuildRelations(posts []PostRecord, generatedAt time.Time) Relations {
	relations := make(map[string][]string)
	for _, post := range posts {
		for _, related := range post.RelatedIDs {
			relations[related] = append(relations[related], post.ID)
		}
		if post.CorrectionOf != "" {
			relations[post.CorrectionOf] = append(relations[post.CorrectionOf], post.ID)
		}
	}
	for key, values := range relations {
		sort.Strings(values)
		relations[key] = unique(values)
	}
	return Relations{SchemaVersion: 1, GeneratedAt: generatedAt.UTC(), Relations: relations}
}

func cards(posts []PostRecord) []FeedCard {
	result := make([]FeedCard, 0, len(posts))
	for _, post := range posts {
		result = append(result, FeedCard{
			ID: post.ID, Title: post.Title, Type: post.Type, Excerpt: plainExcerpt(post.Body),
			PublishedAt: post.PublishedAt.UTC(), Topics: append([]string(nil), post.Topics...),
			EditorialMode: post.EditorialMode, PrimaryTopic: post.PrimaryTopic,
			ReadingMinutes: max(1, len(strings.Fields(plainText(post.Body)))/220), RelatedCount: len(unique(append(append([]string(nil), post.RelatedIDs...), post.CorrectionOf))),
		})
	}
	return result
}

func sortPosts(posts []PostRecord) {
	sort.Slice(posts, func(i, j int) bool {
		if !posts[i].PublishedAt.Equal(posts[j].PublishedAt) {
			return posts[i].PublishedAt.After(posts[j].PublishedAt)
		}
		return posts[i].ID > posts[j].ID
	})
}

var markdownMarkup = regexp.MustCompile(`(?m)(?:[#*_>\-]|\\([\[\]()]))`)

func plainText(body string) string {
	value := strings.ReplaceAll(body, "\n", " ")
	value = markdownMarkup.ReplaceAllString(value, " ")
	return strings.Join(strings.Fields(value), " ")
}

func plainExcerpt(body string) string {
	value := plainText(body)
	runes := []rune(value)
	if len(runes) > 500 {
		runes = runes[:500]
	}
	return string(runes)
}

func unique(values []string) []string {
	seen := make(map[string]bool)
	result := make([]string, 0, len(values))
	for _, value := range values {
		if value != "" && !seen[value] {
			seen[value] = true
			result = append(result, value)
		}
	}
	return result
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func MarshalFeed(value any) ([]byte, error) { return json.Marshal(value) }

func MaterializeFeeds(root string, posts []PostRecord, now time.Time) (map[string]struct{}, error) {
	latest := BuildLatest(posts, now)
	catalog, stats, err := BuildCatalogAndStats(posts, now)
	if err != nil {
		return nil, err
	}
	relations := BuildRelations(posts, now)
	values := map[string]any{
		"feed/latest.json":    latest,
		"feed/catalog.json":   catalog,
		"feed/stats.json":     stats,
		"feed/relations.json": relations,
	}
	for _, month := range catalog.Months {
		feed, err := BuildMonth(posts, month.Month, now)
		if err != nil {
			return nil, err
		}
		values["feed/months/"+month.Month+".json"] = feed
	}
	paths := make(map[string]struct{}, len(values))
	for path, value := range values {
		raw, err := json.MarshalIndent(value, "", "  ")
		if err != nil {
			return nil, err
		}
		raw = append(raw, '\n')
		absolute := filepath.Join(root, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(absolute), 0o700); err != nil {
			return nil, err
		}
		if err := os.WriteFile(absolute, raw, 0o600); err != nil {
			return nil, err
		}
		paths[path] = struct{}{}
	}
	monthDir := filepath.Join(root, "feed", "months")
	entries, err := os.ReadDir(monthDir)
	if err == nil {
		for _, entry := range entries {
			path := "feed/months/" + entry.Name()
			if _, keep := paths[path]; !keep {
				if err := os.Remove(filepath.Join(monthDir, entry.Name())); err != nil {
					return nil, err
				}
			}
		}
	}
	return paths, nil
}

var _ = utf8.RuneSelf
