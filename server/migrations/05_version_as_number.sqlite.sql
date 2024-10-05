--- v1.3.1 - sqlite

ALTER TABLE documents
    RENAME TO documents_old;

CREATE TABLE documents
(
    id           VARCHAR NOT NULL,
    version      BIGINT  NOT NULL,
    content      TEXT    NOT NULL,
    language     VARCHAR NOT NULL DEFAULT 'plaintext',
    PRIMARY KEY (version, id)
);

INSERT INTO documents (id, version, content)
SELECT id, CAST(strftime('%s', '2018-03-31 01:02:03') as BIGINT), content
FROM documents_old;

DROP TABLE documents_old;