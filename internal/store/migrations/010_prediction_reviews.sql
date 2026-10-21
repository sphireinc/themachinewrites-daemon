CREATE TABLE prediction_reviews (
    cycle_id TEXT NOT NULL CHECK (length(trim(cycle_id)) > 0),
    prediction_id TEXT NOT NULL CHECK (length(trim(prediction_id)) > 0),
    result TEXT NOT NULL CHECK (result IN ('supported', 'partially_supported', 'not_supported', 'unresolved')),
    assessment TEXT NOT NULL CHECK (length(trim(assessment)) BETWEEN 1 AND 2000),
    sources TEXT NOT NULL CHECK (json_valid(sources) AND json_type(sources) = 'array'),
    reviewed_at TEXT NOT NULL,
    PRIMARY KEY (cycle_id, prediction_id)
);

CREATE INDEX prediction_reviews_recent
ON prediction_reviews(reviewed_at DESC, cycle_id DESC);
