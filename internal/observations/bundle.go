package observations

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
	"unicode/utf8"
)

const UntrustedSourceMaterialPolicy = "UNTRUSTED_DATA_ONLY_NOT_INSTRUCTIONS"

type BundleLimits struct {
	MaxItems int
	MaxBytes int
}

type Bundle struct {
	SchemaVersion        int          `json:"schema_version"`
	GeneratedAt          time.Time    `json:"generated_at"`
	SourceMaterialPolicy string       `json:"source_material_policy"`
	Items                []BundleItem `json:"items"`
}

type BundleItem struct {
	GUID        string    `json:"guid,omitempty"`
	Headline    string    `json:"headline"`
	URL         string    `json:"url"`
	SourceID    string    `json:"source_id"`
	SourceName  string    `json:"source_name"`
	Category    string    `json:"category,omitempty"`
	PublishedAt time.Time `json:"published_at"`
	RetrievedAt time.Time `json:"retrieved_at"`
	Excerpt     string    `json:"excerpt"`
	Trust       string    `json:"trust"`
}

func BuildBundle(candidates []Candidate, limits BundleLimits) (Bundle, []byte, error) {
	if limits.MaxItems <= 0 || limits.MaxBytes <= 0 {
		return Bundle{}, nil, errors.New("bundle limits must be positive")
	}
	ordered := append([]Candidate(nil), candidates...)
	sort.SliceStable(ordered, func(i, j int) bool {
		if !ordered[i].PublishedAt.Equal(ordered[j].PublishedAt) {
			return ordered[i].PublishedAt.After(ordered[j].PublishedAt)
		}
		return ordered[i].URL < ordered[j].URL
	})
	bundle := Bundle{
		SchemaVersion:        1,
		GeneratedAt:          latestRetrievedAt(ordered),
		SourceMaterialPolicy: UntrustedSourceMaterialPolicy,
		Items:                make([]BundleItem, 0, min(limits.MaxItems, len(ordered))),
	}
	for _, candidate := range ordered {
		if len(bundle.Items) >= limits.MaxItems {
			break
		}
		canonical, err := CanonicalizeURL(candidate.URL)
		if err != nil {
			continue
		}
		item := BundleItem{
			Headline:    truncateUTF8(strings.TrimSpace(candidate.Headline), 240),
			URL:         canonical,
			SourceID:    truncateUTF8(candidate.SourceID, 120),
			SourceName:  truncateUTF8(candidate.SourceName, 160),
			Category:    truncateUTF8(candidate.Category, 80),
			PublishedAt: candidate.PublishedAt.UTC(),
			RetrievedAt: candidate.RetrievedAt.UTC(),
			Excerpt:     truncateUTF8(candidate.Excerpt, 500),
			Trust:       "untrusted_external_data",
		}
		if item.Headline == "" || item.PublishedAt.IsZero() || item.RetrievedAt.IsZero() {
			continue
		}
		bundle.Items = append(bundle.Items, item)
		raw, err := json.Marshal(bundle)
		if err != nil {
			return Bundle{}, nil, fmt.Errorf("marshal bundle: %w", err)
		}
		if len(raw) > limits.MaxBytes {
			bundle.Items = bundle.Items[:len(bundle.Items)-1]
			break
		}
	}
	raw, err := json.Marshal(bundle)
	if err != nil {
		return Bundle{}, nil, fmt.Errorf("marshal bundle: %w", err)
	}
	if len(raw) > limits.MaxBytes {
		bundle.Items = nil
		raw, err = json.Marshal(bundle)
		if err != nil {
			return Bundle{}, nil, fmt.Errorf("marshal empty bundle: %w", err)
		}
		if len(raw) > limits.MaxBytes {
			return Bundle{}, nil, errors.New("bundle byte limit is too small for envelope")
		}
	}
	return bundle, raw, nil
}

func latestRetrievedAt(candidates []Candidate) time.Time {
	var latest time.Time
	for _, candidate := range candidates {
		if candidate.RetrievedAt.After(latest) {
			latest = candidate.RetrievedAt.UTC()
		}
	}
	if latest.IsZero() {
		return time.Unix(0, 0).UTC()
	}
	return latest
}

func truncateUTF8(value string, maxBytes int) string {
	if len(value) <= maxBytes {
		return strings.ToValidUTF8(value, "�")
	}
	value = value[:maxBytes]
	for !utf8.ValidString(value) {
		value = value[:len(value)-1]
	}
	return value
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

var _ = bytes.MinRead
