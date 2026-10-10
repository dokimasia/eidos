<!--
  ~ Copyright Dokimasia B.V. 2026
  ~ SPDX-License-Identifier: Apache-2.0
-->

# Add a module

This guide takes you from an empty directory to a module that the workspace
builds, lints and accepts commits for. Use it for a language satellite, a
plugin, or a shared component.

Before you start, install ergon and the git hooks, as
[CONTRIBUTING](../../CONTRIBUTING.md#setup) describes.

## Steps

1. Create the directory and its `go.mod`. Keep the `eidos-` prefix on the
   directory and drop it from the import path.

   ```sh
   mkdir eidos-<name>
   printf 'module go.dokimi.dev/eidos/<name>\n\ngo 1.27.2\n' > eidos-<name>/go.mod
   ```

2. Add a `use` line to [`go.work`](../../go.work), in sorted order. ergon
   reads the module list from that file.

3. In [`.ergon/local/.commitlint.yaml`](../../.ergon/local/.commitlint.yaml),
   add the bare module name to the scopes of `scope-enum`.

4. If the module is plugin-side code, such as a language satellite, a plugin
   or a helper module they share, add its directory to the `plugin-modules`
   rule in [`.ergon/local/.golangci.yml`](../../.ergon/local/.golangci.yml).
   The rule stops its code from importing the kernel, so it imports the SDK
   facade instead. Leave a host-side module that composes the workspace, such
   as `eidos-reference`, out of the rule, because the facade does not
   re-export the workspace.

5. Run `ergon init sync`, which writes the scope into `.commitlint.yaml` and
   the rule into `.golangci.yml`.

6. Add a row to the module table in the [README](../../README.md). If the
   module is neither the kernel nor a satellite, say what it is and where it
   is in the dependency order.

7. Write `doc.go`, following the rules in
   [CONTRIBUTING](../../CONTRIBUTING.md#documentation). Run
   `ergon license fix` to add the SPDX header.

## Check it worked

```sh
make fmt && make check
```

Then check the new scope. An unlisted scope fails the commit-msg hook rather
than `make check`, so the gate alone will not catch it:

```sh
printf 'feat(<name>): probe\n' > /tmp/scope-probe
ergon tool run common.commitlint -- lint --message /tmp/scope-probe
```

## Rules the new module has to keep

Consumers depend on satellites, satellites depend on the kernel, and nothing
depends the other way. One satellite never imports another. Anything
cross-language goes through the kernel's canonical-type hub.

Plugin-side code imports `go.dokimi.dev/eidos/sdk` and never
`go.dokimi.dev/eidos/core`. The kernel arrives as a transitive dependency.

The kernel takes no third-party dependencies. Put code that needs one in a
satellite or a plugin.

Leave the root module `go.dokimi.dev/eidos` out of `go.work`. It holds no
packages, and `go vet ./...` fails on a module with no Go files.
