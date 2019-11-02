package sqlstore

import (
	"database/sql"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

// Run our DB tests against an in-memory sqlite3 database.
func TestSqlite3(t *testing.T) {

	// NOTE: ":memory" won't work, as it only persists for single connection.
	// Use shared cache to share the database across all connections in
	// this process.
	// see https://github.com/mattn/go-sqlite3#faq
	db, err := sql.Open("sqlite3", "file::memory:?cache=shared")
	if err != nil {
		t.Errorf("New: %s\n", err)
		return
	}
	db.SetConnMaxLifetime(-1)
	db.SetMaxIdleConns(2) // should be default but may change in future
	ss, err := NewFromDB("sqlite3", db)
	if err != nil {
		t.Errorf("New: %s\n", err)
		return
	}
	performDBTests(t, ss)

	defer ss.Close()
}
