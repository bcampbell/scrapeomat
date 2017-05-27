DROP TABLE IF EXISTS article_url CASCADE;
DROP TABLE IF EXISTS article_keyword CASCADE;
DROP TABLE IF EXISTS author_attr CASCADE;
DROP TABLE IF EXISTS article CASCADE;
DROP TABLE IF EXISTS author CASCADE;
DROP TABLE IF EXISTS publication CASCADE;

CREATE TABLE publication (
    id SERIAL PRIMARY KEY,
    code TEXT NOT NULL,
    name TEXT NOT NULL DEFAULT '',
    domain TEXT NOT NULL DEFAULT ''
);
CREATE INDEX ON publication(id);
CREATE INDEX ON publication(code);

CREATE TABLE article (
    id SERIAL PRIMARY KEY,
    added TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    canonical_url TEXT NOT NULL,
    headline TEXT NOT NULL,
    content TEXT NOT NULL DEFAULT '',
    published TIMESTAMP WITH TIME ZONE DEFAULT NULL,
    updated TIMESTAMP WITH TIME ZONE DEFAULT NULL,
    publication_id INT NOT NULL REFERENCES publication (id),
    section TEXT NOT NULL DEFAULT '',
    extra TEXT NOT NULL DEFAULT ''
);
CREATE INDEX ON article(id);
CREATE INDEX ON article(published);
CREATE INDEX ON article(publication_id);

CREATE TABLE author (
    id SERIAL PRIMARY KEY,
    name TEXT NOT NULL,
    rel_link TEXT NOT NULL DEFAULT '',
    email TEXT NOT NULL DEFAULT '',
    twitter TEXT NOT NULL DEFAULT ''
);
CREATE INDEX ON author(id);

CREATE TABLE author_attr (
    id SERIAL PRIMARY KEY,
    author_id INT NOT NULL REFERENCES author (id) ON DELETE CASCADE,
    article_id INT NOT NULL REFERENCES article (id) ON DELETE CASCADE
);
CREATE INDEX ON author_attr(author_id);
CREATE INDEX ON author_attr(article_id);

CREATE TABLE article_url (
    id SERIAL PRIMARY KEY,
    url TEXT NOT NULL,
    article_id INT NOT NULL REFERENCES article (id) ON DELETE CASCADE
);
CREATE INDEX ON article_url(id);
CREATE INDEX ON article_url(article_id);
CREATE INDEX ON article_url(url);


CREATE TABLE article_keyword (
    id SERIAL PRIMARY KEY,
    article_id INT NOT NULL REFERENCES article (id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    url TEXT NOT NULL DEFAULT ''
);
CREATE INDEX ON article_keyword(id);
CREATE INDEX ON article_keyword(article_id);
CREATE INDEX ON article_keyword(name);




