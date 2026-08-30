# Architecture decision records

Each record holds one decision: what we decided, what else we considered, and
what it costs. Once we accept a record, nobody changes what it argues. If the
decision changes, write a new record that supersedes the old one, then link
the old record to the new one.

The table in [21-decisions.md](../architecture/21-decisions.md) lists every
settled decision. Look there first. Write a record here when the event you
said would make you reconsider a decision actually happens, or when the
reasoning no longer fits in one row of that table.
[ADR-0001](0001-use-adrs-for-architecture-decisions.md) says why we work this
way.

| # | Title | Status |
|---|---|---|
| [0001](0001-use-adrs-for-architecture-decisions.md) | Use ADRs for architecture decisions | Accepted |
| [0002](0002-identity-carries-source-language.md) | Identity carries the source language | Accepted |
| [0003](0003-json-codecs-round-trip.md) | JSON codecs round-trip | Accepted |
| [0004](0004-generate-kind-enum-from-schema.md) | Generate the Kind enum from the schema | Accepted |
| [0005](0005-concern-grouped-generated-files.md) | Group generated files by concern | Accepted |
