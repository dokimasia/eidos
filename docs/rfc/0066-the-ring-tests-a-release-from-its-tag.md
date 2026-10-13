---
rfc: 0066
title: The ring tests a release from its tag
author: Roy Klopper <roy.klopper@stealthscale.io>
status: Accepted
created: 2026-10-13
updated: 2026-10-13
discussion: none
supersedes: none
superseded-by: none
produces-adr: ADR-0053
---

<!--
  ~ Copyright Dokimasia B.V. 2026
  ~ SPDX-License-Identifier: Apache-2.0
-->

# RFC-0066: The ring tests a release from its tag

## Summary

The ring job of CI tests the latest published version of each module of the ring against the
kernel and the SDK facade of the checkout. It wrote a scratch module that required the version and
replaced the kernel and the facade with the checkout. The go command then tested the module from its
zip in the module cache. In the first run over the releases of v0.1.0, four of twelve modules failed
for reasons that the zip and the scratch module cause.

We propose that the ring tests each release from the tree of its tag. It reads the tree with
`git archive` from the repository of the checkout. It points the replace directives of the go.mod of
the module at the kernel and the facade of the checkout. It then runs `go test` in the directory of
the module. The ring job checks out the whole history with its tags. Over the checkout of commit
cf24fc1c, the ring passes the twelve releases of v0.1.0.

## Motivation

The CI run of commit cf24fc1c failed in the job Ring. The ring tested twelve releases. Eight passed.
The modules cli, conformance, plugin/shape/tools and reference failed. The failures have four
causes. None of them is a change of the kernel:

- A module zip leaves out each directory that contains a go.mod. The fixtures of conformance and
  reference under `testdata` are such directories. Their tests failed with
  `stat .: no such file or directory`. `go.dokimi.dev/eidos/conformance@v0.1.0` does not have
  `declarative/testdata/tree`.
- The go.mod of each published module replaces its siblings with relative directories, such as
  `../eidos-core`. The go command ignores the replace directives of a dependency. A test that runs
  the go command in the directory of the module makes the module the main module. Its directives
  then apply. The module cache does not have the directories. The build fails with
  `go.dokimi.dev/eidos/core@v0.1.0: replacement directory ../eidos-core does not exist`. The
  acceptance tests of cli, conformance and reference failed this way.
- A zip contains the files of its module alone. The tests of plugin/shape/tools compare their
  output with the files `../spec.schema.json` and `../catalog/registry_wiring.gen.go` of its parent
  module.
- Graph pruning left the requirements of the kernel out of the build list, because the scratch
  module did not require the kernel. plugin/shape/tools built the `frontendtest` of the checkout with
  assert v0.0.0-20261007133442-6f235714117b instead of v0.2.0. The build failed with
  `undefined: assert.Context`.

The CI run of the release commit 1aacd53f passed because the proxy did not serve a release yet. The
run of cf24fc1c was the first run of the ring over releases.

## Detailed design

### Components

| Component | Change |
|---|---|
| `eidos-reference/internal/ring`, `ring.go` | `checkout` gains the root and the directory of each module. `test` writes the tree of the tag and runs `go test` in the module. `extract` and `rewire` are new. `command` runs the go command and git |
| `eidos-reference/internal/ring`, `main.go` | `run` passes each module with its directory |
| `eidos-reference/internal/ring`, tests | The checkout of the tests is a git repository with release tags. New cases cover `extract` and `rewire` |
| `.ergon/local/.github/workflows/ci.yml` | The ring job checks out with `fetch-depth: 0`. `ergon init sync` merges the job into `ci.yml` |
| Documents | The package comment of the ring, the compatibility document and the decision log |

### Invariants

1. The ring tests the tree at the release tag. That tree contains every file of the zip of the
   module.
2. The kernel and the facade come from the checkout. Every other dependency comes from the module
   proxy at the version that the module requires. The requirements of the kernel of the checkout
   can raise that version.
3. No replace directive of the release applies to the test.
4. The ring writes into the scratch directory of its run alone. It reads the git repository and does
   not write it.

### The tree of a release

`read` records the root of the checkout and the directory of each module of the ring:

