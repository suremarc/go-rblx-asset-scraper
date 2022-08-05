package main

import (
	"database/sql"
	// _ "modernc.org/sqlite"
)

type SQLStore struct {
	db *sql.DB
}

const createTableStmt = `
CREATE TABLE IF NOT EXISTS events {

}
`

func NewSQLStore(address string) (*SQLStore, error) {
	db, err := sql.Open("sqlite", address)
	if err != nil {
		return nil, err
	}

	if _, err := db.Exec(createTableStmt); err != nil {
		return nil, err
	}

	return &SQLStore{
		db: db,
	}, nil
}
