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
| [0006](0006-host-is-an-identity.md) | The owner back-pointer is an identity | Accepted |
| [0007](0007-assert-through-the-assert-module.md) | Test code asserts through the assert module | Superseded by [0008](0008-pin-the-assert-module-by-version.md) |
| [0008](0008-pin-the-assert-module-by-version.md) | Pin the assert module by version | Accepted |
| [0009](0009-the-carrier-mark-follows-the-brand.md) | The carrier mark follows the brand | Accepted |
| [0010](0010-export-entries-have-a-five-part-key.md) | An export entry has a five-part key and no signature | Accepted |
| [0011](0011-region-string-tables-and-hashed-edges.md) | Each region has its own string table, and edges are hashed | Accepted |
| [0012](0012-package-reads-depend-on-every-member.md) | A package read whole depends on every member | Accepted |
| [0013](0013-the-parse-memo-is-keyed-by-the-executable.md) | The parse memo is keyed by the executable and can be shared | Accepted |
| [0014](0014-the-protobuf-frontend-resolves-four-features.md) | The protobuf frontend resolves the four features it applies | Accepted |
| [0015](0015-the-delimited-mark-is-stamped-at-the-parse.md) | The delimited mark is stamped at the parse | Accepted |
| [0016](0016-the-proto2-schema-marks-message-fields-optional.md) | The proto2 service schema marks its message fields optional | Accepted |
| [0017](0017-the-service-fixture-edit-narrows-a-field.md) | The service fixture's edit narrows a field | Accepted |
