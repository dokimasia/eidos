# Security policy

## Reporting a vulnerability

Report vulnerabilities privately to **security@dokimi.dev**. Do not open a
public issue — the report should not be public before a fix is available.

Include what you need to make the problem reproducible: the affected module
and version, what you did, what happened, and what you expected. A proof of
concept helps and is not required.

You will get an acknowledgement that the report was received. If you have not
heard back within a week, assume the mail did not arrive and send it again.

## Supported versions

eidos has not had a release. No version is supported yet, and there is
nothing deployed to patch.

When releases begin, this section will name the supported versions. The
policy the specification already commits to is that the kernel is semver
with additive-only changes within a major, and each satellite declares and
CI-proves the kernel range it supports.

## Scope

eidos is a library that generates source code. Two classes of report are
worth calling out, because they are the ones with security consequence
rather than correctness consequence alone:

- **Ownership and overwrite** — anything that makes the tool write outside
  its sink root, overwrite or delete a file it cannot prove it produced, or
  bypass drift refusal. Generated output is committed source, so silently
  replacing a hand-edited file is data loss.
- **Hermeticity** — anything that makes a frontend execute project code or
  build tooling. Frontends read declaratively parseable inputs and never
  execute them; a path that breaks that turns a generation run into
  arbitrary code execution from a checkout.

Determinism failures, refused lowerings, and diagnostics that fire wrongly
are ordinary bugs. Report them as issues.
