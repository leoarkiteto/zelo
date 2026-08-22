# Feature Specification: RBAC Authentication & Authorization

**Feature Branch**: `001-rbac-auth`

**Created**: 2026-08-22

**Status**: Draft

**Input**: User description: "Create authorization/authentication feature based on RBAC (Role-based access control). The user has the following type: - syndic: who manage the condominium, must be an `owner`; - owner: who is owner of an unit and live in the building; - tenant: who rent an unit (can not be a candidate of next syndic);"

## Clarifications

### Session 2026-08-22

- Q: When a person registers as an owner or tenant, how should the system know which condominium and unit they belong to? → A: Invite link/code from the syndic (pre-fills condominium, unit, and role; only invited people can register).
- Q: After registration, who should be able to grant or revoke roles (for example, making an owner the syndic, or correcting a wrong owner/tenant role)? → A: The current syndic manages roles within their own condominium, with system-enforced eligibility rules.
- Q: How should the system handle repeated failed sign-in attempts for the same account? → A: Temporary account lockout after 5 consecutive failures for 15 minutes.
- Q: Should a user be allowed to be signed in from multiple devices or browsers at the same time? → A: Yes, unlimited simultaneous sessions; signing out ends only the current session.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Register with a role (Priority: P1)

A person receives an invitation link or code from the syndic that identifies their condominium, unit, and role (owner or tenant). They complete registration with their own credentials, and the system creates their account with the invited role. The syndic role cannot be selected during registration.

**Why this priority**: Without registered users and assigned roles, no other part of the authorization model can work. This is the entry point for every user.

**Independent Test**: Can be fully tested by registering one owner and one tenant account and confirming each account receives the correct role and can sign in afterwards.

**Acceptance Scenarios**:

1. **Given** a person with a valid owner invitation has not registered yet, **When** they register using the invitation, **Then** the system creates their account with the owner role for the invited condominium and unit.
2. **Given** a person with a valid tenant invitation has not registered yet, **When** they register using the invitation, **Then** the system creates their account with the tenant role for the invited condominium and unit.
3. **Given** the registration form is displayed, **When** a person tries to register as "syndic", **Then** the system does not offer or accept the syndic role as a self-selected option.
4. **Given** a person submits an incomplete or invalid registration, **When** the system validates it, **Then** the account is not created and the person sees clear guidance to correct the errors.

---

### User Story 2 - Sign in and stay signed in appropriately (Priority: P1)

A registered user signs in with their credentials to access the areas allowed for their role. The system recognizes them for the duration of their session, and signing out ends their access.

**Why this priority**: Authentication is the foundation of the feature; every protected action depends on a verified identity and a controlled session.

**Independent Test**: Can be fully tested by signing in with a known account, confirming role-appropriate access is available, then signing out and confirming access is no longer available.

**Acceptance Scenarios**:

1. **Given** a registered user with valid credentials, **When** they sign in, **Then** the system grants an authenticated session linked to their role.
2. **Given** a person enters incorrect credentials, **When** they attempt to sign in, **Then** the system denies access and shows a generic error that does not reveal whether the identifier exists.
3. **Given** an authenticated user, **When** they sign out, **Then** their session ends and protected areas become inaccessible.
4. **Given** an authenticated user whose session has expired due to inactivity, **When** they try to access a protected area, **Then** the system requires them to sign in again.

---

### User Story 3 - Access restricted by role (Priority: P2)

After signing in, each user can only reach the pages and perform the actions their role permits: the syndic manages the condominium, owners manage their own units, and tenants access only their own tenancy information.

**Why this priority**: Role-based access control is the core value of the feature, ensuring each role sees and does only what it should.

**Independent Test**: Can be fully tested by signing in as each of the three roles and attempting the same set of actions, confirming the allowed and denied actions match the role definition.

**Acceptance Scenarios**:

1. **Given** a user signed in as syndic, **When** they access condominium management areas, **Then** the system allows them to manage the condominium.
2. **Given** a user signed in as owner, **When** they access their own unit area, **Then** the system allows them to view and manage their own unit data.
3. **Given** a user signed in as tenant, **When** they access their own tenancy area, **Then** the system allows them to view their own tenancy data.
4. **Given** a user attempts to access an area or action outside their role, **When** the system checks their role, **Then** access is denied with a clear, friendly explanation.
5. **Given** a user signed in as owner or tenant, **When** they attempt a syndic-only action, **Then** the system denies the action.

---

### User Story 4 - Syndic eligibility (Priority: P3)

Only users who currently hold the owner role can become the syndic. The current syndic manages role assignments within their own condominium; the system enforces the eligibility rules. Tenants can never be selected or proposed as syndic, and a user who ceases to be an owner also ceases to be syndic.

**Why this priority**: This encodes the condominium's governance rule in the access model. It is important, but it builds on the sign-in and role enforcement already covered by the earlier stories.

**Independent Test**: Can be fully tested by attempting to assign the syndic role to an owner, to a tenant, and to a user who has just lost owner status, and confirming the rule holds in all three cases.

**Acceptance Scenarios**:

1. **Given** a user currently holds the owner role, **When** the syndic role is assigned to them, **Then** the system accepts the assignment.
2. **Given** a user holds the tenant role, **When** someone attempts to make them a syndic candidate or assign them the syndic role, **Then** the system rejects the attempt.
3. **Given** a user is syndic and no longer holds the owner role, **When** their owner status is removed, **Then** the system revokes the syndic role automatically.
4. **Given** a user has no owner relationship with any unit, **When** they are considered for syndic, **Then** they are not eligible.

