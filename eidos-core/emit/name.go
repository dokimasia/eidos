// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package emit

import "go.dokimi.dev/eidos/core/symbol"

// DeclaredName returns the name a file-level declaration declares in
// its file's scope, and the empty string for a declaration that
// declares none there. A method declares its name in its receiver's
// scope, and a field in its host or in the type that receives it, so
// both return the empty string too. The render reserves the names
// against import bindings, and the layout resolves bare references to
// generated declarations by them.
func DeclaredName(d symbol.Symbol) string {
	switch t := d.(type) {
	case *Struct:
		return t.Name
	case *Interface:
		return t.Name
	case *Enum:
		return t.Name
	case *Sum:
		return t.Name
	case *Function:
		return t.Name
	case *Alias:
		return t.Name
	case *Constant:
		return t.Name
	case *Variable:
		return t.Name
	default:
		return ""
	}
}

// ReceivesOf returns the type that a declaration attaches to from
// outside the declaration of that type: the Receives of a method or of a
// field. It returns nil for a method or a field that its type declares,
// and for every other kind. The settle checks the name of such a
// declaration against the other names that attach to the same type, and
// the name table lists it under the spelling of the type. ReceivesOf
// allocates nothing.
func ReceivesOf(d symbol.Symbol) *TypeRef {
	switch t := d.(type) {
	case *Method:
		return t.Receives
	case *Field:
		return t.Receives
	default:
		return nil
	}
}
