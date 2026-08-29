# Document templates

Repo-local overrides for the ADR, RFC and PRD templates. The authoring tools
check this directory first and fall back to their own defaults.

**Deliberately empty of templates.** A copy of a default template placed here
gains nothing and freezes that template at the day it was copied — the
repository stops inheriting improvements to it and nobody notices for a year.
Add `ADR.md`, `RFC.md` or `PRD.md` here only when this repository genuinely
needs a section the default lacks, and say in the file what the divergence is
for.
