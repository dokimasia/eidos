# Security policy

## Reporting a vulnerability

Email **security@dokimi.dev**. Do not open an issue — the report should stay
private until there is a fix.

Tell us the module and version, what you did, what happened, and what you
expected. Send a proof of concept if you have one.

We reply within 3 working days. If you have not heard back by then, the mail
did not arrive; send it again.

## Supported versions

eidos has no releases yet, so there is no supported version and nothing
deployed to patch.

Once releases start, this section will list them. The kernel follows semantic
versioning and only adds within a major version. Each satellite states which
kernel versions it supports, and its CI tests against both ends of that range.

## What to report

eidos is a library that writes source files. Two kinds of bug have security
consequences rather than just correctness ones:

- **Writing where it should not.** Anything that makes eidos write outside
  the sink root, overwrite or delete a file it cannot prove it wrote, or skip
  the drift check. Generated files are committed source, so overwriting
  someone's hand edit loses their work.
- **Running what it should read.** Frontends parse project files and never
  execute them. Anything that makes a frontend execute project code or build
  tooling turns a generation run into arbitrary code execution from a
  checkout.

Report determinism bugs, wrong refusals and misfiring diagnostics as ordinary
issues.
