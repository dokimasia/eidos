// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package plugin

import "go.dokimi.dev/eidos/core/meta"

// nameKeySuffix follows a target's spelling in the key a name
// override is stamped under.
const nameKeySuffix = ".name"

// Target names a rendering target. It is a registered name: the
// composition declares the targets it recognises, and a plan whose
// backend names anything else is a composition fault. The zero
// Target names nothing.
type Target string

// NameKey returns the key a declaration's name override in the
// target is stamped under: the target's spelling followed by .name,
// so the golang target reads golang.name. The settle reads it on a
// declaration's origin, see [Settle].
func (t Target) NameKey() meta.KeyName {
	return meta.KeyName(string(t) + nameKeySuffix)
}

// Backend is a plan's target role. It names the [Target] the plan
// renders to. A backend that also implements [Renderer] renders the
// plan's emit. A plan has exactly one backend, whose target is a
// registered name. A composition that runs no renderer still
// validates both, so a plan is whole before anything arrives on
// disk.
type Backend interface {
	Plugin
	Target() Target
}
