// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Package builder generates a builder of each struct under the directive
// builder, and the checks of each builder.
//
// A builder has a setter for each field, and each setter returns the
// builder, so calls chain:
//
//	a := NewAddress().WithStreet("Dam 1").AppendLines("2nd floor").Build()
//
// The form of the type of a field decides its setters. A scalar has With<F>,
// a sequence also has Append<F>, bytes also have With<F>String, and a
// mapping and a set also have With<F>Entry and With<F>Entries. An optional
// has With<F>, which takes the value and points the field at it. The
// setters and the constructors have template bodies in the tree of the
// target golang.
//
// # Seeding
//
// New<T> seeds the value in two steps. The function that the param
// defaults of the directive names seeds the whole value first, as in
// //+reference:builder defaults=defaultAddress. The constructor then
// assigns each default that the plugin defaults stamps on a field, in the
// named slot [DefaultsSlot]. builder requires the capability of defaults,
// so the facts of defaults are complete when it runs. A default is a
// literal, the name of a constant or a variable, such as time.Second, or a
// call of a function without arguments. The rules of Go read it in the
// scope of the struct, and the constructor imports its package.
//
// # Checks
//
// The family of the builders under the tag [TestTag] claims a test file for
// each source file through the template builder.test.tmpl. Each record of
// the file is a copy of a builder, which the settle respells as it respells
// the builder. The file checks that New<T>From keeps each field, that
// New<T> sets each default other than the zero value, and that each With<F>
// sets its field. The samples of the fields come from the rules of Go,
// which read the kernel's sample facts first. A generic struct is checked at
// the witnesses of its type parameters. It has no checks where a parameter
// has no witness, and a field whose type refers to a parameter has none.
// The tests import the standard library and the packages of the defaults
// alone.
//
// # Diagnostics
//
// builder reports [BadDefault] at a field whose default is no value of its
// type, and writes nothing for the struct.
//
// # Dependency position
//
// The package imports the package defaults of the module, the root of the
// Go satellite, the root, diag, directive, emit, meta, node, plugin, rules
// and symbol packages of the SDK facade, and the Go stdlib. Its output is
// Go alone.
package builder
