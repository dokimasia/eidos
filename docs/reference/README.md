# Reference

Look up a flag, a field, an exit code or a default here. Reference pages
describe what things are. They never tell you what to do. The
[how-to guides](../how-to/README.md) do that.

Most of these pages are generated, and the tag pipeline regenerates them for
every release. The backend and plugin registrations produce the funcmap
reference. The registered schemas produce the directive reference. The
published JSON Schema produces the config reference. The diag registry
produces the diagnostic-code index. The completeness check produces the
per-language support matrices. A stale page blocks the release, and
[14-distribution-and-cli.md](../architecture/14-distribution-and-cli.md)
explains why.

Write a page here by hand only when no registry can return for it. Anything
hand-written stops matching the code within two releases.
