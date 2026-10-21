package publication

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLatestFeedUsesNewestFifty(t *testing.T) {
	base := time.Date(2026, 10, 18, 20, 0, 0, 0, time.UTC)
	posts := make([]PostRecord, 0, 51)
	for index := 0; index < 51; index++ {
		id, err := GenerateID(fmt.Sprintf("feed-%d", index), base.Add(time.Duration(index)*time.Minute))
		if err != nil {
			t.Fatal(err)
		}
		posts = append(posts, PostRecord{
			ID: id, Title: fmt.Sprintf("A publication %d", index), Type: "observation",
			PublishedAt: base.Add(time.Duration(index) * time.Minute), Topics: []string{"technology"},
			Body: "Plain text body with enough words to read.", Path: "unused",
		})
	}
	latest := BuildLatest(posts, base.Add(time.Hour))
	if len(latest.Posts) != 50 || latest.Posts[0].Title != "A publication 50" || latest.Posts[49].Title != "A publication 1" {
		t.Fatalf("latest count/bounds = %d, first=%q last=%q", len(latest.Posts), latest.Posts[0].Title, latest.Posts[49].Title)
	}
	raw, err := json.Marshal(latest)
	if err != nil || len(raw) == 0 {
		t.Fatal(err)
	}
}

func TestFeedCardsExposeEditorialModeAndPrimaryTopicIndependently(t *testing.T) {
	at := time.Date(2026, 10, 18, 20, 0, 0, 0, time.UTC)
	id, err := GenerateID("feed-editorial-metadata", at)
	if err != nil {
		t.Fatal(err)
	}
	feed := BuildLatest([]PostRecord{{
		ID: id, Title: "A philosophical observation", Type: "reflection", PublishedAt: at,
		Topics: []string{"identity", "ethics"}, EditorialMode: "philosophy", PrimaryTopic: "identity",
		Body: "A careful reflection.",
	}}, at.Add(time.Minute))
	if len(feed.Posts) != 1 || feed.Posts[0].EditorialMode != "philosophy" || feed.Posts[0].PrimaryTopic != "identity" || feed.Posts[0].Topics[1] != "ethics" {
		t.Fatalf("feed card did not preserve independent editorial metadata: %#v", feed.Posts)
	}
}

func TestMaterializeFeedsWritesLatestMonthCatalogStatsAndRelations(t *testing.T) {
	root := t.TempDir()
	at := time.Date(2026, 10, 18, 20, 0, 0, 0, time.UTC)
	id, err := GenerateID("materialize", at)
	if err != nil {
		t.Fatal(err)
	}
	posts := []PostRecord{{
		ID: id, Title: "A materialized post", Type: "observation", PublishedAt: at,
		Topics: []string{"technology"}, Body: "A bounded body.", RelatedIDs: []string{}, CorrectionOf: "",
	}}
	paths, err := MaterializeFeeds(root, posts, at.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{
		"feed/latest.json", "feed/catalog.json", "feed/stats.json", "feed/relations.json", "feed/months/2026-10.json",
	} {
		if _, ok := paths[path]; !ok {
			t.Errorf("missing materialized path %s", path)
		}
		if info, err := os.Stat(filepath.Join(root, filepath.FromSlash(path))); err != nil || info.IsDir() {
			t.Errorf("artifact %s: %v", path, err)
		}
	}
}

func TestMonthCatalogStatsAndRelations(t *testing.T) {
	leap := time.Date(2024, 2, 29, 23, 59, 0, 0, time.UTC)
	march := time.Date(2024, 3, 1, 0, 0, 0, 0, time.UTC)
	old := leap.AddDate(0, 0, -31)
	posts := []PostRecord{
		{ID: "01JEXAMPLE00000000000000000", Title: "Leap post", Type: "observation", PublishedAt: leap,
			Topics: []string{"technology"}, Body: "one two three four five", RelatedIDs: []string{"01JEXAMPLE00000000000000001"}, CorrectionOf: "01JEXAMPLE00000000000000002"},
		{ID: "01JEXAMPLE00000000000000001", Title: "March post", Type: "correction", PublishedAt: march,
			Topics: []string{"society"}, Body: "one two three", CorrectionOf: "01JEXAMPLE00000000000000000"},
		{ID: "01JEXAMPLE00000000000000002", Title: "Old post", Type: "analysis", PublishedAt: old,
			Topics: []string{"technology"}, Body: "one two"},
	}
	month, err := BuildMonth(posts, "2024-02", leap.AddDate(0, 0, 1))
	if err != nil || month.Total != 1 || month.Posts[0].Title != "Leap post" {
		t.Fatalf("month = %#v, %v", month, err)
	}
	catalog, stats, err := BuildCatalogAndStats(posts, march.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if catalog.Months[0].Month != "2024-03" || catalog.Months[1].Month != "2024-02" || stats.TotalPosts != 3 {
		t.Fatalf("catalog/stats = %#v / %#v", catalog, stats)
	}
	relations := BuildRelations(posts, march.Add(time.Hour))
	if len(relations.Relations["01JEXAMPLE00000000000000000"]) != 1 ||
		len(relations.Relations["01JEXAMPLE00000000000000001"]) != 1 ||
		len(relations.Relations["01JEXAMPLE00000000000000002"]) != 1 {
		t.Fatalf("relations = %#v", relations.Relations)
	}
}
