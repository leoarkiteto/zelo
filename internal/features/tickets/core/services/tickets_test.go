package services

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/leoarkiteto/zelo/internal/features/tickets/core/domain"
	"github.com/leoarkiteto/zelo/internal/shared/model"
)

type fakeTicketStore struct {
	tickets  map[string]domain.Ticket
	replies  map[string][]domain.TicketReply
	next     int
	replyErr error
}

func newFakeTicketStore() *fakeTicketStore {
	return &fakeTicketStore{tickets: map[string]domain.Ticket{}, replies: map[string][]domain.TicketReply{}}
}

func (f *fakeTicketStore) Create(_ context.Context, t domain.Ticket) (string, error) {
	f.next++
	id := "t" + string(rune('0'+f.next))
	t.ID = id
	t.CreatedAt = time.Now()
	f.tickets[id] = t
	return id, nil
}

func (f *fakeTicketStore) GetByID(_ context.Context, id string) (domain.Ticket, error) {
	t, ok := f.tickets[id]
	if !ok {
		return domain.Ticket{}, model.ErrNotFound
	}
	return t, nil
}

func (f *fakeTicketStore) ListMine(_ context.Context, condominiumID, authorID string) ([]domain.Ticket, error) {
	var out []domain.Ticket
	for _, t := range f.tickets {
		if t.CondominiumID == condominiumID && t.AuthorID == authorID {
			out = append(out, t)
		}
	}
	return out, nil
}

func (f *fakeTicketStore) ListInbox(_ context.Context, condominiumID string, status domain.TicketStatus) ([]domain.Ticket, error) {
	var out []domain.Ticket
	for _, t := range f.tickets {
		if t.CondominiumID == condominiumID && (status == "" || t.Status == status) {
			out = append(out, t)
		}
	}
	return out, nil
}

func (f *fakeTicketStore) AddReply(_ context.Context, r domain.TicketReply) (string, error) {
	if f.replyErr != nil {
		return "", f.replyErr
	}
	t, ok := f.tickets[r.TicketID]
	if !ok || t.Status != domain.StatusOpen {
		return "", model.ErrNotFound
	}
	f.next++
	id := "r" + string(rune('0'+f.next))
	r.ID = id
	r.CreatedAt = time.Now()
	f.replies[r.TicketID] = append(f.replies[r.TicketID], r)
	return id, nil
}

func (f *fakeTicketStore) ListReplies(_ context.Context, ticketID string) ([]domain.TicketReply, error) {
	return f.replies[ticketID], nil
}

func (f *fakeTicketStore) Close(_ context.Context, id, condominiumID, closedBy string) error {
	t, ok := f.tickets[id]
	if !ok || t.CondominiumID != condominiumID || t.Status != domain.StatusOpen {
		return model.ErrNotFound
	}
	now := time.Now()
	t.Status = domain.StatusClosed
	t.ClosedBy = closedBy
	t.ClosedAt = &now
	f.tickets[id] = t
	return nil
}

type fakeUnitResolver struct {
	unit model.Unit
	err  error
}

func (f *fakeUnitResolver) GetActiveUnitForUser(context.Context, string, string) (model.Unit, error) {
	return f.unit, f.err
}

type fakeAudit struct {
	events []model.AuditEventType
}

func (f *fakeAudit) RecordEvent(_ context.Context, _ *string, eventType model.AuditEventType, _ map[string]any) error {
	f.events = append(f.events, eventType)
	return nil
}

func newService(store *fakeTicketStore, units *fakeUnitResolver, audit *fakeAudit) *TicketService {
	return &TicketService{Tickets: store, Units: units, Audit: audit}
}

func validInput() TicketInput {
	return TicketInput{Title: "  Leak under the sink  ", Category: domain.CategoryRepair, Description: "  Leak under the sink  "}
}

// --- CreateTicket (US1) ---

func TestCreateTicketSuccess(t *testing.T) {
	store := newFakeTicketStore()
	units := &fakeUnitResolver{unit: model.Unit{ID: "u1"}}
	audit := &fakeAudit{}
	svc := newService(store, units, audit)

	got, err := svc.CreateTicket(context.Background(), "author", "c1", validInput())
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if got.Status != domain.StatusOpen {
		t.Errorf("status = %q, want open", got.Status)
	}
	if got.UnitID != "u1" || got.AuthorID != "author" || got.CondominiumID != "c1" {
		t.Errorf("ticket metadata mismatch: %+v", got)
	}
	if got.Title != "Leak under the sink" {
		t.Errorf("title not trimmed: %q", got.Title)
	}
	if got.Description != "Leak under the sink" {
		t.Errorf("description not trimmed: %q", got.Description)
	}
	if len(audit.events) != 1 || audit.events[0] != model.AuditTicketCreated {
		t.Errorf("audit events = %v, want [ticket_created]", audit.events)
	}
}

