package handlers

import (
	"errors"
	"github.com/leoarkiteto/zelo/internal/shared/httpx"
	"net/http"

	"github.com/leoarkiteto/zelo/internal/features/directory/core/services"
	"github.com/leoarkiteto/zelo/internal/features/directory/templates"
	"github.com/leoarkiteto/zelo/internal/shared/model"
)

// directoryGET renders the directory list with search and category filters.
func (h *Handler) directoryGET(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	categoryID := r.URL.Query().Get("category")
	data, err := h.directoryData(r, q, categoryID, flashMessage(r))
	if err != nil {
		http.Error(w, "Failed to load directory", http.StatusInternalServerError)
		return
	}
	httpx.Render(w, r, templates.DirectoryPage(data))
}

// directoryNewGET renders the listing submission form.
func (h *Handler) directoryNewGET(w http.ResponseWriter, r *http.Request) {
	data, err := h.listingFormData(r, templates.ListingFormValues{}, false, "", "", "")
	if err != nil {
		http.Error(w, "Failed to load directory", http.StatusInternalServerError)
		return
	}
	httpx.Render(w, r, templates.ListingFormPage(data))
}

// directoryCreatePOST validates and stores a new listing.
func (h *Handler) directoryCreatePOST(w http.ResponseWriter, r *http.Request) {
	u := httpx.CurrentUser(r)
	sess := httpx.CurrentSession(r)
	values := templates.ListingFormValues{
		Name:       r.FormValue("name"),
		CategoryID: r.FormValue("category_id"),
		Phone:      r.FormValue("phone"),
		Notes:      r.FormValue("notes"),
	}
	in := services.ListingInput{
		Name:             values.Name,
		CategoryID:       values.CategoryID,
		Phone:            values.Phone,
		Notes:            values.Notes,
		ConfirmDuplicate: r.FormValue("confirm_duplicate") == "1",
	}

	_, duplicate, err := h.deps.Directory.CreateListing(r.Context(), u.ID, sess.CondominiumID, in)
	switch {
	case duplicate:
		h.renderListingForm(w, r, values, false, "",
			"A listing with this phone number already exists. Tick the confirmation box to save it anyway.")
		return
	case errors.Is(err, services.ErrNoUnit):
		h.renderListingForm(w, r, values, false, "You need an active unit in this condominium to recommend a professional.", "")
		return
	case errors.Is(err, services.ErrInvalidCategory):
		h.renderListingForm(w, r, values, false, "Please choose a category from the list.", "")
		return
	case errors.Is(err, services.ErrInvalidPhone):
		h.renderListingForm(w, r, values, false, "Please enter a valid phone number (7-15 digits).", "")
		return
	case errors.Is(err, services.ErrInvalidListing):
		h.renderListingForm(w, r, values, false, "Please complete all required fields.", "")
		return
	case err != nil:
		http.Error(w, "Failed to create listing", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/directory?flash=listing-created", http.StatusSeeOther)
}

// directoryEditGET renders the syndic edit form for a listing.
func (h *Handler) directoryEditGET(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	listing, err := h.directoryListingForCondominium(r, id)
	if err != nil {
		httpx.RenderError(w, r, http.StatusNotFound, "This listing is no longer available.")
		return
	}
	values := templates.ListingFormValues{
		Name:       listing.Name,
		CategoryID: listing.CategoryID,
		Phone:      listing.Phone,
		Notes:      listing.Notes,
	}
	data, err := h.listingFormData(r, values, true, listing.ID, "", "")
	if err != nil {
		http.Error(w, "Failed to load directory", http.StatusInternalServerError)
		return
	}
	httpx.Render(w, r, templates.ListingFormPage(data))
}

// directoryEditPOST updates a listing (syndic moderation).
func (h *Handler) directoryEditPOST(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	u := httpx.CurrentUser(r)
	sess := httpx.CurrentSession(r)
	values := templates.ListingFormValues{
		Name:       r.FormValue("name"),
		CategoryID: r.FormValue("category_id"),
		Phone:      r.FormValue("phone"),
		Notes:      r.FormValue("notes"),
	}
	err := h.deps.Directory.EditListing(r.Context(), u.ID, id, sess.CondominiumID, services.ListingInput{
		Name: values.Name, CategoryID: values.CategoryID, Phone: values.Phone, Notes: values.Notes,
	})
	switch {
	case errors.Is(err, services.ErrNotFound):
		httpx.RenderError(w, r, http.StatusNotFound, "This listing is no longer available.")
		return
	case errors.Is(err, services.ErrInvalidCategory):
		h.renderListingForm(w, r, values, true, "Please choose a category from the list.", "")
		return
	case errors.Is(err, services.ErrInvalidPhone):
		h.renderListingForm(w, r, values, true, "Please enter a valid phone number (7-15 digits).", "")
		return
	case errors.Is(err, services.ErrInvalidListing):
		h.renderListingForm(w, r, values, true, "Please complete all required fields.", "")
		return
	case err != nil:
		http.Error(w, "Failed to update listing", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/directory?flash=listing-updated", http.StatusSeeOther)
}

// directoryDeleteConfirmGET renders the delete confirmation page.
func (h *Handler) directoryDeleteConfirmGET(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	listing, err := h.directoryListingForCondominium(r, id)
	if err != nil {
		httpx.RenderError(w, r, http.StatusNotFound, "This listing is no longer available.")
		return
	}
	shell, err := httpx.ShellData(r, h.deps.Roles, "/directory")
	if err != nil {
		http.Error(w, "Failed to load session", http.StatusInternalServerError)
		return
	}
	httpx.Render(w, r, templates.DeleteConfirmPage(templates.DeleteConfirmData{
		Shell:   shell,
		CSRF:    httpx.CurrentSession(r).CSRFToken,
		Listing: listingView(listing),
	}))
}

// directoryDeletePOST deletes a listing (syndic moderation).
func (h *Handler) directoryDeletePOST(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	u := httpx.CurrentUser(r)
	sess := httpx.CurrentSession(r)
	if err := h.deps.Directory.DeleteListing(r.Context(), u.ID, id, sess.CondominiumID); err != nil {
		if errors.Is(err, services.ErrNotFound) {
			httpx.RenderError(w, r, http.StatusNotFound, "This listing is no longer available.")
			return
		}
		http.Error(w, "Failed to delete listing", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/directory?flash=listing-deleted", http.StatusSeeOther)
}

// categoriesGET renders the syndic category management page.
func (h *Handler) categoriesGET(w http.ResponseWriter, r *http.Request) {
	data, err := h.categoriesData(r, "", flashMessage(r))
	if err != nil {
		http.Error(w, "Failed to load categories", http.StatusInternalServerError)
		return
	}
	httpx.Render(w, r, templates.CategoriesPage(data))
}

// categoriesCreatePOST adds a category.
func (h *Handler) categoriesCreatePOST(w http.ResponseWriter, r *http.Request) {
	u := httpx.CurrentUser(r)
	sess := httpx.CurrentSession(r)
	name := r.FormValue("name")
	if _, err := h.deps.Directory.CreateCategory(r.Context(), u.ID, sess.CondominiumID, name); err != nil {
		if errors.Is(err, services.ErrDuplicate) {
			h.renderCategoriesWithError(w, r, "A category with this name already exists.")
			return
		}
		if errors.Is(err, services.ErrInvalidListing) {
			h.renderCategoriesWithError(w, r, "Category name must be 1-80 characters.")
			return
		}
		http.Error(w, "Failed to add category", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/directory/categories?flash=category-added", http.StatusSeeOther)
}

// categoriesRenamePOST renames a category.
func (h *Handler) categoriesRenamePOST(w http.ResponseWriter, r *http.Request) {
	u := httpx.CurrentUser(r)
	sess := httpx.CurrentSession(r)
	id := r.PathValue("id")
	name := r.FormValue("name")
	if err := h.deps.Directory.RenameCategory(r.Context(), u.ID, id, sess.CondominiumID, name); err != nil {
		switch {
		case errors.Is(err, services.ErrNotFound):
			httpx.RenderError(w, r, http.StatusNotFound, "This category is no longer available.")
		case errors.Is(err, services.ErrDuplicate):
			h.renderCategoriesWithError(w, r, "A category with this name already exists.")
		case errors.Is(err, services.ErrInvalidListing):
			h.renderCategoriesWithError(w, r, "Category name must be 1-80 characters.")
		default:
			http.Error(w, "Failed to rename category", http.StatusInternalServerError)
		}
		return
	}
	http.Redirect(w, r, "/directory/categories?flash=category-renamed", http.StatusSeeOther)
}

// categoriesDeactivatePOST deactivates a category.
func (h *Handler) categoriesDeactivatePOST(w http.ResponseWriter, r *http.Request) {
	u := httpx.CurrentUser(r)
	sess := httpx.CurrentSession(r)
	id := r.PathValue("id")
	if err := h.deps.Directory.DeactivateCategory(r.Context(), u.ID, id, sess.CondominiumID); err != nil {
		if errors.Is(err, services.ErrNotFound) {
			httpx.RenderError(w, r, http.StatusNotFound, "This category is no longer available.")
			return
		}
		http.Error(w, "Failed to deactivate category", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/directory/categories?flash=category-deactivated", http.StatusSeeOther)
}

// --- shared helpers ---

func (h *Handler) directoryData(r *http.Request, query, categoryID, flash string) (templates.DirectoryPageData, error) {
	sess := httpx.CurrentSession(r)
	shell, err := httpx.ShellData(r, h.deps.Roles, "/directory")
	if err != nil {
		return templates.DirectoryPageData{}, err
	}
	categories, err := h.deps.Categories.ListActiveCategories(r.Context(), sess.CondominiumID)
	if err != nil {
		return templates.DirectoryPageData{}, err
	}
	listings, err := h.deps.Directory.SearchListings(r.Context(), sess.CondominiumID, query, categoryID)
	if err != nil {
		return templates.DirectoryPageData{}, err
	}
	isSyndic, err := h.isSyndic(r)
	if err != nil {
		return templates.DirectoryPageData{}, err
	}
	views := make([]templates.ListingView, 0, len(listings))
	for _, l := range listings {
		views = append(views, listingView(l))
	}
	return templates.DirectoryPageData{
		Shell:            shell,
		CSRF:             sess.CSRFToken,
		Listings:         views,
		Categories:       categoryOptions(categories),
		Query:            query,
		SelectedCategory: categoryID,
		IsSyndic:         isSyndic,
		Flash:            flash,
	}, nil
}

func (h *Handler) renderListingForm(w http.ResponseWriter, r *http.Request, values templates.ListingFormValues, isEdit bool, errorMsg, duplicateWarning string) {
	data, err := h.listingFormData(r, values, isEdit, r.PathValue("id"), errorMsg, duplicateWarning)
	if err != nil {
		http.Error(w, "Failed to load directory", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusBadRequest)
	httpx.Render(w, r, templates.ListingFormPage(data))
}

func (h *Handler) listingFormData(r *http.Request, values templates.ListingFormValues, isEdit bool, listingID, errorMsg, duplicateWarning string) (templates.ListingFormData, error) {
	sess := httpx.CurrentSession(r)
	shell, err := httpx.ShellData(r, h.deps.Roles, "/directory")
	if err != nil {
		return templates.ListingFormData{}, err
	}
	categories, err := h.deps.Categories.ListActiveCategories(r.Context(), sess.CondominiumID)
	if err != nil {
		return templates.ListingFormData{}, err
	}
	return templates.ListingFormData{
		Shell:            shell,
		CSRF:             sess.CSRFToken,
		Categories:       categoryOptions(categories),
		Values:           values,
		IsEdit:           isEdit,
		ListingID:        listingID,
		Error:            errorMsg,
		DuplicateWarning: duplicateWarning,
	}, nil
}

func (h *Handler) categoriesData(r *http.Request, errorMsg, flash string) (templates.CategoriesPageData, error) {
	sess := httpx.CurrentSession(r)
	shell, err := httpx.ShellData(r, h.deps.Roles, "/directory/categories")
	if err != nil {
		return templates.CategoriesPageData{}, err
	}
	categories, err := h.deps.Categories.ListCategories(r.Context(), sess.CondominiumID)
	if err != nil {
		return templates.CategoriesPageData{}, err
	}
	views := make([]templates.CategoryView, 0, len(categories))
	for _, c := range categories {
		views = append(views, templates.CategoryView{ID: c.ID, Name: c.Name, Active: c.Active})
	}
	return templates.CategoriesPageData{Shell: shell, CSRF: sess.CSRFToken, Categories: views, Error: errorMsg, Flash: flash}, nil
}

func (h *Handler) renderCategoriesWithError(w http.ResponseWriter, r *http.Request, message string) {
	data, err := h.categoriesData(r, message, "")
	if err != nil {
		http.Error(w, "Failed to load categories", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusBadRequest)
	httpx.Render(w, r, templates.CategoriesPage(data))
}

// directoryListingForCondominium loads a listing and verifies it belongs to
// the session's condominium.
func (h *Handler) directoryListingForCondominium(r *http.Request, id string) (model.ServiceProviderListing, error) {
	listing, err := h.deps.Listings.GetListingByID(r.Context(), id)
	if err != nil {
		return model.ServiceProviderListing{}, err
	}
	if listing.CondominiumID != httpx.CurrentSession(r).CondominiumID {
		return model.ServiceProviderListing{}, services.ErrNotFound
	}
	return listing, nil
}

// isSyndic reports whether the current user holds the syndic role.
func (h *Handler) isSyndic(r *http.Request) (bool, error) {
	roles, err := h.deps.Roles.ActiveRolesForUser(r.Context(), httpx.CurrentUser(r).ID, httpx.CurrentSession(r).CondominiumID)
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

func listingView(l model.ServiceProviderListing) templates.ListingView {
	return templates.ListingView{
		ID:           l.ID,
		Name:         l.Name,
		CategoryName: l.CategoryName,
		Phone:        l.Phone,
		Notes:        l.Notes,
		UnitCode:     l.RecommendedByUnitCode,
	}
}

func categoryOptions(categories []model.ServiceCategory) []templates.CategoryOption {
	out := make([]templates.CategoryOption, 0, len(categories))
	for _, c := range categories {
		out = append(out, templates.CategoryOption{ID: c.ID, Name: c.Name})
	}
	return out
}

// flashMessage maps a ?flash= query value to a user-facing confirmation.
func flashMessage(r *http.Request) string {
	switch r.URL.Query().Get("flash") {
	case "listing-created":
		return "Listing added to the directory."
	case "listing-updated":
		return "Listing updated."
	case "listing-deleted":
		return "Listing deleted."
	case "category-added":
		return "Category added."
	case "category-renamed":
		return "Category renamed."
	case "category-deactivated":
		return "Category deactivated."
	default:
		return ""
	}
}
