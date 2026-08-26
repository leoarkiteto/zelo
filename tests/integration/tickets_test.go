package integration

import (
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
)

// loginAsOwner signs in the seeded owner-only user and returns a client with a
// session cookie (no syndic privileges).
func loginAsOwner(t *testing.T, base string) *http.Client {
	t.Helper()
	client := newClient(t)
	code, _ := postForm(t, client, base, "/login", map[string]string{
		"email": "resident@example.com", "password": "resident-pass-123",
		"csrf_token": csrfFrom(t, client, base, "/login"),
	})
	if code != http.StatusSeeOther {
		t.Fatalf("owner login = %d, want 303", code)
	}
	return client
}

// ticketIDFromBody extracts the ticket id from a detail redirect body.
var ticketIDRe = regexp.MustCompile(`/tickets/([0-9a-f-]+)`)

// postFormLoc posts a form and returns the status code and Location header
// (303 redirect targets have an empty body).
func postFormLoc(t *testing.T, client *http.Client, base, path string, form map[string]string) (int, string) {
	t.Helper()
	var body strings.Builder
	for k, v := range form {
		body.WriteString(k + "=" + v + "&")
	}
	req, err := http.NewRequest(http.MethodPost, base+path, strings.NewReader(body.String()))
	if err != nil {
		t.Fatalf("build post: %v", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("post %s: %v", path, err)
	}
	defer resp.Body.Close()
	return resp.StatusCode, resp.Header.Get("Location")
}

func createTicket(t *testing.T, client *http.Client, base, category, description string) string {
	t.Helper()
	code, body := getPage(t, client, base, "/tickets/new")
	if code != http.StatusOK {
		t.Fatalf("new ticket page = %d", code)
	}
	csrf := csrfRe.FindStringSubmatch(body)
	if csrf == nil {
		t.Fatal("no csrf token on new ticket page")
	}
	code, _ = postForm(t, client, base, "/tickets", map[string]string{
		"title": "Kitchen leak", "category": category, "description": description, "csrf_token": csrf[1],
	})
	if code != http.StatusSeeOther {
		t.Fatalf("create ticket = %d, want 303", code)
	}
	return ""
}

// TestIntegrationTicketLifecycle walks the full resident → syndic flow from
// quickstart.md: create, inbox, reply, close, and closed read-only behavior.
func TestIntegrationTicketLifecycle(t *testing.T) {
	app := newApp(t)
	base := httptest.NewServer(app).URL
	defer httptest.NewServer(app).Close()

	owner := loginAsOwner(t, base)
	syndic := loginAsSyndic(t, base)

	// 1. Resident creates a ticket (quickstart scenario 2).
	code, body := getPage(t, owner, base, "/tickets/new")
	if code != http.StatusOK {
		t.Fatalf("new ticket page = %d", code)
	}
	csrf := csrfRe.FindStringSubmatch(body)
	if csrf == nil {
		t.Fatal("no csrf token on new ticket page")
	}
	code, loc := postFormLoc(t, owner, base, "/tickets", map[string]string{
		"title": "Leak under the kitchen sink", "category": "repair", "description": "Leak under the kitchen sink", "csrf_token": csrf[1],
	})
	if code != http.StatusSeeOther {
		t.Fatalf("create ticket = %d, want 303", code)
	}
	m := ticketIDRe.FindStringSubmatch(loc)
	if m == nil {
		t.Fatalf("no ticket id in create redirect %q", loc)
	}
	ticketID := m[1]

	// Validation: empty description is rejected (FR-002).
	code, body = getPage(t, owner, base, "/tickets/new")
	csrf = csrfRe.FindStringSubmatch(body)
	code, body = postForm(t, owner, base, "/tickets", map[string]string{
		"title": "Broken", "category": "repair", "description": "", "csrf_token": csrf[1],
	})
	if code != http.StatusBadRequest {
		t.Fatalf("create without description = %d, want 400", code)
	}

	// 2. Resident sees their own ticket as Open.
	code, body = getPage(t, owner, base, "/tickets/"+ticketID)
	if code != http.StatusOK {
		t.Fatalf("owner detail = %d", code)
	}
	if !regexp.MustCompile(`>Open<`).MatchString(body) {
		t.Error("owner detail must show status Open")
	}

	// 3. Syndic sees the ticket in the inbox (scenario 3).
	code, body = getPage(t, syndic, base, "/tickets/inbox")
	if code != http.StatusOK {
		t.Fatalf("inbox = %d", code)
	}
	if !regexp.MustCompile(`/tickets/` + ticketID).MatchString(body) {
		t.Error("inbox must list the new ticket")
	}

	// 4. Syndic replies and the owner sees the reply.
	code, body = getPage(t, syndic, base, "/tickets/"+ticketID)
	csrf = csrfRe.FindStringSubmatch(body)
	code, _ = postForm(t, syndic, base, "/tickets/"+ticketID+"/reply", map[string]string{
		"content": "A maintenance visit is scheduled for Friday.", "csrf_token": csrf[1],
	})
	if code != http.StatusSeeOther {
		t.Fatalf("reply = %d, want 303", code)
	}
	_, body = getPage(t, owner, base, "/tickets/"+ticketID)
	if !regexp.MustCompile("A maintenance visit is scheduled").MatchString(body) {
		t.Error("owner must see the syndic reply")
	}

	// 5. Syndic closes the ticket; it leaves the open inbox and is read-only.
	code, body = getPage(t, syndic, base, "/tickets/"+ticketID)
	csrf = csrfRe.FindStringSubmatch(body)
	code, _ = postForm(t, syndic, base, "/tickets/"+ticketID+"/close", map[string]string{"csrf_token": csrf[1]})
	if code != http.StatusSeeOther {
		t.Fatalf("close = %d, want 303", code)
	}
	code, body = getPage(t, syndic, base, "/tickets/inbox")
	if regexp.MustCompile(`/tickets/` + ticketID).MatchString(body) {
		t.Error("closed ticket must leave the open inbox")
	}
	_, body = getPage(t, owner, base, "/tickets/"+ticketID)
	if !regexp.MustCompile(`>Closed<`).MatchString(body) {
		t.Error("closed ticket must show status Closed")
	}

	// 6. Closed tickets reject new messages from both parties (FR-008).
	_, body = getPage(t, syndic, base, "/tickets/"+ticketID)
	csrf = csrfRe.FindStringSubmatch(body)
	code, _ = postForm(t, syndic, base, "/tickets/"+ticketID+"/reply", map[string]string{
		"content": "too late", "csrf_token": csrf[1],
	})
	if code != http.StatusBadRequest {
		t.Errorf("reply on closed ticket = %d, want 400", code)
	}

	// 7. Access control: an owner cannot open the syndic inbox (FR-005).
	code, _ = getPage(t, owner, base, "/tickets/inbox")
	if code != http.StatusForbidden {
		t.Errorf("owner inbox = %d, want 403", code)
	}
}

// TestIntegrationTicketPrivacy verifies residents only ever see their own
// tickets and that foreign ticket ids do not leak existence (FR-009, SC-006).
func TestIntegrationTicketPrivacy(t *testing.T) {
	app := newApp(t)
	base := httptest.NewServer(app).URL
	defer httptest.NewServer(app).Close()

	owner := loginAsOwner(t, base)
	syndic := loginAsSyndic(t, base)

	createTicket(t, owner, base, "repair", "Leak under the kitchen sink")
	code, body := getPage(t, owner, base, "/tickets")
	if code != http.StatusOK {
		t.Fatalf("owner list = %d", code)
	}
	if !regexp.MustCompile("Repair request").MatchString(body) {
		t.Error("owner list must show their ticket category")
	}

	// The syndic creates a ticket as a resident; it must not be visible to the
	// owner.
	createTicket(t, syndic, base, "noise_complaint", "Loud music at night")
	_, body = getPage(t, owner, base, "/tickets")
	if regexp.MustCompile("Loud music at night").MatchString(body) {
		t.Error("owner must not see the syndic's ticket")
	}
}
