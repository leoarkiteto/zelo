package repositories

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/leoarkiteto/zelo/internal/features/invoices/domain"
	"github.com/leoarkiteto/zelo/internal/shared/pagination"
)

// SupplierStore persists suppliers (fornecedores).
type SupplierStore struct {
	db *sql.DB
}

// NewSupplierStore creates a SupplierStore.
func NewSupplierStore(db *sql.DB) *SupplierStore { return &SupplierStore{db: db} }

const supplierCols = `id, condominium_id, name, nif, country, address, postal_code,
	locality, email, phone, created_at, updated_at`

func scanSupplier(sc interface{ Scan(...any) error }) (domain.Supplier, error) {
	var s domain.Supplier
	err := sc.Scan(&s.ID, &s.CondominiumID, &s.Name, &s.NIF, &s.Country, &s.Address,
		&s.PostalCode, &s.Locality, &s.Email, &s.Phone, &s.CreatedAt, &s.UpdatedAt)
	return s, err
}

func normalizeSupplier(s domain.Supplier) domain.Supplier {
	if strings.TrimSpace(s.Country) == "" {
		s.Country = "PT"
	}
	s.Name = strings.TrimSpace(s.Name)
	s.NIF = domain.NormalizeNIF(s.Country, s.NIF)
	return s
}

func mapSupplierErr(err error) error {
	if pe := pgError(err); pe != nil {
		switch {
		case pe.Code == "23505" && pe.ConstraintName == constraintSupplierNIF:
			return ErrDuplicateNIF
		case pe.Code == "22P02":
			return ErrNotFound
		}
	}
	return err
}

// CreateSupplier inserts a supplier of s.CondominiumID. actor is the user id
// recorded as created_by/updated_by. A taken NIF returns ErrDuplicateNIF.
func (r *SupplierStore) CreateSupplier(ctx context.Context, s domain.Supplier, actor string) (domain.Supplier, error) {
	s = normalizeSupplier(s)
	row := r.db.QueryRowContext(ctx, `
		INSERT INTO suppliers (condominium_id, name, nif, country, address, postal_code,
			locality, email, phone, created_by, updated_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $10)
		RETURNING `+supplierCols,
		s.CondominiumID, s.Name, s.NIF, s.Country, s.Address, s.PostalCode,
		s.Locality, s.Email, s.Phone, actor)
	out, err := scanSupplier(row)
	if err != nil {
		return domain.Supplier{}, fmt.Errorf("create supplier: %w", mapSupplierErr(err))
	}
	return out, nil
}

// UpdateSupplier replaces the editable fields of a supplier of the
// condominium. ErrNotFound when it does not exist there.
func (r *SupplierStore) UpdateSupplier(ctx context.Context, s domain.Supplier, actor string) error {
	s = normalizeSupplier(s)
	res, err := r.db.ExecContext(ctx, `
		UPDATE suppliers
		SET name = $3, nif = $4, country = $5, address = $6, postal_code = $7,
			locality = $8, email = $9, phone = $10, updated_by = $11, updated_at = now()
		WHERE condominium_id = $1 AND id = $2`,
		s.CondominiumID, s.ID, s.Name, s.NIF, s.Country, s.Address, s.PostalCode,
		s.Locality, s.Email, s.Phone, actor)
	if err != nil {
		return fmt.Errorf("update supplier: %w", mapSupplierErr(err))
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// GetSupplier returns a supplier of the condominium, or ErrNotFound.
func (r *SupplierStore) GetSupplier(ctx context.Context, condoID, id string) (domain.Supplier, error) {
	s, err := scanSupplier(r.db.QueryRowContext(ctx,
		`SELECT `+supplierCols+` FROM suppliers WHERE condominium_id = $1 AND id = $2`, condoID, id))
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Supplier{}, ErrNotFound
	}
	if err != nil {
		return domain.Supplier{}, fmt.Errorf("get supplier: %w", mapSupplierErr(err))
	}
	return s, nil
}

// FindByNIF returns the supplier with this (already normalized) NIF in the
// condominium, or ErrNotFound.
func (r *SupplierStore) FindByNIF(ctx context.Context, condoID, nif string) (domain.Supplier, error) {
	if nif == "" {
		return domain.Supplier{}, ErrNotFound
	}
	s, err := scanSupplier(r.db.QueryRowContext(ctx,
		`SELECT `+supplierCols+` FROM suppliers WHERE condominium_id = $1 AND nif = $2`, condoID, nif))
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Supplier{}, ErrNotFound
	}
	if err != nil {
		return domain.Supplier{}, fmt.Errorf("find supplier by nif: %w", err)
	}
	return s, nil
}

