// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Command ring tests the published version of each module of the canary
// ring of eidos against the kernel and the SDK facade of a checkout.
//
// CI tests every module of the workspace at the commit of the checkout. A
// published module of the ring also runs with a kernel that is newer than
// the kernel of its release. The satellites and eidos-reference are such
// modules. ring builds that combination:
//
//  1. It reads the go.work of the checkout. It lists each module that
//     requires the kernel go.dokimi.dev/eidos/core. The kernel and the SDK
//     facade go.dokimi.dev/eidos/sdk are not in the ring.
//  2. It queries the module proxy for the latest version of each module of
//     the ring. go list -m <module>@latest runs the same query.
//  3. For a release or a pre-release, it reads the go.mod of the release tag
//     from the git repository of the checkout. The tag of a module in a
//     subdirectory is <dir>/<version>. The tag of the module at the root is
//     the version. A release that requires the kernel or the facade on an
//     earlier compatibility line than the checkout releases is earlier, and
//     ring does not test it.
//  4. It writes the tree of the tag of any other release into a scratch
//     directory. In the go.mod of the module, it drops each replace
//     directive and replaces the kernel and the SDK facade with their
//     directories in the checkout. It then runs go test -mod=mod over the
//     packages of the module in the directory of the module.
//
// The compatibility line of a version in v0 is its major and its minor, such
// as v0.3. The line of a later version is its major, such as v2. A minor
// version of v0 can break the API of the kernel. The release that moves the
// line of the kernel also releases each module of the ring with the new
// requirement. The version that the checkout releases of the kernel or the
// facade is the version of its replace directive in the go.work, raised by
// the highest bump of the module in the changesets of the checkout. A module
// without a versioned replace directive in the go.work does not make a
// release earlier.
//
// A module zip leaves out each directory that has a go.mod and each file
// outside the module. A fixture under testdata can be such a directory. The
// tree of the tag has both. The tests of a release read the same files there
// as in the CI run of the release.
//
// A module is unpublished when the proxy returns a pseudo-version or answers
// 404 Not Found or 410 Gone. The proxy returns a pseudo-version for a module
// without a release tag. ring does not test an unpublished module. Any other
// error of the query fails the module. A proxy that does not answer fails
// the run.
//
// # Usage
//
//	go run ./eidos-reference/internal/ring [-root dir]
//
// -root is the directory of the go.work of the checkout. Its default is the
// working directory. It is the root of the git repository with the release
// tags. The go command reads settings such as GOPROXY from the environment
// of the ring. It runs without a workspace.
//
// # Output
//
// ring writes one line for each module of the ring to standard output in
// the order of the module paths:
//
//	ok <module>@<version>
//	FAIL <module>@<version>: <error>
//	FAIL <module>: <error>
//	unpublished <module>: <reason>
//	earlier <module>@<version>: <reason>
//
// The output of go test follows the line of a module whose test fails. The
// second form of FAIL reports a query that failed. The reason of an earlier
// release contains the requirement of the release and the version that the
// checkout releases.
//
// ring exits with status 0 when no module fails. An earlier release does not
// fail. It exits with status 1 when a module fails or the scratch directory of
// the run cannot be created. It exits with status 64 for a command line or a
// checkout that it cannot read. A Go panic exits with status 2.
//
// # Dependency position
//
// The package imports golang.org/x/mod/modfile, golang.org/x/mod/module,
// golang.org/x/mod/semver and the Go stdlib. It runs the go command and git.
// It does not import a package of eidos.
package main
