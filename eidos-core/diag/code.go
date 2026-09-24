// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package diag

import (
	"cmp"
	"fmt"
	"slices"
	"strconv"
)

// Prefix owns a range of codes: the kernel's, a satellite's, and a
// consumer's own registered when the workspace builds. Uppercase
// letters, nothing else, so the spelled code splits back into its
// prefix and number without escaping: a hyphen inside the prefix
// would blur the join, and a digit would blur where the number
// begins.
type Prefix string

// Valid reports whether p is spelled the way a code requires: one
// character at least, every one an uppercase letter.
func (p Prefix) Valid() bool {
	if p == "" {
		return false
	}
	for i := range len(p) {
		if p[i] < 'A' || p[i] > 'Z' {
			return false
		}
	}
	return true
}

// KernelPrefix owns every code the kernel reports.
const KernelPrefix Prefix = "EID"

// codeDigits is the width a code's number is padded to, so codes
// sort and read alike however small the number is.
const codeDigits = 4

// Code identifies a finding across releases.
//
// The zero Code names nothing and reports true from [Code.IsZero].
type Code struct {
	Prefix Prefix
	Number int
}

// CodeSpec is what a registration declares.
type CodeSpec struct {
	// Number is unique within the prefix.
	Number int
	// Meaning anchors the code in the published index. A changed
	// meaning is a new code, never an edit.
	Meaning string
}

// Registry holds every registered code and the meaning each one
// carries.
//
// A Registry is not safe for concurrent use. Registration happens at
// package initialization and when the workspace builds, both of
// which are single-threaded.
type Registry struct {
	meanings map[Code]string
}

// NewRegistry returns an empty registry.
func NewRegistry() *Registry {
	return &Registry{meanings: map[Code]string{}}
}

// Register records a code and returns it.
//
// It refuses a prefix that is not uppercase letters, a number below
// 1, and a spec without a meaning. A number claimed twice within one
// prefix is an error naming both meanings, because the second
// claimant would otherwise report under the first one's identity.
// It returns an error rather than panicking so that a caller
// collecting faults reports every one.
func (r *Registry) Register(p Prefix, s CodeSpec) (Code, error) {
	if !p.Valid() {
		return Code{}, fmt.Errorf(
			"diag: code %d claims prefix %q, which is not uppercase letters: "+
				"a code belongs to whoever owns it, spelled so it splits back",
			s.Number, p,
		)
	}
	if s.Number < 1 {
		return Code{}, fmt.Errorf("diag: %s claims number %d: a code's number counts from 1",
			p, s.Number)
	}
	if s.Meaning == "" {
		return Code{}, fmt.Errorf("diag: %s-%0*d names no meaning: the index anchors to it",
			p, codeDigits, s.Number)
	}

	code := Code{Prefix: p, Number: s.Number}
	if held, taken := r.meanings[code]; taken {
		return Code{}, fmt.Errorf("diag: %s is claimed twice: %q and %q", code, held, s.Meaning)
	}
	r.meanings[code] = s.Meaning
	return code, nil
}

// MustRegister records a code and panics on every refusal
// [Registry.Register] returns.
//
// It is what a package uses to declare its codes at initialization,
// where a refused code is a defect in the source rather than a
// condition a run can meet, and where there is no sink to report
// into yet. Everything a workspace populates from config uses
// [Registry.Register] and collects the faults instead.
func MustRegister(p Prefix, s CodeSpec) Code {
	code, err := kernel.Register(p, s)
	if err != nil {
		panic(err)
	}
	return code
}

// kernel holds the codes packages declare at initialization.
var kernel = NewRegistry()

// Kernel returns the registry [MustRegister] records into.
func Kernel() *Registry { return kernel }

// Meaning returns what a registered code means, and false for a code
// this registry does not hold.
func (r *Registry) Meaning(c Code) (string, bool) {
	meaning, known := r.meanings[c]
	return meaning, known
}

// Codes returns every registered code, in prefix then number order.
func (r *Registry) Codes() []Code {
	out := make([]Code, 0, len(r.meanings))
	for code := range r.meanings {
		out = append(out, code)
	}
	slices.SortFunc(out, func(a, b Code) int {
		return cmp.Or(cmp.Compare(a.Prefix, b.Prefix), cmp.Compare(a.Number, b.Number))
	})
	return out
}

// IsZero reports whether the code names nothing.
func (c Code) IsZero() bool { return c == Code{} }

// String renders the code as its prefix and padded number.
//
// A number wider than the padding is not truncated: the padding sets
// a floor on the width, never a ceiling.
func (c Code) String() string {
	// A code is spelled into every diagnostic a run reports, so the
	// parts are written into one buffer rather than concatenated in
	// steps. The buffer is sized for a prefix and a number no
	// registration exceeds, and grows on the heap if one does.
	var (
		scratch [32]byte
		digits  [20]byte
	)
	number := strconv.AppendInt(digits[:0], int64(c.Number), 10)

	out := append(scratch[:0], c.Prefix...)
	out = append(out, '-')
	for range codeDigits - len(number) {
		out = append(out, '0')
	}
	return string(append(out, number...))
}
