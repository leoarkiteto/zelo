package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/leoarkiteto/zelo/internal/features/tickets/domain"
	"github.com/leoarkiteto/zelo/internal/features/tickets/services"
	"github.com/leoarkiteto/zelo/internal/shared/middleware"
	"github.com/leoarkiteto/zelo/internal/shared/model"
)

type fakeRoles struct{ roles []model.Role }

func (f *fakeRoles) ActiveRolesForUser(_ context.Context, _, _ string) ([]model.Role, error) {
	return f.roles, nil
}

type fakeSessionReader struct{ sess model.Session }

func (f fakeSessionReader) Read(*http.Request) (model.Session, error) { return f.sess, nil }

type fakeUserLoader struct{ u model.User }

func (f fakeUserLoader) GetUserByID(_ context.Context, _ string) (model.User, error) { return f.u, nil }

type fakeUnits struct {
	unit    model.Unit
	units   []model.Unit
	unitErr error
}

func (f *fakeUnits) GetActiveUnitForUser(context.Context, string, string) (model.Unit, error) {
	return f.unit, f.unitErr
}

func (f *fakeUnits) ListUnitsForCondominium(context.Context, string) ([]model.Unit, error) {
	return f.units, nil
}

type fakeTickets struct {
	createErr error
	replyErr  error
	closeErr  error
	listMine  []domain.Ticket
	listInbox []domain.Ticket
	ticket    domain.Ticket
	ticketErr error
	replies   []domain.TicketReply
	lastInput services.TicketInput
	lastReply string
	lastClose string
	created   bool
}

func (f *fakeTickets) CreateTicket(_ context.Context, _, _ string, in services.TicketInput) (domain.Ticket, error) {
	f.created = true
	f.lastInput = in
	if f.createErr != nil {
		return domain.Ticket{}, f.createErr
	}
	return domain.Ticket{ID: "t1", Status: domain.StatusOpen, Category: in.Category, Description: in.Description}, nil
}

func (f *fakeTickets) ReplyToTicket(_ context.Context, _, id, _, content string) error {
	f.lastReply = id
	return f.replyErr
}

func (f *fakeTickets) CloseTicket(_ context.Context, _, id, _ string) error {
	f.lastClose = id
	return f.closeErr
}

func (f *fakeTickets) ListMine(context.Context, string, string) ([]domain.Ticket, error) {
	return f.listMine, nil
}

func (f *fakeTickets) ListInbox(context.Context, string, domain.TicketStatus) ([]domain.Ticket, error) {
	return f.listInbox, nil
}

func (f *fakeTickets) GetTicket(_ context.Context, _ string) (domain.Ticket, error) {
	return f.ticket, f.ticketErr
}

func (f *fakeTickets) ListReplies(context.Context, string) ([]domain.TicketReply, error) {
	return f.replies, nil
}

func ticketsRouter(user *model.User, sess *model.Session, deps Deps) http.Handler {
	mux := http.NewServeMux()
	RegisterRoutes(mux, deps)
	var root http.Handler = mux
	if user != nil && sess != nil {
		root = middleware.WithLocale(root)
		root = middleware.WithUser(fakeSessionReader{sess: *sess}, fakeUserLoader{u: *user})(root)
	}
	return root
}

func ownerSession() (*model.User, *model.Session) {
	user := &model.User{ID: "u1", Email: "owner@example.com", Status: model.UserStatusActive}
	sess := &model.Session{TokenHash: "th", UserID: "u1", CondominiumID: "c1", CSRFToken: "x"}
	return user, sess
}

func syndicSession() (*model.User, *model.Session) {
	user := &model.User{ID: "u2", Email: "syndic@example.com", Status: model.UserStatusActive}
	sess := &model.Session{TokenHash: "th", UserID: "u2", CondominiumID: "c1", CSRFToken: "x"}
	return user, sess
}

func baseDeps(tk *fakeTickets, roles ...model.Role) Deps {
	return Deps{
		Roles:   &fakeRoles{roles: roles},
		Units:   &fakeUnits{unit: model.Unit{ID: "u1", Code: "A-101"}},
		Tickets: tk,
	}
}

func openTicket(authorID string) domain.Ticket {
	return domain.Ticket{
		ID: "t1", CondominiumID: "c1", AuthorID: authorID, UnitID: "u1",
		Title: "Leak under the sink", Category: domain.CategoryRepair, Description: "Leak",
		Status: domain.StatusOpen, CreatedAt: time.Now(),
	}
}

