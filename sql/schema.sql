CREATE TABLE IF NOT EXISTS documents
(
    id VARCHAR NOT NULL,
    PRIMARY KEY (id)
);

CREATE TABLE IF NOT EXISTS files
(
    id          VARCHAR NOT NULL,
    document_id VARCHAR NOT NULL REFERENCES documents (id) ON DELETE CASCADE,
    name        VARCHAR NOT NULL,
    version     BIGINT  NOT NULL,
    content     TEXT    NOT NULL,
    language    VARCHAR NOT NULL,
    PRIMARY KEY (id),
    UNIQUE (document_id, name, version)
);

CREATE TABLE IF NOT EXISTS webhooks
(
    id          VARCHAR NOT NULL,
    document_id VARCHAR NOT NULL,
    url         VARCHAR NOT NULL,
    secret      VARCHAR NOT NULL,
    events      VARCHAR NOT NULL,
    PRIMARY KEY (id)
);
