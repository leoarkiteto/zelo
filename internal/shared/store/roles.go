package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/leoarkiteto/zelo/internal/shared/model"
)

// RoleStore persists RBAC role assignments.
type RoleStore struct {
	db *sql.DB
}

// NewRoleStore creates a RoleStore.
func NewRoleStore(db *sql.DB) *RoleStore { return &RoleStore{db: db} }

// GrantRole inserts an active role assignment.
func (s *RoleStore) GrantRole(ctx context.Context, assignment model.RoleAssignment) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO user_roles (user_id, condominium_id, role)
		VALUES ($1, $2, $3)`,
		assignment.UserID, assignment.CondominiumID, assignment.Role)
	if err != nil {
		if IsUniqueViolation(err) {
			return ErrDuplicate
		}
		return fmt.Errorf("grant role: %w", err)
	}
	return nil
}

// RevokeRole soft-revokes a role assignment.
func (s *RoleStore) RevokeRole(ctx context.Context, userID, condominiumID string, role model.Role) error {
	res, err := s.db.ExecContext(ctx, `
		UPDATE user_roles SET revoked_at = now()
		WHERE user_id = $1 AND condominium_id = $2 AND role = $3 AND revoked_at IS NULL`,
		userID, condominiumID, role)
	if err != nil {
		return fmt.Errorf("revoke role: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// ActiveRolesForUser returns the user's active roles in a condominium.
func (s *RoleStore) ActiveRolesForUser(ctx context.Context, userID, condominiumID string) ([]model.Role, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT role FROM user_roles
		WHERE user_id = $1 AND condominium_id = $2 AND revoked_at IS NULL`,
		userID, condominiumID)
	if err != nil {
		return nil, fmt.Errorf("active roles: %w", err)
	}
	defer rows.Close()

	var roles []model.Role
	for rows.Next() {
		var r model.Role
		if err := rows.Scan(&r); err != nil {
			return nil, err
		}
		roles = append(roles, r)
	}
	return roles, rows.Err()
}

// FirstActiveRoleForUser returns the user's earliest active role assignment.
func (s *RoleStore) FirstActiveRoleForUser(ctx context.Context, userID string) (model.RoleAssignment, error) {
	var a model.RoleAssignment
	err := s.db.QueryRowContext(ctx, `
		SELECT user_id, condominium_id, role, granted_at FROM user_roles
		WHERE user_id = $1 AND revoked_at IS NULL
		ORDER BY granted_at LIMIT 1`, userID).Scan(
		&a.UserID, &a.CondominiumID, &a.Role, &a.GrantedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return model.RoleAssignment{}, ErrNotFound
	}
	if err != nil {
		return model.RoleAssignment{}, fmt.Errorf("first active role: %w", err)
	}
	return a, nil
}

// ActiveSyndicForCondominium returns the active syndic assignment, if any.
func (s *RoleStore) ActiveSyndicForCondominium(ctx context.Context, condominiumID string) (model.RoleAssignment, error) {
	var a model.RoleAssignment
	a.CondominiumID = condominiumID
	a.Role = model.RoleSyndic
	err := s.db.QueryRowContext(ctx, `
		SELECT user_id, granted_at FROM user_roles
		WHERE condominium_id = $1 AND role = 'syndic' AND revoked_at IS NULL
		LIMIT 1`, condominiumID).Scan(&a.UserID, &a.GrantedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return model.RoleAssignment{}, ErrNotFound
	}
	if err != nil {
		return model.RoleAssignment{}, fmt.Errorf("active syndic: %w", err)
	}
	return a, nil
}

// HasActiveRole reports whether the user holds an active role in the condominium.
func (s *RoleStore) HasActiveRole(ctx context.Context, userID, condominiumID string, role model.Role) (bool, error) {
	var n int
	err := s.db.QueryRowContext(ctx, `
		SELECT count(*) FROM user_roles
		WHERE user_id = $1 AND condominium_id = $2 AND role = $3 AND revoked_at IS NULL`,
		userID, condominiumID, role).Scan(&n)
	if err != nil {
		return false, fmt.Errorf("has active role: %w", err)
	}
	return n > 0, nil
}

// UserRolesRow is a user with their active roles in a condominium.
type UserRolesRow struct {
	UserID string
	Email  string
	Roles  []model.Role
}

// ListUsersWithRoles returns every user with at least one role in the
// condominium, aggregated by user.
func (s *RoleStore) ListUsersWithRoles(ctx context.Context, condominiumID string) ([]UserRolesRow, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT u.id, u.email, r.role FROM user_roles r
		JOIN users u ON u.id = r.user_id
		WHERE r.condominium_id = $1 AND r.revoked_at IS NULL
		ORDER BY u.email, r.role`, condominiumID)
	if err != nil {
		return nil, fmt.Errorf("list users with roles: %w", err)
	}
	defer rows.Close()

	byUser := map[string]*UserRolesRow{}
	var order []string
	for rows.Next() {
		var id, email string
		var role model.Role
		if err := rows.Scan(&id, &email, &role); err != nil {
			return nil, err
		}
		row, ok := byUser[id]
		if !ok {
			row = &UserRolesRow{UserID: id, Email: email}
			byUser[id] = row
			order = append(order, id)
		}
		row.Roles = append(row.Roles, role)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	out := make([]UserRolesRow, 0, len(order))
	for _, id := range order {
		out = append(out, *byUser[id])
	}
	return out, nil
}
