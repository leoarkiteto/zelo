package main

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/leoarkiteto/zelo/internal/shared/security"
)

// Development seed constants. Every seeded user shares the same password so
// the demo data is easy to explore (see the -seed flag help).
const (
	seedCondominiumName = "Residencial Riviera"
	seedSyndicEmail     = "syndic@example.com"
	seedPassword        = "demo-password-123"
)

// seedOccupant describes one seeded resident of the demo condominium.
type seedOccupant struct {
	email   string // login email
	unit    string // unit code, must exist in seedUnits
	tenancy string // occupancy type: "owner" | "tenant"
	role    string // condominium role: "owner" | "tenant" (the syndic gets "syndic" too)
	lang    string // interface language: "", "en" or "pt-br"
}

// seedUnits are the demo condominium's units; B-301/B-302 stay unoccupied so
// the management screens have assignable units.
var seedUnits = []string{
	"A-101", "A-102", "A-201", "A-202", "A-301", "A-302",
	"B-101", "B-102", "B-201", "B-202", "B-301", "B-302",
}

var seedOccupants = []seedOccupant{
	{email: "syndic@example.com", unit: "A-101", tenancy: "owner", role: "owner", lang: "en"},
	{email: "maria.oliveira@example.com", unit: "A-102", tenancy: "owner", role: "owner", lang: "pt-br"},
	{email: "joao.santos@example.com", unit: "A-201", tenancy: "owner", role: "owner", lang: "pt-br"},
	{email: "carla.pereira@example.com", unit: "B-101", tenancy: "owner", role: "owner", lang: "en"},
	{email: "pedro.lima@example.com", unit: "B-201", tenancy: "owner", role: "owner", lang: "pt-br"},
	{email: "ana.costa@example.com", unit: "A-301", tenancy: "owner", role: "owner", lang: "en"},
	{email: "bruno.almeida@example.com", unit: "A-202", tenancy: "tenant", role: "tenant", lang: "pt-br"},
	{email: "julia.rocha@example.com", unit: "B-102", tenancy: "tenant", role: "tenant", lang: "en"},
	{email: "rafael.martins@example.com", unit: "B-202", tenancy: "tenant", role: "tenant", lang: "pt-br"},
	{email: "fernanda.souza@example.com", unit: "A-302", tenancy: "tenant", role: "tenant", lang: "pt-br"},
}

// seedListing describes a service provider recommendation in the directory.
type seedListing struct {
	name        string
	category    string
	phone       string
	notes       string
	unit        string // recommended_by_unit_code
	submittedBy string // submitter email
}

var seedListings = []seedListing{
	{name: "Zé Encanamentos", category: "Plumbing", phone: "(11) 98877-1122",
		notes: "Ótimo atendimento, resolveu um vazamento no AP A-201 em menos de um dia.", unit: "A-201", submittedBy: "joao.santos@example.com"},
	{name: "Manutenção Predial Santos", category: "Plumbing", phone: "(11) 93322-8877",
		notes: "Bombeiro hidráulico, manutenção de caixa d'água e aquecedores.", unit: "A-101", submittedBy: "syndic@example.com"},
	{name: "Luz & Força Elétrica", category: "Electrical", phone: "(11) 97654-3344",
		notes: "Elétrica predial e residencial com nota fiscal.", unit: "A-101", submittedBy: "syndic@example.com"},
	{name: "Ar-Condicionado Frio Total", category: "Electrical", phone: "(11) 96611-7788",
		notes: "Instalação e manutenção de ar-condicionado.", unit: "B-102", submittedBy: "julia.rocha@example.com"},
	{name: "Dona Bete Limpeza", category: "Cleaning", phone: "(11) 95500-2211",
		notes: "Faxinas semanais e limpeza pós-obra.", unit: "A-102", submittedBy: "maria.oliveira@example.com"},
	{name: "Pinturas Silva", category: "Painting", phone: "(11) 94433-9090",
		notes: "Pintura de apartamentos e das áreas comuns.", unit: "A-301", submittedBy: "ana.costa@example.com"},
	{name: "Verde Jardim", category: "Gardening", phone: "(11) 98811-5566",
		notes: "Jardinagem e poda — cuida do jardim do hall todos os sábados.", unit: "B-101", submittedBy: "carla.pereira@example.com"},
	{name: "Dedetizadora Livre de Pragas", category: "Pest Control", phone: "(11) 97700-1122",
		notes: "Dedetização trimestral de todo o prédio.", unit: "A-101", submittedBy: "syndic@example.com"},
	{name: "Chaveiro 24h Rápido", category: "Locksmith", phone: "(11) 99988-4433",
		notes: "Abertura de portas e troca de fechaduras, atende a qualquer hora.", unit: "B-202", submittedBy: "rafael.martins@example.com"},
	{name: "Gás & Encanações Prado", category: "Plumbing", phone: "(11) 91100-3344",
		notes: "Instalação de aquecedores e encanamentos.", unit: "A-302", submittedBy: "fernanda.souza@example.com"},
}

