// Package testutil provides shared helpers for integration tests.
package testutil

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"strings"

	_ "github.com/jackc/pgx/v5/stdlib"
)

// TestDatabaseURL returns a TEST_DATABASE_URL pointing at a per-suffix
// database on the same server, creating it on first use. Isolating packages
// (e.g. "store", "handler") avoids truncation races when `go test` runs
// package test binaries in parallel against one PostgreSQL server.
//
// It skips silently when TEST_DATABASE_URL is unset, matching the repo's
// existing integration-test convention (tests are skipped without it).
func TestDatabaseURL(suffix string) string {
	base := os.Getenv("TEST_DATABASE_URL")
	if base == "" {
		return ""
	}
	u, err := url.Parse(base)
	if err != nil {
		panic(fmt.Sprintf("parse TEST_DATABASE_URL: %v", err))
	}
	name := sanitizeName(strings.TrimPrefix(u.Path, "/") + "_" + suffix)
	if err := ensureDatabase(u, name); err != nil {
		panic(fmt.Sprintf("ensure test database %s: %v", name, err))
	}
	u.Path = "/" + name
	return u.String()
}

func sanitizeName(s string) string {
	var b strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_' {
			b.WriteRune(r)
		}
	}
	if b.Len() == 0 {
		return "zelo_test"
	}
	return b.String()
}

func ensureDatabase(u *url.URL, name string) error {
	admin := *u
	admin.Path = "/postgres"
	db, err := sql.Open("pgx", admin.String())
	if err != nil {
		return err
	}
	defer db.Close()
	ctx := context.Background()
	var exists bool
	if err := db.QueryRowContext(ctx,
		`SELECT EXISTS(SELECT 1 FROM pg_database WHERE datname = $1)`, name).Scan(&exists); err != nil {
		return err
	}
	if exists {
		return nil
	}
	if _, err := db.ExecContext(ctx, "CREATE DATABASE "+name); err != nil {
		return err
	}
	return nil
}
