ALTER TABLE documents ADD COLUMN version TIMESTAMP NOT NULL DEFAULT now();
ALTER TABLE documents DROP COLUMN created_at;
ALTER TABLE documents DROP COLUMN updated_at;
ALTER TABLE documents DROP CONSTRAINT documents_pkey;
ALTER TABLE documents ADD PRIMARY KEY (version, id);
ALTER TABLE documents DROP COLUMN version;
ALTER TABLE documents ADD COLUMN version bigint NOT NULL DEFAULT 1;