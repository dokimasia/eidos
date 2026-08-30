// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package plugin

import (
	"go.dokimi.dev/eidos/core/directive"
	"go.dokimi.dev/eidos/core/meta"
)

// The capability surfaces: interfaces the composition asserts on a
// plugin to learn what it declared. Each answers data built once;
// none is invoked during a phase.

// DirectiveProvider declares the schemas a plugin owns, for
// registration at composition.
type DirectiveProvider interface {
	Directives() []directive.Schema
}

// KeyProvider registers the plugin's metadata keys at composition.
// The call runs once per workspace, and the plugin keeps the typed
// handles it is answered: they are composition constants, the same
// class as a directive schema, not run state. It takes the
// registry rather than answering data because registration answers
// the handles back, which data cannot. Faults are collected by the
// caller, so the error names what failed rather than stopping the
// bill.
type KeyProvider interface {
	Keys(r *meta.Registry) error
}

// OutputProvider declares the file families a generator emits.
type OutputProvider interface {
	Outputs() []Output
}

// CapabilityProvider orders a plugin: a priority per role seat,
// plus the capability topology inside one priority bucket.
type CapabilityProvider interface {
	Priority(r Role) int
	Provides() []Capability
	Requires() []Capability
}

// OptionsProvider declares a typed options struct: a pointer whose
// exported fields are the options, whose tags document them, and
// whose constructed values are the defaults. The composition
// populates it and folds the populated value into the fingerprint.
type OptionsProvider interface {
	Options() any
}

// Versioned contributes to the run fingerprint.
type Versioned interface {
	Version() string
}
