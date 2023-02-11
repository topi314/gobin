CREATE TABLE IF NOT EXISTS documents
(
    id           VARCHAR PRIMARY KEY,
    version      TIMESTAMP NOT NULL,
    content      TEXT      NOT NULL,
    language     VARCHAR   NOT NULL,
    update_token VARCHAR   NOT NULL
);