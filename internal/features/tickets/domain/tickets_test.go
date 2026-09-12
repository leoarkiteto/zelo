package domain

import (
	"strings"
	"testing"
	"time"
)

func TestTicketCategoryValid(t *testing.T) {
	valid := map[TicketCategory]bool{
		CategoryRepair:         true,
		CategoryNoiseComplaint: true,
		CategoryAssemblyTopic:  true,
		CategoryOther:          true,
		"plumbing":             false,
		"":                     false,
	}
	for c, want := range valid {
		if got := c.Valid(); got != want {
			t.Errorf("Valid(%q) = %v, want %v", c, got, want)
		}
	}
	if got := len(Categories()); got != 4 {
		t.Errorf("len(Categories()) = %d, want 4", got)
	}
}

func TestValidateTitle(t *testing.T) {
	long := strings.Repeat("t", 121)
	cases := []struct {
		name string
		in   string
		want bool
	}{
		{"empty", "", false},
		{"whitespace", "   ", false},
		{"one char", "a", true},
		{"max length", strings.Repeat("t", 120), true},
		{"too long", long, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := ValidateTitle(c.in)
			if got := err == nil; got != c.want {
				t.Errorf("ValidateTitle(%q) error=%v, want success=%v", c.in, err, c.want)
			}
		})
	}
}

func TestTicketStatusValid(t *testing.T) {
	if !StatusOpen.Valid() || !StatusClosed.Valid() {
		t.Error("open and closed must be valid statuses")
	}
	if (TicketStatus("reopened")).Valid() {
		t.Error("reopened must not be a valid status")
	}
}

func TestValidateDescription(t *testing.T) {
	long := strings.Repeat("a", 2001)
	cases := []struct {
		name string
		in   string
		want bool
	}{
		{"empty", "", false},
		{"whitespace", "   ", false},
		{"one char", "a", true},
		{"max length", strings.Repeat("a", 2000), true},
		{"too long", long, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := ValidateDescription(c.in)
			if got := err == nil; got != c.want {
				t.Errorf("ValidateDescription(%q) error=%v, want success=%v", c.in, err, c.want)
			}
		})
	}
}

func TestValidateContent(t *testing.T) {
	long := strings.Repeat("x", 2001)
	cases := []struct {
		name string
		in   string
		want bool
	}{
		{"empty", "", false},
		{"whitespace", " \n ", false},
		{"max length", strings.Repeat("x", 2000), true},
		{"too long", long, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := ValidateContent(c.in)
			if got := err == nil; got != c.want {
				t.Errorf("ValidateContent(%q) error=%v, want success=%v", c.in, err, c.want)
			}
		})
	}
}

func TestTicketTransitions(t *testing.T) {
	open := Ticket{Status: StatusOpen}
	if !open.CanReply() || !open.CanClose() {
		t.Error("open ticket must allow reply and close")
	}
	closed := Ticket{Status: StatusClosed, ClosedAt: &time.Time{}, ClosedBy: "u2"}
	if closed.CanReply() || closed.CanClose() {
		t.Error("closed ticket must be read-only (no reply, no close)")
	}
}
