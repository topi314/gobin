--- v1.2.0

ALTER TABLE documents
    DROP COLUMN created_at;

ALTER TABLE documents
    DROP COLUMN updated_at;

ALTER TABLE documents
    RENAME TO documents_old;

CREATE TABLE documents
(
    id           VARCHAR   NOT NULL,
    version      TIMESTAMP NOT NULL,
    content      TEXT      NOT NULL,
    update_token VARCHAR   NOT NULL,
    language     VARCHAR   NOT NULL DEFAULT 'plaintext',
    PRIMARY KEY (version, id)
);

INSERT INTO documents (id, version, content, update_token)
SELECT id, current_timestamp, content, update_token
FROM documents_old;

DROP TABLE documents_old;