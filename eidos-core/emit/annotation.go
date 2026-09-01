// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package emit

// Annotation is one structured marker a generated declaration
// carries: a Java annotation, a Rust attribute, a TypeScript or
// Python decorator, a C# attribute.
//
// Name is the marker's spelling without its sigil: "Override",
// "derive", "Component". Args holds each argument's source
// spelling verbatim, unevaluated, the way [Constant.Value] holds
// an expression; the target's template supplies the sigil and the
// argument delimiters its language writes. An Annotation is a
// plain value: copy it freely.
type Annotation struct {
	Name string   `json:"name"`
	Args []string `json:"args,omitzero"`
}

// Annotations is the marker list a declaration carries, in the
// order the target writes them.
type Annotations []Annotation
