# eidos documentation

| I want to know | Go to |
|---|---|
| What the system is and how it fits together | [architecture](architecture/README.md) — the specification, 21 bounded contexts plus a glossary |
| Why the system is the way it is | [adr](adr/README.md) — architecture decision records |
| How one declaration travels the whole system | [00-one-declarations-journey](architecture/00-one-declarations-journey.md) — the on-ramp |
| What a term means | [GLOSSARY](architecture/GLOSSARY.md) |

The specification is closed: every component, contract, and policy appears in
exactly one of its documents. Settled decisions live inline in the context
they belong to, indexed by the
[decision log](architecture/21-decisions.md).

User-facing documentation — tutorials, how-to guides, and the reference
surface — does not exist yet. Most of that surface is specified as generated
per release (the funcmap, directive, config and diagnostic-code references,
and the per-language support matrices); see
[14-distribution-and-cli](architecture/14-distribution-and-cli.md).

Contributors start at [CONTRIBUTING](../CONTRIBUTING.md).
