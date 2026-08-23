# Feature Specification: i18n Language Switcher

**Feature Branch**: `005-i18n-language-switcher`

**Created**: 2026-08-23

**Status**: Draft

**Input**: User description: "Add feature about internationalization, the user can switch between `en` and `pt-br` when necessary" — refined during planning: toggle button in the profile page, default language `en`, one country flag per language in the UI, and the preference saved so it is applied when the user logs in.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - User toggles the interface language from the profile page (Priority: P1)

A signed-in user opens their profile page and sees a language toggle with the two available languages, each identified by its language name and a country flag. When the user switches between English (`en`) and Brazilian Portuguese (`pt-br`), all visible interface text — menus, buttons, labels, and messages — changes to the chosen language immediately, without losing the page they were on or any information they had already entered.

**Why this priority**: This is the core of the request. If users cannot switch languages from their profile, the feature does not exist.

**Independent Test**: Sign in, open the profile page, switch the language from `en` to `pt-br`, and confirm every visible interface text changes to Brazilian Portuguese immediately. Switch back and confirm it reverts to English.

**Acceptance Scenarios**:

1. **Given** a signed-in user is on the profile page in English, **When** they toggle to Português (BR), **Then** all visible interface text changes to Brazilian Portuguese immediately and the profile data remains intact.
2. **Given** a signed-in user is on the profile page in Brazilian Portuguese, **When** they toggle to English, **Then** the interface changes to English immediately.
3. **Given** a signed-in user has unsaved input on the profile page, **When** they toggle the language, **Then** their input is preserved and the form labels and validation messages update to the chosen language.

---

### User Story 2 - Language preference is saved and applied at login (Priority: P2)

When a signed-in user changes their language, the choice is saved to their account. The next time they log in — on any device — the application starts in that saved language, so they do not have to switch again.

**Why this priority**: Persistence is what makes the toggle worthwhile. Without it, users would have to repeat the switch on every login.

**Independent Test**: Sign in, toggle the language to `pt-br`, sign out, then sign in again. Confirm the application appears in Brazilian Portuguese immediately after login.

**Acceptance Scenarios**:

1. **Given** a signed-in user toggles the language to `pt-br`, **When** they sign out and sign back in, **Then** the interface appears in Brazilian Portuguese.
2. **Given** a signed-in user toggles the language to English, **When** they sign in on another device, **Then** the interface appears in English there too.
3. **Given** a user has never changed their language, **When** they log in, **Then** the interface appears in the default language, English.

---

### User Story 3 - Each language option shows its country flag (Priority: P3)

The language toggle presents each language clearly: English with the United States flag and Brazilian Portuguese with the Brazil flag. The flags help users recognize their language at a glance, and each option also has an accessible text label so the choice is clear to everyone.

**Why this priority**: The flags are an explicit part of the request and improve recognition, but the core value of switching and saving works without them.

**Independent Test**: Open the profile page and confirm the toggle shows exactly two options: English with the United States flag and Português (BR) with the Brazil flag.

**Acceptance Scenarios**:

1. **Given** a signed-in user opens the profile page, **When** they look at the language toggle, **Then** they see an English option with the United States flag and a Português (BR) option with the Brazil flag.
2. **Given** the language toggle, **When** a screen reader or keyboard user reaches it, **Then** each option is announced with its language name and the flag is not the only way to tell the options apart.
3. **Given** the current language is English, **When** the user views the toggle, **Then** the English option is clearly indicated as selected.

---

### User Story 4 - Every user-facing message appears in the selected language (Priority: P4)

All user-facing text is available in both languages, including navigation, buttons, labels, help text, validation messages, error messages, empty states, confirmations, and notifications. Users never see mixed languages, cryptic technical codes, or blank text.

**Why this priority**: A partially translated interface looks broken and erodes user trust. Complete coverage is what makes the first three stories valuable.

**Independent Test**: Walk through the main flows of the application once in English and once in Brazilian Portuguese and confirm every user-facing message appears in the selected language.

**Acceptance Scenarios**:

