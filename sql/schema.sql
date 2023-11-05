CREATE TABLE IF NOT EXISTS documents
(
    id       VARCHAR NOT NULL,
    version  BIGINT  NOT NULL,
    content  TEXT    NOT NULL,
    language VARCHAR NOT NULL,
    PRIMARY KEY (id, version)
);

CREATE TABLE IF NOT EXISTS webhooks
(
    id          BIGSERIAL NOT NULL,
    document_id VARCHAR   NOT NULL REFERENCES documents (id) ON DELETE CASCADE,
    url         VARCHAR   NOT NULL,
    secret      VARCHAR   NOT NULL,
    events      VARCHAR   NOT NULL,
    PRIMARY KEY (id)
);

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