var seedCategories = []string{
	"Plumbing", "Electrical", "Cleaning", "Painting", "Gardening", "Pest Control", "Locksmith",
}

// seedPayable describes one accounts-payable entry. Due dates are computed
// relative to "now" so the demo always has pending, overdue, and settled data.
type seedPayable struct {
	title           string
	category        string
	amountCents     int64
	dueInDays       int
	status          string // "pending" | "settled" | "canceled"
	settleAfterDays int    // settlement date = due date + this offset (0 when not settled)
	supplier        string
}

var seedPayables = []seedPayable{
	{title: "Manutenção predial mensal", category: "maintenance", amountCents: 185000, dueInDays: 10, status: "pending", supplier: "Predial Serviços Ltda"},
	{title: "Limpeza das áreas comuns", category: "cleaning", amountCents: 95000, dueInDays: -3, status: "pending", supplier: "LimpaTudo Serviços"},
	{title: "Energia elétrica (Enel)", category: "utilities", amountCents: 234567, dueInDays: -45, status: "settled", settleAfterDays: 5, supplier: "Enel Distribuição"},
	{title: "Água e esgoto (Sabesp)", category: "utilities", amountCents: 89000, dueInDays: -20, status: "settled", settleAfterDays: 2, supplier: "Sabesp"},
	{title: "Salário do zelador", category: "payroll", amountCents: 320000, dueInDays: -5, status: "settled", settleAfterDays: 3, supplier: "Pedro Costa (zelador)"},
	{title: "Auditoria contábil anual", category: "third_party_services", amountCents: 150000, dueInDays: 30, status: "pending", supplier: "Andrade Contabilidade"},
	{title: "Manutenção dos elevadores", category: "maintenance", amountCents: 120000, dueInDays: -60, status: "settled", settleAfterDays: 5, supplier: "Elevadores Modernos Ltda"},
	{title: "Seguro do condomínio", category: "other", amountCents: 75000, dueInDays: 20, status: "pending", supplier: "Seguros Proteja"},
	{title: "Dedetização trimestral", category: "third_party_services", amountCents: 60000, dueInDays: -10, status: "settled", settleAfterDays: 3, supplier: "Livre de Pragas Dedetização"},
	{title: "Pintura do hall de entrada", category: "maintenance", amountCents: 480000, dueInDays: 45, status: "pending", supplier: "Pinturas Silva"},
}

// seedFee describes one unit's condo fee receivable.
type seedFee struct {
	unit        string
	amountCents int64
	status      string // "settled" | "pending" | "overdue" (pending with a past due date)
}

var seedFees = []seedFee{
	{unit: "A-101", amountCents: 115000, status: "settled"},
	{unit: "A-102", amountCents: 115000, status: "settled"},
	{unit: "A-201", amountCents: 121000, status: "pending"},
	{unit: "A-202", amountCents: 121000, status: "pending"},
	{unit: "A-301", amountCents: 98000, status: "overdue"},
	{unit: "A-302", amountCents: 98000, status: "pending"},
	{unit: "B-101", amountCents: 133000, status: "settled"},
	{unit: "B-102", amountCents: 133000, status: "pending"},
	{unit: "B-201", amountCents: 139000, status: "overdue"},
	{unit: "B-202", amountCents: 139000, status: "pending"},
}

