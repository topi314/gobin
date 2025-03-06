--- v1.3.1 - postgres

ALTER TABLE documents
    ALTER COLUMN version TYPE BIGINT USING EXTRACT(EPOCH FROM version);
