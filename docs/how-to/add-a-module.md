# Add a module

Adds a language satellite, a plugin, or a shared component to the workspace.

Before you start, run `make bootstrap` in a working checkout.

## Steps

1. Create the directory and its `go.mod`. Keep the `eidos-` prefix on the
   directory; drop it from the import path.

   ```sh
   mkdir eidos-<name>
   printf 'module go.dokimi.dev/eidos/<name>\n\ngo 1.27.0\n' > eidos-<name>/go.mod
   ```

2. Add a `use` line to [`go.work`](../../go.work), in sorted order. ergon
   reads the module list from that file, which is why `.ergon.yaml` leaves
   `modules:` empty.

3. In [`.ergon.yaml`](../../.ergon.yaml), add a coverage layer under
   `checks.coverage.packages` and the bare module name under
   `checks.commit_msg.scopes`. Use `line: 85` for the kernel and shared
   components, `line: 75` for a satellite.

4. Add a row to the module table in the [README](../../README.md).

5. Add a row to the topology table in
   [01-repos-and-kernel.md](../architecture/01-repos-and-kernel.md). If the
   module is neither the kernel nor a satellite, say what it is and where it
   sits in the dependency order.

6. Write `doc.go`, following the rules in
   [CONTRIBUTING](../../CONTRIBUTING.md#documentation). Run `make fmt` to add
   the SPDX header.

## Check it worked

```sh
make fmt && make check
```

Then check the new scope, because an unlisted scope fails the commit-msg hook
rather than `make check`:

```sh
printf 'feat(<name>): probe\n' > /tmp/scope-probe
ergon check commit-msg /tmp/scope-probe
```

## Rules the new module has to keep

Consumers depend on satellites, satellites depend on the kernel, and nothing
depends the other way. One satellite never imports another; anything
cross-language goes through the kernel's canonical-type hub.

The kernel takes no third-party dependencies. Put code that needs one in a
satellite or a plugin.

Leave the root module `go.dokimi.dev/eidos` out of `go.work`. It holds no
packages, and `go vet ./...` fails on a module with no Go files.
