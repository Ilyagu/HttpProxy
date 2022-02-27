CREATE TABLE IF NOT EXISTS requests
(
    id      serial NOT NULL PRIMARY KEY,
    method  text   NOT NULL,
    host    text   NOT NULL,
    path    text   NOT NULL,
    headers jsonb  NOT NULL,
    body    text   NOT NULL,
    created_at TIMESTAMPTZ DEFAULT NOW()
);