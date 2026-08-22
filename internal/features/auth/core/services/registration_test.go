package services

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/leoarkiteto/zelo/internal/shared/model"
)

type testHasher struct{}

func (testHasher) HashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func (testHasher) Hash(password string) (string, error) {
	return testHasher{}.HashToken(password), nil
}

func (testHasher) Verify(hash, password string) (bool, error) {
	return hash == testHasher{}.HashToken(password), nil
}

func (testHasher) ValidatePassword(password string) error {
	if len(password) < 12 {
		return errors.New("too short")
	}
	return nil
}

type fakeUserStore struct {
	users map[string]model.User
	seq   int
}

func newFakeUserStore() *fakeUserStore {
	return &fakeUserStore{users: map[string]model.User{}}
}

func (f *fakeUserStore) CreateUser(_ context.Context, u model.User) (string, error) {
	for _, existing := range f.users {
		if strings.EqualFold(existing.Email, u.Email) {
			return "", model.ErrDuplicate
		}
	}
	f.seq++
	id := "user-" + string(rune('a'+f.seq-1))
	u.ID = id
	f.users[id] = u
	return id, nil
}

func (f *fakeUserStore) GetUserByEmail(_ context.Context, email string) (model.User, error) {
	for _, u := range f.users {
		if strings.EqualFold(u.Email, email) {
			return u, nil
		}
	}
	return model.User{}, model.ErrNotFound
}

func (f *fakeUserStore) GetUserByID(_ context.Context, id string) (model.User, error) {
	u, ok := f.users[id]
	if !ok {
		return model.User{}, model.ErrNotFound
	}
	return u, nil
}

func (f *fakeUserStore) UpdatePassword(_ context.Context, id, hash string) error {
	u, ok := f.users[id]
	if !ok {
		return model.ErrNotFound
	}
	u.PasswordHash = hash
	f.users[id] = u
	return nil
}

func (f *fakeUserStore) RecordFailedSignIn(_ context.Context, id string) error {
	u := f.users[id]
	u.FailedSignInCount++
	f.users[id] = u
	return nil
}

func (f *fakeUserStore) ApplyLock(_ context.Context, id string, until time.Time) error {
	u := f.users[id]
	u.LockedUntil = &until
	u.FailedSignInCount = 0
	f.users[id] = u
	return nil
}

func (f *fakeUserStore) ClearLock(_ context.Context, id string) error {
	u := f.users[id]
	u.LockedUntil = nil
	u.FailedSignInCount = 0
	f.users[id] = u
	return nil
}

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

type fakeInvitationStore struct {
	invitations map[string]model.Invitation
}

func newFakeInvitationStore() *fakeInvitationStore {
	return &fakeInvitationStore{invitations: map[string]model.Invitation{}}
}

func (f *fakeInvitationStore) CreateInvitation(_ context.Context, inv model.Invitation) (string, error) {
	inv.ID = "inv-" + inv.TokenHash[:8]
	f.invitations[inv.ID] = inv
	return inv.ID, nil
}

func (f *fakeInvitationStore) GetInvitationByTokenHash(_ context.Context, hash string) (model.Invitation, error) {
	for _, inv := range f.invitations {
		if inv.TokenHash == hash {
			return inv, nil
		}
	}
	return model.Invitation{}, model.ErrNotFound
}

func (f *fakeInvitationStore) MarkInvitationAccepted(_ context.Context, id string) error {
	inv, ok := f.invitations[id]
	if !ok {
		return model.ErrNotFound
	}
	inv.Status = model.InvitationAccepted
	f.invitations[id] = inv
	return nil
}

func (f *fakeInvitationStore) RevokeInvitation(_ context.Context, id string) error {
	inv, ok := f.invitations[id]
	if !ok {
		return model.ErrNotFound
	}
	inv.Status = model.InvitationRevoked
	f.invitations[id] = inv
	return nil
}

type fakeAudit struct {
	events []model.AuditEvent
}

func (f *fakeAudit) RecordEvent(_ context.Context, userID *string, eventType model.AuditEventType, details map[string]any) error {
	f.events = append(f.events, model.AuditEvent{UserID: userID, EventType: eventType, Details: details})
	return nil
}

