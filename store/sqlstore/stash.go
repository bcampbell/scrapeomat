package sqlstore

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/bcampbell/scrapeomat/store"
)

// Stash adds or updates articles in the database.
// If the article has an ID, it's assumed to be an update.
// If it doesn't, then it's an add.
//
// Returns a list of article IDs, one per input article.
func (ss *SQLStore) Stash(arts ...*store.Article) ([]int, error) {
	var err error
	var tx *sql.Tx
	tx, err = ss.db.Begin()
	if err != nil {
		return nil, err
	}

	defer func() {
		if err == nil {
			tx.Commit()
		} else {
			tx.Rollback()
		}
	}()

	ids := make([]int, 0, len(arts))
	for _, art := range arts {
		var artID int
		artID, err := ss.stashArticle(tx, art)
		if err != nil {
			return nil, err
		}
		ids = append(ids, artID)
	}
	return ids, nil
}

func (ss *SQLStore) stashArticle(tx *sql.Tx, art *store.Article) (int, error) {
	pubID, err := ss.findOrCreatePublication(tx, &art.Publication)
	if err != nil {
		return 0, err
	}

	artID := art.ID

	extra := []byte{}
	if art.Extra != nil {
		extra, err = json.Marshal(art.Extra)
		if err != nil {
			return 0, err
		}
	}

	if artID == 0 {
		// it's a new article
		artID, err = ss.insertArticle(tx, art, pubID, extra)
		if err != nil {
			return 0, err
		}
	} else {
		// updating an existing article

		q := `UPDATE article SET (canonical_url, headline, content, published, updated, publication_id, section,extra,added) = (?,?,?,?,?,?,?,?,` + ss.nowSQL() + `) WHERE id=?`
		_, err = tx.Exec(ss.rebind(q),
			art.CanonicalURL,
			art.Headline,
			art.Content,
			ss.cvtTime(art.Published),
			ss.cvtTime(art.Updated),
			pubID,
			art.Section,
			extra,
			artID)
		if err != nil {
			return 0, err
		}

		// delete old urls
		_, err = tx.Exec(ss.rebind(`DELETE FROM article_url WHERE article_id=?`), artID)
		if err != nil {
			return 0, err
		}

		// delete old keywords
		_, err = tx.Exec(ss.rebind(`DELETE FROM article_keyword WHERE article_id=?`), artID)
		if err != nil {
			return 0, err
		}

		// delete old authors
		_, err = tx.Exec(ss.rebind(`DELETE FROM author WHERE id IN (SELECT author_id FROM author_attr WHERE article_id=?)`), artID)
		if err != nil {
			return 0, err
		}
		_, err = tx.Exec(ss.rebind(`DELETE FROM author_attr WHERE article_id=?`), artID)
		if err != nil {
			return 0, err
		}
	}

	for _, u := range art.URLs {
		_, err = tx.Exec(ss.rebind(`INSERT INTO article_url(article_id,url) VALUES(?,?)`), artID, u)
		if err != nil {
			return 0, fmt.Errorf("failed adding url %s: %s", u, err)
		}
	}

	for _, k := range art.Keywords {
		_, err = tx.Exec(ss.rebind(`INSERT INTO article_keyword(article_id,name,url) VALUES(?,?,?)`),
			artID,
			k.Name,
			k.URL)
		if err != nil {
			return 0, fmt.Errorf("failed adding keyword %s (%s): %s", k.Name, k.URL, err)
		}
	}

	for _, author := range art.Authors {
		var authorID int
		result, err := tx.Exec(ss.rebind(`INSERT INTO author(name,rel_link,email,twitter) VALUES (?,?,?,?)`),
			author.Name,
			author.RelLink,
			author.Email,
			author.Twitter)
		if err != nil {
			return 0, err
		}
		// TODO: LastInsertId() not supported on postgres
		tmpID, err := result.LastInsertId()
		if err != nil {
			return 0, err
		}
		authorID = int(tmpID)

		_, err = tx.Exec(ss.rebind(`INSERT INTO author_attr(author_id,article_id) VALUES (?,?)`),
			authorID,
			artID)
		if err != nil {
			return 0, err
		}
	}

	// all good.
	return artID, nil
}

