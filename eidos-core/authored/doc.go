// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Package authored holds the kernel's two authored-value
// annotators: the handlers behind the sample and witness
// directives, which stamp what an author stated onto the kernel's
// own keys before any derivation runs.
//
// [Sample] reads a sample instance on a declaration that carries
// one type and stamps its value and alternate as text in the source
// language. [Witness] reads a witness instance on a generic
// declaration and stamps the resolved type onto each named type
// parameter, refusing a key that names none of the declaration's
// parameters.
//
// Both are ordinary annotators built on the authoring surface, and
// a composition lists them like any other. The kernel's workspace
// registers the two schemas and the three keys on its own; it
// registers no plugin on anyone's behalf, so a composition that
// wants authored values lists these two.
//
// # Dependency position
//
// core/authored imports the authoring root, core/diag,
// core/directive, core/meta, core/node, core/plugin, core/position
// and the Go stdlib. It is imported by compositions, never by the
// kernel.
package authored
