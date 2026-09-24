// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package plugin

// Target names a rendering target. It is a registered name: the
// composition declares the targets it recognises, and a plan whose
// backend names anything else is a composition fault. The zero
// Target names nothing.
type Target string

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
