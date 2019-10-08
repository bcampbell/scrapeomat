-- add keyword storage
BEGIN;

CREATE TABLE article_keyword (
    id SERIAL PRIMARY KEY,
    article_id INT NOT NULL REFERENCES article (id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    url TEXT NOT NULL DEFAULT ''
);
CREATE INDEX ON article_keyword(id);
CREATE INDEX ON article_keyword(article_id);
CREATE INDEX ON article_keyword(name);

COMMIT;

