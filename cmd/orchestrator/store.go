package main

import (
	"context"
	"database/sql"
	"strings"
	"time"

	"github.com/suremarc/go-rblx-asset-scraper/packages/scraper/sync/client"
	"github.com/suremarc/go-rblx-asset-scraper/packages/scraper/sync/ranges"

	_ "github.com/lib/pq"
)

type SQL struct {
	db     *sql.DB
	upsert *sql.Stmt
	query  *sql.Stmt
}

const (
	createTableStmt = `
CREATE TABLE IF NOT EXISTS events (
	range varchar(32),
	status_code DOUBLE,
	successes DOUBLE,
	failures DOUBLE,
	total DOUBLE,
	duration_ms DOUBLE,
	last_attempt_utc DOUBLE,
	PRIMARY KEY (range)
);`

	upsertStmt = `INSERT INTO events VALUES ($1, $2, $3, $4, $5, $6, $7) ON CONFLICT (range) DO UPDATE SET status_code=$2, successes=$3, failures=$4, total=$5, duration_ms=$6, last_attempt_utc=$7`
	queryStmt  = `SELECT status_code FROM events WHERE range=$1`
)

func NewSQL(address string) (*SQL, error) {
	db, err := sql.Open("postgres", address)
	if err != nil {
		return nil, err
	}

	if _, err = db.Exec(createTableStmt); err != nil {
		// try replacing the double type
		_, err = db.Exec(strings.ReplaceAll(createTableStmt, "DOUBLE", "DOUBLE PRECISION"))
	}

	if err != nil {
		return nil, err
	}

	if err != nil {
		return nil, err
	}

	s := SQL{
		db: db,
	}

	if s.upsert, err = db.Prepare(upsertStmt); err != nil {
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

	if _, err := s.upsert.ExecContext(
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
