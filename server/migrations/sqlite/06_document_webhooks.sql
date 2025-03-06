--- v1.6.0

CREATE TABLE webhooks
(
    id          VARCHAR NOT NULL,
    document_id VARCHAR NOT NULL,
    url         VARCHAR NOT NULL,
    secret      VARCHAR NOT NULL,
    events      VARCHAR NOT NULL,
    PRIMARY KEY (id)
);