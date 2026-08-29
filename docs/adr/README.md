# Architecture decision records

Each record holds one decision: what was decided, what else was considered,
and what it costs. Once a record is accepted, nobody edits its argument. A
decision that changes gets a new record that supersedes the old one, and the
old one gains a pointer forward.

The [decision log](../architecture/21-decisions.md) lists every settled
decision and is the place to look one up. A decision moves into a record here
when its revisit trigger fires, or when its argument no longer fits in one
row of that table. See
[ADR-0001](0001-use-adrs-for-architecture-decisions.md).

| # | Title | Status |
|---|---|---|
| [0001](0001-use-adrs-for-architecture-decisions.md) | Use ADRs for architecture decisions | Accepted |
