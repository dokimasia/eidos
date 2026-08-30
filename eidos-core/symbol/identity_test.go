// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package symbol_test

import (
	"testing"

	"go.dokimi.dev/eidos/core/symbol"
)

func TestIdentity(t *testing.T) {
	t.Parallel()

	t.Run("String", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			id   symbol.Identity
			want string
		}{
			{
				name: "package",
				id: symbol.Identity{
					Lang:    "golang",
					Package: "svc/store",
					Kind:    symbol.KindPackage,
				},
				want: "golang:svc/store",
			},
			{
				name: "file",
				id: symbol.Identity{
					Lang:    "golang",
					Package: "svc/store",
					Name:    "store.go",
					Kind:    symbol.KindFile,
				},
				want: "golang:svc/store/store.go",
			},
			{
				name: "top-level type",
				id: symbol.Identity{
					Lang:    "golang",
					Package: "svc/store",
					Name:    "Store",
					Kind:    symbol.KindStruct,
				},
				want: "golang:svc/store.Store",
			},
			{
				name: "function carries parens",
				id: symbol.Identity{
					Lang:    "golang",
					Package: "svc/store",
					Name:    "Open",
					Disc:    "string",
					Kind:    symbol.KindFunction,
				},
				want: "golang:svc/store.Open(string)",
			},
			{
				name: "member field",
				id: symbol.Identity{
					Lang:    "golang",
					Package: "svc/store",
					Owner:   "Store",
					Name:    "timeout",
					Kind:    symbol.KindField,
				},
				want: "golang:svc/store.Store#timeout",
			},
			{
				name: "method with discriminator",
				id: symbol.Identity{
					Lang:    "golang",
					Package: "svc/store",
					Owner:   "Store",
					Name:    "Get",
					Disc:    "ctx,string",
					Kind:    symbol.KindMethod,
				},
				want: "golang:svc/store.Store#Get(ctx,string)",
			},
			{
				name: "nullary method still carries parens",
				id: symbol.Identity{
					Lang:    "golang",
					Package: "svc/store",
					Owner:   "Store",
					Name:    "Close",
					Kind:    symbol.KindMethod,
				},
				want: "golang:svc/store.Store#Close()",
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
				if got := tt.id.String(); got != tt.want {
					t.Fatalf("String() = %q, want %q", got, tt.want)
				}
			})
		}
	})

	t.Run("Parse", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name    string
			in      string
			want    symbol.Identity
			wantErr bool
		}{
			{
				name: "package",
				in:   "golang:svc/store",
				want: symbol.Identity{
					Lang:    "golang",
					Package: "svc/store",
					Kind:    symbol.KindPackage,
				},
			},
			{
				name: "top-level name leaves the kind undetermined",
				in:   "golang:svc/store.Store",
				want: symbol.Identity{Lang: "golang", Package: "svc/store", Name: "Store"},
			},
			{
				name: "function",
				in:   "golang:svc/store.Open(string)",
				want: symbol.Identity{
					Lang:    "golang",
					Package: "svc/store",
					Name:    "Open",
					Disc:    "string",
					Kind:    symbol.KindFunction,
				},
			},
			{
				name: "member leaves the kind undetermined",
				in:   "golang:svc/store.Store#timeout",
				want: symbol.Identity{
					Lang:    "golang",
					Package: "svc/store",
					Owner:   "Store",
					Name:    "timeout",
				},
			},
			{
				name: "method",
				in:   "golang:svc/store.Store#Get(ctx,string)",
				want: symbol.Identity{
					Lang:    "golang",
					Package: "svc/store",
					Owner:   "Store",
					Name:    "Get",
					Disc:    "ctx,string",
					Kind:    symbol.KindMethod,
				},
			},
			{
				name: "nullary method",
				in:   "golang:svc/store.Store#Close()",
				want: symbol.Identity{
					Lang:    "golang",
					Package: "svc/store",
					Owner:   "Store",
					Name:    "Close",
					Kind:    symbol.KindMethod,
				},
			},
			{name: "rejects empty input", in: "", wantErr: true},
			{name: "rejects missing separator", in: "nolang", wantErr: true},
			{name: "rejects empty path", in: "golang:", wantErr: true},
			{name: "rejects empty language", in: ":svc/store", wantErr: true},
			{
				name:    "rejects unclosed discriminator",
				in:      "golang:svc/store.Open(string",
				wantErr: true,
			},
			{name: "rejects empty owner", in: "golang:svc/store.#name", wantErr: true},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
				got, err := symbol.Parse(tt.in)
				if tt.wantErr {
					if err == nil {
						t.Fatalf("Parse(%q): error = nil, want non-nil", tt.in)
					}
					return
				}
				if err != nil {
					t.Fatalf("Parse(%q): unexpected error: %v", tt.in, err)
				}
				if got != tt.want {
					t.Fatalf("Parse(%q) = %+v, want %+v", tt.in, got, tt.want)
				}
			})
		}
	})

	t.Run("equality", func(t *testing.T) {
		t.Parallel()

		t.Run("two overloads answer distinct identities", func(t *testing.T) {
			t.Parallel()

			// Overloads share every part but the discriminator, which
			// is the whole reason it exists: Java, Kotlin, C# and
			// TypeScript all admit two methods under one name.
			base := symbol.Identity{
				Lang:    "java",
				Package: "svc/store",
				Owner:   "Store",
				Name:    "Get",
				Kind:    symbol.KindMethod,
			}
			byKey := base
			byKey.Disc = "String"
			byIndex := base
			byIndex.Disc = "int"

			if byKey == byIndex {
				t.Fatal("two overloads share one identity: the discriminator does not separate them")
			}
			if byKey.String() == byIndex.String() {
				t.Fatalf("two overloads spell alike: %q", byKey.String())
			}
		})

		t.Run("a language without overloads answers one identity", func(t *testing.T) {
			t.Parallel()

			first := symbol.Identity{
				Lang: "golang", Package: "svc/store", Owner: "Store",
				Name: "Get", Kind: symbol.KindMethod,
			}
			second := first
			if first != second {
				t.Fatal("one declaration answered two identities")
			}
		})

		t.Run("the parts that make an identity all count", func(t *testing.T) {
			t.Parallel()

			base := symbol.Identity{
				Lang: "golang", Package: "svc/store", Owner: "Store",
				Name: "Get", Kind: symbol.KindMethod, Disc: "ctx",
			}
			tests := map[string]symbol.Identity{
				"language": {
					Lang: "protobuf", Package: base.Package, Owner: base.Owner,
					Name: base.Name, Kind: base.Kind, Disc: base.Disc,
				},
				"package": {
					Lang: base.Lang, Package: "svc/cache", Owner: base.Owner,
					Name: base.Name, Kind: base.Kind, Disc: base.Disc,
				},
				"owner": {
					Lang: base.Lang, Package: base.Package, Owner: "Cache",
					Name: base.Name, Kind: base.Kind, Disc: base.Disc,
				},
				"name": {
					Lang: base.Lang, Package: base.Package, Owner: base.Owner,
					Name: "Put", Kind: base.Kind, Disc: base.Disc,
				},
				"kind": {
					Lang: base.Lang, Package: base.Package, Owner: base.Owner,
					Name: base.Name, Kind: symbol.KindField, Disc: base.Disc,
				},
			}
			for part, other := range tests {
				t.Run(part, func(t *testing.T) {
					t.Parallel()
					if base == other {
						t.Fatalf("identities matching except for the %s compared equal", part)
					}
				})
			}
		})
	})

	t.Run("IsZero", func(t *testing.T) {
		t.Parallel()

		if !(symbol.Identity{}).IsZero() {
			t.Fatal("zero Identity: IsZero() = false, want true")
		}
		if (symbol.Identity{Lang: "golang"}).IsZero() {
			t.Fatal("populated Identity: IsZero() = true, want false")
		}
	})
}
