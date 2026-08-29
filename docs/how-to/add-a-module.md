# Add a module

Adds a new component module — a language satellite, a plugin, or shared
infrastructure — to the workspace.

**Precondition:** a working checkout with `make bootstrap` already run.

## Steps

1. Create the directory and its `go.mod`. The directory keeps the `eidos-`
   prefix; the import path drops it.

   ```sh
   mkdir eidos-<name>
   printf 'module go.dokimi.dev/eidos/<name>\n\ngo 1.27.0\n' > eidos-<name>/go.mod
   ```

2. Add a `use` line to [`go.work`](../../go.work), in sorted order. This is
   the module list ergon discovers from, which is why `.ergon.yaml` keeps
   `modules: []`.

3. In [`.ergon.yaml`](../../.ergon.yaml), add the coverage layer under
   `checks.coverage.packages` (85 for kernel-tier and shared infrastructure,
   75 for a satellite) and the bare module name under
   `checks.commit_msg.scopes`.

4. Add the row to the module table in the [README](../../README.md).

5. Add the row to the topology table in
   [01-repos-and-kernel.md](../architecture/01-repos-and-kernel.md). If the
   module is neither kernel nor satellite, say what it is and where it sits
   in the dependency order.

6. Write `doc.go` per the documentation rules in
   [CONTRIBUTING](../../CONTRIBUTING.md#documentation), and apply the SPDX
   header with `make fmt`.

## Verify

```sh
make fmt && make check
```

Then confirm the new scope is accepted, since an unlisted scope fails the
commit-msg hook rather than the gate:

```sh
printf 'feat(<name>): probe\n' > /tmp/scope-probe && ergon check commit-msg /tmp/scope-probe
```

## Constraints

Dependencies point one way: consumers, then satellites, then the kernel. A
satellite never imports a satellite — cross-language needs go through the
kernel's canonical-type hub. The kernel carries zero third-party
dependencies as a hard property, so a module that needs one is not the
kernel.

The root module `go.dokimi.dev/eidos` is not a workspace member and must not
become one: it holds no packages, and `go vet ./...` exits 1 on a module with
no Go files.
