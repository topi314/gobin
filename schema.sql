CREATE TABLE documents
(
    id           VARCHAR PRIMARY KEY,
    content      TEXT      NOT NULL,
    language     VARCHAR   NOT NULL,
    update_token VARCHAR   NOT NULL,
    created_at   TIMESTAMP NOT NULL,
    updated_at   TIMESTAMP NOT NULL
);