```go
// checkout is the workspace of a checkout. The ring reads it from the
// go.work of the checkout.
type checkout struct {
    // root is the absolute directory of the go.work. It is the root of the
    // git repository whose release tags the ring reads.
    root string
    // kernel and facade are the absolute directories of the kernel and of
    // the SDK facade.
    kernel, facade string
    // ring are the modules that require the kernel in the order of their
    // paths. The kernel and the facade are not among them.
    ring []member
}

// member is a module of the ring.
type member struct {
    // path is the module path.
    path string
    // dir is the directory of the module relative to the root, with slashes
    // as the separators. It is "." for the module at the root.
    dir string
}
```

By the rule of the go command, the tag of a module in a subdirectory of its repository is
`<dir>/<version>`. The tag of the module at the root is the version. proxy.golang.org records the
origin of each version. The origin of `go.dokimi.dev/eidos/cli@v0.1.0` has the subdirectory
`eidos-cli` and the reference `refs/tags/eidos-cli/v0.1.0`. For each of the twelve releases, the
commit of the origin is the commit of the tag. `test` takes these steps:

1. It runs `git -c core.autocrlf=input -c core.eol=lf archive --format=tar refs/tags/<dir>/<version>`
   in the root. The go command archives a module version from git with the same two settings. Each
   text file then has the line endings of the zip.
2. `extract` writes the directories and the regular files of the archive into the scratch directory
   of the module through an `os.Root`. It writes each file with the mode 0o600. A module zip does
   not record a file mode. The go command writes each file of the module cache with the mode
   0o444. `extract` skips the global header in which git records the commit. Any other entry, such
   as a symbolic link, fails the module.
3. `extract` writes the go.mod of the module as `rewire` returns it. `rewire` drops each replace
   directive. It then replaces the kernel and the facade with their directories in the checkout.
4. `go test -mod=mod -count=1 ./...` runs in the directory of the module with `GOWORK=off`. The
   requirements of the kernel of the checkout apply, because the module is the main module and
   requires the kernel. `-mod=mod` lets the go command write the requirements and the sums of the
   new graph into the go.mod and the go.sum of the tree.

```go
// test runs go test over the packages of version of the module m against
// the kernel and the SDK facade of the checkout c. It returns the output of
// go test.
func test(ctx context.Context, env []string, c checkout, dir string, m member, version string) ([]byte, error)

// extract creates dir. It writes the directories and the regular files of
// the tar archive that r reads into dir. goMod is a name of the archive.
// extract writes that file as rewire returns it for c.
func extract(r io.Reader, dir, goMod string, c checkout) error

// rewire returns the go.mod data of a release without its replace
// directives. In their place, the go.mod replaces the kernel and the SDK
// facade with their directories in c. name is the file name of data. The
// errors of the parser contain it.
func rewire(name string, data []byte, c checkout) ([]byte, error)

// command runs program with args in dir under ctx. The program runs with
// the environment env and without a workspace. command returns what the
// program writes to its standard output.
func command(ctx context.Context, env []string, dir, program string, args ...string) ([]byte, error)
```

The archive of a tag contains the whole repository at the tag. The tests of plugin/shape/tools read
files of their parent module. A test can read any file of the repository. The archive of
`eidos-cli/v0.1.0` has 18,544,640 bytes, with 513 directories and 2,232 files.

### The CI job

The ring job checks out the whole history with `fetch-depth: 0`. The checkout then has every tag.
The history of the repository packs to 17.56 MiB.

### Tests

The checkout of the tests is a git repository with one commit, the release. Its kernel and its
facade have the major version 1. The tags `passing/v0.1.0` and `failing/v0.1.0` point at the
release. The go.mod of each release replaces the kernel, the facade and a sibling with directories
of the tree. The tree does not have a directory of the sibling. The test of each release checks the
major versions of the kernel and of the facade. It also reads a nested module of its testdata. The
working tree raises the kernel and the facade to version 2. git runs without the variables `GIT_*`
of the environment of the test and without the configuration of the system and the user.

- The passing release expects version 2. It passes against the checkout alone.
- The failing release expects version 1. Its failure shows that the ring tests the tree of the tag,
  because its test in the working tree expects version 2.
- A version without a tag fails with the error of git.
- `extract` writes a directory and a file and skips the global header. It returns an error for a
  symbolic link, an archive that does not read, a file that ends before its size, an entry outside
  its directory, and a directory that it cannot create.
- `rewire` replaces the kernel and the facade. It drops the directive of the sibling. It returns the
  error of a go.mod that does not parse.

With this change, the ring over the checkout of cf24fc1c prints `ok` for the twelve releases and
`unpublished` for the root module.

### Verdicts