---

### Edge Cases

- A user tries to register more than once with the same identifier; the system rejects duplicates with clear guidance.
- A user ends up with no role (e.g., role removed or data inconsistency); the system denies all protected access until a valid role is restored.
- A syndic loses owner status mid-session; their next protected action reflects the revoked syndic privileges.
- A tenant attempts to use a direct link to a syndic-only area; the system denies access rather than showing the area.
- A user's account is disabled or suspended; all active sessions stop granting access.
- A user forgets their password; the system offers a password reset flow that does not reveal account existence.
- A person tries to register with an invalid, expired, or already-used invitation; the system rejects the attempt and directs them to contact the syndic.
- An account reaches 5 consecutive failed sign-in attempts; the system locks it for 15 minutes and rejects further sign-in attempts until the lock expires.
- A condominium has no active syndic; the system denies syndic-only actions until a system administrator appoints a new syndic.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST require authentication (a unique identifier plus a secret) before allowing access to any protected area.
- **FR-002**: System MUST support exactly three roles: syndic, owner, and tenant.
- **FR-003**: System MUST allow new users to register only via a syndic-issued invitation that identifies the condominium, unit, and invited role (owner or tenant); the syndic role MUST NOT be selectable during registration.
- **FR-004**: System MUST assign the syndic role only to a user who currently holds the owner role.
- **FR-005**: System MUST prevent any user who is not a current owner from becoming a syndic candidate or holding the syndic role, including tenants.
- **FR-006**: System MUST deny access to any protected area or action when the signed-in user's role does not permit it, and show a clear message.
- **FR-007**: System MUST allow the syndic to manage the condominium (for example, units, residents, and condominium-level settings).
- **FR-008**: System MUST allow owners to view and manage their own unit information only.
- **FR-009**: System MUST allow tenants to view their own tenancy information only.
- **FR-010**: System MUST automatically revoke the syndic role when the holder no longer holds the owner role.
- **FR-011**: System MUST provide a password reset flow for users who cannot sign in.
- **FR-012**: System MUST end the user's session on sign-out and after 12 hours of inactivity or a 7-day absolute lifetime, whichever comes first.
- **FR-013**: System MUST record security-relevant events: successful sign-in, failed sign-in, sign-out, account lockout, role changes, and denied access attempts.
- **FR-014**: System MUST ensure each user holds at most one of the owner or tenant role at a time within the condominium; the syndic role MAY be held in addition to the owner role.
- **FR-015**: System MUST allow the current syndic to grant or revoke owner, tenant, and syndic roles within their own condominium, subject to the eligibility rules in FR-004, FR-005, and FR-014.
- **FR-016**: System MUST temporarily lock an account for 15 minutes after 5 consecutive failed sign-in attempts and reject further sign-in attempts until the lock expires.
- **FR-017**: System MUST allow a user to hold multiple simultaneous sessions across devices; signing out MUST end only the current session.

### Key Entities *(include if feature involves data)*

- **User**: A person with an account; holds credentials, contact identifier, and account status.
- **Role**: One of syndic, owner, or tenant; determines what the user can see and do.
- **Condominium**: The building and community being managed; the scope within which roles apply.
- **Unit**: A property within the condominium; the owner and tenant roles are tied to a unit relationship.
- **Membership**: The association between a User, a Role, and the Condominium (and Unit where applicable); the single source of truth for a user's current role.
- **Invitation**: A single-use, expiring link or code issued by the syndic that identifies the condominium, unit, and invited role (owner or tenant) for a new user.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: A user with valid credentials can sign in and reach a role-appropriate area in under 2 minutes.
- **SC-002**: 100% of access attempts to areas or actions outside the user's role are denied.
- **SC-003**: 100% of attempts to make a tenant or non-owner the syndic are rejected.
- **SC-004**: When a syndic loses owner status, their syndic privileges are revoked before their next protected action.
- **SC-005**: 100% of security-relevant events (sign-in, failed sign-in, sign-out, account lockout, role change, denied access) are recorded.
- **SC-006**: 90% of first-time users complete registration without external help.
- **SC-007**: After 5 consecutive failed sign-in attempts, the account is locked and sign-in is rejected until the lock expires after 15 minutes.

## Assumptions

- Users authenticate with a unique identifier (email address) and a password; social or external identity providers are out of scope for this feature.
- A user account belongs to one condominium for the initial version and holds at most one of owner/tenant, with syndic as an additional role on top of owner.
- Owner or tenant role is determined by a syndic-issued invitation that identifies the condominium, unit, and invited role; verification of property ownership or rental contracts is out of scope.
- Invitations are single-use and expire 7 days after issuance.
- Password reset links expire 1 hour after the reset request.
- At most one user holds the active syndic role per condominium at a time.
- Password reset is delivered through the user's registered email address.
- Sessions expire after 12 hours of inactivity or 7 days of absolute lifetime, whichever comes first; a user may hold multiple simultaneous sessions across devices.
- Role names are used in English: syndic, owner, and tenant.
- The application UI is English-only for v1.
- If a condominium has no active syndic, a system administrator appoints one outside the application.
