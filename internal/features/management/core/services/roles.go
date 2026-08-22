package services

import (
	"context"
	"errors"

	"github.com/leoarkiteto/zelo/internal/features/management/core/ports"
	"github.com/leoarkiteto/zelo/internal/shared/model"
)

var (
	// ErrNotSyndic is returned when a non-syndic attempts to manage roles.
	ErrNotSyndic = errors.New("only the syndic can manage roles")
	// ErrTargetNotOwner is returned when granting syndic to a non-owner.
	ErrTargetNotOwner = errors.New("target user does not hold the owner role")
	// ErrTenantNotEligible is returned when granting syndic to a tenant.
	ErrTenantNotEligible = errors.New("tenant cannot be syndic")
)

// RoleService enforces syndic eligibility and role management rules (US4).
type RoleService struct {
	Roles ports.RoleStore
	Audit ports.AuditRecorder
}

// GrantRole grants a role, enforcing that the caller is the current syndic and
// that a syndic assignment requires an active owner role.
func (s *RoleService) GrantRole(ctx context.Context, callerUserID, targetUserID, condominiumID string, role model.Role) error {
	if err := s.requireSyndic(ctx, callerUserID, condominiumID); err != nil {
		return err
	}
	if role == model.RoleSyndic {
		tenant, err := s.Roles.HasActiveRole(ctx, targetUserID, condominiumID, model.RoleTenant)
		if err != nil {
			return err
		}
		if tenant {
			return ErrTenantNotEligible
		}
		owner, err := s.Roles.HasActiveRole(ctx, targetUserID, condominiumID, model.RoleOwner)
		if err != nil || !owner {
			return ErrTargetNotOwner
		}
	}
	if err := s.Roles.GrantRole(ctx, model.RoleAssignment{
		UserID:        targetUserID,
		CondominiumID: condominiumID,
		Role:          role,
	}); err != nil {
		return err
	}
	uid := callerUserID
	return s.Audit.RecordEvent(ctx, &uid, model.AuditRoleGranted,
		map[string]any{"target_user_id": targetUserID, "role": role, "condominium_id": condominiumID})
}

// RevokeRole revokes a role; revoking owner automatically revokes syndic.
func (s *RoleService) RevokeRole(ctx context.Context, callerUserID, targetUserID, condominiumID string, role model.Role) error {
	if err := s.requireSyndic(ctx, callerUserID, condominiumID); err != nil {
		return err
	}
	if role == model.RoleOwner {
		if syndic, err := s.Roles.HasActiveRole(ctx, targetUserID, condominiumID, model.RoleSyndic); err == nil && syndic {
			if err := s.Roles.RevokeRole(ctx, targetUserID, condominiumID, model.RoleSyndic); err == nil {
				uid := callerUserID
				_ = s.Audit.RecordEvent(ctx, &uid, model.AuditRoleRevoked,
					map[string]any{"target_user_id": targetUserID, "role": model.RoleSyndic, "reason": "owner revoked"})
			}
		}
	}
	if err := s.Roles.RevokeRole(ctx, targetUserID, condominiumID, role); err != nil {
		return err
	}
	uid := callerUserID
	return s.Audit.RecordEvent(ctx, &uid, model.AuditRoleRevoked,
		map[string]any{"target_user_id": targetUserID, "role": role, "condominium_id": condominiumID})
}

func (s *RoleService) requireSyndic(ctx context.Context, callerUserID, condominiumID string) error {
	syndic, err := s.Roles.ActiveSyndicForCondominium(ctx, condominiumID)
	if err != nil || syndic.UserID != callerUserID {
		return ErrNotSyndic
	}
	return nil
}
