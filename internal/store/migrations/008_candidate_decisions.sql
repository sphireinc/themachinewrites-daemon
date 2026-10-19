ALTER TABLE observed_items ADD COLUMN decision_state TEXT NOT NULL DEFAULT 'pending'
  CHECK (decision_state IN ('pending', 'evaluating', 'deferred', 'published', 'failed'));
ALTER TABLE observed_items ADD COLUMN decision_updated_at TEXT;

CREATE INDEX observed_items_decision
ON observed_items(decision_state, published_at DESC, fingerprint ASC);