func TestTicketsUnauthenticatedRedirects(t *testing.T) {
	router := ticketsRouter(nil, nil, baseDeps(&fakeTickets{}, model.RoleOwner))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/tickets", nil))
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/login" {
		t.Fatalf("GET /tickets = %d %q, want 303 /login", rec.Code, rec.Header().Get("Location"))
	}
}

func TestTicketsMemberAndSyndicAccess(t *testing.T) {
	user, sess := ownerSession()
	deps := baseDeps(&fakeTickets{}, model.RoleOwner)

	// Owner reaches member routes.
	rec := httptest.NewRecorder()
	ticketsRouter(user, sess, deps).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/tickets", nil))
	if rec.Code != http.StatusOK {
		t.Errorf("owner GET /tickets = %d, want 200", rec.Code)
	}

	// Owner is forbidden from the syndic inbox.
	rec = httptest.NewRecorder()
	ticketsRouter(user, sess, deps).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/tickets/inbox", nil))
	if rec.Code != http.StatusForbidden {
		t.Errorf("owner GET /tickets/inbox = %d, want 403", rec.Code)
	}

	// Syndic reaches the inbox.
	u2, s2 := syndicSession()
	deps2 := baseDeps(&fakeTickets{}, model.RoleSyndic)
	rec = httptest.NewRecorder()
	ticketsRouter(u2, s2, deps2).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/tickets/inbox", nil))
	if rec.Code != http.StatusOK {
		t.Errorf("syndic GET /tickets/inbox = %d, want 200", rec.Code)
	}
}

func TestTicketsNewGETRequiresActiveUnit(t *testing.T) {
	user, sess := ownerSession()
	deps := baseDeps(&fakeTickets{}, model.RoleOwner)
	deps.Units = &fakeUnits{unitErr: model.ErrNotFound}

	rec := httptest.NewRecorder()
	ticketsRouter(user, sess, deps).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/tickets/new", nil))
	if rec.Code != http.StatusForbidden {
		t.Errorf("GET /tickets/new without unit = %d, want 403", rec.Code)
	}
}

func TestTicketsNewGETWithUnit(t *testing.T) {
	user, sess := ownerSession()
	rec := httptest.NewRecorder()
	ticketsRouter(user, sess, baseDeps(&fakeTickets{}, model.RoleOwner)).
		ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/tickets/new", nil))
	if rec.Code != http.StatusOK {
		t.Errorf("GET /tickets/new = %d, want 200", rec.Code)
	}
}

func TestTicketsCreatePOST(t *testing.T) {
	user, sess := ownerSession()
	tk := &fakeTickets{}
	router := ticketsRouter(user, sess, baseDeps(tk, model.RoleOwner))

	form := url.Values{"title": {"Leak under the sink"}, "category": {"repair"}, "description": {"Leak under the sink"}}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/tickets", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	router.ServeHTTP(rec, req)
	if !tk.created {
		t.Fatal("CreateTicket was not called")
	}
	if tk.lastInput.Title != "Leak under the sink" || tk.lastInput.Category != domain.CategoryRepair || tk.lastInput.Description != "Leak under the sink" {
		t.Errorf("input = %+v, want title/category/description", tk.lastInput)
	}
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/tickets/t1?flash=created" {
		t.Errorf("POST /tickets = %d %q, want 303 /tickets/t1?flash=created", rec.Code, rec.Header().Get("Location"))
	}
}

func TestTicketsCreatePOSTValidationError(t *testing.T) {
	user, sess := ownerSession()
	tk := &fakeTickets{createErr: services.ErrInvalidDescription}
	router := ticketsRouter(user, sess, baseDeps(tk, model.RoleOwner))

	form := url.Values{"title": {"Leak"}, "category": {"repair"}, "description": {""}}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/tickets", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("POST /tickets with invalid description = %d, want 400", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "Description is required") {
		t.Errorf("expected a description error message, got %s", rec.Body.String()[:min(200, rec.Body.Len())])
	}
}

func TestInboxGETDefaultOpen(t *testing.T) {
	u2, s2 := syndicSession()
	tk := &fakeTickets{listInbox: []domain.Ticket{openTicket("u1")}}
	rec := httptest.NewRecorder()
	ticketsRouter(u2, s2, baseDeps(tk, model.RoleSyndic)).
		ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/tickets/inbox", nil))
	if rec.Code != http.StatusOK {
		t.Errorf("GET /tickets/inbox = %d, want 200", rec.Code)
	}
}

