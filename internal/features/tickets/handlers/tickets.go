package handlers

import (
	"errors"
	"net/http"

	"github.com/leoarkiteto/zelo/internal/features/tickets/core/domain"
	"github.com/leoarkiteto/zelo/internal/features/tickets/core/services"
	tickettemplates "github.com/leoarkiteto/zelo/internal/features/tickets/templates"
	"github.com/leoarkiteto/zelo/internal/shared/httpx"
	"github.com/leoarkiteto/zelo/internal/shared/i18n"
	"github.com/leoarkiteto/zelo/internal/shared/model"
)

// ticketsGET renders the current user's own tickets (FR-009).
func (h *Handler) ticketsGET(w http.ResponseWriter, r *http.Request) {
	u := httpx.CurrentUser(r)
	sess := httpx.CurrentSession(r)
	locale := i18n.LanguageFrom(r.Context())

	shell, err := httpx.ShellData(r, h.deps.Roles, "/tickets")
	if err != nil {
		httpx.RenderError(w, r, http.StatusInternalServerError, i18n.T(locale, "tickets.error.load_session"))
		return
	}
	tickets, err := h.deps.Tickets.ListMine(r.Context(), u.ID, sess.CondominiumID)
	if err != nil {
		httpx.RenderError(w, r, http.StatusInternalServerError, i18n.T(locale, "tickets.error.load"))
		return
	}
	data := tickettemplates.TicketsPageData{
		Shell:  shell,
		Locale: locale,
		CSRF:   sess.CSRFToken,
		Flash:  flashMessage(r, locale),
	}
	for _, t := range tickets {
		data.Tickets = append(data.Tickets, h.ticketView(locale, r, t, nil))
	}
	httpx.Render(w, r, tickettemplates.TicketsPage(data))
}

// ticketsNewGET renders the ticket creation form. Residents without an active
// unit get a 403 with a friendly explanation (contracts/http-routes.md).
func (h *Handler) ticketsNewGET(w http.ResponseWriter, r *http.Request) {
	u := httpx.CurrentUser(r)
	sess := httpx.CurrentSession(r)
	locale := i18n.LanguageFrom(r.Context())

	if _, err := h.deps.Units.GetActiveUnitForUser(r.Context(), u.ID, sess.CondominiumID); err != nil {
		httpx.RenderError(w, r, http.StatusForbidden, i18n.T(locale, "tickets.no_unit"))
		return
	}
	shell, err := httpx.ShellData(r, h.deps.Roles, "/tickets/new")
	if err != nil {
		httpx.RenderError(w, r, http.StatusInternalServerError, i18n.T(locale, "tickets.error.load_session"))
		return
	}
	httpx.Render(w, r, tickettemplates.TicketsNewPage(tickettemplates.TicketsNewPageData{
		Shell:      shell,
		Locale:     locale,
		CSRF:       sess.CSRFToken,
		Categories: tickettemplates.CategoryOptions(locale),
	}))
}

// ticketsCreatePOST creates a ticket (FR-001..FR-003).
func (h *Handler) ticketsCreatePOST(w http.ResponseWriter, r *http.Request) {
	u := httpx.CurrentUser(r)
	sess := httpx.CurrentSession(r)
	values := tickettemplates.FormValues{
		Title:       r.FormValue("title"),
		Category:    r.FormValue("category"),
		Description: r.FormValue("description"),
	}

	ticket, err := h.deps.Tickets.CreateTicket(r.Context(), u.ID, sess.CondominiumID, services.TicketInput{
		Title:       values.Title,
		Category:    domain.TicketCategory(values.Category),
		Description: values.Description,
	})
	if err != nil {
		h.renderCreateError(w, r, values, err)
		return
	}
	http.Redirect(w, r, "/tickets/"+ticket.ID+"?flash=created", http.StatusSeeOther)
}

// inboxGET renders the syndic's ticket inbox with a status filter (FR-005).
func (h *Handler) inboxGET(w http.ResponseWriter, r *http.Request) {
	sess := httpx.CurrentSession(r)
	locale := i18n.LanguageFrom(r.Context())

	shell, err := httpx.ShellData(r, h.deps.Roles, "/tickets/inbox")
	if err != nil {
		httpx.RenderError(w, r, http.StatusInternalServerError, i18n.T(locale, "tickets.error.load_session"))
		return
	}
	raw := r.URL.Query().Get("status")
	status := domain.StatusOpen
	switch raw {
	case "closed":
		status = domain.StatusClosed
	case "all":
		status = ""
	}
	tickets, err := h.deps.Tickets.ListInbox(r.Context(), sess.CondominiumID, status)
	if err != nil {
		httpx.RenderError(w, r, http.StatusInternalServerError, i18n.T(locale, "tickets.error.load"))
		return
	}
	data := tickettemplates.TicketsInboxPageData{
		Shell:  shell,
		Locale: locale,
		CSRF:   sess.CSRFToken,
		Status: raw,
		Flash:  flashMessage(r, locale),
	}
	for _, t := range tickets {
		data.Tickets = append(data.Tickets, h.ticketView(locale, r, t, nil))
	}
	httpx.Render(w, r, tickettemplates.TicketsInboxPage(data))
}

