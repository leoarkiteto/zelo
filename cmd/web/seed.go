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

	financedomain "github.com/leoarkiteto/zelo/internal/features/finance/domain"
	financerepositories "github.com/leoarkiteto/zelo/internal/features/finance/repositories"
	financeservices "github.com/leoarkiteto/zelo/internal/features/finance/services"
	"github.com/leoarkiteto/zelo/internal/shared/permille"
	"github.com/leoarkiteto/zelo/internal/shared/security"
	"github.com/leoarkiteto/zelo/internal/shared/store"
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
	// Settled energy, water and payroll live in the 12-month history below
	// (seedHistory) so they are not duplicated here.
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
// It is idempotent: the base step exits cleanly if the seed syndic already
// exists, and the quota layer (seedQuotas) then runs on every call, so a
// database seeded by an older version also gets the quota demo data.
func seedDemoData(ctx context.Context, logger *slog.Logger, db *sql.DB, passwordPepper string) error {
	return seedDemoDataAt(ctx, logger, db, passwordPepper, time.Now())
}

// seedDemoDataAt is seedDemoData with an injectable clock. The same now drives
// the base step and the quota layer so their months agree.
func seedDemoDataAt(ctx context.Context, logger *slog.Logger, db *sql.DB, passwordPepper string, now time.Time) error {
	if err := seedBase(ctx, logger, db, passwordPepper, now); err != nil {
		return err
	}
	return seedQuotas(ctx, logger, db, now)
}

