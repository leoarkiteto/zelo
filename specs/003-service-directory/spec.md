# Feature Specification: Service Provider Directory

**Feature Branch**: `003-service-directory`

**Created**: 2026-08-22

**Status**: Draft

**Input**: User description: "Implement a service provider directory feature. Allows residents to register recommended service professionals (including name, speciality/category, phone number, notes, and the recommending resident's unit). Residents can search and filter the directory by category or keyword. The syndic can moderate, edit or delete listings. Directory access must be restricted to authenticated users belonging to condominium."

## Clarifications

### Session 2026-08-22

- Q: Should new listings submitted by residents appear in the directory immediately, or should they require syndic approval before becoming visible? → A: Listings are visible immediately; the syndic moderates afterwards by editing or deleting.
- Q: Who defines and maintains the list of categories/specialities that residents choose from when registering a professional? → A: The syndic can add, rename, and deactivate categories.
- Q: Can residents edit or delete the listings they themselves submitted? → A: Only the syndic can edit or delete listings; residents must contact the syndic.
- Q: How should the system handle duplicate recommendations of the same professional, such as a second resident submitting the same phone number? → A: Warn the resident if a listing with the same phone number already exists, but still allow saving.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Resident registers a recommended professional (Priority: P1)

A resident who has a trusted service professional (e.g., a plumber, electrician, or cleaner) can add that professional to the condominium directory so other residents can find them. The resident fills in the professional's name, speciality/category, phone number, optional notes, and their own unit is recorded automatically as the recommender.

**Why this priority**: The directory has value only if it contains listings. Registration is the primary way the directory grows and is the foundation that all other flows depend on.

**Independent Test**: Log in as a resident, submit a new listing with valid data, and verify the listing appears in the directory with the resident's unit shown as the recommender.

**Acceptance Scenarios**:

1. **Given** an authenticated resident of the condominium is on the directory page, **When** they submit a listing with name, category, phone number, and notes, **Then** the listing is saved and appears in the directory with the recommending resident's unit.
2. **Given** an authenticated resident submits a listing, **When** required fields (name, category, phone number) are missing or invalid, **Then** the system shows a clear validation message and does not save the listing.
3. **Given** an authenticated resident submits a listing, **When** the submission succeeds, **Then** the system shows a confirmation message and the listing is immediately visible to other authenticated residents.

---

### User Story 2 - Resident searches and filters the directory (Priority: P2)

A resident can search the directory by keyword and filter by category to quickly find a professional for a specific need.

**Why this priority**: Search and filtering make the directory usable as it grows; without them, residents must browse the entire list to find a professional.

**Independent Test**: With several listings present, log in as a resident and search by a keyword (e.g., a name or note text) and by category, and verify that only matching listings are shown.

**Acceptance Scenarios**:

1. **Given** the directory contains multiple listings, **When** a resident searches by keyword, **Then** the system shows listings whose name, category, or notes match the keyword.
2. **Given** the directory contains multiple listings, **When** a resident filters by category, **Then** the system shows only listings in that category.
3. **Given** a resident searches for a term with no matches, **When** the search executes, **Then** the system shows a friendly "no results" message.

---

### User Story 3 - Syndic moderates, edits, and deletes listings (Priority: P3)

The syndic can review directory content and edit or remove listings that are outdated, inaccurate, inappropriate, or duplicated, keeping the directory trustworthy.

**Why this priority**: Moderation protects directory quality and trust, but it becomes necessary only after residents have started adding and using listings.

**Independent Test**: Log in as the syndic, edit an existing listing's phone number, and delete another listing, then verify the changes are reflected for all residents.

**Acceptance Scenarios**:

1. **Given** the syndic is viewing the directory, **When** they edit a listing's fields, **Then** the updated information is saved and shown to residents.
2. **Given** the syndic chooses to delete a listing, **When** they confirm the deletion, **Then** the listing is removed from the directory and no longer appears in searches.
3. **Given** the syndic deletes or edits a listing, **When** the action completes, **Then** the system shows a confirmation message and the change is immediately reflected.

---

### Edge Cases

