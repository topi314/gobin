--- v1.3.0 -> v1.4.0
CREATE TABLE IF NOT EXISTS failed_webhook_events
(
    id              VARCHAR   NOT NULL,
    webhook_id      VARCHAR   NOT NULL,
    body            TEXT      NOT NULL,
    created_at      TIMESTAMP NOT NULL DEFAULT now(),
    attempts        INT       NOT NULL DEFAULT 0,
    last_attempt_at TIMESTAMP NOT NULL DEFAULT now(),
    status          VARCHAR   NOT NULL DEFAULT 'pending',
    PRIMARY KEY (id),
    FOREIGN KEY (webhook_id) REFERENCES webhooks (id) ON DELETE CASCADE
);
--- v1.2.0 -> v1.3.0
ALTER TABLE documents DROP COLUMN update_token;
--- v1.1.0 -> v1.2.0
ALTER TABLE documents DROP COLUMN created_at;
ALTER TABLE documents DROP COLUMN updated_at;
ALTER TABLE documents DROP CONSTRAINT documents_pkey;
ALTER TABLE documents ADD COLUMN version bigint NOT NULL DEFAULT 1;
ALTER TABLE documents ADD PRIMARY KEY (version, id);
