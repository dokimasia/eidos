// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package shape

import "go.dokimi.dev/eidos/sdk/plugin"

// Capability is the capability that the plugin shape provides. An
// annotator whose predicates or handlers read a classification requires
// it, so the workspace runs the annotator after the plugin shape.
const Capability plugin.Capability = "shape.classified"

// Shape is the name of a shape. A callable has at most one shape. The
// package declares one constant for each shape spec, such as [Writer].
type Shape string

// Mixin is the name of a mixin. A callable has any number of mixins.
// The package declares one constant for each mixin spec, such as
// [Atomic].
type Mixin string

// Contract is the name of a contract. A contract binds callables to
// the roles of a protocol. The package declares one constant for each
// contract spec, such as [Tx].
type Contract string

// Role is the name of a role of a contract. The package declares one
// constant for each role of each contract spec, such as [TxCommit].
type Role string
