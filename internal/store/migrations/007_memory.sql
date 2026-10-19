CREATE TABLE memories (
    id TEXT PRIMARY KEY CHECK (length(trim(id)) > 0),
    kind TEXT NOT NULL CHECK (kind IN ('observation', 'open_question', 'prediction', 'correction', 'revisit', 'topic')),
    summary TEXT NOT NULL CHECK (length(trim(summary)) > 0),
    evidence_links TEXT NOT NULL DEFAULT '[]' CHECK (json_valid(evidence_links)),
    related_post_ids TEXT NOT NULL DEFAULT '[]' CHECK (json_valid(related_post_ids)),
    updated_at TEXT NOT NULL,
    source_hash TEXT NOT NULL CHECK (length(trim(source_hash)) > 0),
    commit_sha TEXT
);

CREATE INDEX memories_recent
ON memories(updated_at DESC, id ASC);

CREATE INDEX memories_kind
ON memories(kind, updated_at DESC);
