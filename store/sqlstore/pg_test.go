package sqlstore

import (
	"database/sql"
	"os"
	"testing"

	_ "github.com/lib/pq"
)

// TestPostgres runs the store tests against a postgresql database.
// It requires a test database to be set up in advance, with a
// schema loaded.
// The connection string should be in envvar SCRAPEOMAT_PGTEST.
// If it is not set, the postgres testing is skippped.
//
// Example setup:
//
// Add a line to `pg_hba.conf` to allow our test user to access the test
// database:
//
//    local   scrapetest      timmytestfish                           peer map=scrapedev
//
// Map OS username to our test user, in `pg_ident.conf`:
//
//    scrapedev       ben                     timmytestfish
//
// Tell postgres to reload its config:
//    $ sudo systemctl reload postgresql
//
//
// Create the test user and database and load the schema:
//
//    $ sudo -u postgres createuser --no-superuser --no-createrole --no-createdb timmytestfish
//    $ sudo -u postgres createdb -O timmytestfish -E utf8 scrapetest
//    $ cat pg/schema.sql | psql -U timmytestfish scrapetest
//
//    $ export SCRAPEOMAT_PGTEST="user=timmytestfish dbname=scrapetest host=/var/run/postgresql sslmode=disable"
//    $ go test
//

func TestPostgres(t *testing.T) {

	connStr := os.Getenv("SCRAPEOMAT_PGTEST")
	if connStr == "" {
		t.Skip("SCRAPEOMAT_PGTEST not set - skipping postgresql tests")
	}

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		t.Fatal(err.Error())
	}

	// Make sure we don't accidentally screw up real data!
	var cnt int
	err = db.QueryRow("SELECT COUNT(*) FROM article").Scan(&cnt)
	if err != nil {
		t.Fatal(err.Error())
	}
	if cnt > 0 {
		t.Fatal("Database already contains articles - refusing to clobber.")
	}

	ss, err := NewFromDB("postgres", db)
	if err != nil {
		t.Fatal(err.Error())
	}

	// clear out db when we're done.
	defer func() {
		_, err = db.Exec("DELETE FROM article")
		if err != nil {
			t.Fatal(err.Error())
		}
		ss.Close()
	}()

	// Now run the tests!
	performDBTests(t, ss)
}
