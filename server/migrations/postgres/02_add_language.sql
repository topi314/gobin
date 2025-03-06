--- v1.1.0

ALTER TABLE documents
    ADD COLUMN language VARCHAR NOT NULL DEFAULT 'plaintext';