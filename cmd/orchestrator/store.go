package main

import (
	"context"
	"database/sql"
	"time"

	"github.com/suremarc/go-rblx-asset-scraper/packages/scraper/sync/client"
	"github.com/suremarc/go-rblx-asset-scraper/packages/scraper/sync/ranges"
	_ "modernc.org/sqlite"
)

type SQL struct {
	db     *sql.DB
	insert *sql.Stmt
	query  *sql.Stmt
}

const (
	createTableStmt = `
CREATE TABLE IF NOT EXISTS events (
	range varchar(32),
	status_code int,
	successes int,
	failures int,
	total int,
	duration_ms int,
	last_attempt_utc int,
	PRIMARY KEY(group)
);`

	insertStmt = `INSERT INTO events VALUES ($1, $2, $3, $4, $5, $6, $7)`
	queryStmt  = `SELECT status_code FROM events WHERE group=$1`
)

func NewSQL(address string) (*SQL, error) {
	db, err := sql.Open("sqlite", address)
	if err != nil {
		return nil, err
	}

	if _, err := db.Exec(createTableStmt); err != nil {
		return nil, err
	}

	s := SQL{
		db: db,
	}

	if s.insert, err = db.Prepare(insertStmt); err != nil {
		return nil, err
	}

	if s.query, err = db.Prepare(queryStmt); err != nil {
		return nil, err
	}

	return &s, nil
}

func (s *SQL) Log(ctx context.Context, rng ranges.Range, resp *client.Response) error {
	txt, err := rng.MarshalText()
	if err != nil {
		return err
	}

	if _, err := s.insert.ExecContext(
		ctx,
		txt,
		resp.StatusCode,
		resp.Successes,
		resp.Failures,
		resp.Total,
		resp.DurationMilliseconds,
		time.Now().UnixMilli(),
	); err != nil {
		return err
	}

	return nil
}

func (s *SQL) Query(ctx context.Context, rng ranges.Range) (statusCode int, err error) {
	txt, err := rng.MarshalText()
	if err != nil {
		return 0, err
	}

	if err := s.query.QueryRow(txt).Scan(&statusCode); err != nil {
		return 0, err
	}

	return statusCode, nil
}
