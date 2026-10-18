CREATE TABLE observed_items (
    url_canonical_hash TEXT NOT NULL CHECK (length(trim(url_canonical_hash)) > 0),
    source_id TEXT NOT NULL,
    discovered_at TEXT NOT NULL,
    fingerprint TEXT NOT NULL UNIQUE CHECK (length(trim(fingerprint)) > 0),
    last_seen_at TEXT NOT NULL
);

CREATE INDEX observed_items_source_discovered
ON observed_items(source_id, discovered_at);
