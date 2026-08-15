CREATE TABLE meetings (
    id VARCHAR(255) NOT NULL,
    user_id VARCHAR(255) NOT NULL,
    meeting_name VARCHAR(255) NULL,
    file_path TEXT NULL,
    original_file_name TEXT NULL,
    created_at TIMESTAMP NOT NULL,
    updated_at TIMESTAMP NOT NULL,
    PRIMARY KEY(id),
    FOREIGN KEY (user_id) REFERENCES users (id)
);