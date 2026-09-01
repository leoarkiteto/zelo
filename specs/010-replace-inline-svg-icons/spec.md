# Feature Specification: Replace Inline SVG Icons

**Feature Branch**: `010-replace-inline-svg-icons`

**Created**: 2026-09-01

**Status**: Draft

**Input**: User description: "replace inlines `<svg/>` icons to use a library"

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Existing icons render through the icon library with no visible change (Priority: P1)

Every icon that users see today — login page password visibility toggle, breadcrumb separators, navigation arrows, mobile menu toggle, and the shared icon set used across the portal — keeps looking and behaving exactly as it does now, but is now sourced from a single curated icon library instead of hand-drawn artwork embedded directly in page templates.

**Why this priority**: This is the core request. Moving every current icon to the library eliminates duplicated, hand-maintained icon markup and proves the replacement approach works for all existing usages.

**Independent Test**: Render each page that currently contains an icon and compare it side by side with the current version; every icon must appear in the same position, size, and style, and the pages must otherwise be unchanged.

**Acceptance Scenarios**:

1. **Given** a user opens any page that currently displays an icon, **When** the page renders, **Then** every icon looks identical to the current version (same meaning, position, size, and outline style).
2. **Given** a page that currently embeds hand-drawn icon artwork, **When** the migration is complete, **Then** no page template contains a private copy of icon artwork; all icons come from the shared icon library.
3. **Given** the full application test suite, **When** tests run after migration, **Then** all tests that passed before the change still pass, with icon assertions updated to verify the library-backed rendering.

---

### User Story 2 - The shared icon component becomes the single way to use icons (Priority: P2)

Developers and maintainers have one shared, documented way to place an icon on a page: request it from the icon library through the existing shared icon component. Feature templates stop embedding their own icon artwork, and future pages automatically use the same component.

**Why this priority**: A single entry point is what makes the library useful beyond the initial migration — it prevents new hand-drawn icon markup from reappearing in feature templates.

**Independent Test**: Open the shared icon component, verify it exposes every icon currently used in the application, and confirm no feature template contains its own icon artwork.

**Acceptance Scenarios**:

1. **Given** a feature template needs an icon, **When** a developer adds it, **Then** they reference the shared icon component and do not write or copy icon artwork into the feature.
2. **Given** the icons currently used across the application, **When** a developer inspects the shared icon component, **Then** each icon is available by name and documented in one place.
3. **Given** two pages use the same icon, **When** the icon's source is updated in the library, **Then** both pages reflect the update without further edits.

---

### User Story 3 - Password visibility toggle keeps its behavior and accessibility (Priority: P3)

The login page's show/hide password control continues to switch between the eye and eye-off icons, updates its accessible label and pressed state, and remains operable by mouse and keyboard, now rendering both icons from the library.

**Why this priority**: The password toggle is the most behavior-sensitive icon usage — it combines two icons with interactive state and accessibility labels — so it must be verified explicitly even though it is part of the migration.

**Independent Test**: On the login page, click or keyboard-activate the password toggle repeatedly and confirm the field masks/reveals the password, the icon switches between eye and eye-off, and the accessible label and pressed state stay correct.

**Acceptance Scenarios**:

1. **Given** the login page, **When** a user activates the show/hide password control, **Then** the password field switches between masked and visible and the icon switches between eye and eye-off.
2. **Given** the show/hide password control, **When** the state changes, **Then** the accessible label and pressed state update to match the new state.
3. **Given** a user tabs to the show/hide password control, **When** they activate it with the keyboard, **Then** the same toggle behavior works as with a mouse click.

---

### Edge Cases

- What happens when a page needs an icon that is not yet in the library? The shared icon component must provide a defined fallback icon so the page still renders, and the missing icon is added to the library as a follow-up change rather than drawn inline in the feature template.
- What happens to icon artwork that is decorative versus meaningful? Decorative icons stay hidden from assistive technology; interactive icons keep their accessible labels, so nothing is announced differently after the migration.
- What happens when an icon has size or spacing overrides? The shared icon component must accept the existing size and styling overrides so each usage keeps its exact appearance.
- What happens to non-icon letter marks or logos? They are out of scope and remain unchanged.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The application MUST render all user-visible icons from a single curated icon library; no page template may contain hand-drawn icon artwork.
- **FR-002**: The shared icon component MUST be the only way feature templates include icons.
- **FR-003**: Every icon currently used in the application MUST be available through the shared icon component: building, mail, users, home, key, chevron-left, arrow-right, clipboard, tag, info, eye, eye-off, and menu.
- **FR-004**: Each migrated icon MUST preserve its current meaning, position, and size on the page; its visual style follows the selected Google icons library.
- **FR-005**: The password visibility toggle MUST continue to switch between eye and eye-off icons and update its accessible label and pressed state when activated.
- **FR-006**: Interactive icons MUST retain their accessible labels; decorative icons MUST remain hidden from assistive technology.
- **FR-007**: The shared icon component MUST support the size and styling overrides currently used by existing pages.
- **FR-008**: The shared icon component MUST provide a visible fallback icon for unknown icon requests so pages never render broken icon artwork.

### Key Entities *(include if feature involves data)*

- **Icon**: A named entry in the curated icon library. It has an identifier, a visual meaning, an outline style, and a default size; it can be rendered on a page through the shared icon component.
- **Icon usage**: A location where an icon appears in the user interface. It references one library icon and may add sizing, styling, interactive state, and accessibility information.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: 100% of user-visible icons render through the shared icon library; no hand-drawn icon artwork remains in page templates.
- **SC-002**: Every affected page renders with icons in the same position and size as before, with no unintended layout shifts when compared side by side.
- **SC-003**: All automated rendering tests pass after the migration, including the shared icon component tests and the login page toggle tests.
- **SC-004**: A developer can add an already-available library icon to a page in under 10 minutes without drawing or copying icon artwork.
- **SC-005**: The password visibility toggle remains fully operable by mouse and keyboard with correct accessible labels after the migration.

## Assumptions

- Google icons (Material Symbols) is the selected icon library, per user direction during planning; icons use its outlined style at 24 px.
- The icon library is delivered with the application's own static assets and works with server-rendered pages; end users are not required to enable or download anything beyond the standard page load.
- The existing shared icon component and its current call sites remain the public entry point; only the source of the icon artwork changes internally.
- All current inline icon artwork in page templates is in scope: the shared icon set, the login password visibility toggle, the breadcrumb separators, the navigation arrows, and the mobile menu toggle.
- Brand letter marks and logos are not icons and are out of scope.