func TestCreateTicketRejectsInvalidTitle(t *testing.T) {
	svc := newService(newFakeTicketStore(), &fakeUnitResolver{unit: model.Unit{ID: "u1"}}, &fakeAudit{})
	for _, title := range []string{"", "   ", strings.Repeat("t", 121)} {
		in := validInput()
		in.Title = title
		if _, err := svc.CreateTicket(context.Background(), "a", "c1", in); !errors.Is(err, ErrInvalidTitle) {
			t.Errorf("title %q err = %v, want ErrInvalidTitle", title, err)
		}
	}
}

func TestCreateTicketRejectsInvalidCategory(t *testing.T) {
	svc := newService(newFakeTicketStore(), &fakeUnitResolver{unit: model.Unit{ID: "u1"}}, &fakeAudit{})
	in := validInput()
	in.Category = "plumbing"
	if _, err := svc.CreateTicket(context.Background(), "a", "c1", in); !errors.Is(err, ErrInvalidCategory) {
		t.Errorf("err = %v, want ErrInvalidCategory", err)
	}
}

func TestCreateTicketRejectsEmptyOrLongDescription(t *testing.T) {
	svc := newService(newFakeTicketStore(), &fakeUnitResolver{unit: model.Unit{ID: "u1"}}, &fakeAudit{})
	for _, desc := range []string{"", "   ", strings.Repeat("a", 2001)} {
		in := validInput()
		in.Description = desc
		if _, err := svc.CreateTicket(context.Background(), "a", "c1", in); !errors.Is(err, ErrInvalidDescription) {
			t.Errorf("desc %q err = %v, want ErrInvalidDescription", desc, err)
		}
	}
}

func TestCreateTicketRequiresActiveUnit(t *testing.T) {
	svc := newService(newFakeTicketStore(), &fakeUnitResolver{err: model.ErrNotFound}, &fakeAudit{})
	if _, err := svc.CreateTicket(context.Background(), "a", "c1", validInput()); !errors.Is(err, ErrNoUnit) {
		t.Errorf("err = %v, want ErrNoUnit", err)
	}
}

// --- ReplyToTicket (US2) ---

func TestReplyToTicketSuccess(t *testing.T) {
	store := newFakeTicketStore()
	id, err := store.Create(context.Background(), domain.Ticket{
		CondominiumID: "c1", AuthorID: "a", UnitID: "u1",
		Category: domain.CategoryRepair, Description: "d", Status: domain.StatusOpen,
	})
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
	audit := &fakeAudit{}
	svc := newService(store, &fakeUnitResolver{}, audit)

	if err := svc.ReplyToTicket(context.Background(), "syndic", id, "c1", "  Scheduled for Friday  "); err != nil {
		t.Fatalf("reply: %v", err)
	}
	replies, _ := store.ListReplies(context.Background(), id)
	if len(replies) != 1 || replies[0].Content != "Scheduled for Friday" || replies[0].AuthorID != "syndic" {
		t.Errorf("replies mismatch: %+v", replies)
	}
	if len(audit.events) != 1 || audit.events[0] != model.AuditTicketReplied {
		t.Errorf("audit events = %v, want [ticket_replied]", audit.events)
	}
}

func TestReplyToTicketRejectsClosed(t *testing.T) {
	store := newFakeTicketStore()
	now := time.Now()
	id, _ := store.Create(context.Background(), domain.Ticket{
		CondominiumID: "c1", AuthorID: "a", Category: domain.CategoryRepair,
		Description: "d", Status: domain.StatusClosed, ClosedBy: "syndic", ClosedAt: &now,
	})
	svc := newService(store, &fakeUnitResolver{}, &fakeAudit{})
	if err := svc.ReplyToTicket(context.Background(), "syndic", id, "c1", "too late"); !errors.Is(err, ErrClosed) {
		t.Errorf("err = %v, want ErrClosed", err)
	}
}