- What happens when an unauthenticated visitor tries to open the directory? The system must deny access and redirect them to sign in.
- What happens when an authenticated user does not belong to the condominium? The system must deny access to the directory.
- What happens when a resident submits a professional already in the directory? The system warns the resident when a listing with the same phone number already exists, but still allows saving; duplicates appear alongside existing listings and the syndic can later merge or remove them.
- What happens when a search keyword has no matches or a category has no listings? The system shows a clear empty state instead of a blank page.
- What happens when the syndic deletes a listing while a resident is viewing it? The next action taken on that listing fails gracefully with a "no longer available" message.
- What happens when special characters are entered in search or notes? The system handles them safely and displays text as entered.
- What happens when a resident tries to edit or delete someone else's listing? The system does not allow it; only the syndic can edit or delete listings.
- What happens when the phone number has an invalid format? The system rejects the submission with a clear message about the expected format.
- What happens when the syndic deactivates a category that already has listings? The category is removed from the submission and filter lists; existing listings keep their category text and remain searchable by keyword.
- What happens when the syndic renames a category? Existing listings immediately show the new category name.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The system MUST restrict all directory pages and actions to authenticated users who belong to the condominium.
- **FR-002**: The system MUST allow authenticated residents to submit a recommended service professional with name, speciality/category, phone number, optional notes, and the recommending resident's unit.
- **FR-003**: The system MUST record the recommending resident's unit automatically from the resident's profile when a listing is submitted.
- **FR-004**: The system MUST require a professional name, a category, and a phone number for every listing.
- **FR-005**: The system MUST validate the phone number format and reject submissions with an invalid phone number.
- **FR-006**: The system MUST show a friendly error message when required fields are missing or invalid.
- **FR-007**: The system MUST display each listing with the professional's name, category, phone number, notes, and the recommending resident's unit.
- **FR-008**: The system MUST allow residents to search listings by keyword across name, category, and notes.
- **FR-009**: The system MUST allow residents to filter listings by category.
- **FR-010**: The system MUST show a clear empty state when a search or filter returns no listings.
- **FR-011**: The system MUST allow the syndic to add, rename, and deactivate categories.
- **FR-012**: The system MUST only offer active categories when residents submit a listing and when they filter.
- **FR-013**: The system MUST allow the syndic to edit any listing's name, category, phone number, and notes.
- **FR-014**: The system MUST allow the syndic to delete any listing.
- **FR-015**: The system MUST require confirmation before a syndic deletes a listing.
- **FR-016**: The system MUST show a confirmation message after a listing is created, edited, or deleted.
- **FR-017**: The system MUST persist listings so they remain available across sessions.
- **FR-018**: The system MUST deny access to authenticated users who do not belong to the condominium, with an explanatory message.
- **FR-019**: The system MUST deny access to unauthenticated visitors and redirect them to sign in.
- **FR-020**: The system MUST NOT allow residents to edit or delete any listing; only the syndic may edit or delete listings.
- **FR-021**: The system MUST warn the resident during submission when a listing with the same phone number already exists, and MUST still allow saving.

### Key Entities *(include if feature involves data)*

- **Service Provider Listing**: A recommended professional; attributes are name, category, phone number, notes, recommending resident's unit, creation date, and the resident who submitted it.
- **Resident (User)**: An authenticated member of the condominium who can submit listings and use the directory; the listing references the resident's unit.
- **Condominium**: The association whose authenticated members are allowed to access the directory.
- **Syndic**: A privileged user of the condominium who can moderate, edit, and delete listings.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: A resident can complete a new listing in under 2 minutes.
- **SC-002**: 90% of directory searches return matching results in under 3 seconds.
- **SC-003**: 100% of directory access attempts by unauthenticated visitors or non-member users are blocked.
- **SC-004**: The syndic can edit or delete any listing in 3 or fewer actions.
- **SC-005**: 90% of test residents can find a relevant professional using search or category filter on their first attempt.
- **SC-006**: 100% of submitted listings display the correct recommending resident's unit.

## Assumptions

- The existing condominium authentication and membership model is reused; no new sign-in mechanism is introduced.
- Listings submitted by residents are visible immediately; the syndic moderates after publication by editing or deleting inappropriate content.
- Categories are selected from a syndic-managed list; the syndic can add, rename, and deactivate categories to keep filtering consistent.
- Phone numbers are stored as entered and validated for a basic acceptable format.
- Residents cannot edit or delete their own listings after submission in this version; changes go through the syndic.
- The directory shows the recommending resident's unit, not the resident's full name, to preserve privacy.
- One professional may be recommended more than once; the system warns about the same phone number but still allows saving, and the syndic resolves duplicates.
