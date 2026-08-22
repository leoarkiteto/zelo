package main

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"

	"github.com/leoarkiteto/zelo/internal/auth"
)

// seedFirstCondominium bootstraps a development condominium with one unit and
// a syndic user. It is idempotent: it exits cleanly if the seed user exists.
func seedFirstCondominium(ctx context.Context, logger *slog.Logger, db *sql.DB, passwordPepper string) error {
	email := "syndic@example.com"
	var existing string
	err := db.QueryRowContext(ctx,
		`SELECT id FROM users WHERE lower(email) = lower($1)`, email).Scan(&existing)
	if err == nil {
		logger.Info("seed user already exists", "email", email)
		return nil
	}
	if err != sql.ErrNoRows {
		return fmt.Errorf("check seed user: %w", err)
	}

	hasher := auth.NewPasswordHasher(passwordPepper)
	hash, err := hasher.Hash("syndic-password-123")
	if err != nil {
		return fmt.Errorf("hash seed password: %w", err)
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	var condoID string
	if err := tx.QueryRowContext(ctx,
		`INSERT INTO condominiums (name) VALUES ('Demo Condominium') RETURNING id`).Scan(&condoID); err != nil {
		return fmt.Errorf("seed condominium: %w", err)
	}
	var unitID string
	if err := tx.QueryRowContext(ctx,
		`INSERT INTO units (condominium_id, code) VALUES ($1, 'A-101') RETURNING id`, condoID).Scan(&unitID); err != nil {
		return fmt.Errorf("seed unit: %w", err)
	}
	var userID string
	if err := tx.QueryRowContext(ctx,
		`INSERT INTO users (email, password_hash) VALUES ($1, $2) RETURNING id`, email, hash).Scan(&userID); err != nil {
		return fmt.Errorf("seed user: %w", err)
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO unit_occupancies (user_id, unit_id, occupancy_type) VALUES ($1, $2, 'owner')`,
		userID, unitID); err != nil {
		return fmt.Errorf("seed occupancy: %w", err)
	}
	for _, role := range []string{"owner", "syndic"} {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO user_roles (user_id, condominium_id, role) VALUES ($1, $2, $3)`,
			userID, condoID, role); err != nil {
			return fmt.Errorf("seed role %s: %w", role, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit seed: %w", err)
	}
	return nil
}
