package model

import "time"

// AuditEventType enumerates security-relevant events (spec FR-013).
type AuditEventType string

const (
	AuditSignIn              AuditEventType = "sign_in"
	AuditFailedSignIn        AuditEventType = "failed_sign_in"
	AuditSignOut             AuditEventType = "sign_out"
	AuditAccountLocked       AuditEventType = "account_locked"
	AuditRoleGranted         AuditEventType = "role_granted"
	AuditRoleRevoked         AuditEventType = "role_revoked"
	AuditAccessDenied        AuditEventType = "access_denied"
	AuditListingCreated      AuditEventType = "listing_created"
	AuditListingEdited       AuditEventType = "listing_edited"
	AuditListingDeleted      AuditEventType = "listing_deleted"
	AuditCategoryCreated     AuditEventType = "category_created"
	AuditCategoryRenamed     AuditEventType = "category_renamed"
	AuditCategoryDeactivated AuditEventType = "category_deactivated"
	AuditAccountCreated      AuditEventType = "account_created"
	AuditAccountEdited       AuditEventType = "account_edited"
	AuditAccountSettled      AuditEventType = "account_settled"
	AuditAccountCanceled     AuditEventType = "account_canceled"
	AuditTicketCreated       AuditEventType = "ticket_created"
	AuditTicketReplied       AuditEventType = "ticket_replied"
	AuditTicketClosed        AuditEventType = "ticket_closed"

	AuditCondominiumCurrencyChanged AuditEventType = "condominium_currency_changed"
	AuditSupplierCreated            AuditEventType = "supplier_created"
	AuditSupplierUpdated            AuditEventType = "supplier_updated"
	AuditInvoiceCreated             AuditEventType = "invoice_created"
	AuditInvoiceUpdated             AuditEventType = "invoice_updated"
	AuditInvoiceRegistered          AuditEventType = "invoice_registered"
	AuditInvoiceDraftDeleted        AuditEventType = "invoice_draft_deleted"
	AuditInvoiceAnnulled            AuditEventType = "invoice_annulled"
	AuditInvoicePaymentAdded        AuditEventType = "invoice_payment_added"
	AuditInvoiceAttachmentAdded     AuditEventType = "invoice_attachment_added"
	AuditInvoiceAttachmentRemoved   AuditEventType = "invoice_attachment_removed"

	AuditUnitCreated           AuditEventType = "unit_created"
	AuditUnitUpdated           AuditEventType = "unit_updated"
	AuditBudgetCreated         AuditEventType = "budget_created"
	AuditBudgetUpdated         AuditEventType = "budget_updated"
	AuditBudgetApproved        AuditEventType = "budget_approved"
	AuditBudgetReopened        AuditEventType = "budget_reopened"
	AuditQuotaIssued           AuditEventType = "quota_issued"
	AuditQuotaIssuanceCanceled AuditEventType = "quota_issuance_canceled"
)

// Valid reports whether t is a supported audit event type.
func (t AuditEventType) Valid() bool {
	switch t {
	case AuditSignIn, AuditFailedSignIn, AuditSignOut, AuditAccountLocked,
		AuditRoleGranted, AuditRoleRevoked, AuditAccessDenied,
		AuditListingCreated, AuditListingEdited, AuditListingDeleted,
		AuditCategoryCreated, AuditCategoryRenamed, AuditCategoryDeactivated,
		AuditAccountCreated, AuditAccountEdited, AuditAccountSettled, AuditAccountCanceled,
		AuditTicketCreated, AuditTicketReplied, AuditTicketClosed,
		AuditCondominiumCurrencyChanged, AuditSupplierCreated, AuditSupplierUpdated,
		AuditInvoiceCreated, AuditInvoiceUpdated, AuditInvoiceRegistered,
		AuditInvoiceDraftDeleted, AuditInvoiceAnnulled, AuditInvoicePaymentAdded,
		AuditInvoiceAttachmentAdded, AuditInvoiceAttachmentRemoved,
		AuditUnitCreated, AuditUnitUpdated, AuditBudgetCreated, AuditBudgetUpdated,
		AuditBudgetApproved, AuditBudgetReopened, AuditQuotaIssued, AuditQuotaIssuanceCanceled:
		return true
	default:
		return false
	}
}

// AuditEvent is a security-relevant event record.
type AuditEvent struct {
	ID        string
	UserID    *string
	EventType AuditEventType
	Details   map[string]any
	CreatedAt time.Time
}
