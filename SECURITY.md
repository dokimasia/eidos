# Security policy

## Reporting a vulnerability

Email **security@dokimi.dev**. Do not open an issue. The report should stay
private until there is a fix.

Tell us the module and version, what you did, what happened, and what you
expected. Send a proof of concept if you have one.

We reply within 3 working days. If you have not heard back by then, the mail
did not arrive. Send it again.

## Supported versions

eidos has no releases, so no version is supported and there is nothing
deployed to patch.

Once releases start, this section will list them. The kernel follows semantic
versioning and only adds within a major version. Each satellite states which
kernel versions it supports, and its CI tests against both ends of that
range.

## What to report

eidos is a library that writes source files. Two kinds of bug have security
consequences rather than only correctness ones.

The first is writing where it should not. Report anything that makes eidos
write outside the sink root, overwrite or delete a file it cannot prove it
wrote, or skip the drift check. Generated files are committed source, so
overwriting someone's hand edit loses their work.

The second is running what it should read. Frontends parse project files and
never execute them. Report anything that makes a frontend execute project
code or build tooling, because that turns a generation run into arbitrary
code execution from a checkout.

Report determinism bugs, wrong refusals and misfiring diagnostics as ordinary
issues.