func invitationFixture(tokenHash string, status model.InvitationStatus, expiresIn time.Duration) model.Invitation {
	return model.Invitation{
		ID:            "inv-1",
		TokenHash:     tokenHash,
		CondominiumID: "condo-1",
		UnitID:        "unit-1",
		InvitedRole:   model.RoleOwner,
		InvitedEmail:  "owner@example.com",
		Status:        status,
		ExpiresAt:     time.Now().Add(expiresIn),
		CreatedBy:     "syndic-1",
	}
}

func TestRegistrationServiceRegisterValid(t *testing.T) {
	token := "invite-token-1"
	invStore := newFakeInvitationStore()
	invStore.invitations["inv-1"] = invitationFixture(testHasher{}.HashToken(token), model.InvitationPending, 24*time.Hour)
	userStore := newFakeUserStore()
	roleStore := &fakeRoleStore{}
	svc := &RegistrationService{
		Users: userStore, Roles: roleStore, Invitations: invStore,
		Passwords: testHasher{}, Tokens: testHasher{}, Now: time.Now,
	}

	err := svc.Register(context.Background(), token, "owner@example.com", "valid-password-123")
	if err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	if len(userStore.users) != 1 {
		t.Fatalf("expected 1 user, got %d", len(userStore.users))
	}
	roles, _ := roleStore.ActiveRolesForUser(context.Background(), "user-a", "condo-1")
	if len(roles) != 1 || roles[0] != model.RoleOwner {
		t.Fatalf("roles = %v, want [owner]", roles)
	}
	if invStore.invitations["inv-1"].Status != model.InvitationAccepted {
		t.Fatal("invitation was not marked accepted")
	}
}

func TestRegistrationServiceRegisterInvalidInvitation(t *testing.T) {
	token := "invite-token-expired"
	invStore := newFakeInvitationStore()
	invStore.invitations["inv-1"] = invitationFixture(testHasher{}.HashToken(token), model.InvitationPending, -time.Hour)
	svc := &RegistrationService{
		Users: newFakeUserStore(), Roles: &fakeRoleStore{}, Invitations: invStore,
		Passwords: testHasher{}, Tokens: testHasher{}, Now: time.Now,
	}
	err := svc.Register(context.Background(), token, "owner@example.com", "valid-password-123")
	if !errors.Is(err, ErrInvalidInvitation) {
		t.Fatalf("Register() error = %v, want ErrInvalidInvitation", err)
	}

	used := newFakeInvitationStore()
	used.invitations["inv-1"] = invitationFixture(testHasher{}.HashToken("used"), model.InvitationAccepted, 24*time.Hour)
	svc.Invitations = used
	err = svc.Register(context.Background(), "used", "owner@example.com", "valid-password-123")
	if !errors.Is(err, ErrInvalidInvitation) {
		t.Fatalf("Register() error = %v, want ErrInvalidInvitation for used token", err)
	}
}

func TestRegistrationServiceRegisterEmailMismatch(t *testing.T) {
	token := "invite-token-3"
	invStore := newFakeInvitationStore()
	invStore.invitations["inv-1"] = invitationFixture(testHasher{}.HashToken(token), model.InvitationPending, 24*time.Hour)
	svc := &RegistrationService{
		Users: newFakeUserStore(), Roles: &fakeRoleStore{}, Invitations: invStore,
		Passwords: testHasher{}, Tokens: testHasher{}, Now: time.Now,
	}
	err := svc.Register(context.Background(), token, "other@example.com", "valid-password-123")
	if !errors.Is(err, ErrEmailMismatch) {
		t.Fatalf("Register() error = %v, want ErrEmailMismatch", err)
	}
}

func TestRegistrationServiceRegisterDuplicateEmail(t *testing.T) {
	token := "invite-token-4"
	invStore := newFakeInvitationStore()
	invStore.invitations["inv-1"] = invitationFixture(testHasher{}.HashToken(token), model.InvitationPending, 24*time.Hour)
	userStore := newFakeUserStore()
	_, _ = userStore.CreateUser(context.Background(), model.User{Email: "owner@example.com", PasswordHash: "x"})
	svc := &RegistrationService{
		Users: userStore, Roles: &fakeRoleStore{}, Invitations: invStore,
		Passwords: testHasher{}, Tokens: testHasher{}, Now: time.Now,
	}
	err := svc.Register(context.Background(), token, "owner@example.com", "valid-password-123")
	if !errors.Is(err, ErrEmailTaken) {
		t.Fatalf("Register() error = %v, want ErrEmailTaken", err)
	}
}
