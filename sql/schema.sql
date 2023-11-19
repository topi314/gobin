CREATE TABLE IF NOT EXISTS files
(
    id          VARCHAR NOT NULL,
    document_id VARCHAR NOT NULL,
    name        VARCHAR NOT NULL,
    version     BIGINT  NOT NULL,
    content     TEXT    NOT NULL,
    language    VARCHAR NOT NULL,
    PRIMARY KEY (id, version),
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
