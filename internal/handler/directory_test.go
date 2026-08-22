package handler

import (
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"strings"
	"testing"
)

// loginAsSyndic signs in the seeded syndic user and returns a client with a
// session cookie.
func loginAsSyndic(t *testing.T, base string) *http.Client {
	t.Helper()
	client := newClient(t)
	code, _ := postForm(t, client, base, "/login", map[string]string{
		"email": "syndic@example.com", "password": "syndic-pass-123",
		"csrf_token": csrfFrom(t, client, base, "/login"),
	})
	if code != http.StatusSeeOther {
		t.Fatalf("syndic login = %d, want 303", code)
	}
	return client
}

// createDirectoryCategory adds a category as the syndic and returns its id.
func createDirectoryCategory(t *testing.T, client *http.Client, base, name string) string {
	t.Helper()
	code, body := getPage(t, client, base, "/directory/categories")
	if code != http.StatusOK {
		t.Fatalf("categories page = %d", code)
	}
	csrf := csrfRe.FindStringSubmatch(body)
	if csrf == nil {
		t.Fatal("no csrf token on categories page")
	}
	code, body = postForm(t, client, base, "/directory/categories", map[string]string{
		"name": name, "csrf_token": csrf[1],
	})
	if code != http.StatusSeeOther {
		t.Fatalf("add category %s = %d, want 303", name, code)
	}
	// Re-read the categories page and extract the category id from the
	// rename form action.
	code, body = getPage(t, client, base, "/directory/categories")
	if code != http.StatusOK {
		t.Fatalf("categories page after add = %d", code)
	}
	re := regexp.MustCompile(`/directory/categories/([0-9a-f-]+)/rename`)
	m := re.FindStringSubmatch(body)
	if m == nil {
		t.Fatalf("no rename form for category %s", name)
	}
	return m[1]
}

// createDirectoryListing submits a listing as the given client.
func createDirectoryListing(t *testing.T, client *http.Client, base, categoryID, name, phone, notes, confirm string) int {
	t.Helper()
	code, body := getPage(t, client, base, "/directory/new")
	if code != http.StatusOK {
		t.Fatalf("new listing page = %d", code)
	}
	csrf := csrfRe.FindStringSubmatch(body)
	if csrf == nil {
		t.Fatal("no csrf token on new listing page")
	}
	form := map[string]string{
		"name": name, "category_id": categoryID, "phone": phone, "notes": notes,
		"csrf_token": csrf[1],
	}
	if confirm != "" {
		form["confirm_duplicate"] = confirm
	}
	code, _ = postForm(t, client, base, "/directory", form)
	return code
}

func TestIntegrationDirectoryMemberAccessAndSubmit(t *testing.T) {
	router := newIntegrationRouter(t)
	ts := httptest.NewServer(router)
	defer ts.Close()
	base := ts.URL
	client := loginAsSyndic(t, base)

	// Unauthenticated access redirects to login.
	if code, _ := getPage(t, newClient(t), base, "/directory"); code != http.StatusSeeOther {
		t.Fatalf("anonymous /directory = %d, want 303", code)
	}

	code, body := getPage(t, client, base, "/directory")
	if code != http.StatusOK || !strings.Contains(body, "Service directory") {
		t.Fatalf("directory page = %d, want 200 with title", code)
	}
	if !strings.Contains(body, "No listings found") {
		t.Fatal("empty directory should show empty state")
	}

	catID := createDirectoryCategory(t, client, base, "Plumber")
	if code := createDirectoryListing(t, client, base, catID, "Ana's Plumbing", "+55 11 91234-5678", "Fast and reliable", ""); code != http.StatusSeeOther {
		t.Fatalf("create listing = %d, want 303", code)
	}
	code, body = getPage(t, client, base, "/directory")
	if code != http.StatusOK || !strings.Contains(body, "Ana&#39;s Plumbing") {
		t.Fatalf("directory after create = %d, missing listing", code)
	}
	if !strings.Contains(body, "Unit B-1") {
		t.Fatalf("directory missing recommending unit, body=%s", body)
	}
	_, body = getPage(t, client, base, "/directory?flash=listing-created")
	if !strings.Contains(body, "Listing added to the directory.") {
		t.Fatal("flash message not shown after create")
	}

	// Missing phone → 400 with validation message, nothing saved.
	if code := createDirectoryListing(t, client, base, catID, "No Phone", "", "x", ""); code != http.StatusBadRequest {
		t.Fatalf("invalid listing = %d, want 400", code)
	}
	code, body = getPage(t, client, base, "/directory")
	if strings.Contains(body, "No Phone") {
		t.Fatal("invalid listing was saved")
	}
}

