// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package symbol_test

import (
	"slices"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

	"go.dokimi.dev/eidos/core/symbol"
)

// method is the identity the allocation checks and the benchmarks spell
// and parse: a method with a discriminator, every part populated.
var method = symbol.Identity{
	Lang:    "golang",
	Package: "svc/store",
	Owner:   "Store",
	Name:    "Get",
	Disc:    "ctx,string",
	Kind:    symbol.KindMethod,
}

// boundary is method with a shorter discriminator. Its 28 bytes of parts
// and five separators spell 33 bytes, one byte past the 32-byte
// allocation class.
var boundary = symbol.Identity{
	Lang:    "golang",
	Package: "svc/store",
	Owner:   "Store",
	Name:    "Get",
	Disc:    "ctx,s",
	Kind:    symbol.KindMethod,
}

// boundarySize is the length of boundary's spelling.
const boundarySize = 33

// parseCase is one spelling and what [symbol.Parse] makes of it.
type parseCase struct {
	name    string
	in      string
	want    symbol.Identity
	wantErr bool
}

// An identity spells in the canonical grammar, parses back from it, and
// orders on every part that makes it.
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
				name: "spells a package as its language before its path",
				id: symbol.Identity{
					Lang:    "golang",
					Package: "svc/store",
					Kind:    symbol.KindPackage,
				},
				want: "golang:svc/store",
			},
			{
				name: "spells a file under its package path",
				id: symbol.Identity{
					Lang:    "golang",
					Package: "svc/store",
					Name:    "store.go",
					Kind:    symbol.KindFile,
				},
				want: "golang:svc/store/store.go",
			},
			{
				name: "spells a top-level type after a dot",
				id: symbol.Identity{
					Lang:    "golang",
					Package: "svc/store",
					Name:    "Store",
					Kind:    symbol.KindStruct,
				},
				want: "golang:svc/store.Store",
			},
			{
				name: "spells a function's discriminator in parentheses",
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
				name: "spells a member behind its owner's hash mark",
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
				name: "spells a method's discriminator in parentheses",
				id:   method,
				want: "golang:svc/store.Store#Get(ctx,string)",
			},
			{
				name: "spells empty parentheses for a nullary method",
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

	t.Run("IsZero", func(t *testing.T) {
		t.Parallel()

		t.Run("reports true for the zero identity", func(t *testing.T) {
			t.Parallel()

			assert.True(t, (symbol.Identity{}).IsZero(), "the zero Identity names nothing")
		})

		t.Run("reports false for an identity with a language", func(t *testing.T) {
			t.Parallel()

			assert.False(t, (symbol.Identity{Lang: "golang"}).IsZero(), "a populated Identity names something")
		})
	})

	t.Run("PackageIdentity", func(t *testing.T) {
		t.Parallel()

		pkg := symbol.Identity{Lang: "golang", Package: "svc/store", Kind: symbol.KindPackage}
		tests := []struct {
			name string
			give symbol.Identity
		}{
			{name: "returns the package of a member", give: symbol.Identity{
				Lang: "golang", Package: "svc/store", Owner: "Store", Name: "Get",
				Kind: symbol.KindMethod, Disc: "ctx",
			}},
			{name: "returns the package of a file", give: symbol.Identity{
				Lang: "golang", Package: "svc/store", Name: "svc/store/store.go", Kind: symbol.KindFile,
			}},
			{name: "returns a package's own identity", give: pkg},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.Equal(t, tt.give.PackageIdentity(), pkg, "the language and the package path alone")
			})
		}
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

		t.Run("separates two overloads by their discriminators", func(t *testing.T) {
			t.Parallel()

			// Overloads share every part but the discriminator, which
			// is the whole reason it exists: Java, Kotlin, C# and
			// TypeScript all admit two methods under one name.
			overload := symbol.Identity{
				Lang: "java", Package: "svc/store", Owner: "Store", Name: "Get", Kind: symbol.KindMethod,
			}
			byKey, byIndex := overload, overload
			byKey.Disc, byIndex.Disc = "String", "int"
			assert.NotEqual(t, byKey.Compare(byIndex), 0, "the discriminator separates two overloads")
			assert.NotEqual(t, byKey.String(), byIndex.String(), "and their spellings differ with them")
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

// A spelling allocates the string it returns, and nothing else reads
// or compares with an allocation. The check runs alone, because
// AllocsPerRun counts every goroutine's allocations and refuses to run
// beside parallel tests.
func TestIdentityAllocs(t *testing.T) {
	spelled := method.String()
	other := method
	other.Name = "Put"
	assert.MaxAllocs(t, func() {
		if method.String() != spelled {
			t.Fatal("String spelled another identity")
		}
	}, 1, "String allocates the string it returns")
	assert.MaxAllocs(t, func() {
		if len(boundary.String()) != boundarySize {
			t.Fatal("String spelled another length")
		}
	}, 1, "String allocates one string for a spelling past the 32-byte class")
	assert.MaxAllocs(t, func() {
		if _, err := symbol.Parse(spelled); err != nil {
			t.Fatalf("Parse(%q): unexpected error: %v", spelled, err)
		}
	}, 0, "Parse allocates nothing")
	assert.MaxAllocs(t, func() {
		if method.Compare(other) >= 0 {
			t.Fatal("Compare ordered Get after Put")
		}
	}, 0, "Compare allocates nothing")
	assert.MaxAllocs(t, func() {
		if method.PackageIdentity().IsZero() {
			t.Fatal("PackageIdentity returned the zero identity")
		}
	}, 0, "PackageIdentity and IsZero allocate nothing")
}

// Parse reads spellings a person typed into a manifest or a
// directive, so it meets bytes nothing generated. It returns for any
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
			"Parse returns on any bytes without panicking")
		if refusal != nil {
			assert.Equal(t, id, symbol.Identity{},
				"a refused spelling names nothing")
			return
		}

		assert.NotPanics(t, func() { spelling = id.String() },
			"an identity spells on any parts without panicking")
		respelt, refusal = symbol.Parse(spelling)
		assert.NoError(t, refusal, "what Parse read spells back into the grammar")
		assert.Equal(t, respelt, id, "and reads back as the identity it spelled")
	})
}

