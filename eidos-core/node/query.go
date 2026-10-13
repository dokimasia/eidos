// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package node

import "iter"

// DeclsOf returns the top-level declarations of p of the type T, such as
// every *Interface of a package, in the order of the files of p and of
// their declarations. A nil p and a nil file yield nothing. It allocates
// nothing beyond the iterator.
func DeclsOf[T Declaration](p *Package) iter.Seq[T] {
	return func(yield func(T) bool) {
		if p == nil {
			return
		}
		for _, f := range p.Files {
			if f == nil {
				continue
			}
			for _, sym := range f.Decls {
				if decl, is := sym.(T); is && !yield(decl) {
					return
				}
			}
		}
	}
}

// DeclNamed returns the first top-level declaration of p of the type T
// whose identity has the name, and false where p declares none. A
// declaration has its identity once the resolution step of the load
// assigned it. It allocates nothing.
func DeclNamed[T Declaration](p *Package, name string) (T, bool) {
	for decl := range DeclsOf[T](p) {
		if decl.Identity().Name == name {
			return decl, true
		}
	}
	var none T
	return none, false
}

// MemberNamed returns the first member of a list whose identity has the
// name, such as a method of a struct, and false where the list has none.
// It skips a nil member. It allocates nothing.
func MemberNamed[E any, T interface {
	*E
	Declaration
}](members []T, name string) (T, bool) {
	for _, m := range members {
		if m != nil && m.Identity().Name == name {
			return m, true
		}
	}
	return nil, false
}
