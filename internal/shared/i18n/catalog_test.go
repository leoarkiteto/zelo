package i18n

import "testing"

// requiredMessages lists the user-facing messages every feature ships in both
// languages (FR-008). Add a key here whenever a new user-facing string is
// introduced so the catalog can never silently miss a translation.
var requiredMessages = map[string][]MessageKey{
	"shell": {
		"nav.dashboard", "nav.directory", "nav.management", "nav.invitations",
		"nav.roles", "nav.unit", "nav.tenancy", "nav.group",
		"topbar.sign_out", "topbar.brand_subtitle", "topbar.toggle_nav",
	},
	"error": {
		"error.title", "error.go_to_dashboard",
	},
	"profile": {
		"profile.title", "profile.subtitle", "profile.language_heading",
		"profile.language_hint", "profile.option_en", "profile.option_pt_br",
		"profile.error.invalid_language", "profile.error.load_failed",
		"profile.error.save_failed",
	},
	"auth": {
		"auth.sign_in", "auth.sign_in_title", "auth.welcome_back",
		"auth.email", "auth.password", "auth.forgot_password",
		"auth.password_placeholder", "auth.reset_link", "auth.show_password", "auth.hide_password",
		"auth.register", "auth.create_account", "auth.complete_registration",
		"auth.confirm_password", "auth.password_hint", "auth.repeat_password",
		"auth.invited_as", "auth.reset_password", "auth.forgot_heading",
		"auth.forgot_subtitle", "auth.reset_sent", "auth.send_reset_link",
		"auth.remembered_it", "auth.choose_new_password",
		"auth.password_min_hint", "auth.new_password", "auth.set_new_password",
	},
	"home": {
		"home.greeting.morning", "home.greeting.afternoon", "home.greeting.evening",
		"home.dashboard_title", "home.dashboard_subtitle", "home.get_started",
		"home.quick_actions", "home.your_roles",
		"home.action.condominium", "home.action.condominium_desc",
		"home.action.invitations", "home.action.invitations_desc",
		"home.action.roles", "home.action.roles_desc",
		"home.action.unit", "home.action.unit_desc",
		"home.action.tenancy", "home.action.tenancy_desc",
		"home.error.load_roles", "home.error.load_session",
	},
	"area": {
		"area.breadcrumb_label", "area.condominium_title",
		"area.condominium_message", "area.unit_title", "area.unit_message",
		"area.tenancy_title", "area.tenancy_message",
	},
	"common": {
		"common.cancel", "common.clear", "common.search", "common.save_changes",
		"common.delete", "common.edit", "common.actions", "common.status",
		"common.name", "common.email", "common.phone", "common.notes",
		"common.category", "common.dashboard", "common.role", "common.unit",
		"common.back_to_directory", "common.invite_user", "common.recommend",
		"common.owner", "common.tenant",
	},
	"directory": {
		"directory.title", "directory.subtitle", "directory.manage_categories",
		"directory.recommend", "directory.search_label",
		"directory.search_placeholder", "directory.category_label",
		"directory.all_categories", "directory.listings",
		"directory.empty_title", "directory.empty_copy",
		"directory.col.professional", "directory.col.recommended_by",
		"directory.unit_prefix", "directory.new_title",
		"directory.new_subtitle", "directory.edit_title",
		"directory.edit_subtitle", "directory.professional_name",
		"directory.placeholder_name", "directory.phone_number",
		"directory.notes_optional", "directory.confirm_duplicate",
		"directory.add_to_directory", "directory.categories_title",
		"directory.categories_subtitle", "directory.add_category_heading",
		"directory.category_name", "directory.placeholder_category",
		"directory.add_category", "directory.categories_heading",
		"directory.categories_empty_title", "directory.categories_empty_copy",
		"directory.status.active", "directory.status.deactivated",
		"directory.new_name_aria", "directory.rename", "directory.deactivate",
		"directory.delete_title", "directory.delete_subtitle",
		"directory.delete_warning_1", "directory.delete_warning_2",
		"directory.error.no_unit", "directory.error.category",
		"directory.error.phone", "directory.error.required",
		"directory.error.listing_gone", "directory.error.duplicate",
		"directory.error.category_gone", "directory.error.category_duplicate",
		"directory.error.category_name", "directory.flash.created",
		"directory.flash.updated", "directory.flash.deleted",
		"directory.flash.category_added", "directory.flash.category_renamed",
		"directory.flash.category_deactivated", "directory.error.load_listings",
		"directory.error.load_session", "directory.error.create_listing",
		"directory.error.update_listing", "directory.error.delete_listing",
		"directory.error.load_categories", "directory.error.add_category",
		"directory.error.rename_category", "directory.error.deactivate_category",
	},
	"invitations": {
		"invitations.title", "invitations.subtitle",
		"invitations.create_heading", "invitations.email_optional",
		"invitations.create_button", "invitations.recent_heading",
		"invitations.empty_title", "invitations.empty_copy",
		"invitations.col.expires", "invitations.revoke",
		"invitations.status.pending", "invitations.status.used",
		"invitations.status.revoked", "invitations.error.role",
		"invitations.error.load", "invitations.error.create",
		"invitations.error.revoke", "invitations.flash.created",
	},
	"roles": {
		"roles.title", "roles.subtitle", "roles.empty_title",
		"roles.empty_copy", "roles.col.user", "roles.col.roles",
		"roles.grant", "roles.revoke", "roles.grant_aria", "roles.revoke_aria",
		"roles.error.unknown_role", "roles.error.not_syndic",
		"roles.error.syndic_owner", "roles.error.tenant_not_eligible",
		"roles.error.load", "roles.error.change",
	},
}

// TestCatalogCoversRequiredFeatureMessages guards FR-008: every required
// user-facing message must exist in both en and pt-br.
func TestCatalogCoversRequiredFeatureMessages(t *testing.T) {
	for group, keys := range requiredMessages {
		for _, key := range keys {
			if _, ok := catalog[LanguageEN][key]; !ok {
				t.Errorf("missing English message %q in group %s", key, group)
			}
			if _, ok := catalog[LanguagePTBR][key]; !ok {
				t.Errorf("missing pt-br message %q in group %s", key, group)
			}
		}
	}
}
