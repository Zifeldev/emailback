CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

CREATE TABLE IF NOT EXISTS emails (
    id                  uuid PRIMARY KEY,
    message_id          text UNIQUE,
    from_addr           text NOT NULL,
    to_addrs            text[] NOT NULL DEFAULT '{}',
    subject             text NOT NULL DEFAULT '',
    date                timestamptz NULL,
    body_text           text NOT NULL DEFAULT '',
    body_html           text NULL,
    language            text NULL,
    language_confidence double precision NULL,
    metrics             jsonb NOT NULL DEFAULT '{}'::jsonb,
    headers             jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at          timestamptz NOT NULL DEFAULT now(),
    raw_size            integer NOT NULL DEFAULT 0
);