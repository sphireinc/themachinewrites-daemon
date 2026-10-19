package research

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/url"
	"sort"
	"strings"
	"time"
)

type SourceReference struct {
	SourceID    string
	URL         string
	RetrievedAt time.Time
	Excerpt     string
}

type Claim struct {
	ID      string
	Text    string
	Sources []SourceReference
}

func NewClaim(text string, references []SourceReference) (Claim, error) {
	text = strings.Join(strings.Fields(text), " ")
	if text == "" || len([]rune(text)) > 1000 {
		return Claim{}, errors.New("claim text must contain 1-1000 characters")
	}
	if len(references) == 0 || len(references) > 50 {
		return Claim{}, errors.New("claim requires 1-50 source references")
	}
	seen := map[string]bool{}
	var normalized []SourceReference
	for _, reference := range references {
		if strings.TrimSpace(reference.SourceID) == "" || strings.TrimSpace(reference.URL) == "" || reference.RetrievedAt.IsZero() {
			return Claim{}, errors.New("claim references require source ID, URL and retrieval time")
		}
		parsed, err := url.Parse(reference.URL)
		if err != nil || parsed.Scheme != "https" || parsed.Hostname() == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" || strings.EqualFold(parsed.Hostname(), "example.invalid") {
			return Claim{}, errors.New("claim reference URL must be canonical credential-free HTTPS")
		}
		parsed.Path = parsed.EscapedPath()
		canonical := parsed.String()
		key := reference.SourceID + "\n" + canonical
		if seen[key] {
			return Claim{}, errors.New("claim contains duplicate source reference")
		}
		seen[key] = true
		reference.URL = canonical
		reference.SourceID = strings.TrimSpace(reference.SourceID)
		reference.Excerpt = strings.Join(strings.Fields(reference.Excerpt), " ")
		normalized = append(normalized, reference)
	}
	digest := sha256.Sum256([]byte(text))
	sort.Slice(normalized, func(i, j int) bool {
		if normalized[i].SourceID != normalized[j].SourceID {
			return normalized[i].SourceID < normalized[j].SourceID
		}
		return normalized[i].URL < normalized[j].URL
	})
	return Claim{ID: "claim-" + hex.EncodeToString(digest[:8]), Text: text, Sources: normalized}, nil
}

func (c Claim) IndependentSourceCount() int {
	evidence := make([]Evidence, 0, len(c.Sources))
	for _, source := range c.Sources {
		evidence = append(evidence, Evidence{SourceID: source.SourceID, Text: source.Excerpt})
	}
	count := IndependentEvidenceCount(evidence)
	if count == 0 {
		unique := map[string]bool{}
		for _, source := range c.Sources {
			unique[source.SourceID] = true
		}
		return len(unique)
	}
	return min(count, len(c.Sources))
}