| Item | Verdict | Reason | Reverses the verdict |
|---|---|---|---|
| The tree of the release tag | in | It has the nested modules and the files outside the module that the tests of a release read | none |
| `git archive` with the line endings of the go command | in | The go command reads a module version from git with the same settings | none |
| The whole repository at the tag | in | A test can read a file of another module | none |
| Each replace directive of the release dropped | in | A consumer of the release does not read them. A directive into the tree builds against the kernel of the release | none |
| The executable bit of a file | out | The module cache does not contain an executable file | A test of a release that runs a file of its tree |
| Symbolic links | out | The repository does not have one. A module zip leaves them out | A symbolic link in the repository |
| The commit that the proxy records | out | `go mod download -json` prints the origin after it downloads the zip of the version. The tag points at the same commit for each release of v0.1.0 | A tag that moves after its release |
| The whole history in the ring job | in | `git archive` needs the commit of each tag | none |

### Failures

| Condition | Report |
|---|---|
| A tag that the checkout does not have, as in a shallow clone | FAIL with the error of git |
| A go.mod of the release that does not parse | FAIL with the error of the parser |
| An entry of the archive that is not a directory or a regular file | FAIL with the name of the entry |
| A test of the release that fails against the checkout | FAIL with the output of go test |

### Migration

1. Let `read` record the root and the directory of each module.
2. Let `test` write the tree of the tag through `extract` and `rewire`. Let it run `go test` in the
   directory of the module.
3. Let the ring job fetch the whole history. Merge the job into `ci.yml` with `ergon init sync`.
4. Build the checkout of the tests as a git repository with release tags.
5. Describe the mechanism in the package comment of the ring and in the compatibility document.

## Alternatives

### The scratch module with the kernel among its requirements

The scratch module could require the kernel and the facade. With the requirements of the kernel in
the build list, plugin/shape/tools would build. The zip would still not have a nested module or a
file outside the module. The go.mod in the module cache would still replace the siblings with
directories that do not exist there.

### Releases that a zip can test

The fixtures under testdata could keep their go.mod under another name and restore it at test time.
The go.mod of each module could leave its replace directives to `go.work`, as the go.mod of the root
module does. The release v0.1.0 would stay untestable, because both changes apply to new releases
alone. The tests of plugin/shape/tools would still read files of their parent module. The change
touches 29 fixtures and the go.mod files of 13 modules.

### The workspace of the tag

The ring could keep the go.work of the tree and point its kernel and facade at the checkout. The
tests would then build against the sources of the siblings at the tag. A consumer of the release
builds against the versions that the module requires.

### The archive of the forge

GitHub serves an archive of each tag. A ring that downloads it needs access to github.com and
depends on one forge. The checkout already has the objects of each tag.

### A worktree of the tag

`git worktree add` could check out the tag beside the checkout. It records the worktree in the
repository. A run that ends early leaves the record behind. `git archive` reads the repository and
does not write it.

## Drawbacks

- The ring job fetches the whole history and every tag. The packed history has 17.56 MiB.
- Each release writes the whole repository at its tag into the scratch directory. At v0.1.0 a
  release writes 18.5 MB in 2,745 entries. A run over the twelve releases writes about 220 MB.
- The ring depends on git and on the tag rule of the go command. A release whose tag the checkout
  lacks fails.
- The ring does not test the zip that the proxy serves. A package that needs a file that the zip
  leaves out, such as a symbolic link, passes the ring and fails in the build of a consumer.
- `go test -mod=mod` rewrites the go.mod and the go.sum of the tree. The checksum database verifies
  the sums of the new requirements, as it did for the scratch module.

## Open questions

None.

## Unresolved and future work

None.

## References

- [RFC-0026](0026-the-reference-ensemble-and-the-canary-ring.md), the ring and its scratch module.
- [15](../architecture/15-compatibility.md), the canary ring.
- [21-decisions.md](../architecture/21-decisions.md), the decision log.
- The CI run of commit cf24fc1c: <https://github.com/dokimasia/eidos/actions/runs/38446301335>
- Go Modules Reference, the tag of a module in a subdirectory:
  <https://go.dev/ref/mod#vcs-version>
- Go Modules Reference, the files that a module zip leaves out:
  <https://go.dev/ref/mod#zip-path-size-constraints>
- Go Modules Reference, graph pruning: <https://go.dev/ref/mod#graph-pruning>
- The archive of a module version in the go command:
  <https://github.com/golang/go/blob/go1.27.2/src/cmd/go/internal/modfetch/codehost/git.go>