func TestIntegrationDirectoryDuplicateWarning(t *testing.T) {
	router := newIntegrationRouter(t)
	ts := httptest.NewServer(router)
	defer ts.Close()
	base := ts.URL
	client := loginAsSyndic(t, base)
	catID := createDirectoryCategory(t, client, base, "Plumber")

	if code := createDirectoryListing(t, client, base, catID, "Ana's Plumbing", "+55 11 91234-5678", "", ""); code != http.StatusSeeOther {
		t.Fatalf("first listing = %d, want 303", code)
	}
	// Same phone, different formatting, no confirmation → 200 with warning.
	code, body := getPage(t, client, base, "/directory/new")
	if code != http.StatusOK {
		t.Fatalf("new listing page = %d", code)
	}
	csrf := csrfRe.FindStringSubmatch(body)
	code, body = postForm(t, client, base, "/directory", map[string]string{
		"name": "Ana Again", "category_id": catID, "phone": "55 11 91234 5678",
		"csrf_token": csrf[1],
	})
	if code != http.StatusBadRequest {
		t.Fatalf("duplicate without confirm = %d, want 400", code)
	}
	if !strings.Contains(body, "already exists") || !strings.Contains(body, "confirm_duplicate") {
		t.Fatalf("duplicate warning missing from response")
	}
	// Confirmed duplicate → saved.
	if code := createDirectoryListing(t, client, base, catID, "Ana Again", "55 11 91234 5678", "", "1"); code != http.StatusSeeOther {
		t.Fatalf("confirmed duplicate = %d, want 303", code)
	}
	code, body = getPage(t, client, base, "/directory")
	if !strings.Contains(body, "Ana Again") {
		t.Fatal("confirmed duplicate listing not saved")
	}
}

func TestIntegrationDirectorySearchAndFilter(t *testing.T) {
	router := newIntegrationRouter(t)
	ts := httptest.NewServer(router)
	defer ts.Close()
	base := ts.URL
	client := loginAsSyndic(t, base)

	plumbingID := createDirectoryCategory(t, client, base, "Plumber")
	electricID := createDirectoryCategory(t, client, base, "Electrician")
	createDirectoryListing(t, client, base, plumbingID, "Ana's Plumbing", "+55 11 91234-5678", "24h emergency service", "")
	createDirectoryListing(t, client, base, electricID, "Bob Electrician", "+55 11 98765-4321", "", "")

	code, body := getPage(t, client, base, "/directory?q=electrician")
	if code != http.StatusOK || !strings.Contains(body, "Bob Electrician") || strings.Contains(body, "Ana&#39;s Plumbing") {
		t.Fatalf("keyword search results wrong")
	}
	code, body = getPage(t, client, base, "/directory?q=emergency")
	if !strings.Contains(body, "Ana&#39;s Plumbing") {
		t.Fatalf("notes keyword search failed")
	}
	code, body = getPage(t, client, base, "/directory?category="+plumbingID)
	if !strings.Contains(body, "Ana&#39;s Plumbing") || strings.Contains(body, "Bob Electrician") {
		t.Fatalf("category filter results wrong")
	}
	code, body = getPage(t, client, base, "/directory?q=zzz-nope")
	if !strings.Contains(body, "No listings found") {
		t.Fatalf("empty state missing for no-match search")
	}
}

func TestIntegrationDirectorySyndicModeration(t *testing.T) {
	router := newIntegrationRouter(t)
	ts := httptest.NewServer(router)
	defer ts.Close()
	base := ts.URL
	client := loginAsSyndic(t, base)
	catID := createDirectoryCategory(t, client, base, "Plumber")
	createDirectoryListing(t, client, base, catID, "Ana's Plumbing", "+55 11 91234-5678", "", "")

	// Find the listing id from the edit link.
	_, body := getPage(t, client, base, "/directory")
	re := regexp.MustCompile(`href="/directory/([0-9a-f-]+)/edit"`)
	m := re.FindStringSubmatch(body)
	if m == nil {
		t.Fatal("no edit link for listing")
	}
	listingID := m[1]

	// Edit flow.
	code, body := getPage(t, client, base, "/directory/"+listingID+"/edit")
	if code != http.StatusOK {
		t.Fatalf("edit page = %d", code)
	}
	csrf := csrfRe.FindStringSubmatch(body)
	code, _ = postForm(t, client, base, "/directory/"+listingID+"/edit", map[string]string{
		"name": "Ana's Pipes", "category_id": catID, "phone": "+55 11 91234-5678",
		"csrf_token": csrf[1],
	})
	if code != http.StatusSeeOther {
		t.Fatalf("edit submit = %d, want 303", code)
	}
	_, body = getPage(t, client, base, "/directory")
	if !strings.Contains(body, "Ana&#39;s Pipes") {
		t.Fatal("edited listing name not shown")
	}

	// Delete flow with confirmation page.
	code, body = getPage(t, client, base, "/directory/"+listingID+"/delete")
	if code != http.StatusOK || !strings.Contains(body, "Delete listing") {
		t.Fatalf("delete confirm page = %d", code)
	}
	csrf = csrfRe.FindStringSubmatch(body)
	code, _ = postForm(t, client, base, "/directory/"+listingID+"/delete", map[string]string{"csrf_token": csrf[1]})
	if code != http.StatusSeeOther {
		t.Fatalf("delete submit = %d, want 303", code)
	}
	_, body = getPage(t, client, base, "/directory")
	if strings.Contains(body, "Ana&#39;s Pipes") {
		t.Fatal("deleted listing still shown")
	}
}

