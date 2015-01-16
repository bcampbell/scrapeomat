-- add section column to article
BEGIN;
ALTER TABLE article ADD COLUMN section TEXT NOT NULL DEFAULT '';
COMMIT;
