--- v1.0.0

CREATE TABLE documents
(
    id           VARCHAR PRIMARY KEY,
    content      TEXT      NOT NULL,
    update_token VARCHAR   NOT NULL,
    created_at   TIMESTAMP NOT NULL,
    updated_at   TIMESTAMP NOT NULL
);