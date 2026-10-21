CREATE TABLE editorial_cycles (
    cycle_id TEXT PRIMARY KEY CHECK (length(trim(cycle_id)) > 0),
    requested_mode TEXT NOT NULL CHECK (length(trim(requested_mode)) > 0),
    selected_mode TEXT NOT NULL DEFAULT '',
    eligibility_reason TEXT NOT NULL,
    eligible_modes TEXT NOT NULL CHECK (json_valid(eligible_modes)),
    topics TEXT NOT NULL CHECK (json_valid(topics)),
    outcome TEXT NOT NULL CHECK (outcome IN ('pending', 'published', 'deferred', 'investigated', 'dry_run', 'no_eligible_mode', 'failed')),
    publication_id TEXT,
    selected_at TEXT NOT NULL,
    completed_at TEXT
);

CREATE INDEX editorial_cycles_recent
ON editorial_cycles(selected_at DESC, cycle_id DESC);
