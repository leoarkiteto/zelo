package repositories

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/leoarkiteto/zelo/internal/features/tickets/domain"
	sharedstore "github.com/leoarkiteto/zelo/internal/shared/store"
	"github.com/leoarkiteto/zelo/internal/shared/testutil"
)

func openTicketsTestDB(t *testing.T) *sql.DB {
	t.Helper()
	url := testutil.TestDatabaseURL("tickets")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping integration test")
	}
	ctx := context.Background()
	db, err := sharedstore.Open(url)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := sharedstore.Migrate(ctx, db, "../../../../migrations"); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if _, err := db.ExecContext(ctx, `
		TRUNCATE ticket_replies, tickets, unit_occupancies, user_roles, units,
		condominiums, users RESTART IDENTITY CASCADE`); err != nil {
		t.Fatalf("truncate: %v", err)
	}
	return db
}

func seedTicketsFixture(t *testing.T, db *sql.DB) (condoID, unitID, authorID, syndicID string) {
	t.Helper()
	ctx := context.Background()
	if err := db.QueryRowContext(ctx,
		`INSERT INTO condominiums (name) VALUES ('Ticket Condo') RETURNING id`).Scan(&condoID); err != nil {
		t.Fatalf("seed condo: %v", err)
	}
	if err := db.QueryRowContext(ctx,
		`INSERT INTO units (condominium_id, code) VALUES ($1, 'A-1') RETURNING id`, condoID).Scan(&unitID); err != nil {
		t.Fatalf("seed unit: %v", err)
	}
	if err := db.QueryRowContext(ctx,
		`INSERT INTO users (email, password_hash) VALUES ('author@example.com', 'x') RETURNING id`).Scan(&authorID); err != nil {
		t.Fatalf("seed author: %v", err)
	}
	if err := db.QueryRowContext(ctx,
		`INSERT INTO users (email, password_hash) VALUES ('syndic@example.com', 'x') RETURNING id`).Scan(&syndicID); err != nil {
		t.Fatalf("seed syndic: %v", err)
	}
	return condoID, unitID, authorID, syndicID
}

func baseTicket(condoID, unitID, authorID string) domain.Ticket {
	return domain.Ticket{
		CondominiumID: condoID,
		AuthorID:      authorID,
		UnitID:        unitID,
		Title:         "Leak under the kitchen sink",
		Category:      domain.CategoryRepair,
		Description:   "Leak under the kitchen sink",
		Status:        domain.StatusOpen,
	}
}

func TestTicketStoreCreateGet(t *testing.T) {
	db := openTicketsTestDB(t)
	store := NewTicketStore(db)
	condoID, unitID, authorID, _ := seedTicketsFixture(t, db)

	id, err := store.Create(context.Background(), baseTicket(condoID, unitID, authorID))
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	got, err := store.GetByID(context.Background(), id)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.CondominiumID != condoID || got.AuthorID != authorID || got.UnitID != unitID {
		t.Errorf("ticket metadata mismatch: %+v", got)
	}
	if got.Status != domain.StatusOpen || got.Category != domain.CategoryRepair || got.Description != "Leak under the kitchen sink" {
		t.Errorf("ticket fields mismatch: %+v", got)
	}
	if got.Title != "Leak under the kitchen sink" {
		t.Errorf("ticket title mismatch: %q", got.Title)
	}
	if got.ClosedAt != nil || got.ClosedBy != "" {
		t.Errorf("open ticket must not be closed: %+v", got)
	}

	if _, err := store.GetByID(context.Background(), "00000000-0000-0000-0000-000000000000"); !errors.Is(err, sharedstore.ErrNotFound) {
		t.Errorf("missing ticket error = %v, want ErrNotFound", err)
	}
}

