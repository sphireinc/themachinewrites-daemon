package observations

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"github.com/TheMachineWrites/themachinewrites-daemon/internal/store"
)

type Candidate struct {
	URL         string
	Headline    string
	SourceID    string
	SourceName  string
	Category    string
	PublishedAt time.Time
	Excerpt     string
	RetrievedAt time.Time
}

func Ingest(ctx context.Context, runtime *store.Store, candidate Candidate) (bool, error) {
	if runtime == nil {
		return false, errors.New("runtime store is required")
	}
	canonicalURL, err := CanonicalizeURL(candidate.URL)
	if err != nil {
		return false, err
	}
	if strings.TrimSpace(candidate.Headline) == "" || strings.TrimSpace(candidate.SourceID) == "" {
		return false, errors.New("headline and source_id are required")
	}
	if candidate.PublishedAt.IsZero() || candidate.RetrievedAt.IsZero() {
		return false, errors.New("published_at and retrieved_at are required")
	}
	fingerprint, err := Fingerprint(canonicalURL, candidate.Headline, candidate.SourceName)
	if err != nil {
		return false, err
	}
	urlHash := sha256.Sum256([]byte(canonicalURL))
	return runtime.UpsertCandidate(ctx, store.Candidate{
		URLCanonicalHash: hex.EncodeToString(urlHash[:]),
		URL:              canonicalURL,
		SourceID:         candidate.SourceID,
		SourceName:       candidate.SourceName,
		Category:         candidate.Category,
		Headline:         strings.TrimSpace(candidate.Headline),
		PublishedAt:      candidate.PublishedAt.UTC().Format(time.RFC3339Nano),
		Excerpt:          candidate.Excerpt,
		DiscoveredAt:     candidate.RetrievedAt.UTC().Format(time.RFC3339Nano),
		Fingerprint:      fingerprint,
		LastSeenAt:       candidate.RetrievedAt.UTC().Format(time.RFC3339Nano),
	})
}