// likePattern escapes LIKE wildcards of q and wraps it for a contains match.
func likePattern(q string) string {
	q = strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(q)
	return "%" + q + "%"
}

// SearchSuppliers lists suppliers of the condominium whose name or NIF
// contains q (case-insensitive), ordered by name. An empty q lists all.
// limit <= 0 means 50.
func (r *SupplierStore) SearchSuppliers(ctx context.Context, condoID, q string, limit int) ([]domain.Supplier, error) {
	if limit <= 0 {
		limit = 50
	}
	q = strings.TrimSpace(q)
	args := []any{condoID, limit}
	where := "condominium_id = $1"
	if q != "" {
		args = append(args, likePattern(q), likePattern(strings.ReplaceAll(q, " ", "")))
		where += ` AND (name ILIKE $3 ESCAPE '\' OR nif ILIKE $4 ESCAPE '\')`
	}
	rows, err := r.db.QueryContext(ctx,
		`SELECT `+supplierCols+` FROM suppliers WHERE `+where+` ORDER BY lower(name), id LIMIT $2`, args...)
	if err != nil {
		return nil, fmt.Errorf("search suppliers: %w", err)
	}
	defer rows.Close()
	var out []domain.Supplier
	for rows.Next() {
		s, err := scanSupplier(rows)
		if err != nil {
			return nil, fmt.Errorf("scan supplier: %w", err)
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// SearchSuppliersPage is the paged variant of SearchSuppliers used by the
// suppliers list page: same match and order, one page of pagination.PageSize
// rows plus the clamped page (a page past the last resolves to the last).
func (r *SupplierStore) SearchSuppliersPage(ctx context.Context, condoID, q string, requested int) ([]domain.Supplier, pagination.Page, error) {
	q = strings.TrimSpace(q)
	args := []any{condoID}
	where := "condominium_id = $1"
	if q != "" {
		args = append(args, likePattern(q), likePattern(strings.ReplaceAll(q, " ", "")))
		where += ` AND (name ILIKE $2 ESCAPE '\' OR nif ILIKE $3 ESCAPE '\')`
	}
	var total int
	if err := r.db.QueryRowContext(ctx, `SELECT count(*) FROM suppliers WHERE `+where, args...).Scan(&total); err != nil {
		return nil, pagination.Page{}, fmt.Errorf("count suppliers: %w", err)
	}
	p := pagination.New(requested, total)
	pageArgs := append(append([]any{}, args...), p.Size, p.Offset())
	limit := "$" + strconv.Itoa(len(args)+1)
	offset := "$" + strconv.Itoa(len(args)+2)
	rows, err := r.db.QueryContext(ctx,
		`SELECT `+supplierCols+` FROM suppliers WHERE `+where+
			` ORDER BY lower(name), id LIMIT `+limit+` OFFSET `+offset, pageArgs...)
	if err != nil {
		return nil, pagination.Page{}, fmt.Errorf("search suppliers page: %w", err)
	}
	defer rows.Close()
	var out []domain.Supplier
	for rows.Next() {
		s, err := scanSupplier(rows)
		if err != nil {
			return nil, pagination.Page{}, fmt.Errorf("scan supplier: %w", err)
		}
		out = append(out, s)
	}
	return out, p, rows.Err()
}
