// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Package registrygen registers each struct under the directive register
// in the function init of its package.
//
// Each struct adds one record to the file of the family registry of its
// package. The family claims its files through the template registry.tmpl,
// which writes one function init for each file:
//
//	// init registers each struct of the package under the directive register.
//	func init() {
//		gob.Register(Item{})
//	}
//
// A declared function init for each struct would declare one name twice in
// a package, and the settle checks the names of a package for collisions.
// A record declares no name of its file, so the records of the structs
// share one function. The template records the import of the package of
// the function through the builtin use, under the name that [Callee]
// contains.
//
// # Options
//
// The options package and func set the function, and default to
// [DefaultPackage] and [DefaultFunc]. The function takes one value of any
// type, as gob.Register does.
//
// # Diagnostics
//
// registrygen reports [Generic] at a struct with type parameters, and the
// package internal/callee reports empty options. registrygen records
// nothing for a struct with a finding.
//
// # Dependency position
//
// The package imports the package internal/callee of the module, the root
// of the Go satellite, and the root, diag, directive, emit and plugin
// packages of the SDK facade. Its output is Go alone.
package registrygen
