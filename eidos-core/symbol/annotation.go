// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package symbol

// Annotation is one structured marker a declaration carries: a
// Java annotation, a TypeScript or Python decorator, a Rust
// attribute, a proto option. The name is spelled without its
// sigil and the arguments verbatim, the argument delimiters left
// to whichever language writes them: a frontend records what the
// source stated, a generator states what the target must write,
// and the Tier-2 annotation rules read either statically. An
// Annotation is a plain value: copy it freely.
type Annotation struct {
	Name string   `json:"name"`
	Args []string `json:"args,omitzero"`
}

// Annotations is a declaration's marker list, in source order on
// the node side and in the order the target writes them on the
// emit side.
type Annotations []Annotation
