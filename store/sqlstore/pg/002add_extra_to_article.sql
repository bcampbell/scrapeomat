-- add 'extra' column to article
BEGIN;
ALTER TABLE article ADD COLUMN extra TEXT NOT NULL DEFAULT '';
COMMIT;
