--- v2.0.0 -> v2.1.0
ALTER TABLE files
    ADD COLUMN expires_at TIMESTAMP;

--- v1.6.0 -> v2.0.0
CREATE TABLE IF NOT EXISTS files
(
    name             VARCHAR NOT NULL,
    document_id      VARCHAR NOT NULL,
    document_version BIGINT  NOT NULL,
    content          TEXT    NOT NULL,
    language         VARCHAR NOT NULL,
    PRIMARY KEY (name, document_id, document_version)
);

INSERT INTO files (name, document_id, document_version, content, language)
SELECT 'untitled' as name, id as document_id, version * 1000 as document_version, content, language
FROM documents;

DROP TABLE documents;

--- v1.4.0 -> v1.6.0
CREATE TABLE IF NOT EXISTS webhooks
(
    id          VARCHAR NOT NULL,
    document_id VARCHAR NOT NULL,
    url         VARCHAR NOT NULL,
    secret      VARCHAR NOT NULL,
    events      VARCHAR NOT NULL,
    PRIMARY KEY (id)
);
--- v1.2.0 -> v1.3.0
ALTER TABLE documents
    DROP COLUMN update_token;
--- v1.1.0 -> v1.2.0
ALTER TABLE documents
    DROP COLUMN created_at,
    DROP COLUMN updated_at,
    DROP CONSTRAINT documents_pkey,
    ADD COLUMN version bigint NOT NULL DEFAULT 1,
    ADD PRIMARY KEY (version, id);
