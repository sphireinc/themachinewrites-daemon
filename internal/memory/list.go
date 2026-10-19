package memory

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/TheMachineWrites/themachinewrites-daemon/internal/store"
)

func List(ctx context.Context, runtime *store.Store) ([]Memory, error) {
	rows, err := runtime.DB().QueryContext(ctx, `
        SELECT id, kind, summary, evidence_links, related_post_ids, updated_at, source_hash,
               COALESCE(commit_sha, '')
        FROM memories ORDER BY updated_at DESC, id ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []Memory
	for rows.Next() {
		var memory Memory
		var linksRaw, postsRaw, updatedAt string
		if err := rows.Scan(&memory.ID, &memory.Kind, &memory.Summary, &linksRaw, &postsRaw,
			&updatedAt, &memory.SourceHash, &memory.CommitSHA); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(linksRaw), &memory.EvidenceLinks); err != nil {
			return nil, fmt.Errorf("decode evidence links: %w", err)
		}
		if err := json.Unmarshal([]byte(postsRaw), &memory.RelatedPostIDs); err != nil {
			return nil, fmt.Errorf("decode related post IDs: %w", err)
		}
		memory.UpdatedAt, err = time.Parse(time.RFC3339Nano, updatedAt)
		if err != nil {
			return nil, fmt.Errorf("parse memory timestamp: %w", err)
		}
		result = append(result, memory)
	}
	return result, rows.Err()
}
