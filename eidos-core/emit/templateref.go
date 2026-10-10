// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package emit

import "go.dokimi.dev/eidos/core/diag"

// TemplateRef claims a body for a template. The name resolves in its
// owner's template tree for the plan's target, never in the
// backend's. Owner names the plugin whose tree that is, and a
// reference without an owner resolves in the tree of the plugin
// whose unit contains the body. A reference one plugin places inside
// another plugin's declaration, through a slot, therefore renders
// through its own plugin's tree. The same emit graph renders through
// a different tree per plan, and the generator never sees a
// language.
//
// Data is the plugin-supplied payload the template executes over.
// The codec encodes it as generic JSON values, so a decoded
// reference reads its payload dynamically, which is how a template
// reads it anyway.
type TemplateRef struct {
	Name  string      `json:"name"`
	Data  any         `json:"data,omitzero"`
	Owner diag.Origin `json:"owner,omitzero"`
}
