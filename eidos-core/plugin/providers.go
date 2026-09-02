// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package plugin

import (
	"go.dokimi.dev/eidos/core/directive"
	"go.dokimi.dev/eidos/core/meta"
)

// The capability surfaces: interfaces the composition asserts on a
// plugin to learn what it declared. Each returns data built once;
// none is invoked during a phase.

// DirectiveProvider declares the schemas a plugin owns, for
// registration at composition.
type DirectiveProvider interface {
	Directives() []directive.Schema
}

// KeyProvider registers the plugin's metadata keys at composition.
// The call runs once per workspace, and the plugin keeps the typed
// handles it is returned: they are composition constants, the same
// class as a directive schema, not run state. It takes the
// registry rather than returning data because registration returns
// the handles back, which data cannot. Faults are collected by the
// caller, so the error names what failed rather than stopping the
// collection.
type KeyProvider interface {
	Keys(r *meta.Registry) error
}

// OutputProvider declares the file families a generator emits.
type OutputProvider interface {
	Outputs() []Output
}

// CapabilityProvider orders a plugin: a priority per role role,
// plus the capability topology inside one priority bucket.
type CapabilityProvider interface {
	Priority(r Role) int
	Provides() []Capability
	Requires() []Capability
}

// OptionsProvider declares a typed options struct: a pointer whose
// exported fields are the options, whose tags document them, and
// whose constructed values are the defaults. The composition
// populates it; the workspace folds it into the composition
// fingerprint a load keys units by.
type OptionsProvider interface {
	Options() any
}

// Versioned declares a behavior version. Bump it with any change
// to what the plugin produces: a frontend's version folds into
// every unit key, and every version folds into the composition
// fingerprint, so a stale version serves stale cached work with
// nothing reporting why.
type Versioned interface {
	Version() string
}