func TestTicketDetailOwnership(t *testing.T) {
	// Owner may open their own ticket.
	user, sess := ownerSession()
	tk := &fakeTickets{ticket: openTicket("u1")}
	rec := httptest.NewRecorder()
	ticketsRouter(user, sess, baseDeps(tk, model.RoleOwner)).
		ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/tickets/t1", nil))
	if rec.Code != http.StatusOK {
		t.Errorf("owner GET own ticket = %d, want 200", rec.Code)
	}

	// Owner is 404 for another resident's ticket (no existence leak, FR-009).
	tk2 := &fakeTickets{ticket: openTicket("someone-else")}
	rec = httptest.NewRecorder()
	ticketsRouter(user, sess, baseDeps(tk2, model.RoleOwner)).
		ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/tickets/t1", nil))
	if rec.Code != http.StatusNotFound {
		t.Errorf("owner GET foreign ticket = %d, want 404", rec.Code)
	}

	// Syndic may open any ticket of the condominium.
	u2, s2 := syndicSession()
	tk3 := &fakeTickets{ticket: openTicket("someone-else")}
	rec = httptest.NewRecorder()
	ticketsRouter(u2, s2, baseDeps(tk3, model.RoleSyndic)).
		ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/tickets/t1", nil))
	if rec.Code != http.StatusOK {
		t.Errorf("syndic GET any ticket = %d, want 200", rec.Code)
	}
}

func TestTicketReplyPOST(t *testing.T) {
	u2, s2 := syndicSession()
	tk := &fakeTickets{}
	router := ticketsRouter(u2, s2, baseDeps(tk, model.RoleSyndic))

	form := url.Values{"content": {"Scheduled for Friday"}}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/tickets/t1/reply", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetPathValue("id", "t1")
	router.ServeHTTP(rec, req)
	if tk.lastReply != "t1" {
		t.Errorf("reply id = %q, want t1", tk.lastReply)
	}
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/tickets/t1?flash=replied" {
		t.Errorf("POST reply = %d %q, want 303 /tickets/t1?flash=replied", rec.Code, rec.Header().Get("Location"))
	}
}

func TestTicketReplyPOSTClosed(t *testing.T) {
	u2, s2 := syndicSession()
	tk := &fakeTickets{replyErr: services.ErrClosed}
	router := ticketsRouter(u2, s2, baseDeps(tk, model.RoleSyndic))

	form := url.Values{"content": {"too late"}}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/tickets/t1/reply", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetPathValue("id", "t1")
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("POST reply on closed ticket = %d, want 400", rec.Code)
	}
}

func TestTicketClosePOST(t *testing.T) {
	u2, s2 := syndicSession()
	tk := &fakeTickets{}
	router := ticketsRouter(u2, s2, baseDeps(tk, model.RoleSyndic))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/tickets/t1/close", nil)
	req.SetPathValue("id", "t1")
	router.ServeHTTP(rec, req)
	if tk.lastClose != "t1" {
		t.Errorf("close id = %q, want t1", tk.lastClose)
	}
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/tickets/inbox?flash=closed" {
		t.Errorf("POST close = %d %q, want 303 /tickets/inbox?flash=closed", rec.Code, rec.Header().Get("Location"))
	}
}

func TestTicketClosePOSTErrors(t *testing.T) {
	u2, s2 := syndicSession()

	tk := &fakeTickets{closeErr: services.ErrClosed}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/tickets/t1/close", nil)
	req.SetPathValue("id", "t1")
	ticketsRouter(u2, s2, baseDeps(tk, model.RoleSyndic)).ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("close closed ticket = %d, want 400", rec.Code)
	}

	tk2 := &fakeTickets{closeErr: services.ErrNotFound}
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/tickets/t1/close", nil)
	req.SetPathValue("id", "t1")
	ticketsRouter(u2, s2, baseDeps(tk2, model.RoleSyndic)).ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Errorf("close missing ticket = %d, want 404", rec.Code)
	}
}

func TestTicketsGETList(t *testing.T) {
	user, sess := ownerSession()
	tk := &fakeTickets{listMine: []domain.Ticket{openTicket("u1")}}
	rec := httptest.NewRecorder()
	ticketsRouter(user, sess, baseDeps(tk, model.RoleOwner)).
		ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/tickets", nil))
	if rec.Code != http.StatusOK {
		t.Errorf("GET /tickets = %d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "Leak under the sink") {
		t.Errorf("list page should show the ticket title")
	}
	if !strings.Contains(rec.Body.String(), "Repair request") {
		t.Errorf("list page should show the category label")
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
