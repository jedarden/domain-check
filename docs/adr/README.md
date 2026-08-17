# Architecture Decision Records (ADRs)

This directory contains Architecture Decision Records for the domain-check project. ADRs document significant architectural decisions, their context, and consequences.

## What are ADRs?

Architecture Decision Records are a way to document important architectural decisions in a way that preserves the context of why those decisions were made. Each ADR includes:

- **Status:** Accepted, Proposed, Deprecated, or Superseded
- **Context:** What is the issue that we're facing that motivated this decision?
- **Decision:** What are we going to do?
- **Alternatives Considered:** What other approaches did we consider and why were they rejected?
- **Consequences:** What becomes easier or harder as a result of this decision?

## ADR Index

| ADR | Title | Status | Date |
|-----|-------|--------|------|
| [001](./001-domain-watch-webhook-notifications.md) | Domain Watch: Webhook-Based Notifications for Availability Changes | Accepted | 2026-07-20 |

## ADR Lifecycle

1. **Proposed:** A decision is proposed for discussion
2. **Accepted:** The decision has been made and will be implemented
3. **Deprecated:** The decision is no longer recommended but is still in use
4. **Superseded:** The decision has been replaced by a newer ADR

## Template

When creating a new ADR, use the following template:

```markdown
# ADR-XXX: [Title]

**Status:** [Proposed | Accepted | Deprecated | Superseded]  
**Date:** YYYY-MM-DD  
**Supersedes:** ADR-XXX (if applicable)  

## Context

[What is the issue that we're facing that motivated this decision?]

## Decision

[What are we going to do?]

## Alternatives Considered

1. **Alternative 1:** [Description]
2. **Alternative 2:** [Description]
   - Rejected because: [Reason]

## Consequences

- **Positive:** [What becomes easier?]
- **Negative:** [What becomes harder?]
- **Operational:** [What operational changes are required?]

## References

- [Related documentation]
- [Related issues or beads]
```