// seedBase is the original seed step: it skips when the seed syndic exists.
func seedBase(ctx context.Context, logger *slog.Logger, db *sql.DB, passwordPepper string, now time.Time) error {
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

	// Condo fees of months -1..+1 are real quotas now (seedQuotas), so the
	// manual ones are only created outside that window.
	baseFees := seedFeesToInsert()
	for _, f := range baseFees {
		dueDate := seedFeeDueDate(f, now)
		status := f.status
		if status == "overdue" {
			status = "pending" // "overdue" is derived at read time, never persisted
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
			status, settlementDate, unitIDs[f.unit], seedFeePaymentCode(dueDate.Format("2006-01"), f.unit),
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

	// 5b. Twelve months of settled entradas and saídas for the financial charts.
	history := seedHistoryToInsert(now)
	historyCount := 0
	for _, h := range history {
		var unitID, paymentCode, supplier any
		if h.accountType == "receivable" {
			if h.unit != "" {
				unitID = unitIDs[h.unit]
			}
			paymentCode = h.paymentCode
		} else {
			supplier = h.supplier
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO finance_accounts
				(condominium_id, account_type, title, category, amount_cents, due_date,
				 status, settlement_date, unit_id, payment_code, supplier_payee, created_by)
			VALUES ($1, $2, $3, $4, $5, $6, 'settled', $7, $8, $9, $10, $11)`,
			condoID, h.accountType, h.title, h.category, h.amountCents, h.dueDate,
			h.settlementDate, unitID, paymentCode, supplier, userIDs[seedSyndicEmail]); err != nil {
			return fmt.Errorf("seed history account %s: %w", h.title, err)
		}
		historyCount++
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
		"finance_accounts", len(seedPayables)+len(baseFees)+len(extraReceivables)+historyCount,
		"password", seedPassword)
	return nil
}

// seedHistoryMonths is how many months (ending with the current one) of
// settled history are seeded for the financial dashboard.
const seedHistoryMonths = 12

// seedDeficitOffset is the month offset (relative to now) that carries an
// extraordinary expense so saídas exceed entradas.
const seedDeficitOffset = -4

// seedHistoryAccount is one settled account of the 12-month history.
type seedHistoryAccount struct {
	monthOffset    int // 0 = current month, -11 = eleven months ago
	accountType    string
	title          string
	category       string
	amountCents    int64
	dueDate        time.Time
	settlementDate time.Time
	unit           string // receivables only
	paymentCode    string // receivables only
	supplier       string // payables only
}

// seedMonthlyExpense is a recurring payable; amounts vary by month via
// seedVariation.
type seedMonthlyExpense struct {
	title     string
	category  string
	baseCents int64
	supplier  string
	day       int
	varies    bool
}

var seedMonthlyExpenses = []seedMonthlyExpense{
	{title: "Energia elétrica (Enel)", category: "utilities", baseCents: 234000, supplier: "Enel Distribuição", day: 12, varies: true},
	{title: "Água e esgoto (Sabesp)", category: "utilities", baseCents: 89000, supplier: "Sabesp", day: 14, varies: true},
	{title: "Salário do zelador", category: "payroll", baseCents: 320000, supplier: "Pedro Costa (zelador)", day: 5},
	{title: "Salário da portaria", category: "payroll", baseCents: 280000, supplier: "Portaria 24h Ltda", day: 5},
	{title: "Limpeza das áreas comuns", category: "cleaning", baseCents: 95000, supplier: "LimpaTudo Serviços", day: 8},
	{title: "Manutenção predial mensal", category: "maintenance", baseCents: 135000, supplier: "Predial Serviços Ltda", day: 18, varies: true},
	{title: "Jardinagem e piscina", category: "third_party_services", baseCents: 60000, supplier: "Verde Jardim", day: 20, varies: true},
}

// seedVariation is a deterministic per-month multiplier (in percent) so the
// charts are not flat. Indexed by month offset modulo its length.
var seedVariation = []int64{100, 108, 96, 112, 103, 94, 118, 99, 105, 91, 110, 101}

func variationFor(offset int) int64 {
	i := (-offset) % len(seedVariation)
	return seedVariation[i]
}

// clampDay returns day `day` of the month at `offset` from now, never later
// than today (settlements cannot lie in the future).
func clampDay(now time.Time, offset, day int) time.Time {
	d := monthDay(now, offset, day)
	today := now.UTC().Truncate(24 * time.Hour)
	if d.After(today) {
		return today
	}
	return d
}

// seedHistory builds, without touching the database, twelve months of settled
// entradas (condo fees per unit plus occasional extras) and saídas (recurring
// expenses) ending with the current month. One month (seedDeficitOffset) has an
// extraordinary maintenance expense so saídas exceed entradas.
func seedHistory(now time.Time) []seedHistoryAccount {
	var out []seedHistoryAccount
	for i := seedHistoryMonths - 1; i >= 0; i-- {
		offset := -i
		month := monthDay(now, offset, 1).Format("2006-01")

		for _, f := range seedFees {
			out = append(out, seedHistoryAccount{
				monthOffset: offset, accountType: "receivable",
				title:       fmt.Sprintf("Taxa condominial — AP %s", f.unit),
				category:    "condo_fee",
				amountCents: f.amountCents,
				dueDate:     clampDay(now, offset, 10), settlementDate: clampDay(now, offset, 10+int(f.amountCents%5)),
				unit: f.unit, paymentCode: seedFeePaymentCode(month, f.unit),
			})
		}
		// Occasional extra entradas.
		if i%3 == 1 {
			out = append(out, seedHistoryAccount{
				monthOffset: offset, accountType: "receivable",
				title: "Reserva do salão de festas", category: "common_area_reservation",
				amountCents: 30000 + int64(i)*2500,
				dueDate:     clampDay(now, offset, 16), settlementDate: clampDay(now, offset, 17),
				unit: "A-202", paymentCode: "RESERVA-" + month,
			})
		}
		if i%4 == 2 {
			out = append(out, seedHistoryAccount{
				monthOffset: offset, accountType: "receivable",
				title: "Juros de mora — AP B-201", category: "fine_interest",
				amountCents: 2843 + int64(i)*100,
				dueDate:     clampDay(now, offset, 12), settlementDate: clampDay(now, offset, 22),
				unit: "B-201", paymentCode: "JUROS-" + month,
			})
		}

		for _, e := range seedMonthlyExpenses {
			amount := e.baseCents
			if e.varies {
				amount = amount * variationFor(offset) / 100
			}
			out = append(out, seedHistoryAccount{
				monthOffset: offset, accountType: "payable",
				title: e.title, category: e.category, amountCents: amount,
				dueDate:        clampDay(now, offset, e.day),
				settlementDate: clampDay(now, offset, e.day),
				supplier:       e.supplier,
			})
		}
		if offset == seedDeficitOffset {
			out = append(out, seedHistoryAccount{
				monthOffset: offset, accountType: "payable",
				title: "Reforma emergencial do telhado", category: "maintenance",
				amountCents: 950000,
				dueDate:     clampDay(now, offset, 15), settlementDate: clampDay(now, offset, 15),
				supplier: "Telhados & Cia Engenharia",
			})
		}
	}
	return out
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

// seedIsQuotaMonth reports whether a month offset (relative to now) belongs to
// the quota window -1..+1, where condo fees are real quotas, not manual rows.
func seedIsQuotaMonth(offset int) bool { return offset >= -1 && offset <= 1 }

// seedFeesToInsert is seedFees minus the manual fees of the quota window.
// Every base fee sits in month -1 or +1, so the base seed creates none.
func seedFeesToInsert() []seedFee {
	var out []seedFee
	for _, f := range seedFees {
		if !seedIsQuotaMonth(seedFeeMonthOffset(f)) {
			out = append(out, f)
		}
	}
	return out
}

// seedHistoryToInsert is seedHistory minus the condo fees of the quota window
// (months -1..+1, which the quota layer issues), so no unit gets a manual fee
// and a quota for one month. Older months and non-fee entries are unchanged.
func seedHistoryToInsert(now time.Time) []seedHistoryAccount {
	var out []seedHistoryAccount
	for _, h := range seedHistory(now) {
		if h.accountType == "receivable" && h.category == "condo_fee" && seedIsQuotaMonth(h.monthOffset) {
			continue
		}
		out = append(out, h)
	}
	return out
}

// seedFeePaymentCode is the payment code of a unit's condo fee; month is "YYYY-MM".
func seedFeePaymentCode(month, unit string) string {
	return fmt.Sprintf("BOLETO-%s-%s", month, unit)
}

// seedFeeMonthOffset is the month (relative to now) a base fee is due in:
// settled and overdue fees fall in last month, pending ones in next month.
func seedFeeMonthOffset(f seedFee) int {
	if f.status == "settled" || f.status == "overdue" {
		return -1
	}
	return 1
}

// seedFeeDueDate is the due date of a base condo fee (the 10th of its month).
func seedFeeDueDate(f seedFee, now time.Time) time.Time {
	return monthDay(now, seedFeeMonthOffset(f), 10)
}

// monthDay returns the `day` of a month shifted by `months` from now.
func monthDay(now time.Time, months int, day int) time.Time {
	return time.Date(now.Year(), now.Month()+time.Month(months), day, 0, 0, 0, 0, time.UTC)
}

// seedSharePPM is the fixed share table of the seed units (ppm, 1,000,000 =
// 100%): larger on upper floors and in block B. It sums to exactly 1,000,000.
var seedSharePPM = map[string]int{
	"A-101": 70000, "A-102": 70000, "A-201": 75000, "A-202": 75000, "A-301": 80000, "A-302": 80000,
	"B-101": 85000, "B-102": 85000, "B-201": 90000, "B-202": 90000, "B-301": 100000, "B-302": 100000,
}

// seedBudgetLine is one line of the seed budget (payable categories only).
type seedBudgetLine struct {
	description string
	category    financedomain.Category
	amountCents int64
}

// seedBudgetLines total R$ 156.000,00 per year.
var seedBudgetLines = []seedBudgetLine{
	{"Salários e encargos", financedomain.Category("payroll"), 6000000},
	{"Energia e água", financedomain.Category("utilities"), 3000000},
	{"Manutenção predial e elevadores", financedomain.Category("maintenance"), 2400000},
	{"Limpeza das áreas comuns", financedomain.Category("cleaning"), 1800000},
	{"Jardinagem, piscina e dedetização", financedomain.Category("third_party_services"), 1200000},
	{"Seguro e despesas gerais", financedomain.Category("other"), 1200000},
}

// Seed budget parameters.
const (
	seedBudgetDueDay    = 25
	seedBudgetReserveBP = 1000 // 10%
)

// seedPrevUnsettledUnit stays pending (so overdue) in the previous period.
const seedPrevUnsettledUnit = "A-301"

// seedCurrentSettledUnits are the units whose current-period quota is settled.
var seedCurrentSettledUnits = []string{"A-101", "A-102", "B-101"}

// seedQuotaPeriod is a calendar month the quota layer issues.
type seedQuotaPeriod struct {
	Year  int
	Month int
}

// seedQuotaPeriods returns the previous and the current period for now. In
// January the previous period is December of the previous year.
func seedQuotaPeriods(now time.Time) (prev, cur seedQuotaPeriod) {
	p, c := monthDay(now, -1, 1), monthDay(now, 0, 1)
	return seedQuotaPeriod{Year: p.Year(), Month: int(p.Month())},
		seedQuotaPeriod{Year: c.Year(), Month: int(c.Month())}
}

// seedPrevSettleDate is the settlement date of the previous period: the 20th
// of that month (always in the past).
func seedPrevSettleDate(now time.Time) time.Time { return monthDay(now, -1, 20) }

// seedCurrentSettleDate is the settlement date of the current period: the 5th
// of the month, clamped to today.
func seedCurrentSettleDate(now time.Time) time.Time { return clampDay(now, 0, 5) }

// seedLegacyFeeCodePatterns are the LIKE patterns of the payment codes
// (BOLETO-YYYY-MM-<unit>) of the manual condo fees of months -1, 0 and +1.
func seedLegacyFeeCodePatterns(now time.Time) [3]string {
	var out [3]string
	for i, off := range []int{-1, 0, 1} {
		out[i] = seedFeePaymentCode(monthDay(now, off, 1).Format("2006-01"), "%")
	}
	return out
}

// seedPeriodIndex is the budget period (1-based) holding the calendar month.
func seedPeriodIndex(p financedomain.Periodicity, month int) int {
	if p == financedomain.PeriodicityQuarterly {
		return (month-1)/3 + 1
	}
	return month
}

// seedQuotas is the quota layer of the seed. It runs on every -seed call (also
// when the base step was skipped) and is idempotent: shares only when all are
// zero, one budget per year, Issue is idempotent, settlements only touch
// pending quotas, and the legacy manual condo fees of months -1..+1 are
// removed before issuing.
func seedQuotas(ctx context.Context, logger *slog.Logger, db *sql.DB, now time.Time) error {
	var condoID, syndicID string
	err := db.QueryRowContext(ctx, `
		SELECT c.id, u.id
		FROM users u
		JOIN user_roles ur ON ur.user_id = u.id AND ur.role = 'syndic'
		JOIN condominiums c ON c.id = ur.condominium_id
		WHERE lower(u.email) = lower($1) AND c.name = $2
		LIMIT 1`, seedSyndicEmail, seedCondominiumName).Scan(&condoID, &syndicID)
	if err == sql.ErrNoRows {
		logger.Warn("quota seed skipped: seed condominium or syndic not found")
		return nil
	}
	if err != nil {
		return fmt.Errorf("find seed condominium: %w", err)
	}

	units := store.NewUnitStore(db)
	condominiums := store.NewCondominiumStore(db)
	audit := store.NewAuditStore(db)
	quotaStore := financerepositories.NewQuotaStore(db)
	accounts := financerepositories.NewAccountStore(db)
	nowFn := func() time.Time { return now }
	quotas := &financeservices.QuotaService{
		Store: quotaStore, Occupancies: units, Units: units, Audit: audit,
		Currency: condominiums.GetCurrency, Now: nowFn,
	}
	fin := &financeservices.FinanceService{
		Accounts: accounts, Summaries: accounts, Overviews: accounts, Units: units,
		Audit: audit, QuotaLinks: quotaStore, Now: nowFn,
	}

	// 1. Unit shares.
	unitList, err := units.ListUnitsForCondominium(ctx, condoID)
	if err != nil {
		return fmt.Errorf("list seed units: %w", err)
	}
	allZero, covered := len(unitList) > 0, true
	for _, u := range unitList {
		if u.SharePPM != 0 {
			allZero = false
		}
		if _, ok := seedSharePPM[u.Code]; !ok {
			covered = false
		}
	}
	switch {
	case !allZero:
		logger.Info("quota seed: unit shares already set, left untouched")
	case !covered:
		logger.Warn("quota seed: units outside the seed share table, shares not assigned")
	default:
		for _, u := range unitList {
			if err := units.UpdateUnit(ctx, condoID, u.ID, u.Code, seedSharePPM[u.Code]); err != nil {
				return fmt.Errorf("assign share to %s: %w", u.Code, err)
			}
		}
		logger.Info("quota seed: unit shares assigned", "units", len(unitList), "total_ppm", 1000000)
	}

	// Issuing needs the shares to sum to exactly 100%. Check before touching
	// anything, so a skipped or partial share setup never leaves the quota
	// months with neither the legacy fees nor quotas.
	if unitList, err = units.ListUnitsForCondominium(ctx, condoID); err != nil {
		return fmt.Errorf("list seed units: %w", err)
	}
	sum := 0
	for _, u := range unitList {
		sum += u.SharePPM
	}
	if sum != permille.Total {
		logger.Warn("quota seed skipped: unit shares do not sum to 100%; fix them in /condominium/units and re-run -seed",
			"total_ppm", sum, "want_ppm", permille.Total)
		return nil
	}

	// 2. Legacy manual condo fees of months -1..+1 (never rows linked to a quota).
	pats := seedLegacyFeeCodePatterns(now)
	res, err := db.ExecContext(ctx, `
		DELETE FROM finance_accounts
		WHERE condominium_id = $1 AND account_type = 'receivable' AND category = 'condo_fee'
		  AND (payment_code LIKE $2 OR payment_code LIKE $3 OR payment_code LIKE $4)
		  AND id NOT IN (SELECT finance_account_id FROM quota_charges)`,
		condoID, pats[0], pats[1], pats[2])
	if err != nil {
		return fmt.Errorf("remove legacy condo fees: %w", err)
	}
	removed, _ := res.RowsAffected()
	logger.Info("quota seed: legacy manual condo fees removed", "deleted", removed)

	// 3. Budgets, issuances and settlements.
	prev, cur := seedQuotaPeriods(now)
	unitIDs := make(map[string]string, len(unitList))
	for _, u := range unitList {
		unitIDs[u.Code] = u.ID
	}
	for _, step := range []struct {
		period   seedQuotaPeriod
		settle   []string
		settleOn time.Time
	}{
		{prev, nil, seedPrevSettleDate(now)},
		{cur, seedCurrentSettledUnits, seedCurrentSettleDate(now)},
	} {
		budget, err := seedEnsureBudget(ctx, logger, quotas, syndicID, condoID, step.period.Year)
		if err != nil {
			return err
		}
		index := seedPeriodIndex(budget.Periodicity, step.period.Month)
		_, created, err := quotas.Issue(ctx, syndicID, condoID, budget.ID, index)
		if err != nil {
			return fmt.Errorf("issue %04d-%02d: %w", step.period.Year, step.period.Month, err)
		}
		if created {
			logger.Info("quota seed: period issued", "year", step.period.Year, "month", step.period.Month)
		} else {
			logger.Info("quota seed: period already issued, skipped", "year", step.period.Year, "month", step.period.Month)
		}

		codes := step.settle
		if codes == nil { // previous period: everyone but the unit that stays overdue
			for _, u := range unitList {
				if u.Code != seedPrevUnsettledUnit {
					codes = append(codes, u.Code)
				}
			}
		}
		settled := 0
		for _, code := range codes {
			unitID, ok := unitIDs[code]
			if !ok {
				continue
			}
			rows, err := quotaStore.UnitReceivables(ctx, condoID, unitID)
			if err != nil {
				return fmt.Errorf("list receivables of %s: %w", code, err)
			}
			for _, r := range rows {
				if !r.IsQuota || r.Period.Year != budget.Year || r.Period.Index != index ||
					r.Account.Status != financedomain.StatusPending {
					continue
				}
				if err := fin.SettleAccount(ctx, syndicID, r.Account.ID, condoID,
					step.settleOn.Format("2006-01-02")); err != nil {
					return fmt.Errorf("settle quota of %s: %w", code, err)
				}
				settled++
			}
		}
		logger.Info("quota seed: quotas settled", "year", step.period.Year, "month", step.period.Month, "settled_now", settled)
	}
	return nil
}

// seedEnsureBudget returns the approved budget of year: it creates and
// approves one when missing, approves an existing draft (adding the seed lines
// when it has none) and reuses an approved one.
func seedEnsureBudget(ctx context.Context, logger *slog.Logger, quotas *financeservices.QuotaService,
	actorID, condoID string, year int) (financeservices.Budget, error) {
	list, err := quotas.ListBudgets(ctx, condoID)
	if err != nil {
		return financeservices.Budget{}, fmt.Errorf("list budgets: %w", err)
	}
	var b financeservices.Budget
	lines, found := 0, false
	for _, s := range list {
		if s.Year == year {
			b, lines, found = s.Budget, s.LineCount, true
			break
		}
	}
	if !found {
		rate := seedBudgetReserveBP
		b, err = quotas.CreateBudget(ctx, actorID, condoID, financeservices.BudgetInput{
			Year: year, Periodicity: financedomain.PeriodicityMonthly, DueDay: seedBudgetDueDay, ReserveRateBP: &rate,
		})
		if err != nil {
			return b, fmt.Errorf("create budget %d: %w", year, err)
		}
		logger.Info("quota seed: budget created", "year", year)
	} else {
		logger.Info("quota seed: budget reused", "year", year, "status", string(b.Status))
	}
	if b.Status == financedomain.BudgetDraft {
		if lines == 0 {
			for _, l := range seedBudgetLines {
				if _, err := quotas.AddLine(ctx, actorID, condoID, b.ID, financeservices.LineInput{
					Description: l.description, Category: l.category, AmountCents: l.amountCents,
				}); err != nil {
					return b, fmt.Errorf("add budget line %q: %w", l.description, err)
				}
			}
		}
		if err := quotas.ApproveBudget(ctx, actorID, condoID, b.ID); err != nil {
			return b, fmt.Errorf("approve budget %d: %w", year, err)
		}
		b.Status = financedomain.BudgetApproved
		logger.Info("quota seed: budget approved", "year", year)
	}
	return b, nil
}

// invitationTokenHash derives a deterministic, unique token hash for a seed
// invitation so re-runs stay idempotent.
func invitationTokenHash(email string) string {
	sum := sha256.Sum256([]byte("zelo-seed-invitation-" + email))
	return hex.EncodeToString(sum[:])
}
