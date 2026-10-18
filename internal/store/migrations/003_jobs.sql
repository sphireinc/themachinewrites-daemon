CREATE TABLE jobs (
    job_id TEXT PRIMARY KEY CHECK (length(trim(job_id)) > 0),
    state TEXT NOT NULL CHECK (state IN (
        'pending', 'leased', 'evaluating', 'validated', 'publishing',
        'published', 'deferred', 'failed', 'cancelled'
    )),
    attempts INTEGER NOT NULL DEFAULT 0 CHECK (attempts >= 0),
    lease_deadline TEXT,
    worker_id TEXT,
    candidate_path TEXT,
    post_id TEXT,
    commit_sha TEXT,
    last_error TEXT,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    CHECK (
        (state = 'leased' AND lease_deadline IS NOT NULL AND worker_id IS NOT NULL)
        OR (state <> 'leased')
    )
);

CREATE INDEX jobs_claimable
ON jobs(state, lease_deadline, created_at);

CREATE UNIQUE INDEX jobs_active_post_id
ON jobs(post_id)
WHERE post_id IS NOT NULL AND state IN ('validated', 'publishing', 'published');

CREATE TABLE publication_outbox (
    post_id TEXT PRIMARY KEY CHECK (length(trim(post_id)) > 0),
    job_id TEXT NOT NULL UNIQUE REFERENCES jobs(job_id),
    state TEXT NOT NULL DEFAULT 'pending' CHECK (state IN ('pending', 'publishing', 'published', 'failed')),
    commit_sha TEXT,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    CHECK (
        (state = 'published' AND commit_sha IS NOT NULL)
        OR state <> 'published'
    )
);

CREATE INDEX publication_outbox_retryable
ON publication_outbox(state, updated_at);
