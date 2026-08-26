# Feature Specification: Ticket Management

**Feature Branch**: `007-ticket-management`

**Created**: 2026-08-25

**Status**: Draft

**Input**: User description: "Ticket management. The user can create ticket in its space and the syndic will receive to reply. The user can create a ticket to request some repair in the building, to complain about noise, to suggest topic to discuss in the next condominium session. The syndic can reply the ticket after close the ticket."

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Resident creates a ticket (Priority: P1)

The resident opens their own space in the application and creates a ticket by choosing one of four categories — repair request in the building, noise complaint, suggestion of a topic for the next condominium assembly, or other — and writing a short title and a description. The ticket is saved immediately with status `Open` and becomes visible to the syndic for handling.

**Why this priority**: Creating the ticket is the entry point of the whole feature. Without it, there is nothing for the syndic to receive or reply to.

**Independent Test**: Authenticated as a resident, create a ticket with a title, category and description, and confirm it is saved as `Open` and appears in the syndic's inbox.

**Acceptance Scenarios**:

1. **Given** a resident authenticated in their own space, **When** they create a ticket with a valid category (repair request, noise complaint, assembly topic suggestion, or other), a title and a description, **Then** the ticket is saved with status `Open`, records the author, unit, title, category, description and creation date, and appears in the syndic's inbox.
2. **Given** the ticket creation form, **When** the resident submits the ticket without a description, **Then** the system shows a clear message indicating the missing field and does not save the ticket.
3. **Given** the ticket creation form, **When** the resident submits the ticket without a title, **Then** the system shows a clear message indicating the missing field and does not save the ticket.
4. **Given** the ticket creation form, **When** the resident submits the ticket with an invalid or missing category, **Then** the system blocks the submission with a clear message.
5. **Given** the ticket creation form, **When** the resident writes a description longer than the allowed limit, **Then** the system rejects the submission and indicates the maximum allowed length.

---

### User Story 2 - Syndic views tickets and replies (Priority: P1)

The syndic opens the ticket inbox and sees all open tickets with the category, author, unit and creation date. The syndic opens a ticket and writes a reply; the reply is attached to the ticket with the syndic's name and timestamp and is immediately visible to the resident who created the ticket.

**Why this priority**: The reply is the syndic's core action — it is how the resident's request, complaint or suggestion is answered. Without it the feature only collects tickets.

**Independent Test**: Authenticated as the syndic, open an open ticket in the inbox, submit a reply, and confirm the reply appears on the ticket for the resident.

**Acceptance Scenarios**:

1. **Given** open tickets in the inbox, **When** the syndic opens the inbox, **Then** all open tickets are listed with category, author, unit and creation date.
2. **Given** an open ticket, **When** the syndic submits a non-empty reply, **Then** the reply is attached to the ticket with the syndic's name and timestamp and is visible to the resident.
3. **Given** the reply form on an open ticket, **When** the syndic submits an empty reply, **Then** the system blocks it with a clear message and does not attach anything.

---

### User Story 3 - Syndic closes the ticket (Priority: P2)

After answering the ticket, the syndic closes it. The ticket's status changes from `Open` to `Closed` with a closing date, it leaves the open inbox, and no further messages can be added by either party.

**Why this priority**: Closing is the completion step of the flow — it marks the ticket as handled and keeps the inbox clean. It is a natural follow-up to the reply but not required for the feature to deliver value.

**Independent Test**: Authenticated as the syndic, close an open ticket and confirm its status becomes `Closed` and that both parties can no longer add messages to it.

**Acceptance Scenarios**:

1. **Given** an open ticket, **When** the syndic closes it, **Then** the status becomes `Closed`, a closing date is recorded, and the ticket no longer appears in the open inbox.
2. **Given** a closed ticket, **When** the resident tries to add a message, **Then** the system blocks it and shows that the ticket is closed.
3. **Given** a closed ticket, **When** the syndic tries to add a message, **Then** the system blocks it and shows that the ticket is closed.
4. **Given** an open ticket with no reply, **When** the syndic closes it, **Then** closing is still allowed; the resident sees a closed ticket without an answer.

---

### User Story 4 - Resident tracks their tickets (Priority: P2)

The resident opens their ticket list and sees only the tickets they created, each with its status (`Open` or `Closed`) and, when present, the syndic's reply and its date. Tickets from other residents are never shown.

**Why this priority**: Tracking gives the resident visibility into whether their request was answered and closed. It depends on tickets existing, so it comes after creation; it is still core to a complete flow.

**Independent Test**: Authenticated as a resident, open the ticket list and confirm only their own tickets are shown with the correct status and any reply.

**Acceptance Scenarios**:

1. **Given** a resident with tickets, **When** they open their ticket list, **Then** only their own tickets are shown, each with its status (`Open` or `Closed`).
2. **Given** a ticket with a syndic reply, **When** the resident opens it, **Then** they see the reply content and its date.
3. **Given** a resident with no tickets, **When** they open the ticket list, **Then** an empty state with a friendly message and a shortcut to create a ticket is shown.

---

### Edge Cases

