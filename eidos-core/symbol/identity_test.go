// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package symbol_test

import (
	"slices"
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/symbol"
)

// parseCase is one spelling and what [symbol.Parse] makes of it.
type parseCase struct {
	name    string
	in      string
	want    symbol.Identity
	wantErr bool
}

// parseCases returns the pinned grammar: the spellings Parse accepts
// with the identities it reads, and the spellings it refuses.
//
// [FuzzParse] seeds its corpus from the same table, so the fuzzer
// starts inside the grammar rather than spending its budget
// discovering the colon.
func parseCases() []parseCase {
	return []parseCase{
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
			// The discriminator search admits an opening paren at the
			// very front, so a spelling that is nothing but a
			// discriminator is refused for its missing name rather
			// than read as a package.
			name:    "a discriminator with nothing before it",
			in:      "golang:(int)",
			wantErr: true,
		},
		{
			// The name search admits a dot at the very front of the
			// last segment, so a name directly after the slash keeps
			// its package rather than reading as one whole path.
			name: "a name opening the segment after the slash",
			in:   "golang:svc/.Name",
			want: symbol.Identity{Lang: "golang", Package: "svc/", Name: "Name"},
		},
		{
			name: "a name opening a spelling that has no slash",
			in:   "golang:.Name",
			want: symbol.Identity{Lang: "golang", Name: "Name"},
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
		{
			name:    "rejects a discriminator on a bare package",
			in:      "golang:svc/store()",
			wantErr: true,
		},
		{name: "rejects an empty name after the dot", in: "golang:svc/store.", wantErr: true},
	}
}

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
				assert.Equal(t, tt.id.String(), tt.want,
					"the identity spells the canonical grammar")
			})
		}
	})

	t.Run("Parse", func(t *testing.T) {
		t.Parallel()

		for _, tt := range parseCases() {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
				got, err := symbol.Parse(tt.in)
				if tt.wantErr {
					assert.HasError(t, err, "a spelling outside the grammar is refused")
					return
				}
				assert.NoError(t, err, "a spelling inside the grammar parses")
				assert.Equal(t, got, tt.want, "and reads back the parts it wrote")
			})
		}
	})

	t.Run("equality", func(t *testing.T) {
		t.Parallel()

		t.Run("two overloads get distinct identities", func(t *testing.T) {
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

			assert.NotEqual(t, byKey, byIndex,
				"the discriminator separates two overloads")
			assert.NotEqual(t, byKey.String(), byIndex.String(),
				"and their spellings differ with them")
		})

		t.Run("a language without overloads returns one identity", func(t *testing.T) {
			t.Parallel()

			first := symbol.Identity{
				Lang: "golang", Package: "svc/store", Owner: "Store",
				Name: "Get", Kind: symbol.KindMethod,
			}
			second := first
			assert.Equal(t, first, second,
				"a language without overloads returns one identity")
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
					assert.NotEqual(t, base, other,
						"every part that makes an identity counts")
				})
			}
		})
	})

	t.Run("IsZero", func(t *testing.T) {
		t.Parallel()

		assert.True(t, (symbol.Identity{}).IsZero(),
			"the zero Identity names nothing")
		assert.False(t, (symbol.Identity{Lang: "golang"}).IsZero(),
			"a populated Identity names something")
	})

	t.Run("Compare", func(t *testing.T) {
		t.Parallel()

		base := symbol.Identity{
			Lang: "golang", Package: "svc/store", Owner: "Store",
			Name: "Get", Kind: symbol.KindMethod, Disc: "ctx",
		}

		t.Run("returns zero for one identity", func(t *testing.T) {
			t.Parallel()

			same := base
			assert.Equal(t, base.Compare(same), 0,
				"an identity sorts with its copy")
		})

		t.Run("orders on every part that makes an identity", func(t *testing.T) {
			t.Parallel()

			// Each entry differs from base in one field alone, and
			// sorts after it, so a comparison that skipped that field
			// would return zero.
			raise := func(alter func(*symbol.Identity)) symbol.Identity {
				other := base
				alter(&other)
				return other
			}
			greater := map[string]symbol.Identity{
				"language":      raise(func(id *symbol.Identity) { id.Lang = "protobuf" }),
				"package":       raise(func(id *symbol.Identity) { id.Package = "svc/zache" }),
				"owner":         raise(func(id *symbol.Identity) { id.Owner = "Zache" }),
				"name":          raise(func(id *symbol.Identity) { id.Name = "Put" }),
				"discriminator": raise(func(id *symbol.Identity) { id.Disc = "ctx,string" }),
			}
			for part, other := range greater {
				t.Run(part, func(t *testing.T) {
					t.Parallel()

					assert.True(t, base.Compare(other) < 0,
						"a lesser identity sorts before a greater one")
					assert.True(t, other.Compare(base) > 0,
						"and the comparison reverses with its arguments")
				})
			}
		})

		t.Run("separates two identities differing only in kind", func(t *testing.T) {
			t.Parallel()

			field := base
			field.Kind = symbol.KindField
			assert.NotEqual(t, field.Compare(base), 0,
				"two identities differing only in kind sort apart")
		})

		t.Run("sorts a slice into one order", func(t *testing.T) {
			t.Parallel()

			ordered := []symbol.Identity{
				{Lang: "golang", Package: "svc/cache", Kind: symbol.KindPackage},
				{Lang: "golang", Package: "svc/store", Kind: symbol.KindPackage},
				{Lang: "golang", Package: "svc/store", Name: "Store", Kind: symbol.KindStruct},
			}
			shuffled := []symbol.Identity{ordered[2], ordered[0], ordered[1]}
			slices.SortFunc(shuffled, symbol.Identity.Compare)

			assert.Equal(t, shuffled, ordered,
				"Compare sorts a slice into the one canonical order")
		})
	})
}