func (ss *SQLStore) insertArticle(tx *sql.Tx, art *store.Article, pubID int, extra []byte) (int, error) {
	switch ss.insertIDType() {
	case RESULT:
		{
			q := `INSERT INTO article(canonical_url, headline, content, published, updated, publication_id, section,extra) VALUES(?,?,?,?,?,?,?,?)`
			result, err := tx.Exec(ss.rebind(q),
				art.CanonicalURL,
				art.Headline,
				art.Content,
				ss.cvtTime(art.Published),
				ss.cvtTime(art.Updated),
				pubID,
				art.Section,
				extra)
			if err != nil {
				return 0, err
			}
			tmpID, err := result.LastInsertId()
			if err != nil {
				return 0, err
			}
			return int(tmpID), nil
		}
	case RETURNING:
		{
			var lastID int
			q := `INSERT INTO article(canonical_url, headline, content, published, updated, publication_id, section,extra) VALUES(?,?,?,?,?,?,?,?) RETURNING id`
			err := tx.QueryRow(ss.rebind(q),
				art.CanonicalURL,
				art.Headline,
				art.Content,
				ss.cvtTime(art.Published),
				ss.cvtTime(art.Updated),
				pubID,
				art.Section,
				extra).Scan(&lastID)
			if err != nil {
				return 0, err
			}
			return lastID, nil
		}
	default:
		return 0, errors.New("unsupported db driver")
	}
}

func (ss *SQLStore) findOrCreatePublication(tx *sql.Tx, pub *store.Publication) (int, error) {
	pubID, err := ss.findPublication(tx, pub)
	if err != nil {
		return 0, err
	}
	if pubID != 0 {
		return pubID, nil
	}
	return ss.insertPublication(tx, pub)
}

// returns 0 if no match
func (ss *SQLStore) findPublication(tx *sql.Tx, pub *store.Publication) (int, error) {
	var pubID int
	var err error

	if pub.Code != "" {

		err = tx.QueryRow(ss.rebind(`SELECT id FROM publication WHERE code=?`), pub.Code).Scan(&pubID)
		if err == nil {
			return pubID, nil // return existing publication
		}
		if err != sql.ErrNoRows {
			return 0, err
		}
	}

	if pub.Name != "" {

		err = tx.QueryRow(ss.rebind(`SELECT id FROM publication WHERE name=?`), pub.Name).Scan(&pubID)
		if err == nil {
			return pubID, nil // return existing publication
		}
		if err != sql.ErrNoRows {
			return 0, err
		}
	}

	// TODO: publications can have multiple domains...
	if pub.Domain != "" {
		err = tx.QueryRow(ss.rebind(`SELECT id FROM publication WHERE domain=?`), pub.Domain).Scan(&pubID)
		if err == nil {
			return pubID, nil // return existing publication
		}
		if err != sql.ErrNoRows {
			return 0, err
		}
	}

	return 0, nil // no match
}

func (ss *SQLStore) insertPublication(tx *sql.Tx, pub *store.Publication) (int, error) {
	switch ss.insertIDType() {
	case RESULT: // sqlite, mysql...
		{
			result, err := tx.Exec(ss.rebind(`INSERT INTO publication(code,name,domain) VALUES(?,?,?)`),
				pub.Code,
				pub.Name,
				pub.Domain)
			if err != nil {
				return 0, err
			}
			pubID, err := result.LastInsertId()
			if err != nil {
				return 0, err
			}
			return int(pubID), nil
		}
	case RETURNING: // postgresql
		{
			var lastID int
			err := tx.QueryRow(ss.rebind(`INSERT INTO publication(code,name,domain) VALUES(?,?,?) RETURNING id`),
				pub.Code,
				pub.Name,
				pub.Domain).Scan(&lastID)
			if err != nil {
				return 0, err
			}
			return lastID, nil
		}
	default:
		return 0, errors.New("unsupported db driver")
	}

}
