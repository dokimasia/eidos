// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package golang_test

import (
	"testing"

	"go.dokimi.dev/assert"

	golang "go.dokimi.dev/eidos/lang/go"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The receiver fixture: the hosts a method receives, and the names
// its signature takes.
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

// receiving returns a method that receives host, with parameters
// of the given names.
func receiving(host string, params ...string) *emit.Method {
	m := &emit.Method{Name: putMethod, Receives: &emit.TypeRef{Spelling: host}}
	for _, name := range params {
		m.Params = append(m.Params, &emit.Param{Name: name, Type: &emit.TypeRef{Spelling: sessionTy}})
	}
	return m
}

// A Go stub declares its methods on a pointer, and the receiver's name
// must not collide with the signature's, so both are contract.
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
				name: "names the receiver recv past a result and a type parameter",
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

				assert.True(t, golang.PointerReceiver(tt.give).Receiver == nil, "no host, no receiver")
			})
		}

		t.Run("returns nil for a nil method", func(t *testing.T) {
			t.Parallel()

			assert.True(t, golang.PointerReceiver(nil) == nil, "nothing to state a receiver on")
		})

		t.Run("returns the method it is given", func(t *testing.T) {
			t.Parallel()

			m := receiving(storeHost)
			assert.True(t, golang.PointerReceiver(m) == m, "the call chains after Mirror")
		})
	})
}
