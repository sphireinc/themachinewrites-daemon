CREATE TABLE schema_bootstrap (
    singleton INTEGER PRIMARY KEY CHECK (singleton = 1)
);

INSERT INTO schema_bootstrap(singleton) VALUES (1);
