CREATE TABLE ingestion_cursors (
    source_id TEXT PRIMARY KEY CHECK (length(trim(source_id)) > 0),
    etag TEXT,
    last_modified TEXT,
    last_checked_at TEXT NOT NULL,
    error_count INTEGER NOT NULL DEFAULT 0 CHECK (error_count >= 0)
);