// BenchmarkIdentity measures the spelling, the parse and the comparison
// of an identity: every ordering the kernel makes deterministic, a
// graph's indexes, a read set and a manifest, compares identities, and
// every record spells them.
func BenchmarkIdentity(b *testing.B) {
	b.Run("String", func(b *testing.B) {
		b.Run("a method", func(b *testing.B) {
			c := bench.Start(b).MaxAllocs(1)
			defer c.End()
			var got string
			for c.Loop() {
				got = method.String()
			}
			assert.Equal(b, got, "golang:svc/store.Store#Get(ctx,string)", "String spells the method")
		})

		b.Run("a 33-byte spelling", func(b *testing.B) {
			c := bench.Start(b).MaxAllocs(1)
			defer c.End()
			var got string
			for c.Loop() {
				got = boundary.String()
			}
			assert.Length(b, got, boundarySize, "the spelling is 33 bytes")
		})
	})

	b.Run("Parse", func(b *testing.B) {
		spelled := method.String()
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		var (
			got symbol.Identity
			err error
		)
		for c.Loop() {
			got, err = symbol.Parse(spelled)
		}
		assert.NoError(b, err, "the spelling parses")
		assert.Equal(b, got, method, "back to the method")
	})

	b.Run("IsZero", func(b *testing.B) {
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		got := true
		for c.Loop() {
			got = method.IsZero()
		}
		assert.False(b, got, "the method names something")
	})

	b.Run("Compare", func(b *testing.B) {
		other := method
		other.Name = "Put"
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		order := 0
		for c.Loop() {
			order = method.Compare(other)
		}
		assert.True(b, order < 0, "Get sorts before Put")
	})

	b.Run("PackageIdentity", func(b *testing.B) {
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		var got symbol.Identity
		for c.Loop() {
			got = method.PackageIdentity()
		}
		assert.Equal(b, got.Package, method.Package, "the package of the method")
	})
}

// parseCases returns the pinned grammar: the spellings Parse accepts
// with the identities it reads, and the spellings it refuses.
//
// [FuzzParse] seeds its corpus from the same table, so the fuzzer
// starts inside the grammar and spends no budget discovering the
// colon.
func parseCases() []parseCase {
	return []parseCase{
		{
			name: "returns a package identity for a bare path",
			in:   "golang:svc/store",
			want: symbol.Identity{
				Lang:    "golang",
				Package: "svc/store",
				Kind:    symbol.KindPackage,
			},
		},
		{
			name: "leaves the kind of a top-level name undetermined",
			in:   "golang:svc/store.Store",
			want: symbol.Identity{Lang: "golang", Package: "svc/store", Name: "Store"},
		},
		{
			// The discriminator search admits an opening paren at the
			// very front, so a spelling that is nothing but a
			// discriminator is refused for its missing name, and is not
			// read as a package.
			name:    "returns an error for a discriminator with nothing before it",
			in:      "golang:(int)",
			wantErr: true,
		},
		{
			// The name search admits a dot at the very front of the
			// last segment, so a name directly after the slash keeps
			// its package, and does not read as one whole path.
			name: "reads a name that opens the segment after the slash",
			in:   "golang:svc/.Name",
			want: symbol.Identity{Lang: "golang", Package: "svc/", Name: "Name"},
		},
		{
			name: "reads a name that opens a spelling without a slash",
			in:   "golang:.Name",
			want: symbol.Identity{Lang: "golang", Name: "Name"},
		},
		{
			name: "returns a function identity for a name with a discriminator",
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
			name: "leaves the kind of a member undetermined",
			in:   "golang:svc/store.Store#timeout",
			want: symbol.Identity{
				Lang:    "golang",
				Package: "svc/store",
				Owner:   "Store",
				Name:    "timeout",
			},
		},
		{
			name: "returns a method identity for a member with a discriminator",
			in:   "golang:svc/store.Store#Get(ctx,string)",
			want: method,
		},
		{
			name: "returns a method identity for a member with empty parentheses",
			in:   "golang:svc/store.Store#Close()",
			want: symbol.Identity{
				Lang:    "golang",
				Package: "svc/store",
				Owner:   "Store",
				Name:    "Close",
				Kind:    symbol.KindMethod,
			},
		},
		{name: "returns an error for empty input", in: "", wantErr: true},
		{name: "returns an error for a spelling without a separator", in: "nolang", wantErr: true},
		{name: "returns an error for an empty path", in: "golang:", wantErr: true},
		{name: "returns an error for an empty language", in: ":svc/store", wantErr: true},
		{
			name:    "returns an error for an unclosed discriminator",
			in:      "golang:svc/store.Open(string",
			wantErr: true,
		},
		{name: "returns an error for an empty owner", in: "golang:svc/store.#name", wantErr: true},
		{
			name:    "returns an error for a discriminator on a bare package",
			in:      "golang:svc/store()",
			wantErr: true,
		},
		{name: "returns an error for an empty name after the dot", in: "golang:svc/store.", wantErr: true},
	}
}
