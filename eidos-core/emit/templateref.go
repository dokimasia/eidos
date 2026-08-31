// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package emit

// TemplateRef claims a body for a template. The name resolves in
// the emitting plugin's template tree for the plan's target, never
// in the backend's or another plugin's, so the same emit graph renders
// through a different tree per plan while the generator stays
// never seeing languages.
//
// Data is the plugin-supplied payload the template executes over.
// The codec carries it as generic JSON values, so a decoded
// reference reads its payload dynamically, which is how a template
// reads it anyway.
type TemplateRef struct {
	Name string `json:"name"`
	Data any    `json:"data,omitzero"`
}
