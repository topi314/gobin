--- v2.0.0

CREATE TABLE files
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
