// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package golang_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

	golang "go.dokimi.dev/eidos/lang/go"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The hosts a fixture method receives, and the names its signature
// takes.
const (
	storeHost  = "Store"
	cacheHost  = "Cache"
	boxHost    = "Box"
	umlautHost = "Übung"
	putMethod  = "Put"
	sessionArg = "s"
	sessionTy  = "Session"
	paramT     = "T"
)

// The names and spellings a pointer receiver takes, pinned.
const (
	storePointer = "*Store"
	boxPointer   = "*Box[T]"
)

// receiverAllocs is a pointer receiver named by the host's first
// letter: the receiver, its pointer reference, the reference's
// spelling, its element list and the copy of the received reference.
const receiverAllocs = 5

// A Go stub declares its methods on a pointer, and the receiver's name
// must not collide with a name of the signature. Both are pinned.
func TestReceiver(t *testing.T) {
	t.Parallel()

	t.Run("PointerReceiver", func(t *testing.T) {
		t.Parallel()

		names := []struct {
			name string
			give *emit.Method
			want string
		}{
			{
				name: "names the receiver by the host's first letter",
				give: receiving(cacheHost), want: "c",
			},
			{
				name: "names the receiver by the host's first two letters past a parameter",
				give: receiving(storeHost, sessionArg), want: "st",
			},
			{
				name: "names the receiver recv where its signature takes the short names",
				give: &emit.Method{
					Name:       putMethod,
					Receives:   &emit.TypeRef{Spelling: storeHost},
					TypeParams: []*emit.TypeParam{{Name: "s"}},
					Returns:    []*emit.Return{{Name: "st", Type: &emit.TypeRef{Spelling: sessionTy}}},
				},
				want: "recv",
			},
			{
				name: "numbers the receiver once every short name is taken",
				give: receiving(storeHost, "s", "st", "recv", "recv0"), want: "recv1",
			},
			{
				name: "names the receiver by the host's first character",
				give: receiving(umlautHost), want: "ü",
			},
			{
				name: "names the receiver by the first two characters of a host outside ASCII",
				give: receiving(umlautHost, "ü"), want: "üb",
			},
			{
				name: "names the receiver recv past the one letter of a one-letter host",
				give: receiving("X", "x"), want: "recv",
			},
		}
		for _, tt := range names {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.Equal(t, golang.PointerReceiver(tt.give).Receiver.Name, tt.want,
					"the first free candidate")
			})
		}

		t.Run("returns a receiver pointing at the host", func(t *testing.T) {
			t.Parallel()

			got := golang.PointerReceiver(receiving(storeHost)).Receiver.Type
			assert.Equal(t, got, &emit.TypeRef{
				Spelling: storePointer,
				Form:     symbol.FormOptional,
				Elems:    []*emit.TypeRef{{Spelling: storeHost}},
			}, "the pointer form over the received reference, which the settle respells")
		})

		t.Run("returns a receiver pointing at a generic host with its arguments", func(t *testing.T) {
			t.Parallel()

			m := &emit.Method{
				Name:     putMethod,
				Receives: &emit.TypeRef{Spelling: boxHost, Args: []*emit.TypeRef{{Spelling: paramT}}},
			}
			assert.Equal(t, golang.PointerReceiver(m).Receiver.Type.Spelling, boxPointer,
				"Go restates the host's arguments in the receiver")
		})

		unchanged := []struct {
			name string
			give *emit.Method
		}{
			{name: "returns a method that receives no type unchanged", give: &emit.Method{Name: putMethod}},
			{
				name: "returns a method whose received type spells nothing unchanged",
				give: &emit.Method{Name: putMethod, Receives: &emit.TypeRef{}},
			},
		}
		for _, tt := range unchanged {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.Nil(t, golang.PointerReceiver(tt.give).Receiver, "no host, no receiver")
			})
		}

		t.Run("returns nil for a nil method", func(t *testing.T) {
			t.Parallel()

			assert.Nil(t, golang.PointerReceiver(nil), "nothing to state a receiver on")
		})

		t.Run("returns the method it is given", func(t *testing.T) {
			t.Parallel()

			m := receiving(storeHost)
			assert.Equal(t, golang.PointerReceiver(m), m, "the call chains after Mirror", assert.ByIdentity())
		})
	})
}

// A pointer receiver allocates its nodes, and no name for a host whose
// first letter is ASCII. The ordinary run, which runs no benchmark,
// checks that ceiling here.
func TestReceiverAllocs(t *testing.T) {
	m := receiving(storeHost)
	var got *emit.Method
	assert.MaxAllocs(t, func() { got = golang.PointerReceiver(m) }, receiverAllocs,
		"PointerReceiver allocates the receiver's nodes")
	assert.Equal(t, got.Receiver.Name, "s", "PointerReceiver names the receiver")
}

// BenchmarkReceiver measures the receiver a plugin states once per stub
// method.
func BenchmarkReceiver(b *testing.B) {
	b.Run("PointerReceiver", func(b *testing.B) {
		m := receiving(storeHost)
		c := bench.Start(b).MaxAllocs(receiverAllocs)
		defer c.End()
		var got *emit.Method
		for c.Loop() {
			got = golang.PointerReceiver(m)
		}
		assert.Equal(b, got.Receiver.Type.Spelling, storePointer, "PointerReceiver points at the host")
	})
}

// receiving returns a method that receives host, with parameters
// of the given names.
func receiving(host string, params ...string) *emit.Method {
	m := &emit.Method{Name: putMethod, Receives: &emit.TypeRef{Spelling: host}}
	for _, name := range params {
		m.Params = append(m.Params, &emit.Param{Name: name, Type: &emit.TypeRef{Spelling: sessionTy}})
	}
	return m
}