func TestReplyToTicketRejectsEmptyContent(t *testing.T) {
	store := newFakeTicketStore()
	id, _ := store.Create(context.Background(), domain.Ticket{
		CondominiumID: "c1", AuthorID: "a", Category: domain.CategoryRepair,
		Description: "d", Status: domain.StatusOpen,
	})
	svc := newService(store, &fakeUnitResolver{}, &fakeAudit{})
	if err := svc.ReplyToTicket(context.Background(), "syndic", id, "c1", "   "); !errors.Is(err, ErrInvalidContent) {
		t.Errorf("err = %v, want ErrInvalidContent", err)
	}
}

func TestReplyToTicketForeignCondominium(t *testing.T) {
	store := newFakeTicketStore()
	id, _ := store.Create(context.Background(), domain.Ticket{
		CondominiumID: "c1", AuthorID: "a", Category: domain.CategoryRepair,
		Description: "d", Status: domain.StatusOpen,
	})
	svc := newService(store, &fakeUnitResolver{}, &fakeAudit{})
	if err := svc.ReplyToTicket(context.Background(), "syndic", id, "c2", "hello"); !errors.Is(err, ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

// --- CloseTicket (US3) ---

func TestCloseTicketSuccess(t *testing.T) {
	store := newFakeTicketStore()
	id, _ := store.Create(context.Background(), domain.Ticket{
		CondominiumID: "c1", AuthorID: "a", Category: domain.CategoryRepair,
		Description: "d", Status: domain.StatusOpen,
	})
	audit := &fakeAudit{}
	svc := newService(store, &fakeUnitResolver{}, audit)

	if err := svc.CloseTicket(context.Background(), "syndic", id, "c1"); err != nil {
		t.Fatalf("close: %v", err)
	}
	closed, _ := store.GetByID(context.Background(), id)
	if closed.Status != domain.StatusClosed || closed.ClosedBy != "syndic" || closed.ClosedAt == nil {
		t.Errorf("closed ticket mismatch: %+v", closed)
	}
	if len(audit.events) != 1 || audit.events[0] != model.AuditTicketClosed {
		t.Errorf("audit events = %v, want [ticket_closed]", audit.events)
	}
}

func TestCloseTicketAllowsWithoutReply(t *testing.T) {
	store := newFakeTicketStore()
	id, _ := store.Create(context.Background(), domain.Ticket{
		CondominiumID: "c1", AuthorID: "a", Category: domain.CategoryNoiseComplaint,
		Description: "d", Status: domain.StatusOpen,
	})
	svc := newService(store, &fakeUnitResolver{}, &fakeAudit{})
	if err := svc.CloseTicket(context.Background(), "syndic", id, "c1"); err != nil {
		t.Errorf("closing without a reply must be allowed: %v", err)
	}
}

func TestCloseTicketRejectsClosed(t *testing.T) {
	store := newFakeTicketStore()
	now := time.Now()
	id, _ := store.Create(context.Background(), domain.Ticket{
		CondominiumID: "c1", AuthorID: "a", Category: domain.CategoryRepair,
		Description: "d", Status: domain.StatusClosed, ClosedBy: "syndic", ClosedAt: &now,
	})
	svc := newService(store, &fakeUnitResolver{}, &fakeAudit{})
	if err := svc.CloseTicket(context.Background(), "syndic", id, "c1"); !errors.Is(err, ErrClosed) {
		t.Errorf("err = %v, want ErrClosed", err)
	}
}

// --- ListMine (US4) ---

func TestListMineReturnsOnlyOwnTickets(t *testing.T) {
	store := newFakeTicketStore()
	svc := newService(store, &fakeUnitResolver{}, &fakeAudit{})
	ctx := context.Background()

	svc.CreateTicket(ctx, "a1", "c1", validInput())
	svc.CreateTicket(ctx, "a2", "c1", validInput())
	svc.CreateTicket(ctx, "a1", "c1", validInput())

	mine, err := svc.ListMine(ctx, "a1", "c1")
	if err != nil {
		t.Fatalf("list mine: %v", err)
	}
	if len(mine) != 2 {
		t.Errorf("len(mine) = %d, want 2 (only a1's tickets)", len(mine))
	}
	for _, tk := range mine {
		if tk.AuthorID != "a1" {
			t.Errorf("foreign ticket leaked into ListMine: %+v", tk)
		}
	}
}
