package services

import (
	"context"
	"errors"
	"testing"

	"github.com/leoarkiteto/zelo/internal/features/profile/domain"
	"github.com/leoarkiteto/zelo/internal/shared/i18n"
	"github.com/leoarkiteto/zelo/internal/shared/model"
)

type fakeUserStore struct {
	user model.User
	err  error
}

func (f *fakeUserStore) GetUserByID(_ context.Context, _ string) (model.User, error) {
	return f.user, f.err
}

type fakePreferenceUpdater struct {
	called   bool
	userID   string
	language string
	err      error
}

func (f *fakePreferenceUpdater) UpdateLanguagePreference(_ context.Context, userID, language string) error {
	f.called = true
	f.userID = userID
	f.language = language
	return f.err
}

func TestGetProfileReturnsCurrentLanguage(t *testing.T) {
	svc := &ProfileService{
		Users: &fakeUserStore{user: model.User{ID: "u1", LanguagePreference: "pt-br"}},
	}
	prof, err := svc.GetProfile(context.Background(), "u1")
	if err != nil {
		t.Fatalf("GetProfile: %v", err)
	}
	if prof.UserID != "u1" {
		t.Errorf("UserID = %q, want u1", prof.UserID)
	}
	if prof.Language != i18n.LanguagePTBR {
		t.Errorf("Language = %q, want %q", prof.Language, i18n.LanguagePTBR)
	}
}

func TestGetProfileDefaultsToEnglish(t *testing.T) {
	svc := &ProfileService{
		Users: &fakeUserStore{user: model.User{ID: "u1", LanguagePreference: ""}},
	}
	prof, err := svc.GetProfile(context.Background(), "u1")
	if err != nil {
		t.Fatalf("GetProfile: %v", err)
	}
	if prof.Language != i18n.Default() {
		t.Errorf("Language = %q, want %q", prof.Language, i18n.Default())
	}
}

func TestChangeLanguagePersistsPreference(t *testing.T) {
	updater := &fakePreferenceUpdater{}
	svc := &ProfileService{Preferences: updater}
	if err := svc.ChangeLanguage(context.Background(), "u1", i18n.LanguagePTBR); err != nil {
		t.Fatalf("ChangeLanguage: %v", err)
	}
	if !updater.called {
		t.Fatal("ChangeLanguage did not call the persistence port")
	}
	if updater.userID != "u1" || updater.language != "pt-br" {
		t.Errorf("persisted (%q, %q), want (u1, pt-br)", updater.userID, updater.language)
	}
}

func TestChangeLanguageRejectsInvalid(t *testing.T) {
	updater := &fakePreferenceUpdater{}
	svc := &ProfileService{Preferences: updater}
	err := svc.ChangeLanguage(context.Background(), "u1", i18n.Language("fr"))
	if !errors.Is(err, domain.ErrInvalidLanguage) {
		t.Fatalf("ChangeLanguage(invalid) = %v, want ErrInvalidLanguage", err)
	}
	if updater.called {
		t.Fatal("ChangeLanguage persisted an invalid language")
	}
}
