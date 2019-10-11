package sqlstore

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"github.com/bcampbell/scrapeomat/store"
)

type Transaction struct {
	ss  *SQLStore
	tx  *sql.Tx
	err error
}

// Transaction public interface
/*
type Txer interface {
	Close() errpr
	Stash(art *Article) int
	Err() error
}*/
func newTransaction(store *SQLStore) *Transaction {
	tx, err := store.db.Begin()
	if err != nil {
		// transaction is borked, but user can keep calling it, and
		// the error will be returned upon Close()
		return &Transaction{ss: store, tx: nil, err: err}
	} else {
		return &Transaction{ss: store, tx: tx, err: err}
	}
}

func (t *Transaction) Close() error {

	if t.tx == nil {
		return t.err
	}

	var e2 error
	if t.err == nil {
		e2 = t.tx.Commit()
	} else {
		e2 = t.tx.Rollback()
	}
	t.tx = nil // to handle double-close

	if e2 != nil {
		t.err = e2
	}

	return t.err
}

func (t *Transaction) Stash(art *store.Article) int {
	if t.err != nil {
		return 0
	}
	tx := t.tx
	ss := t.ss
	pubID, err := t.findOrCreatePublication(&art.Publication)
	if err != nil {
		t.err = err
		return 0
	}

	artID := art.ID

	extra := []byte{}
	if art.Extra != nil {
		extra, err = json.Marshal(art.Extra)
		if err != nil {
			t.err = err
			return 0
		}
	}

	if artID == 0 {
		// it's a new article
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
			t.err = err
			return 0
		}
		// TODO: not supported under PG? (use "RETURNING" syntax)
		tmpID, err := result.LastInsertId()
		if err != nil {
			t.err = err
			return 0
		}
		artID = int(tmpID)
	} else {
		// updating an existing article

		q := `UPDATE article SET (canonical_url, headline, content, published, updated, publication_id, section,extra,added) = (?,?,?,?,?,?,?,?,NOW()) WHERE id=?`
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
			t.err = err
			return 0
		}

		// delete old urls
		_, err = tx.Exec(ss.rebind(`DELETE FROM article_url WHERE article_id=?`), artID)
		if err != nil {
			t.err = err
			return 0
		}

		// delete old keywords
		_, err = tx.Exec(ss.rebind(`DELETE FROM article_keyword WHERE article_id=?`), artID)
		if err != nil {
			t.err = err
			return 0
		}

		// delete old authors
		_, err = tx.Exec(ss.rebind(`DELETE FROM author WHERE id IN (SELECT author_id FROM author_attr WHERE article_id=?)`), artID)
		if err != nil {
			t.err = err
			return 0
		}
		_, err = tx.Exec(ss.rebind(`DELETE FROM author_attr WHERE article_id=?`), artID)
		if err != nil {
			t.err = err
			return 0
		}
	}

	for _, u := range art.URLs {
		_, err = tx.Exec(ss.rebind(`INSERT INTO article_url(article_id,url) VALUES(?,?)`), artID, u)
		if err != nil {
			t.err = fmt.Errorf("failed adding url %s: %s", u, err)
			return 0
		}
	}

	for _, k := range art.Keywords {
		_, err = tx.Exec(ss.rebind(`INSERT INTO article_keyword(article_id,name,url) VALUES(?,?,?)`),
			artID,
			k.Name,
			k.URL)
		if err != nil {
			t.err = fmt.Errorf("failed adding keyword %s (%s): %s", k.Name, k.URL, err)
			return 0
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
			t.err = err
			return 0
		}
		// TODO: LastInsertId() not supported on postgres
		tmpID, err := result.LastInsertId()
		if err != nil {
			t.err = err
			return 0
		}
		authorID = int(tmpID)

		_, err = tx.Exec(ss.rebind(`INSERT INTO author_attr(author_id,article_id) VALUES (?,?)`),
			authorID,
			artID)
		if err != nil {
			t.err = err
			return 0
		}
	}

	// all good.
	return artID
}

func (t *Transaction) findOrCreatePublication(pub *store.Publication) (int, error) {
	pubID, err := t.findPublication(pub)
	if err != nil {
		return 0, err
	}
	if pubID != 0 {
		return pubID, nil
	}
	return t.createPublication(pub)
}

// returns 0 if no match
func (t *Transaction) findPublication(pub *store.Publication) (int, error) {
	var pubID int
	var err error

	if pub.Code != "" {

		err = t.tx.QueryRow(`SELECT id FROM publication WHERE code=?`, pub.Code).Scan(&pubID)
		if err == nil {
			return pubID, nil // return existing publication
		}
		if err != sql.ErrNoRows {
			return 0, err
		}
	}

	if pub.Name != "" {

		err = t.tx.QueryRow(`SELECT id FROM publication WHERE name=?`, pub.Name).Scan(&pubID)
		if err == nil {
			return pubID, nil // return existing publication
		}
		if err != sql.ErrNoRows {
			return 0, err
		}
	}

	// TODO: publications can have multiple domains...
	if pub.Domain != "" {
		err = t.tx.QueryRow(`SELECT id FROM publication WHERE domain=?`, pub.Domain).Scan(&pubID)
		if err == nil {
			return pubID, nil // return existing publication
		}
		if err != sql.ErrNoRows {
			return 0, err
		}
	}

	return 0, nil // no match
}

func (t *Transaction) createPublication(pub *store.Publication) (int, error) {
	// create new
	result, err := t.tx.Exec(`INSERT INTO publication(code,name,domain) VALUES(?,?,?)`,
		pub.Code,
		pub.Name,
		pub.Domain)
	if err != nil {
		return 0, err
	}
	// TODO: postgres support!
	pubID, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}
	return int(pubID), nil
}
