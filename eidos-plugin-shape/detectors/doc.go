// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Package detectors contains the detector of each detected shape of the
// catalog.
//
// A detector, such as [Writer], is a function of the projection of one
// callable and the bound rules of its language. It reads the roles that
// the language gave the parameters and the returns, the error model, the
// name of the callable and the folded shapes of its types. It reads
// nothing of a language, so one detector serves every language whose rules
// project callables. The generated Detections of package catalog lists
// each detector beside the constant of its shape, in the order of
// precedence, so a detected spec without a detector of its name does not
// compile.
//
// # Signatures
//
//   - An input is a parameter with the input role. A context parameter is
//     no input.
//   - A value is a return with the value or the stream role. The error and
//     the ok flag are no values.
//   - A callable fails where the rules of its language give it an error
//     model.
//
// # Overlaps
//
// Two detectors can report one callable, such as [Writer] and [Deleter]
// for Delete(v) error. The yields_to lists of the specs order such pairs,
// and the plugin shape stamps the first shape in the order. A detector
// states its own signature, and does not repeat the rule of a detector
// that ranks before it.
//
// # Allocation contract
//
// A detector allocates nothing of its own. The folds that it asks of the
// bound rules allocate as [go.dokimi.dev/eidos/sdk/rules.Bound.TypeOf]
// states, so a detector allocates nothing over a reference that the
// binding folded before.
//
// # Dependency position
//
// The package imports the SDK facade's node, rules and symbol packages.
package detectors