func TestIntegrationDirectoryCategoryManagement(t *testing.T) {
	router := newIntegrationRouter(t)
	ts := httptest.NewServer(router)
	defer ts.Close()
	base := ts.URL
	client := loginAsSyndic(t, base)
	catID := createDirectoryCategory(t, client, base, "Carpenter")

	// Rename.
	_, body := getPage(t, client, base, "/directory/categories")
	csrf := csrfRe.FindStringSubmatch(body)
	code, _ := postForm(t, client, base, "/directory/categories/"+catID+"/rename", map[string]string{
		"name": "Carpentry", "csrf_token": csrf[1],
	})
	if code != http.StatusSeeOther {
		t.Fatalf("rename = %d, want 303", code)
	}
	code, body = getPage(t, client, base, "/directory/categories")
	if !strings.Contains(body, "Carpentry") {
		t.Fatal("renamed category not shown")
	}

	// Duplicate active name → 400.
	code, body = getPage(t, client, base, "/directory/categories")
	csrf = csrfRe.FindStringSubmatch(body)
	code, _ = postForm(t, client, base, "/directory/categories", map[string]string{
		"name": "carpentry", "csrf_token": csrf[1],
	})
	if code != http.StatusBadRequest {
		t.Fatalf("duplicate category = %d, want 400", code)
	}

	// Deactivate; category disappears from the submission form.
	_, body = getPage(t, client, base, "/directory/categories")
	csrf = csrfRe.FindStringSubmatch(body)
	code, _ = postForm(t, client, base, "/directory/categories/"+catID+"/deactivate", map[string]string{"csrf_token": csrf[1]})
	if code != http.StatusSeeOther {
		t.Fatalf("deactivate = %d, want 303", code)
	}
	_, body = getPage(t, client, base, "/directory/new")
	if strings.Contains(body, "Carpentry") {
		t.Fatal("deactivated category still offered in submission form")
	}
	_, body = getPage(t, client, base, "/directory/categories")
	if !strings.Contains(body, "deactivated") {
		t.Fatal("deactivated badge missing on categories page")
	}
}

func TestIntegrationDirectoryResidentCannotModerate(t *testing.T) {
	router := newIntegrationRouter(t)
	ts := httptest.NewServer(router)
	defer ts.Close()
	base := ts.URL
	syndic := loginAsSyndic(t, base)
	catID := createDirectoryCategory(t, syndic, base, "Plumber")
	createDirectoryListing(t, syndic, base, catID, "Ana's Plumbing", "+55 11 91234-5678", "", "")
	_, body := getPage(t, syndic, base, "/directory")
	re := regexp.MustCompile(`href="/directory/([0-9a-f-]+)/edit"`)
	m := re.FindStringSubmatch(body)
	if m == nil {
		t.Fatal("no edit link for listing")
	}
	listingID := m[1]

	// Register an owner-only resident via an invitation.
	unitID := os.Getenv("TEST_UNIT")
	code, body := postForm(t, syndic, base, "/invitations", map[string]string{
		"unit_id": unitID, "invited_role": "owner", "invited_email": "owner2@example.com",
		"csrf_token": csrfFrom(t, syndic, base, "/invitations"),
	})
	if code != http.StatusOK {
		t.Fatalf("owner invitation = %d", code)
	}
	token := rawTokenFromBody(body)
	owner := newClient(t)
	postForm(t, owner, base, "/register", map[string]string{
		"token": token, "email": "owner2@example.com", "password": "owner-pass-123",
		"password_confirm": "owner-pass-123", "csrf_token": csrfFrom(t, owner, base, "/register?token="+token),
	})
	code, _ = postForm(t, owner, base, "/login", map[string]string{
		"email": "owner2@example.com", "password": "owner-pass-123",
		"csrf_token": csrfFrom(t, owner, base, "/login"),
	})
	if code != http.StatusSeeOther {
		t.Fatalf("owner login = %d", code)
	}

	// Owner can view the directory but cannot moderate.
	if code, _ := getPage(t, owner, base, "/directory"); code != http.StatusOK {
		t.Fatalf("owner /directory = %d, want 200", code)
	}
	if code, _ := getPage(t, owner, base, "/directory/"+listingID+"/edit"); code != http.StatusForbidden {
		t.Fatalf("owner edit page = %d, want 403", code)
	}
	code, _ = postForm(t, owner, base, "/directory/"+listingID+"/delete", map[string]string{
		"csrf_token": csrfFrom(t, owner, base, "/directory/new"),
	})
	if code != http.StatusForbidden {
		t.Fatalf("owner delete = %d, want 403", code)
	}
}
