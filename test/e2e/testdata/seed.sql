-- Seed data applied before pg_upgrade.
CREATE TABLE IF NOT EXISTS e2e_items (
    id   integer PRIMARY KEY,
    name text NOT NULL,
    note text NOT NULL
);

INSERT INTO e2e_items (id, name, note) VALUES
    (1, 'alpha', 'first-row'),
    (2, 'beta',  'second-row'),
    (3, 'gamma', 'third-row')
ON CONFLICT (id) DO UPDATE
SET name = EXCLUDED.name,
    note = EXCLUDED.note;
