CREATE TABLE transcriptions (
    id VARCHAR(255) NOT NULL,
    meeting_id VARCHAR(255) NOT NULL UNIQUE,
    user_id VARCHAR(255) NOT NULL,
    text TEXT NULL,
    created_at TIMESTAMP NOT NULL,
    PRIMARY KEY(id),
    FOREIGN KEY (meeting_id) REFERENCES meetings (id)
);