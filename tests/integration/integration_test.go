// Package integration exercises the composed application end to end.
package integration

import (
	"context"
	"database/sql"
	"io"
	"log/slog"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	authhandlers "github.com/leoarkiteto/zelo/internal/features/auth/handlers"
	authservices "github.com/leoarkiteto/zelo/internal/features/auth/core/services"
	directoryhandlers "github.com/leoarkiteto/zelo/internal/features/directory/handlers"
	dirservices "github.com/leoarkiteto/zelo/internal/features/directory/core/services"
	"github.com/leoarkiteto/zelo/internal/features/directory/repositories"
	homehandlers "github.com/leoarkiteto/zelo/internal/features/home/handlers"
	managementhandlers "github.com/leoarkiteto/zelo/internal/features/management/handlers"
	mgmtservices "github.com/leoarkiteto/zelo/internal/features/management/core/services"
	profilehandlers "github.com/leoarkiteto/zelo/internal/features/profile/handlers"
	profileservices "github.com/leoarkiteto/zelo/internal/features/profile/core/services"
	ticketrepositories "github.com/leoarkiteto/zelo/internal/features/tickets/repositories"
	ticketservices "github.com/leoarkiteto/zelo/internal/features/tickets/core/services"
	tickethandlers "github.com/leoarkiteto/zelo/internal/features/tickets/handlers"
	"github.com/leoarkiteto/zelo/internal/shared/middleware"
	"github.com/leoarkiteto/zelo/internal/shared/security"
	"github.com/leoarkiteto/zelo/internal/shared/store"
	"github.com/leoarkiteto/zelo/internal/shared/testutil"
)

// newApp builds the full composed router against PostgreSQL. Tests are skipped
// unless TEST_DATABASE_URL is set (see quickstart.md).
func newApp(t *testing.T) http.Handler {
	t.Helper()
	url := testutil.TestDatabaseURL("integration")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping integration test")
	}
	redisURL := testutil.TestRedisURL("integration")
	if redisURL == "" {
		t.Skip("TEST_REDIS_URL not set; skipping integration test")
	}
	ctx := context.Background()
	db, err := store.Open(url)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	if err := store.Migrate(ctx, db, "../../migrations"); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if _, err := db.ExecContext(ctx, `
		TRUNCATE audit_events, invitations, service_provider_listings,
		service_categories, ticket_replies, tickets, unit_occupancies, user_roles,
		units, condominiums, users RESTART IDENTITY CASCADE`); err != nil {
		t.Fatalf("truncate: %v", err)
	}
	seedIntegration(t, ctx, db)

	redisClient, err := store.OpenRedis(redisURL)
	if err != nil {
		t.Fatalf("open redis: %v", err)
	}
	t.Cleanup(func() { _ = redisClient.Close() })
	if err := redisClient.FlushDB(ctx).Err(); err != nil {
		t.Fatalf("flush redis: %v", err)
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	users := store.NewUserStore(db)
	roles := store.NewRoleStore(db)
	units := store.NewUnitStore(db)
	invitations := store.NewInvitationStore(db)
	sessions := store.NewRedisSessionStore(redisClient)
	audit := store.NewAuditStore(db)
	listings := repositories.NewListingStore(db)
	categories := repositories.NewCategoryStore(db)
	ticketStore := ticketrepositories.NewTicketStore(db)
	hasher := security.NewPasswordHasher("integration-test-pepper")
	tokens := security.TokenHasher{}
	sessMgr := security.NewSessionManager(sessions, false)

	mux := http.NewServeMux()
	authhandlers.RegisterRoutes(mux, authhandlers.Deps{
		Logger:      logger,
		Sessions:    sessMgr,
		Invitations: invitations,
		Tokens:      tokens,
		Audit:       audit,
		Registration: &authservices.RegistrationService{
			Users: users, Roles: roles, Invitations: invitations,
			Passwords: hasher, Tokens: tokens, Now: time.Now,
		},
		AuthService: &authservices.AuthService{
			Users: users, Roles: roles, Passwords: hasher, Audit: audit, Now: time.Now,
		},
		PasswordReset: &authservices.PasswordResetService{
			Users: users, Passwords: hasher, Tokens: tokens, Now: time.Now,
		},
	})
	homehandlers.RegisterRoutes(mux, homehandlers.Deps{Roles: roles, Audit: audit})
	directoryhandlers.RegisterRoutes(mux, directoryhandlers.Deps{
		Roles:      roles,
		Audit:      audit,
		Listings:   listings,
		Categories: categories,
		Directory: &dirservices.DirectoryService{
			Listings: listings, Categories: categories, Units: units, Audit: audit,
		},
	})
	managementhandlers.RegisterRoutes(mux, managementhandlers.Deps{
		Roles:       roles,
		Audit:       audit,
		Tokens:      tokens,
		Invitations: invitations,
		Units:       units,
		RoleService: &mgmtservices.RoleService{Roles: roles, Audit: audit},
	})
	profilehandlers.RegisterRoutes(mux, profilehandlers.Deps{
		Logger: logger,
		Roles:  roles,
		Profile: &profileservices.ProfileService{
			Users:       users,
			Preferences: users,
		},
	})
	tickethandlers.RegisterRoutes(mux, tickethandlers.Deps{
		Roles: roles,
		Audit: audit,
		Units: units,
		Tickets: &ticketservices.TicketService{
			Tickets: ticketStore,
			Units:   units,
			Audit:   audit,
		},
	})
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.Dir("../../web/static"))))

	var root http.Handler = mux
	root = middleware.Recover(logger)(root)
	root = middleware.Logging(logger)(root)
	root = middleware.SecurityHeaders(root)
	root = middleware.WithLocale(root)
	root = middleware.WithUser(sessMgr, users)(root)
	root = middleware.CSRF(sessMgr)(root)
	return root
}

