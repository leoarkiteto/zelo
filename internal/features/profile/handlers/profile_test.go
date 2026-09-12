package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/leoarkiteto/zelo/internal/features/profile/domain"
	"github.com/leoarkiteto/zelo/internal/shared/i18n"
	"github.com/leoarkiteto/zelo/internal/shared/middleware"
	"github.com/leoarkiteto/zelo/internal/shared/model"
)

type fakeRoles struct {
	roles []model.Role
}

func (f *fakeRoles) ActiveRolesForUser(_ context.Context, _, _ string) ([]model.Role, error) {
	return f.roles, nil
}

type fakeSessionReader struct{ sess model.Session }

func (f fakeSessionReader) Read(*http.Request) (model.Session, error) { return f.sess, nil }

type fakeUserLoader struct{ u model.User }

func (f fakeUserLoader) GetUserByID(_ context.Context, _ string) (model.User, error) { return f.u, nil }

type fakeProfileService struct {
	profile  domain.Profile
	err      error
	lastLang i18n.Language
	changed  bool
}

func (f *fakeProfileService) GetProfile(_ context.Context, userID string) (domain.Profile, error) {
	f.profile.UserID = userID
	return f.profile, f.err
}

func (f *fakeProfileService) ChangeLanguage(_ context.Context, _ string, lang i18n.Language) error {
	f.changed = true
	f.lastLang = lang
	return f.err
}

func profileRouter(user *model.User, sess *model.Session, svc Profile) http.Handler {
	mux := http.NewServeMux()
	RegisterRoutes(mux, Deps{
		Roles:   &fakeRoles{roles: []model.Role{model.RoleSyndic}},
		Profile: svc,
	})
	var root http.Handler = mux
	if user != nil && sess != nil {
		root = middleware.WithLocale(root)
		root = middleware.WithUser(fakeSessionReader{sess: *sess}, fakeUserLoader{u: *user})(root)
	}
	return root
}

func TestProfilePageUnauthenticatedRedirects(t *testing.T) {
	router := profileRouter(nil, nil, &fakeProfileService{})
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/profile", nil))
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/login" {
		t.Fatalf("GET /profile = %d %q, want 303 /login", rec.Code, rec.Header().Get("Location"))
	}
}

func TestProfilePageRendersToggle(t *testing.T) {
	user := &model.User{ID: "u1", Email: "syndic@example.com", Status: model.UserStatusActive}
	sess := &model.Session{TokenHash: "th", UserID: "u1", CondominiumID: "c1", CSRFToken: "x"}
	svc := &fakeProfileService{profile: domain.Profile{Language: i18n.LanguageEN}}
	router := profileRouter(user, sess, svc)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/profile", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /profile = %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	for _, want := range []string{"Interface language", "English", "Português (BR)"} {
		if !strings.Contains(body, want) {
			t.Errorf("profile page is missing %q", want)
		}
	}
}

func TestProfileLanguageHTMXSwapsShell(t *testing.T) {
	user := &model.User{ID: "u1", Email: "syndic@example.com", Status: model.UserStatusActive}
	sess := &model.Session{TokenHash: "th", UserID: "u1", CondominiumID: "c1", CSRFToken: "x"}
	svc := &fakeProfileService{profile: domain.Profile{Language: i18n.LanguageEN}}
	router := profileRouter(user, sess, svc)

	form := url.Values{"language": {"pt-br"}}
	req := httptest.NewRequest(http.MethodPost, "/profile/language", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("HX-Request", "true")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("POST /profile/language (HTMX) = %d, want 200", rec.Code)
	}
	if !svc.changed || svc.lastLang != i18n.LanguagePTBR {
		t.Fatalf("service not called with pt-br: changed=%v lastLang=%q", svc.changed, svc.lastLang)
	}
	body := rec.Body.String()
	if !strings.Contains(body, `<div id="app-shell" lang="pt-br"`) {
		t.Errorf("HTMX response missing re-rendered shell with lang=pt-br")
	}
	// The re-rendered toggle must mark pt-br selected and leave en unchecked
	// (no checked attribute at all on the inactive radio).
	if !strings.Contains(body, `value="pt-br" checked`) {
		t.Errorf("HTMX response does not mark pt-br selected")
	}
	if strings.Contains(body, `value="en" checked`) {
		t.Errorf("HTMX response leaves the inactive en radio checked")
	}
}

func TestProfileLanguagePlainFormRedirects(t *testing.T) {
	user := &model.User{ID: "u1", Email: "syndic@example.com", Status: model.UserStatusActive}
	sess := &model.Session{TokenHash: "th", UserID: "u1", CondominiumID: "c1", CSRFToken: "x"}
	svc := &fakeProfileService{profile: domain.Profile{Language: i18n.LanguageEN}}
	router := profileRouter(user, sess, svc)

	form := url.Values{"language": {"pt-br"}}
	req := httptest.NewRequest(http.MethodPost, "/profile/language", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/profile" {
		t.Fatalf("POST /profile/language (plain) = %d %q, want 303 /profile", rec.Code, rec.Header().Get("Location"))
	}
}

func TestProfileLanguageInvalidReturns400(t *testing.T) {
	user := &model.User{ID: "u1", Email: "syndic@example.com", Status: model.UserStatusActive}
	sess := &model.Session{TokenHash: "th", UserID: "u1", CondominiumID: "c1", CSRFToken: "x"}
	svc := &fakeProfileService{}
	router := profileRouter(user, sess, svc)

	form := url.Values{"language": {"fr"}}
	req := httptest.NewRequest(http.MethodPost, "/profile/language", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("HX-Request", "true")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("POST /profile/language (invalid) = %d, want 400", rec.Code)
	}
	if svc.changed {
		t.Fatal("invalid language was persisted")
	}
}
