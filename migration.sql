ALTER TABLE documents ADD COLUMN version TIMESTAMP NOT NULL DEFAULT now();
ALTER TABLE documents DROP CONSTRAINT documents_pkey;
ALTER TABLE documents ADD PRIMARY KEY (version, id);