// ticketDetailGET renders one ticket with its replies. A member may open only
// their own ticket; the syndic may open any ticket of the session condominium.
// Foreign or missing tickets return 404 (FR-009).
func (h *Handler) ticketDetailGET(w http.ResponseWriter, r *http.Request) {
	u := httpx.CurrentUser(r)
	sess := httpx.CurrentSession(r)
	locale := i18n.LanguageFrom(r.Context())
	id := r.PathValue("id")

	ticket, err := h.deps.Tickets.GetTicket(r.Context(), id)
	if err != nil {
		h.renderTicketGone(w, r, err)
		return
	}
	if ticket.CondominiumID != sess.CondominiumID {
		httpx.RenderError(w, r, http.StatusNotFound, i18n.T(locale, "tickets.error.ticket_gone"))
		return
	}
	isSyndic, err := h.isSyndic(r, u.ID, sess.CondominiumID)
	if err != nil {
		httpx.RenderError(w, r, http.StatusInternalServerError, i18n.T(locale, "tickets.error.load_session"))
		return
	}
	if !isSyndic && ticket.AuthorID != u.ID {
		httpx.RenderError(w, r, http.StatusNotFound, i18n.T(locale, "tickets.error.ticket_gone"))
		return
	}
	replies, err := h.deps.Tickets.ListReplies(r.Context(), id)
	if err != nil {
		httpx.RenderError(w, r, http.StatusInternalServerError, i18n.T(locale, "tickets.error.load"))
		return
	}
	shell, err := httpx.ShellData(r, h.deps.Roles, "/tickets/"+id)
	if err != nil {
		httpx.RenderError(w, r, http.StatusInternalServerError, i18n.T(locale, "tickets.error.load_session"))
		return
	}
	httpx.Render(w, r, tickettemplates.TicketDetailPage(tickettemplates.TicketDetailPageData{
		Shell:    shell,
		Locale:   locale,
		CSRF:     sess.CSRFToken,
		Ticket:   h.ticketView(locale, r, ticket, replies),
		IsSyndic: isSyndic,
		Flash:    flashMessage(r, locale),
	}))
}

// ticketReplyPOST attaches a reply to an open ticket (FR-006, FR-008).
func (h *Handler) ticketReplyPOST(w http.ResponseWriter, r *http.Request) {
	u := httpx.CurrentUser(r)
	sess := httpx.CurrentSession(r)
	id := r.PathValue("id")
	content := r.FormValue("content")

	if err := h.deps.Tickets.ReplyToTicket(r.Context(), u.ID, id, sess.CondominiumID, content); err != nil {
		h.renderReplyError(w, r, id, err)
		return
	}
	http.Redirect(w, r, "/tickets/"+id+"?flash=replied", http.StatusSeeOther)
}

// ticketClosePOST closes an open ticket (FR-007, FR-008).
func (h *Handler) ticketClosePOST(w http.ResponseWriter, r *http.Request) {
	u := httpx.CurrentUser(r)
	sess := httpx.CurrentSession(r)
	locale := i18n.LanguageFrom(r.Context())
	id := r.PathValue("id")

	if err := h.deps.Tickets.CloseTicket(r.Context(), u.ID, id, sess.CondominiumID); err != nil {
		switch {
		case errors.Is(err, services.ErrNotFound):
			httpx.RenderError(w, r, http.StatusNotFound, i18n.T(locale, "tickets.error.ticket_gone"))
		case errors.Is(err, services.ErrClosed):
			httpx.RenderError(w, r, http.StatusBadRequest, i18n.T(locale, "tickets.error.not_open"))
		default:
			httpx.RenderError(w, r, http.StatusInternalServerError, i18n.T(locale, "tickets.error.close"))
		}
		return
	}
	http.Redirect(w, r, "/tickets/inbox?flash=closed", http.StatusSeeOther)
}

// --- shared helpers ---

