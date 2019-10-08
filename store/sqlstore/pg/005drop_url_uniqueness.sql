-- allow multiple articles with same url...
BEGIN;
ALTER TABLE article_url DROP CONSTRAINT article_url_url_key;
COMMIT;

