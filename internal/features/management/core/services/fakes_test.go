package services

import (
	"context"
	"time"

	"github.com/leoarkiteto/zelo/internal/shared/model"
)

type fakeRoleStore struct {
	roles []model.RoleAssignment
}

func (f *fakeRoleStore) GrantRole(_ context.Context, a model.RoleAssignment) error {
	for _, existing := range f.roles {
		if existing.UserID == a.UserID && existing.CondominiumID == a.CondominiumID &&
			existing.Role == a.Role && existing.RevokedAt == nil {
			return model.ErrDuplicate
		}
	}
	f.roles = append(f.roles, a)
	return nil
}

func (f *fakeRoleStore) RevokeRole(_ context.Context, userID, condoID string, role model.Role) error {
	for i := range f.roles {
		r := &f.roles[i]
		if r.UserID == userID && r.CondominiumID == condoID && r.Role == role && r.RevokedAt == nil {
			now := time.Now()
			r.RevokedAt = &now
			return nil
		}
	}
	return model.ErrNotFound
}

func (f *fakeRoleStore) ActiveRolesForUser(_ context.Context, userID, condoID string) ([]model.Role, error) {
	var out []model.Role
	for _, r := range f.roles {
		if r.UserID == userID && r.CondominiumID == condoID && r.RevokedAt == nil {
			out = append(out, r.Role)
		}
	}
	return out, nil
}

func (f *fakeRoleStore) ActiveSyndicForCondominium(_ context.Context, condoID string) (model.RoleAssignment, error) {
	for _, r := range f.roles {
		if r.CondominiumID == condoID && r.Role == model.RoleSyndic && r.RevokedAt == nil {
			return r, nil
		}
	}
	return model.RoleAssignment{}, model.ErrNotFound
}

func (f *fakeRoleStore) FirstActiveRoleForUser(_ context.Context, userID string) (model.RoleAssignment, error) {
	for _, r := range f.roles {
		if r.UserID == userID && r.RevokedAt == nil {
			return r, nil
		}
	}
	return model.RoleAssignment{}, model.ErrNotFound
}

func (f *fakeRoleStore) HasActiveRole(_ context.Context, userID, condoID string, role model.Role) (bool, error) {
	for _, r := range f.roles {
		if r.UserID == userID && r.CondominiumID == condoID && r.Role == role && r.RevokedAt == nil {
			return true, nil
		}
	}
	return false, nil
}

type fakeAudit struct {
	events []model.AuditEvent
}

func (f *fakeAudit) RecordEvent(_ context.Context, userID *string, eventType model.AuditEventType, details map[string]any) error {
	f.events = append(f.events, model.AuditEvent{UserID: userID, EventType: eventType, Details: details})
	return nil
}
