// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package plugin

import (
	"go.dokimi.dev/eidos/core/directive"
	"go.dokimi.dev/eidos/core/meta"
)

// The provider interfaces are the declarations that the workspace reads
// off a plugin when it composes. Each returns data that the plugin built
// once, and no phase calls one. A slice that one of these interfaces
// returns is the storage of the plugin, and a caller does not change it.

// DirectiveProvider declares the schemas of a plugin, which the workspace
// registers when it composes.
type DirectiveProvider interface {
	Directives() []directive.Schema
}

// KeyProvider registers the metadata keys of a plugin when the workspace
// composes. The call runs once for each workspace, and the plugin keeps
// the typed handles that the registry returns. The handles are constants
// of the composition, of the same class as a directive schema, and not run
// state. Keys takes the registry instead of returning data, because the
// registry returns the handles. The workspace collects the error as a
// fault, so the error names what failed and the collection continues.
type KeyProvider interface {
	Keys(r *meta.Registry) error
}

// KeyBinder binds the named key handles of a plugin to the registry of one
// workspace. BindKeys returns a plugin with the bound ids, and leaves the
// receiver unchanged, so one built plugin serves any number of workspaces.
//
// The workspace calls BindKeys on each annotator and generator that
// implements it, once, after it seals the key registry and before the
// first phase call. The workspace schedules the plugin that BindKeys
// returns, and refuses a returned plugin of another type or another name
// than the receiver.
//
// Error modes: one error for each name that r does not contain under the
// value type of its handle. The message of each error contains the plugin
// and the key, and the workspace collects the error as a fault of the
// composition.
type KeyBinder interface {
	BindKeys(r *meta.Registry) (Plugin, error)
}

// OutputProvider declares the file families that a generator emits.
type OutputProvider interface {
	Outputs() []Output
}

// PolicyProvider declares the lowering policies that the spoke of a
// backend reads. The workspace resolves them for each plan of the backend,
// and derives the policy keys of the target and the params of the
// directive of the target from them.
type PolicyProvider interface {
	Policies() []PolicySpec
}

// CapabilityProvider orders a plugin. It returns a priority for each role,
// and the capabilities that order the plugins of one priority. The
// workspace refuses a plugin that implements some of these methods and
// lacks others, and names the methods that it lacks. A plugin that does
// not implement CapabilityProvider runs at priority 0 in each role.
type CapabilityProvider interface {
	// Priority returns the priority of the plugin in the role r. A plugin
	// with a lower priority runs first, so a negative priority runs before
	// the plugins at 0.
	Priority(r Role) int
	// Provides returns the capabilities that the plugin provides. A
	// capability has one provider in a composition.
	Provides() []Capability
	// Requires returns the capabilities that must be present in the
	// composition, and whose providers run before the plugin.
	Requires() []Capability
	// After returns the capabilities whose providers run before the
	// plugin where the composition contains them. Build checks no
	// presence for them.
	After() []Capability
}

// OptionsProvider declares a typed options struct. Options returns a
// pointer whose exported fields are the options, whose tags document them,
// and whose constructed values are the defaults. The workspace writes the
// values of the config into it, and the composition fingerprint includes
// it, so a load keys units by its values.
type OptionsProvider interface {
	Options() any
}

// Versioned declares the version of the behaviour of a component. Raise it
// with every change to what the component produces. The version of a
// frontend is part of every unit key, and every version is part of the
// composition fingerprint. A stale version therefore serves stale cached
// work without a report.
type Versioned interface {
	Version() string
}