// ticketView builds the display shape of a ticket. When unit codes are
// available they are resolved through the handler's unit lister.
func (h *Handler) ticketView(locale i18n.Language, r *http.Request, t domain.Ticket, replies []domain.TicketReply) tickettemplates.TicketView {
	unitCodes := map[string]string{}
	if t.UnitID != "" {
		units, err := h.deps.Units.ListUnitsForCondominium(r.Context(), t.CondominiumID)
		if err == nil {
			for _, u := range units {
				unitCodes[u.ID] = u.Code
			}
		}
	}
	view := tickettemplates.TicketView{
		ID:            t.ID,
		Title:         t.Title,
		CategoryLabel: tickettemplates.CategoryLabel(locale, t.Category),
		Status:        t.Status,
		StatusLabel:   tickettemplates.StatusLabel(locale, t.Status),
		Description:   t.Description,
		AuthorID:      t.AuthorID,
		CreatedAt:     i18n.FormatLongDate(locale, t.CreatedAt),
		UnitCode:      unitCodes[t.UnitID],
	}
	if t.ClosedAt != nil {
		view.ClosedAt = i18n.FormatLongDate(locale, *t.ClosedAt)
	}
	for _, rep := range replies {
		view.Replies = append(view.Replies, tickettemplates.ReplyView{
			Content:   rep.Content,
			AuthorID:  rep.AuthorID,
			CreatedAt: i18n.FormatLongDate(locale, rep.CreatedAt),
		})
	}
	return view
}

func (h *Handler) isSyndic(r *http.Request, userID, condominiumID string) (bool, error) {
	roles, err := h.deps.Roles.ActiveRolesForUser(r.Context(), userID, condominiumID)
	if err != nil {
		return false, err
	}
	for _, role := range roles {
		if role == model.RoleSyndic {
			return true, nil
		}
	}
	return false, nil
}

func (h *Handler) renderCreateError(w http.ResponseWriter, r *http.Request, values tickettemplates.FormValues, err error) {
	locale := i18n.LanguageFrom(r.Context())
	sess := httpx.CurrentSession(r)
	shell, shellErr := httpx.ShellData(r, h.deps.Roles, "/tickets/new")
	if shellErr != nil {
		httpx.RenderError(w, r, http.StatusInternalServerError, i18n.T(locale, "tickets.error.load_session"))
		return
	}
	data := tickettemplates.TicketsNewPageData{
		Shell:      shell,
		Locale:     locale,
		CSRF:       sess.CSRFToken,
		Values:     values,
		Categories: tickettemplates.CategoryOptions(locale),
	}
	switch {
	case errors.Is(err, services.ErrInvalidCategory):
		data.Error = i18n.T(locale, "tickets.error.category")
	case errors.Is(err, services.ErrInvalidTitle):
		data.Error = i18n.T(locale, "tickets.error.title")
	case errors.Is(err, services.ErrInvalidDescription):
		data.Error = i18n.T(locale, "tickets.error.description")
	case errors.Is(err, services.ErrNoUnit):
		data.Error = i18n.T(locale, "tickets.no_unit")
	default:
		data.Error = i18n.T(locale, "tickets.error.create")
	}
	w.WriteHeader(http.StatusBadRequest)
	httpx.Render(w, r, tickettemplates.TicketsNewPage(data))
}

func (h *Handler) renderReplyError(w http.ResponseWriter, r *http.Request, id string, err error) {
	locale := i18n.LanguageFrom(r.Context())
	switch {
	case errors.Is(err, services.ErrNotFound):
		httpx.RenderError(w, r, http.StatusNotFound, i18n.T(locale, "tickets.error.ticket_gone"))
	case errors.Is(err, services.ErrClosed):
		httpx.RenderError(w, r, http.StatusBadRequest, i18n.T(locale, "tickets.error.not_open"))
	case errors.Is(err, services.ErrInvalidContent):
		httpx.RenderError(w, r, http.StatusBadRequest, i18n.T(locale, "tickets.error.reply_content"))
	default:
		httpx.RenderError(w, r, http.StatusInternalServerError, i18n.T(locale, "tickets.error.reply"))
	}
}

func (h *Handler) renderTicketGone(w http.ResponseWriter, r *http.Request, err error) {
	locale := i18n.LanguageFrom(r.Context())
	switch {
	case errors.Is(err, services.ErrNotFound):
		httpx.RenderError(w, r, http.StatusNotFound, i18n.T(locale, "tickets.error.ticket_gone"))
	default:
		httpx.RenderError(w, r, http.StatusInternalServerError, i18n.T(locale, "tickets.error.load"))
	}
}

func flashMessage(r *http.Request, locale i18n.Language) string {
	switch r.URL.Query().Get("flash") {
	case "created":
		return i18n.T(locale, "tickets.flash.created")
	case "replied":
		return i18n.T(locale, "tickets.flash.replied")
	case "closed":
		return i18n.T(locale, "tickets.flash.closed")
	default:
		return ""
	}
}
