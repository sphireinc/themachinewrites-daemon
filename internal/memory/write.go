package memory

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/TheMachineWrites/themachinewrites-daemon/internal/store"
)

type Kind string

const (
	KindObservation  Kind = "observation"
	KindOpenQuestion Kind = "open_question"
	KindPrediction   Kind = "prediction"
	KindCorrection   Kind = "correction"
	KindRevisit      Kind = "revisit"
	KindTopic        Kind = "topic"
)

type Memory struct {
	ID             string
	Kind           Kind
	Summary        string
	EvidenceLinks  []string
	RelatedPostIDs []string
	UpdatedAt      time.Time
	SourceHash     string
	CommitSHA      string
}

func Write(ctx context.Context, runtime *store.Store, memory Memory) error {
	if err := validateMemory(memory); err != nil {
		return err
	}
	links, err := json.Marshal(memory.EvidenceLinks)
	if err != nil {
		return err
	}
	posts, err := json.Marshal(memory.RelatedPostIDs)
	if err != nil {
		return err
	}
	tx, err := runtime.DB().BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `
        INSERT INTO memories(id, kind, summary, evidence_links, related_post_ids, updated_at, source_hash, commit_sha)
        VALUES (?, ?, ?, ?, ?, ?, ?, ?)
        ON CONFLICT(id) DO UPDATE SET
            kind = excluded.kind, summary = excluded.summary,
            evidence_links = excluded.evidence_links,
            related_post_ids = excluded.related_post_ids,
            updated_at = excluded.updated_at, source_hash = excluded.source_hash,
            commit_sha = excluded.commit_sha`,
		memory.ID, memory.Kind, memory.Summary, string(links), string(posts),
		memory.UpdatedAt.UTC().Format(time.RFC3339Nano), memory.SourceHash, nullableSHA(memory.CommitSHA)); err != nil {
		return err
	}
	return tx.Commit()
}

func validateMemory(memory Memory) error {
	if strings.TrimSpace(memory.ID) == "" || !utf8.ValidString(memory.ID) {
		return errors.New("memory ID must be nonempty valid UTF-8")
	}
	switch memory.Kind {
	case KindObservation, KindOpenQuestion, KindPrediction, KindCorrection, KindRevisit, KindTopic:
	default:
		return errors.New("unsupported memory kind")
	}
	if !utf8.ValidString(memory.Summary) || len([]rune(memory.Summary)) < 1 || len([]rune(memory.Summary)) > 2000 {
		return errors.New("memory summary must be 1–2000 valid UTF-8 characters")
	}
	if strings.TrimSpace(memory.SourceHash) == "" || len(memory.SourceHash) > 128 {
		return errors.New("source hash is required")
	}
	for _, link := range memory.EvidenceLinks {
		if !strings.HasPrefix(link, "https://") && !strings.HasPrefix(link, "http://") {
			return errors.New("evidence links must be absolute HTTP(S) URLs")
		}
	}
	if memory.UpdatedAt.IsZero() {
		return errors.New("updated_at is required")
	}
	return nil
}

func nullableSHA(value string) any {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return value
}

var _ = sql.ErrNoRows
var _ = fmt.Errorf
