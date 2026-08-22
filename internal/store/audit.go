package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/leoarkiteto/zelo/internal/model"
)

// AuditStore persists security-relevant events (FR-013).
type AuditStore struct {
	db *sql.DB
}

// NewAuditStore creates an AuditStore.
func NewAuditStore(db *sql.DB) *AuditStore { return &AuditStore{db: db} }

// RecordEvent appends an audit event.
func (s *AuditStore) RecordEvent(ctx context.Context, userID *string, eventType model.AuditEventType, details map[string]any) error {
	if !eventType.Valid() {
		return fmt.Errorf("invalid audit event type %q", eventType)
	}
	payload, err := json.Marshal(details)
	if err != nil {
		return fmt.Errorf("marshal audit details: %w", err)
	}
	var uid any
	if userID != nil {
		uid = *userID
	}
	_, err = s.db.ExecContext(ctx,
		`INSERT INTO audit_events (user_id, event_type, details) VALUES ($1, $2, $3)`,
		uid, eventType, payload)
	if err != nil {
		return fmt.Errorf("record audit event: %w", err)
	}
	return nil
}