func seedIntegration(t *testing.T, ctx context.Context, db *sql.DB) {
	t.Helper()
	hasher := security.NewPasswordHasher("integration-test-pepper")
	hash, err := hasher.Hash("syndic-pass-123")
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	var condoID string
	if err := db.QueryRowContext(ctx, `INSERT INTO condominiums (name) VALUES ('IT Condo') RETURNING id`).Scan(&condoID); err != nil {
		t.Fatalf("seed condo: %v", err)
	}
	var unitID string
	if err := db.QueryRowContext(ctx, `INSERT INTO units (condominium_id, code) VALUES ($1, 'B-1') RETURNING id`, condoID).Scan(&unitID); err != nil {
		t.Fatalf("seed unit: %v", err)
	}
	var syndicID string
	if err := db.QueryRowContext(ctx, `INSERT INTO users (email, password_hash) VALUES ('syndic@example.com', $1) RETURNING id`, hash).Scan(&syndicID); err != nil {
		t.Fatalf("seed syndic: %v", err)
	}
	for _, role := range []string{"owner", "syndic"} {
		if _, err := db.ExecContext(ctx, `INSERT INTO user_roles (user_id, condominium_id, role) VALUES ($1, $2, $3)`, syndicID, condoID, role); err != nil {
			t.Fatalf("seed role: %v", err)
		}
	}
	if _, err := db.ExecContext(ctx,
		`INSERT INTO unit_occupancies (user_id, unit_id, occupancy_type) VALUES ($1, $2, 'owner')`,
		syndicID, unitID); err != nil {
		t.Fatalf("seed occupancy: %v", err)
	}

	// An owner-only member used to verify resident access control (tickets,
	// finance member routes) without syndic privileges.
	ownerHash, err := hasher.Hash("resident-pass-123")
	if err != nil {
		t.Fatalf("hash owner: %v", err)
	}
	var ownerID string
	if err := db.QueryRowContext(ctx, `INSERT INTO users (email, password_hash) VALUES ('resident@example.com', $1) RETURNING id`, ownerHash).Scan(&ownerID); err != nil {
		t.Fatalf("seed owner: %v", err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO user_roles (user_id, condominium_id, role) VALUES ($1, $2, 'owner')`, ownerID, condoID); err != nil {
		t.Fatalf("seed owner role: %v", err)
	}
	if _, err := db.ExecContext(ctx,
		`INSERT INTO unit_occupancies (user_id, unit_id, occupancy_type) VALUES ($1, $2, 'owner')`,
		ownerID, unitID); err != nil {
		t.Fatalf("seed owner occupancy: %v", err)
	}
	t.Setenv("TEST_CONDOMINIUM", condoID)
	t.Setenv("TEST_UNIT", unitID)
}

var csrfRe = regexp.MustCompile(`name="csrf_token" value="([^"]+)"`)

func postForm(t *testing.T, client *http.Client, base, path string, form map[string]string) (int, string) {
	t.Helper()
	var body strings.Builder
	for k, v := range form {
		body.WriteString(k + "=" + v + "&")
	}
	req, err := http.NewRequest(http.MethodPost, base+path, strings.NewReader(body.String()))
	if err != nil {
		t.Fatalf("build post: %v", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("post %s: %v", path, err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(b)
}

func getPage(t *testing.T, client *http.Client, base, path string) (int, string) {
	t.Helper()
	resp, err := client.Get(base + path)
	if err != nil {
		t.Fatalf("get %s: %v", path, err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(b)
}

func newClient(t *testing.T) *http.Client {
	t.Helper()
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("jar: %v", err)
	}
	return &http.Client{Jar: jar, CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
		return http.ErrUseLastResponse
	}}
}

func csrfFrom(t *testing.T, client *http.Client, base, path string) string {
	t.Helper()
	code, body := getPage(t, client, base, path)
	if code != http.StatusOK {
		t.Fatalf("page %s = %d while reading csrf", path, code)
	}
	m := csrfRe.FindStringSubmatch(body)
	if m == nil {
		t.Fatalf("no csrf token on %s", path)
	}
	return m[1]
}

func rawTokenFromBody(body string) string {
	const prefix = "/register?token="
	i := strings.Index(body, prefix)
	if i < 0 {
		return ""
	}
	rest := body[i+len(prefix):]
	end := strings.IndexAny(rest, "\"<& \n")
	if end < 0 {
		return rest
	}
	return rest[:end]
}

func userIDFor(t *testing.T, email, condoID string) string {
	t.Helper()
	url := testutil.TestDatabaseURL("integration")
	db, err := store.Open(url)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()
	var id string
	if err := db.QueryRowContext(context.Background(),
		`SELECT u.id FROM users u JOIN user_roles r ON r.user_id = u.id WHERE lower(u.email)=lower($1) AND r.condominium_id=$2 LIMIT 1`,
		email, condoID).Scan(&id); err != nil {
		t.Fatalf("find user %s: %v", email, err)
	}
	return id
}

func TestIntegrationRegisterLoginAndRoleAccess(t *testing.T) {
	router := newApp(t)
	ts := httptest.NewServer(router)
	defer ts.Close()
	base := ts.URL
	condoID := os.Getenv("TEST_CONDOMINIUM")
	unitID := os.Getenv("TEST_UNIT")

	// Syndic signs in and creates a tenant invitation.
	syndic := newClient(t)
	code, _ := postForm(t, syndic, base, "/login", map[string]string{
		"email": "syndic@example.com", "password": "syndic-pass-123",
		"csrf_token": csrfFrom(t, syndic, base, "/login"),
	})
	if code != http.StatusSeeOther {
		t.Fatalf("syndic login = %d, want 303", code)
	}
	code, body := getPage(t, syndic, base, "/invitations")
	if code != http.StatusOK {
		t.Fatalf("invitations page = %d", code)
	}
	csrf := csrfRe.FindStringSubmatch(body)
	if csrf == nil {
		t.Fatal("no csrf token on invitations page")
	}
	code, body = postForm(t, syndic, base, "/invitations", map[string]string{
		"unit_id": unitID, "invited_role": "tenant", "invited_email": "tenant@example.com", "csrf_token": csrf[1],
	})
	if code != http.StatusOK {
		t.Fatalf("create invitation = %d", code)
	}
	tenantToken := rawTokenFromBody(body)
	if tenantToken == "" {
		t.Fatal("no invitation token in response")
	}

	// Tenant registers, signs in, and sees only their own area.
	tenant := newClient(t)
	code, _ = getPage(t, tenant, base, "/register?token="+tenantToken)
	if code != http.StatusOK {
		t.Fatalf("register page = %d", code)
	}
	code, _ = postForm(t, tenant, base, "/register", map[string]string{
		"token": tenantToken, "email": "tenant@example.com", "password": "tenant-pass-123",
		"password_confirm": "tenant-pass-123", "csrf_token": csrfFrom(t, tenant, base, "/register?token="+tenantToken),
	})
	if code != http.StatusSeeOther {
		t.Fatalf("register = %d, want 303", code)
	}
	code, _ = postForm(t, tenant, base, "/login", map[string]string{
		"email": "tenant@example.com", "password": "tenant-pass-123",
		"csrf_token": csrfFrom(t, tenant, base, "/login"),
	})
	if code != http.StatusSeeOther {
		t.Fatalf("tenant login = %d, want 303", code)
	}
	code, body = getPage(t, tenant, base, "/")
	if code != http.StatusOK || !strings.Contains(body, "My tenancy") {
		t.Fatalf("tenant dashboard = %d, want My tenancy link", code)
	}
	code, _ = getPage(t, tenant, base, "/condominium")
	if code != http.StatusForbidden {
		t.Fatalf("tenant /condominium = %d, want 403", code)
	}

	// Syndic creates an owner invitation.
	code, body = postForm(t, syndic, base, "/invitations", map[string]string{
		"unit_id": unitID, "invited_role": "owner", "invited_email": "owner@example.com", "csrf_token": csrf[1],
	})
	if code != http.StatusOK {
		t.Fatalf("owner invitation = %d", code)
	}
	ownerToken := rawTokenFromBody(body)

	// Owner registers and cannot access the condominium area yet.
	owner := newClient(t)
	code, _ = getPage(t, owner, base, "/register?token="+ownerToken)
	if code != http.StatusOK {
		t.Fatalf("owner register page = %d", code)
	}
	code, _ = postForm(t, owner, base, "/register", map[string]string{
		"token": ownerToken, "email": "owner@example.com", "password": "owner-pass-123",
		"password_confirm": "owner-pass-123", "csrf_token": csrfFrom(t, owner, base, "/register?token="+ownerToken),
	})
	if code != http.StatusSeeOther {
		t.Fatalf("owner register = %d", code)
	}
	code, _ = postForm(t, owner, base, "/login", map[string]string{
		"email": "owner@example.com", "password": "owner-pass-123",
		"csrf_token": csrfFrom(t, owner, base, "/login"),
	})
	if code != http.StatusSeeOther {
		t.Fatalf("owner login = %d", code)
	}
	code, body = getPage(t, owner, base, "/")
	if code != http.StatusOK || !strings.Contains(body, "My unit") {
		t.Fatalf("owner dashboard = %d, want My unit link", code)
	}
	code, _ = getPage(t, owner, base, "/condominium")
	if code != http.StatusForbidden {
		t.Fatalf("owner /condominium = %d, want 403 before syndic grant", code)
	}

	// Syndic grants the owner the syndic role; access is then allowed.
	code, body = getPage(t, syndic, base, "/roles")
	if code != http.StatusOK {
		t.Fatalf("roles page = %d", code)
	}
	syndicCSRF := csrfRe.FindStringSubmatch(body)
	if syndicCSRF == nil {
		t.Fatal("no csrf token on roles page")
	}
	ownerID := userIDFor(t, "owner@example.com", condoID)
	code, _ = postForm(t, syndic, base, "/roles/assign", map[string]string{
		"user_id": ownerID, "role": "syndic", "csrf_token": syndicCSRF[1],
	})
	if code != http.StatusSeeOther {
		t.Fatalf("assign syndic = %d, want 303", code)
	}
	code, _ = getPage(t, owner, base, "/condominium")
	if code != http.StatusOK {
		t.Fatalf("owner /condominium after grant = %d, want 200", code)
	}
}
