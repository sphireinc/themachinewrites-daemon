package memory

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"time"

	"github.com/TheMachineWrites/themachinewrites-daemon/internal/store"
)

type RetrievalOptions struct {
	Topics   []string
	MaxItems int
	MaxBytes int
}

type RetrievedMemory struct {
	ID             string    `json:"id"`
	Kind           Kind      `json:"kind"`
	Summary        string    `json:"summary"`
	EvidenceLinks  []string  `json:"evidence_links"`
	RelatedPostIDs []string  `json:"related_post_ids"`
	UpdatedAt      time.Time `json:"updated_at"`
	SourceHash     string    `json:"source_hash"`
}

type Context struct {
	Items []RetrievedMemory
	JSON  []byte
}

func Retrieve(ctx context.Context, runtime *store.Store, options RetrievalOptions) (Context, error) {
	if options.MaxItems <= 0 || options.MaxBytes <= 0 {
		return Context{}, errors.New("retrieval limits must be positive")
	}
	all, err := List(ctx, runtime)
	if err != nil {
		return Context{}, err
	}
	topics := make(map[string]bool)
	for _, topic := range options.Topics {
		topics[strings.ToLower(topic)] = true
	}
	sort.SliceStable(all, func(i, j int) bool {
		a, b := all[i], all[j]
		aQuestion := a.Kind == KindOpenQuestion
		bQuestion := b.Kind == KindOpenQuestion
		if aQuestion != bQuestion {
			return aQuestion
		}
		aRelevant := topicRelevant(a, topics)
		bRelevant := topicRelevant(b, topics)
		if aRelevant != bRelevant {
			return aRelevant
		}
		if !a.UpdatedAt.Equal(b.UpdatedAt) {
			return a.UpdatedAt.After(b.UpdatedAt)
		}
		return a.ID < b.ID
	})
	result := Context{Items: make([]RetrievedMemory, 0, min(options.MaxItems, len(all)))}
	for _, memory := range all {
		if len(result.Items) >= options.MaxItems {
			break
		}
		item := RetrievedMemory{
			ID: memory.ID, Kind: memory.Kind, Summary: truncate(memory.Summary, 600),
			EvidenceLinks:  append([]string(nil), memory.EvidenceLinks...),
			RelatedPostIDs: append([]string(nil), memory.RelatedPostIDs...),
			UpdatedAt:      memory.UpdatedAt.UTC(), SourceHash: memory.SourceHash,
		}
		proposed := append(append([]RetrievedMemory(nil), result.Items...), item)
		raw, err := json.Marshal(struct {
			Items []RetrievedMemory `json:"items"`
		}{proposed})
		if err != nil {
			return Context{}, err
		}
		if len(raw) > options.MaxBytes {
			break
		}
		result.Items = proposed
		result.JSON = raw
	}
	if result.JSON == nil {
		result.JSON = []byte(`{"items":[]}`)
	}
	return result, nil
}

func topicRelevant(memory Memory, topics map[string]bool) bool {
	if len(topics) == 0 {
		return false
	}
	text := strings.ToLower(memory.Summary + " " + strings.Join(memory.RelatedPostIDs, " "))
	for topic := range topics {
		if strings.Contains(text, topic) {
			return true
		}
	}
	return false
}

func truncate(value string, limit int) string {
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	return string(runes[:limit])
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
