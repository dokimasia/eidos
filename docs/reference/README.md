# Reference

Exhaustive, dry, and complete: every flag, field, code and default, structured
to mirror the code and written to be looked up rather than read.

Most of this surface is **generated from the registries that define it**, per
release — the funcmap reference from backend and plugin registrations, the
directive reference from registered schemas, the config reference from the
published JSON Schema, the diagnostic-code index from the diag registry, and
the per-language support matrices from the completeness rung. Generating it is
part of the tag pipeline, and a stale page blocks a tag the way a failing rung
does; see
[14-distribution-and-cli.md](../architecture/14-distribution-and-cli.md).

Hand-written reference drifts from the code within two releases, so a page
here is committed by hand only where no registry can answer for it.
