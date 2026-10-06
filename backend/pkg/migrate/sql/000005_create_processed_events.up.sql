CREATE TABLE processed_events (
    id         VARCHAR(100) PRIMARY KEY,
    event_type VARCHAR(100) NOT NULL,
    created_at TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);