- What happens when a resident submits a ticket without a description? The submission is blocked with a clear message; nothing is saved.
- What happens when a resident submits a ticket without a title? The submission is blocked with a clear message; nothing is saved.
- What happens when a ticket is submitted with an invalid category? The submission is blocked; only the three supported categories are accepted.
- What happens when a resident without an active unit tries to create a ticket? The creation is refused with a friendly message; no ticket is saved.
- What happens when a description exceeds the allowed length? The submission is rejected and the maximum length is indicated.
- What happens when the syndic submits an empty reply? It is blocked; no empty messages are attached.
- What happens when either party tries to message a closed ticket? It is blocked; closed tickets are read-only.
- What happens when the syndic closes a ticket without replying? Closing is allowed; the ticket is marked `Closed` and the resident sees it unanswered.
- What happens when the resident tries to view another resident's tickets? The tickets are not accessible; each resident sees only their own.
- What happens when the inbox or the ticket list is empty? A friendly empty state is displayed instead of an error.
- What happens when the syndic tries to reopen a closed ticket? Reopening is not supported in v1; a new ticket must be created.
- What happens when a ticket is created for a noise complaint? The ticket identifies the unit and resident to the syndic; anonymous reporting is not available in v1.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The resident MUST be able to create a ticket from their own space, choosing exactly one of four categories: repair request in the building, noise complaint, suggestion of a topic for the next condominium assembly, or other (for requests not covered by the predefined categories).
- **FR-002**: Every ticket MUST have a non-empty title (1–120 characters), a non-empty description (1–2000 characters), and a valid category; the maximum lengths MUST be clearly indicated when exceeded.
- **FR-003**: Every ticket MUST record the author (resident), the unit, the title, the category, the description, the creation date and the status.
- **FR-004**: Valid ticket statuses MUST be `Open` and `Closed`; new tickets MUST start as `Open`.
- **FR-005**: The syndic MUST be able to view all tickets, including open and closed ones, with author, unit, category and creation date; open tickets MUST be listed in the inbox.
- **FR-006**: The syndic MUST be able to reply to an `Open` ticket; each reply MUST record its content, the author and the timestamp, and MUST be visible to the ticket's resident.
- **FR-007**: The syndic MUST be able to reply to an `Open` ticket and then close it; closing MUST record a closing date and change the status to `Closed`. The syndic MAY close a ticket without a reply (for example, duplicate or invalid tickets).
- **FR-008**: `Closed` tickets MUST NOT accept new messages from either the resident or the syndic.
- **FR-009**: The resident MUST be able to view their own tickets with status and replies; tickets belonging to other residents MUST NOT be visible to them.
- **FR-010**: The system MUST display a friendly empty state when there are no tickets to show in the inbox or in the resident's ticket list.

### Key Entities *(include if feature involves data)*

- **Ticket**: represents a request, complaint or suggestion submitted by a resident; main attributes: category (repair request, noise complaint, assembly topic suggestion), description, status (`Open`/`Closed`), author (resident), unit, creation date, closing date, and the replies attached to it.
- **Ticket Reply**: a message from the syndic attached to a ticket; main attributes: content, author, timestamp. A ticket may have zero or more replies.
- **Category**: a fixed list of four values (repair request, noise complaint, suggestion of a topic for the next condominium assembly, other); selected by the resident at creation time.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: A resident creates a ticket in under 2 minutes from their own space.
- **SC-002**: 100% of ticket submissions without a description or with an invalid category are blocked with a clear message.
- **SC-003**: 100% of tickets created by residents appear in the syndic's inbox immediately after creation.
- **SC-004**: The syndic opens a ticket and submits a reply in under 1 minute.
- **SC-005**: 100% of `Closed` tickets reject new messages from both parties.
- **SC-006**: 100% of residents see only their own tickets; no ticket from another resident is ever displayed.
- **SC-007**: A resident finds the status and any reply of a ticket within 30 seconds of opening their ticket list.

## Assumptions

- Actors are the authenticated resident (linked to one unit) and the syndic. A single syndic role is assumed in v1; multiple administrators or board members are out of scope.
- Tickets identify the resident and unit to the syndic; anonymous tickets (e.g., for noise complaints) are out of scope in v1 and may be revisited later.
- The flow is: resident creates → syndic replies → syndic closes. The requester's phrase "the syndic can reply the ticket after close the ticket" is interpreted as: the syndic replies first and then closes the ticket; replies after closing are not possible because closed tickets are read-only. The syndic MAY close a ticket without a final reply; in that case the resident sees a closed ticket with no answer.
- Closed tickets cannot be reopened in v1; additional information is handled through a new ticket.
- Resident follow-up messages on an existing ticket are out of scope in v1; residents who need to add information create a new ticket.
- Attachments (photos of the repair, documents) are out of scope in v1.
- Notifications by e-mail or push are out of scope in v1; the syndic sees new tickets in the inbox and the resident sees replies in their ticket list.
- Priority/severity fields are out of scope in v1; tickets are handled in creation order by the syndic.
- A ticket belongs to exactly one unit; handling of units with multiple residents is out of scope in v1.
- Ticket history is retained as part of the condominium's records; automatic deletion is out of scope in v1.
- In case of concurrent edits of the same ticket, the last saved change prevails; no simultaneous-edit locking in v1.
- This specification is written in English per the requester's language; the application UI copy follows the existing app's language conventions (Portuguese default, per the project's i18n setup).