// seedInvitation describes a pending invitation to join the condominium.
type seedInvitation struct {
	email     string
	unit      string
	role      string // "owner" | "tenant"
	expiresIn time.Duration
}

var seedInvitations = []seedInvitation{
	{email: "novo.morador@example.com", unit: "B-301", role: "owner", expiresIn: 30 * 24 * time.Hour},
	{email: "renata.futura@example.com", unit: "B-302", role: "tenant", expiresIn: 14 * 24 * time.Hour},
}

// seedDemoData populates the database with realistic development data covering
// every feature: a condominium with units, residents (owners, tenants, syndic),
// the service directory, finance accounts, invitations, and audit events.
// It is idempotent: it exits cleanly if the seed syndic already exists.
func seedDemoData(ctx context.Context, logger *slog.Logger, db *sql.DB, passwordPepper string) error {
	var existing string
	err := db.QueryRowContext(ctx,
		`SELECT id FROM users WHERE lower(email) = lower($1)`, seedSyndicEmail).Scan(&existing)
	if err == nil {
		logger.Info("development seed already present, skipping",
			"syndic", seedSyndicEmail,
			"hint", "reset the database (docker compose down -v) and re-run -seed to reseed")
		return nil
	}
	if err != sql.ErrNoRows {
		return fmt.Errorf("check seed sentinel: %w", err)
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin seed transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	hasher := security.NewPasswordHasher(passwordPepper)
	hash, err := hasher.Hash(seedPassword)
	if err != nil {
		return fmt.Errorf("hash seed password: %w", err)
	}

	// 1. Condominium and units.
	var condoID string
	if err := tx.QueryRowContext(ctx,
		`INSERT INTO condominiums (name) VALUES ($1) RETURNING id`, seedCondominiumName).Scan(&condoID); err != nil {
		return fmt.Errorf("seed condominium: %w", err)
	}
	unitIDs := make(map[string]string, len(seedUnits))
	for _, code := range seedUnits {
		var id string
		if err := tx.QueryRowContext(ctx,
			`INSERT INTO units (condominium_id, code) VALUES ($1, $2) RETURNING id`,
			condoID, code).Scan(&id); err != nil {
			return fmt.Errorf("seed unit %s: %w", code, err)
		}
		unitIDs[code] = id
	}

	// 2. Users (one shared demo password).
	userIDs := make(map[string]string, len(seedOccupants))
	for _, o := range seedOccupants {
		var id string
		if err := tx.QueryRowContext(ctx,
			`INSERT INTO users (email, password_hash, language_preference) VALUES ($1, $2, $3) RETURNING id`,
			o.email, hash, nullString(o.lang)).Scan(&id); err != nil {
			return fmt.Errorf("seed user %s: %w", o.email, err)
		}
		userIDs[o.email] = id
	}

	// 3. Occupancies and condominium roles.
	for _, o := range seedOccupants {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO unit_occupancies (user_id, unit_id, occupancy_type) VALUES ($1, $2, $3)`,
			userIDs[o.email], unitIDs[o.unit], o.tenancy); err != nil {
			return fmt.Errorf("seed occupancy for %s: %w", o.email, err)
		}
		roles := []string{o.role}
		if o.email == seedSyndicEmail {
			roles = append(roles, "syndic")
		}
		for _, role := range roles {
			if _, err := tx.ExecContext(ctx,
				`INSERT INTO user_roles (user_id, condominium_id, role) VALUES ($1, $2, $3)`,
				userIDs[o.email], condoID, role); err != nil {
				return fmt.Errorf("seed role %s for %s: %w", role, o.email, err)
			}
		}
	}

	// 4. Service directory: categories then listings.
	categoryIDs := make(map[string]string, len(seedCategories))
	for _, name := range seedCategories {
		var id string
		if err := tx.QueryRowContext(ctx,
			`INSERT INTO service_categories (condominium_id, name) VALUES ($1, $2) RETURNING id`,
			condoID, name).Scan(&id); err != nil {
			return fmt.Errorf("seed category %s: %w", name, err)
		}
		categoryIDs[name] = id
	}
	for _, l := range seedListings {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO service_provider_listings
				(condominium_id, category_id, name, phone, phone_digits, notes,
				 recommended_by_unit_id, recommended_by_unit_code, submitted_by)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
			condoID, categoryIDs[l.category], l.name, l.phone, phoneDigits(l.phone),
			nullString(l.notes), unitIDs[l.unit], l.unit, userIDs[l.submittedBy]); err != nil {
			return fmt.Errorf("seed listing %s: %w", l.name, err)
		}
	}

	// 5. Finance accounts: payables and per-unit condo fee receivables.
	now := time.Now()
	for _, p := range seedPayables {
		dueDate := now.AddDate(0, 0, p.dueInDays).UTC().Truncate(24 * time.Hour)
		var settlementDate any
		if p.status == "settled" {
			settlementDate = dueDate.AddDate(0, 0, p.settleAfterDays)
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO finance_accounts
				(condominium_id, account_type, title, category, amount_cents, due_date,
				 status, settlement_date, supplier_payee, created_by)
			VALUES ($1, 'payable', $2, $3, $4, $5, $6, $7, $8, $9)`,
			condoID, p.title, p.category, p.amountCents, dueDate, p.status,
			settlementDate, p.supplier, userIDs[seedSyndicEmail]); err != nil {
			return fmt.Errorf("seed payable %s: %w", p.title, err)
		}
	}

	lastMonth10 := monthDay(now, -1, 10)
	nextMonth10 := monthDay(now, 1, 10)
	for _, f := range seedFees {
		dueDate := nextMonth10
		status := f.status
		if status == "overdue" {
			status = "pending" // "overdue" is derived at read time, never persisted
			dueDate = lastMonth10
		} else if status == "settled" {
			dueDate = lastMonth10
		}
		var settlementDate any
		if f.status == "settled" {
			settlementDate = monthDay(now, -1, 15)
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO finance_accounts
				(condominium_id, account_type, title, category, amount_cents, due_date,
				 status, settlement_date, unit_id, payment_code, created_by)
			VALUES ($1, 'receivable', $2, 'condo_fee', $3, $4, $5, $6, $7, $8, $9)`,
			condoID, fmt.Sprintf("Taxa condominial — AP %s", f.unit), f.amountCents, dueDate,
			status, settlementDate, unitIDs[f.unit], fmt.Sprintf("BOLETO-%s-%s", dueDate.Format("2006-01"), f.unit),
			userIDs[seedSyndicEmail]); err != nil {
			return fmt.Errorf("seed condo fee for %s: %w", f.unit, err)
		}
	}

	extraReceivables := []struct {
		title          string
		category       string
		amountCents    int64
		dueDate        time.Time
		status         string
		settlementDate any
		unit           string
		paymentCode    string
	}{
		{title: "Juros de mora — AP B-202", category: "fine_interest", amountCents: 2843,
			dueDate: now.AddDate(0, 0, 15).UTC().Truncate(24 * time.Hour), status: "pending",
			unit: "B-202", paymentCode: "JUROS-B202"},
		{title: "Reserva do salão de festas (aniversário)", category: "common_area_reservation",
			amountCents: 30000, dueDate: now.AddDate(0, 0, -8).UTC().Truncate(24 * time.Hour),
			status: "settled", settlementDate: now.AddDate(0, 0, -6).UTC().Truncate(24 * time.Hour),
			paymentCode: "RESERVA-SALAO"},
		{title: "Cota extra — obra da fachada", category: "extraordinary_income",
			amountCents: 250000, dueDate: now.AddDate(0, 0, -70).UTC().Truncate(24 * time.Hour),
			status: "settled", settlementDate: now.AddDate(0, 0, -60).UTC().Truncate(24 * time.Hour),
			paymentCode: "COTA-EXTRA-OBRA"},
	}
	for _, r := range extraReceivables {
		var unitID any
		if r.unit != "" {
			unitID = unitIDs[r.unit]
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO finance_accounts
				(condominium_id, account_type, title, category, amount_cents, due_date,
				 status, settlement_date, unit_id, payment_code, created_by)
			VALUES ($1, 'receivable', $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
			condoID, r.title, r.category, r.amountCents, r.dueDate, r.status,
			r.settlementDate, unitID, r.paymentCode, userIDs[seedSyndicEmail]); err != nil {
			return fmt.Errorf("seed receivable %s: %w", r.title, err)
		}
	}

	// 6. Pending invitations to unoccupied units.
	for _, inv := range seedInvitations {
		token := invitationTokenHash(inv.email)
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO invitations
				(token_hash, condominium_id, unit_id, invited_role, invited_email, status, expires_at, created_by)
			VALUES ($1, $2, $3, $4, $5, 'pending', $6, $7)`,
			token, condoID, unitIDs[inv.unit], inv.role, inv.email,
			now.Add(inv.expiresIn), userIDs[seedSyndicEmail]); err != nil {
			return fmt.Errorf("seed invitation for %s: %w", inv.email, err)
		}
	}

	// 7. A few audit events so the audit trail looks alive.
	auditEvents := []struct {
		user    string
		event   string
		details map[string]any
	}{
		{user: seedSyndicEmail, event: "sign_in", details: map[string]any{"method": "password"}},
		{user: "maria.oliveira@example.com", event: "sign_in", details: map[string]any{"method": "password"}},
		{user: seedSyndicEmail, event: "role_granted", details: map[string]any{"role": "owner", "user": "maria.oliveira@example.com"}},
		{user: seedSyndicEmail, event: "category_created", details: map[string]any{"category_name": "Plumbing"}},
		{user: "joao.santos@example.com", event: "listing_created", details: map[string]any{"condominium_id": condoID}},
		{user: seedSyndicEmail, event: "account_created", details: map[string]any{"account_type": "payable"}},
		{user: "carla.pereira@example.com", event: "sign_out", details: map[string]any{}},
	}
	for _, ev := range auditEvents {
		details, err := json.Marshal(ev.details)
		if err != nil {
			return fmt.Errorf("marshal audit details: %w", err)
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO audit_events (user_id, event_type, details) VALUES ($1, $2, $3)`,
			userIDs[ev.user], ev.event, details); err != nil {
			return fmt.Errorf("seed audit event %s: %w", ev.event, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit seed: %w", err)
	}

	logger.Info("development seed complete",
		"condominium", seedCondominiumName,
		"users", len(seedOccupants),
		"units", len(seedUnits),
		"categories", len(seedCategories),
		"listings", len(seedListings),
		"finance_accounts", len(seedPayables)+len(seedFees)+len(extraReceivables),
		"password", seedPassword)
	return nil
}

// nullString maps an empty string to SQL NULL (for nullable text columns).
func nullString(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// phoneDigits mirrors the directory service normalization: digits only.
func phoneDigits(phone string) string {
	digits := make([]rune, 0, len(phone))
	for _, r := range phone {
		if r >= '0' && r <= '9' {
			digits = append(digits, r)
		}
	}
	return string(digits)
}

// monthDay returns the `day` of a month shifted by `months` from now.
func monthDay(now time.Time, months int, day int) time.Time {
	return time.Date(now.Year(), now.Month()+time.Month(months), day, 0, 0, 0, 0, time.UTC)
}

// invitationTokenHash derives a deterministic, unique token hash for a seed
// invitation so re-runs stay idempotent.
func invitationTokenHash(email string) string {
	sum := sha256.Sum256([]byte("zelo-seed-invitation-" + email))
	return hex.EncodeToString(sum[:])
}