func TestTicketStoreListMineAndInbox(t *testing.T) {
	db := openTicketsTestDB(t)
	store := NewTicketStore(db)
	condoID, unitID, authorID, _ := seedTicketsFixture(t, db)
	ctx := context.Background()

	firstID, err := store.Create(ctx, baseTicket(condoID, unitID, authorID))
	if err != nil {
		t.Fatalf("create first: %v", err)
	}
	second := baseTicket(condoID, unitID, authorID)
	second.Category = domain.CategoryNoiseComplaint
	second.Description = "Loud music after midnight"
	secondID, err := store.Create(ctx, second)
	if err != nil {
		t.Fatalf("create second: %v", err)
	}

	mine, err := store.ListMine(ctx, condoID, authorID)
	if err != nil {
		t.Fatalf("list mine: %v", err)
	}
	if len(mine) != 2 {
		t.Fatalf("len(ListMine) = %d, want 2", len(mine))
	}
	if mine[0].ID != secondID && mine[1].ID != secondID {
		t.Error("ListMine must return both tickets")
	}

	open, err := store.ListInbox(ctx, condoID, domain.StatusOpen)
	if err != nil {
		t.Fatalf("list open inbox: %v", err)
	}
	if len(open) != 2 || open[0].ID != firstID {
		t.Errorf("open inbox must be FIFO (oldest first): %+v", open)
	}

	all, err := store.ListInbox(ctx, condoID, "")
	if err != nil {
		t.Fatalf("list all: %v", err)
	}
	if len(all) != 2 {
		t.Errorf("len(all) = %d, want 2", len(all))
	}
}

func TestTicketStoreReplyAndCloseLifecycle(t *testing.T) {
	db := openTicketsTestDB(t)
	store := NewTicketStore(db)
	condoID, unitID, authorID, syndicID := seedTicketsFixture(t, db)
	ctx := context.Background()

	id, err := store.Create(ctx, baseTicket(condoID, unitID, authorID))
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	// Reply to an open ticket.
	replyID, err := store.AddReply(ctx, domain.TicketReply{TicketID: id, AuthorID: syndicID, Content: "A visit is scheduled."})
	if err != nil {
		t.Fatalf("add reply: %v", err)
	}
	if replyID == "" {
		t.Error("reply id must not be empty")
	}
	replies, err := store.ListReplies(ctx, id)
	if err != nil {
		t.Fatalf("list replies: %v", err)
	}
	if len(replies) != 1 || replies[0].Content != "A visit is scheduled." || replies[0].AuthorID != syndicID {
		t.Errorf("replies mismatch: %+v", replies)
	}

	// Close the open ticket.
	if err := store.Close(ctx, id, condoID, syndicID); err != nil {
		t.Fatalf("close: %v", err)
	}
	closed, err := store.GetByID(ctx, id)
	if err != nil {
		t.Fatalf("get closed: %v", err)
	}
	if closed.Status != domain.StatusClosed || closed.ClosedBy != syndicID || closed.ClosedAt == nil {
		t.Errorf("closed ticket mismatch: %+v", closed)
	}
	if closed.CanReply() || closed.CanClose() {
		t.Error("closed ticket must be read-only")
	}

	// Closed tickets reject further replies at the data layer (FR-008).
	if _, err := store.AddReply(ctx, domain.TicketReply{TicketID: id, AuthorID: syndicID, Content: "Too late"}); !errors.Is(err, sharedstore.ErrNotFound) {
		t.Errorf("reply on closed ticket error = %v, want ErrNotFound", err)
	}
	if err := store.Close(ctx, id, condoID, syndicID); !errors.Is(err, sharedstore.ErrNotFound) {
		t.Errorf("close on closed ticket error = %v, want ErrNotFound", err)
	}

	// Closed tickets leave the open inbox.
	open, err := store.ListInbox(ctx, condoID, domain.StatusOpen)
	if err != nil {
		t.Fatalf("list open inbox: %v", err)
	}
	if len(open) != 0 {
		t.Errorf("open inbox after close = %d, want 0", len(open))
	}
	closedList, err := store.ListInbox(ctx, condoID, domain.StatusClosed)
	if err != nil {
		t.Fatalf("list closed inbox: %v", err)
	}
	if len(closedList) != 1 {
		t.Errorf("closed inbox = %d, want 1", len(closedList))
	}
}
