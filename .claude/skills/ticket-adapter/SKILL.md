---
name: ticket-adapter
description: Normalize Linear and GitHub issue/project tickets into a consistent ticket brief. Use when user provides a ticket URL or ID.
---

# Ticket Adapter

## Supported sources
- Linear issue URL or identifier
- GitHub Issue URL or owner/repo#number
- GitHub Projects item URL (availability of fields depends on GitHub integration/API)
- Manual pasted ticket

## Retrieval policy
Use only an authorized MCP, CLI, or API integration available in this environment.
- For Linear, use configured Linear MCP/API/CLI.
- For GitHub, use configured GitHub MCP, `gh` CLI, or API credentials approved by the organization.
- Never print tokens or secrets.
- Never claim retrieval succeeded unless actual ticket data was returned.
- If source cannot be retrieved, request user-provided ticket content.

## Normalize
Return:
- source
- identifier
- URL
- title
- description
- original AC
- labels/priority/status if available
- comments/context relevant to the ticket
- missing information

Keep original AC intact. Do not write back to ticket unless user explicitly asks.
