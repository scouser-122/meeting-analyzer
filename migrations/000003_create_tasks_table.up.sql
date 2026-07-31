CREATE TABLE tasks (
    id VARCHAR(255) NOT NULL,
    meeting_id VARCHAR(255) NOT NULL,
    status VARCHAR(32) NOT NULL,
    error_message TEXT NULL,
    created_at TIMESTAMP NOT NULL,
    updated_at TIMESTAMP NOT NULL,
    PRIMARY KEY(id),
    FOREIGN KEY (meeting_id) REFERENCES meetings (id)
);