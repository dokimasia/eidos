# Architecture decision records

One decision per record, numbered and immutable once accepted. A changed
decision gets a new ADR that supersedes the old one; the superseded record
keeps its argument and gains a pointer forward.

The specification's [decision log](../architecture/21-decisions.md) is the
source of record for the decision set as a whole. A decision graduates to a
full ADR when its recorded revisit trigger fires, or when its argument
outgrows one table row ([ADR-0001](0001-use-adrs-for-architecture-decisions.md)).

| # | Title | Status |
|---|---|---|
| [0001](0001-use-adrs-for-architecture-decisions.md) | Use ADRs for architecture decisions | Accepted |
