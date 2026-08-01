CREATE TABLE users (
    id VARCHAR(255) NOT NULL,
    external_id VARCHAR(255) NULL,
    created_at TIMESTAMP NOT NULL,
    PRIMARY KEY(id)
);