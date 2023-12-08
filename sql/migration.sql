--- v1.3.0 -> v1.4.0
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
ALTER TABLE documents DROP COLUMN update_token;
--- v1.1.0 -> v1.2.0
ALTER TABLE documents DROP COLUMN created_at;
ALTER TABLE documents DROP COLUMN updated_at;
ALTER TABLE documents DROP CONSTRAINT documents_pkey;
ALTER TABLE documents ADD COLUMN version bigint NOT NULL DEFAULT 1;
ALTER TABLE documents ADD PRIMARY KEY (version, id);