// Parse reads spellings a person typed into a manifest or a
// directive, so it meets bytes nothing generated. It answers on any
// of them, and the grammar is closed under the round trip: an
// identity it read spells back into the same identity.
func FuzzParse(f *testing.F) {
	for _, tt := range parseCases() {
		f.Add(tt.in)
	}

	f.Fuzz(func(t *testing.T, in string) {
		var (
			id       symbol.Identity
			refusal  error
			respelt  symbol.Identity
			spelling string
		)
		assert.NotPanics(t, func() { id, refusal = symbol.Parse(in) },
			"Parse answers on any bytes rather than panicking")
		if refusal != nil {
			assert.Equal(t, id, symbol.Identity{},
				"a refused spelling names nothing")
			return
		}

		assert.NotPanics(t, func() { spelling = id.String() },
			"an identity spells on any parts rather than panicking")
		respelt, refusal = symbol.Parse(spelling)
		assert.NoError(t, refusal, "what Parse read spells back into the grammar")
		assert.Equal(t, respelt, id, "and reads back as the identity it spelled")
	})
}

// An identity is spelled and compared on every ordering the kernel
// makes deterministic: a graph's indexes, a read set, a manifest.
func BenchmarkIdentity(b *testing.B) {
	method := symbol.Identity{
		Lang:    "golang",
		Package: "svc/store",
		Owner:   "Store",
		Name:    "Get",
		Disc:    "ctx,string",
		Kind:    symbol.KindMethod,
	}

	b.Run("String", func(b *testing.B) {
		b.ReportAllocs()

		for b.Loop() {
			_ = method.String()
		}
	})

	b.Run("String of a 33-byte spelling", func(b *testing.B) {
		b.ReportAllocs()

		// Twenty-eight bytes of parts and five separators: one byte
		// past the 32-byte allocation class.
		boundary := method
		boundary.Disc = "ctx,s"
		for b.Loop() {
			_ = boundary.String()
		}
	})

	b.Run("Parse", func(b *testing.B) {
		b.ReportAllocs()

		spelled := method.String()
		for b.Loop() {
			if _, err := symbol.Parse(spelled); err != nil {
				b.Fatalf("Parse(%q): unexpected error: %v", spelled, err)
			}
		}
	})
}
