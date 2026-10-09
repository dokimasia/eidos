// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Package tools generates the registry of the shape catalog through an
// eidos workspace.
//
// [Compose] returns the composition. It composes the spec frontend of
// package specfront, which loads the YAML specs below spec/ of the catalog
// module, and the registry generator of package registry toward the Go
// backend. The module has no main package. Its test TestCompose runs the
// composition over the catalog module and compares each generated file
// and the published spec schema with the file in the catalog module.
// Without -update the test is the mirror guard, and with -update it
// writes the files. The go:generate line of the catalog runs it with
// -update.
//
// # Dependency position
//
// The package imports the kernel's layout, output, rules and workspace
// packages, the Go satellite's root and backend packages, the SDK
// facade's plugin package, and the packages registry and specfront. It
// is the one package of the catalog's modules that imports the kernel
// and a language satellite, and the catalog module does not import it.
package tools
