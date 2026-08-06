CREATE TABLE transcriptions (
    id VARCHAR(255) NOT NULL,
    meeting_id VARCHAR(255) NOT NULL UNIQUE,
    user_id VARCHAR(255) NOT NULL,
    text TEXT NULL,
    created_at TIMESTAMP NOT NULL,
    PRIMARY KEY(id),
    FOREIGN KEY (meeting_id) REFERENCES meetings (id)
);

ALTER TABLE transcriptions ADD COLUMN search_vector tsvector
    GENERATED ALWAYS AS (to_tsvector('russian', coalesce(text, ''))) STORED;

CREATE INDEX idx_transcriptions_search ON transcriptions USING GIN (search_vector);