ALTER TABLE observed_items ADD COLUMN inserted INTEGER NOT NULL DEFAULT 1
CHECK (inserted IN (0, 1));
