# Feature Specification: Document Password Hashing Scheme

**Feature Branch**: `002-document-password-hashing`

**Created**: 2026-08-22

**Status**: Draft

**Input**: User description: "document the update about password hasing: change to use HMAC-SHA256 + Argon2ID + PHC (following OWASP)"

## User Scenarios & Testing *(mandatory)*

### User Story 1 - New Developer Understands the Scheme (Priority: P1)

A developer joining the project wants to know how passwords are protected before touching authentication-related code. They open the canonical password-hashing documentation and immediately learn the scheme: the Argon2id algorithm, the PHC string format used to store hashes, and the secret pepper applied via HMAC-SHA256 before hashing, with an OWASP reference to back it up.

**Why this priority**: Correct, discoverable documentation is the primary value of this feature — without it, contributors will guess, reintroduce weaker schemes, or mishandle the pepper.

**Independent Test**: A new contributor can locate the canonical documentation and state the scheme (algorithm, storage format, pepper role) after reading it once.

**Acceptance Scenarios**:

1. **Given** the repository documentation, **When** a new developer searches for how passwords are hashed, **Then** they find one canonical document describing Argon2id, PHC format, and the HMAC-SHA256 pepper.
2. **Given** the canonical document, **When** the developer reads it, **Then** it explains why each element is used and references the OWASP Password Storage Cheat Sheet.

---

### User Story 2 - Operator Configures the Pepper (Priority: P2)

An operator deploying the application needs to know what secret configuration is required and what its operational implications are. The documentation explains that a password pepper is mandatory at startup, is provided through the environment, and that changing it invalidates all existing password hashes.

**Why this priority**: The pepper is a new required secret; misconfiguration or silent rotation would lock users out or weaken protection, so operational behavior must be explicit.

**Independent Test**: An operator can find the pepper configuration requirement and its rotation behavior in the docs without reading source code.

**Acceptance Scenarios**:

1. **Given** the deployment documentation, **When** an operator looks up required secrets, **Then** the pepper is listed as required, with its environment variable name and where it is set.
2. **Given** the documentation's operational notes, **When** an operator considers rotating the pepper, **Then** the docs state that previously stored hashes become unverifiable and what that means for users.

---

### User Story 3 - Maintainers See Consistent Documentation (Priority: P3)

A maintainer auditing the repository finds that all documentation about password handling reflects the implemented scheme. Stale references to alternative approaches (such as bcrypt) appear only as explicitly rejected options in historical decision records, never as the current scheme.

**Why this priority**: Consistency prevents future contributors from following outdated guidance and keeps security decisions auditable.

**Independent Test**: A search of the repository's documentation finds no statement describing a current password scheme other than the Argon2id + pepper design.

**Acceptance Scenarios**:

1. **Given** the repository documentation, **When** a search is run for alternative schemes (e.g., bcrypt), **Then** matches appear only where an approach is explicitly recorded as rejected.
2. **Given** the updated architecture decision record, **When** a reviewer reads the password-storage decision, **Then** the final decision statement matches the implemented scheme including the pepper.

---

### Edge Cases

- What happens if the pepper is not set at startup? (Documented: application refuses to start.)
- What happens to existing password hashes if the pepper is changed? (Documented: they can no longer be verified.)
- What about password hashes created before the pepper was introduced? (Documented: verification behavior, with guidance that affected users must reset.)
- What if a stored hash does not follow the documented PHC format? (Documented: verification rejects it as malformed.)

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: Documentation MUST describe the password hashing scheme: the Argon2id algorithm, the PHC string format used for stored hashes (including the meaning of its segments: algorithm, version, parameters, salt, hash), and the site pepper applied via HMAC-SHA256 before hashing.
- **FR-002**: Documentation MUST cite the OWASP Password Storage Cheat Sheet as the basis for the scheme and identify each OWASP-aligned element (algorithm choice, salt, parameters, pepper handling).
- **FR-003**: Documentation MUST state that the pepper is a required secret provided through environment configuration, that it is never stored together with the password hashes, and that the application will not start without it.
- **FR-004**: Documentation MUST include operational guidance: rotating or changing the pepper makes existing hashes unverifiable, and hashes created without the pepper also become unverifiable once the pepper is enforced.
- **FR-005**: The architecture decision record for the authentication feature MUST be updated so its final decision statement reflects the implemented scheme, and stale references to alternative current schemes (e.g., bcrypt) MUST be limited to an explicitly rejected-options context.
- **FR-006**: The repository's top-level documentation MUST link to the canonical password-hashing documentation so it is discoverable.
- **FR-007**: Documentation MUST include a concrete example of a stored hash and show how each part of the string maps to the scheme, so readers can recognize a well-formed record.

### Key Entities *(include if feature involves data)*

- **Password hash record**: The stored artifact of the hashing process. It is a PHC string encoding the algorithm, parameters, random salt, and computed hash — and it contains no pepper material, so a database leak alone does not expose the secret.
- **Pepper**: A site-wide secret kept outside the database (environment configuration). It is combined with the password via HMAC-SHA256 before hashing, binding every hash to the secret.
- **Architecture decision record**: The historical record (in the existing RBAC/authentication feature specification) that documents the password-storage decision; it must be kept consistent with the implemented scheme.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: A new contributor can identify the hashing algorithm, storage format, and pepper configuration from the documentation within 10 minutes of first reading it.
- **SC-002**: 100% of repository documentation statements describing the current password scheme match the implemented design; a full-documentation search finds no stale references to bcrypt or pepperless hashing as the current scheme.
- **SC-003**: The pepper's operational behavior (required at startup, rotation invalidates existing hashes) is documented in exactly one canonical location that is linked from the repository's top-level documentation.
- **SC-004**: The documented scheme is verified against the implemented behavior — a reviewer comparing the documentation to the authentication code finds no discrepancies (algorithm, PHC format, parameters, pepper combination).

## Assumptions

- This feature is documentation-only; no application code, configuration, or database changes are in scope.
- A new canonical documentation page under the repository's `docs/` directory is the primary deliverable, and the top-level README gains a link to it.
- The existing architecture decision record for the authentication feature is updated in place; its rejected-options section may keep historical mentions of alternatives (e.g., bcrypt) as long as they are clearly marked as rejected.
- Any pre-existing password hashes stored before the pepper was enforced are handled operationally (documented behavior, user resets) rather than through an automatic code migration, which is out of scope.
- The term "pepper" in the documentation refers to the site-wide secret in environment configuration, distinct from per-password salts embedded in the PHC string.