1. **Given** the interface is in English, **When** a user triggers validation errors, empty states, and confirmations, **Then** all messages appear in English.
2. **Given** the interface is in Brazilian Portuguese, **When** a user triggers validation errors, empty states, and confirmations, **Then** all messages appear in `pt-br`.
3. **Given** any screen in the application, **When** a user reads it in the selected language, **Then** no text appears in the other language or as cryptic technical codes or blank spaces.

---

### Edge Cases

- What happens when a user toggles back and forth between languages? The interface updates each time with no data loss and no interruption to the current task.
- What happens before a user logs in? The application shows English by default; the language toggle is only available on the profile page after login.
- What happens when a user's saved language preference is missing or invalid? The application falls back to the default language, English.
- What happens when a specific message is missing in the selected language? The application falls back to the English version of that message and never shows a technical code, placeholder identifier, or empty text; the gap is recorded for correction.
- What happens with user-generated content such as names, descriptions, and messages? It is always shown exactly as the user entered it and is never machine-translated.
- What happens with dates, times, and numbers? They are displayed according to the conventions of the selected language.
- What happens when a screen-reader or keyboard user toggles the language? The change is announced, focus is preserved, and the toggle remains operable by keyboard; flags are never the only way to identify an option.
- What happens when the profile page has unsaved changes and the user toggles the language? The profile changes are preserved and only the interface language updates.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The application MUST offer exactly two interface languages: English (`en`) and Brazilian Portuguese (`pt-br`).
- **FR-002**: The profile page MUST display a visible, accessible language toggle with exactly two options: English with the United States flag and Português (BR) with the Brazil flag.
- **FR-003**: Signed-in users MUST be able to switch language from the profile page without losing unsaved input or being moved away from the current page.
- **FR-004**: After a user selects a language, all user-facing interface text MUST update to the chosen language immediately.
- **FR-005**: The language choice MUST be saved to the signed-in user's account.
- **FR-006**: When a user logs in, the application MUST apply their saved language preference, falling back to English when no preference exists.
- **FR-007**: The default interface language MUST be English (`en`) for users without a saved preference.
- **FR-008**: All user-facing text — navigation, buttons, labels, help text, validation messages, error messages, empty states, confirmations, and notifications — MUST be available in both languages.
- **FR-009**: If a message is missing in the selected language, the application MUST display the English version and MUST NOT display technical codes, placeholder identifiers, or empty text.
- **FR-010**: Dates, times, and numbers MUST follow the display conventions of the selected language.
- **FR-011**: User-generated content MUST be displayed as entered and MUST NOT be machine-translated.
- **FR-012**: The language toggle MUST be operable by keyboard, announced by assistive technology, and clearly indicate the current language; flags MUST be accompanied by accessible text labels.

### Key Entities

- **Language Preference**: The signed-in user's chosen interface language — `en` or `pt-br` — saved to the account and applied at login.
- **User Account**: A signed-in user's account, which links to a stored language preference.
- **User-Facing Text**: Every label, message, and instruction shown in the interface, each with an English version and a Brazilian Portuguese version.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: 100% of visible interface text changes to the selected language immediately after the user toggles.
- **SC-002**: A signed-in user can change the interface language from the profile page in 2 interactions or fewer and in under 5 seconds.
- **SC-003**: 100% of signed-in users see their saved language immediately after login.
- **SC-004**: 100% of users without a saved preference see English by default.
- **SC-005**: 100% of user-facing text shipped with the feature is available in both `en` and `pt-br`.
- **SC-006**: 0 occurrences of cryptic technical codes, blank text, or mixed-language screens in a manual review of key flows in both languages.
- **SC-007**: 100% of language toggles preserve unsaved user input and current page state.

## Assumptions

- Version 1 supports exactly `en` and `pt-br`; adding more languages is future work.
- English is the default language per the product decision captured during planning.
- The language toggle lives only on the profile page; pages shown before login appear in English.
- English is represented by the United States flag and Brazilian Portuguese by the Brazil flag; each flag is paired with an accessible text label.
- "Internationalization" in version 1 covers the web application interface only. Emails, generated documents, and notifications delivered outside the application are out of scope.
- User-generated content is never translated.
- Translations are supplied and approved by the product owner; no automated machine translation is used.
- Date, time, and number display follows conventional `pt-BR` and `en` formats.
- The existing sign-in and session system is reused to identify the signed-in user and store the per-account language preference.
