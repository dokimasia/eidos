// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend_test

import (
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/lang/protobuf"
	"go.dokimi.dev/eidos/lang/protobuf/backend"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The names of the rpcs and of the messages that the request cases lower.
const (
	getName      = "get"
	getRequest   = "StoreGetRequest"
	getResponse  = "StoreGetResponse"
	keyParam     = "key"
	limitParam   = "limit"
	requestParam = "request"
)

// TestRequests pins how an rpc takes one request and returns one response:
// the message that it keeps, and the message that the lowering declares
// beside the service.
func TestRequests(t *testing.T) {
	t.Parallel()

	t.Run("Lower", func(t *testing.T) {
		t.Parallel()

		kept := []struct {
			name string
			give *emit.Method
		}{
			{
				name: "keeps an rpc that takes a message and returns a message",
				give: &emit.Method{
					Name:    getName,
					Params:  emit.SlotOf(&emit.Param{Name: requestParam, Type: &emit.TypeRef{Spelling: "GetRequest"}}),
					Returns: emit.SlotOf(&emit.Return{Type: &emit.TypeRef{Spelling: rowType}}),
				},
			},
			{
				name: "keeps an rpc that streams messages in both directions",
				give: &emit.Method{
					Name: "watch",
					Params: emit.SlotOf(&emit.Param{Name: requestParam, Type: &emit.TypeRef{
						Spelling: "stream Query",
						Form:     symbol.FormStream,
						Async:    true,
						Elems:    []*emit.TypeRef{{Spelling: "Query"}},
					}}),
					Returns: emit.SlotOf(&emit.Return{Type: &emit.TypeRef{
						Spelling: "stream Row",
						Form:     symbol.FormStream,
						Async:    true,
						Elems:    []*emit.TypeRef{{Spelling: rowType}},
					}}),
				},
			},
			{
				name: "keeps an rpc of a well-known message",
				give: &emit.Method{
					Name: "ping",
					Params: emit.SlotOf(&emit.Param{Name: requestParam, Type: &emit.TypeRef{
						Spelling: protobuf.TypeEmpty, Package: protobuf.EmptyFile,
					}}),
					Returns: emit.SlotOf(&emit.Return{Type: &emit.TypeRef{
						Spelling: protobuf.TypeEmpty, Package: protobuf.EmptyFile,
					}}),
				},
			},
		}
		for _, tt := range kept {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				out, err := backend.Lower(&emit.Interface{Name: storeName, Methods: emit.SlotOf(tt.give)})
				assert.NoError(t, err, "protobuf writes the rpc")
				assert.Nil(t, out, "the lowering declares no message")
			})
		}

		tests := []struct {
			name        string
			give        *emit.Method
			want        []symbol.Symbol
			wantParams  []*emit.Param
			wantReturns []*emit.Return
		}{
			{
				name: "returns a request of the parameters and a response of the result",
				give: &emit.Method{
					Name: getName,
					Params: emit.SlotOf(
						&emit.Param{
							Name:        keyParam,
							Type:        &emit.TypeRef{Spelling: stringType},
							Doc:         []string{"The key of the row."},
							Comment:     "the row key",
							Annotations: symbol.Annotations{{Name: deprecatedOption, Args: []string{trueValue}}},
						},
						&emit.Param{Name: limitParam, Type: &emit.TypeRef{Spelling: int32Type}, Default: "8"},
					),
					Returns: emit.SlotOf(&emit.Return{Type: &emit.TypeRef{Spelling: stringType}, Comment: "the row"}),
				},
				want: []symbol.Symbol{
					&emit.Struct{Name: getRequest, Fields: emit.SlotOf(
						&emit.Field{
							Name:        keyParam,
							Type:        &emit.TypeRef{Spelling: stringType},
							Doc:         []string{"The key of the row."},
							Comment:     "the row key",
							Annotations: symbol.Annotations{{Name: deprecatedOption, Args: []string{trueValue}}},
							Tags:        tag1,
						},
						&emit.Field{Name: limitParam, Type: &emit.TypeRef{Spelling: int32Type}, Value: "8", Tags: tag2},
					)},
					&emit.Struct{Name: getResponse, Fields: emit.SlotOf(
						&emit.Field{
							Name:    "result",
							Type:    &emit.TypeRef{Spelling: stringType},
							Comment: "the row",
							Tags:    tag1,
						},
					)},
				},
				wantParams:  []*emit.Param{{Name: requestParam, Type: &emit.TypeRef{Spelling: getRequest}}},
				wantReturns: []*emit.Return{{Type: &emit.TypeRef{Spelling: getResponse}}},
			},
			{
				name: "returns an empty request for an rpc without a parameter",
				give: &emit.Method{
					Name: getName, Returns: emit.SlotOf(&emit.Return{Type: &emit.TypeRef{Spelling: rowType}}),
				},
				want:        []symbol.Symbol{&emit.Struct{Name: getRequest}},
				wantParams:  []*emit.Param{{Name: requestParam, Type: &emit.TypeRef{Spelling: getRequest}}},
				wantReturns: []*emit.Return{{Type: &emit.TypeRef{Spelling: rowType}}},
			},
			{
				name: "returns an empty response for an rpc without a result",
				give: &emit.Method{
					Name:   getName,
					Params: emit.SlotOf(&emit.Param{Name: requestParam, Type: &emit.TypeRef{Spelling: "Query"}}),
				},
				want:        []symbol.Symbol{&emit.Struct{Name: getResponse}},
				wantParams:  []*emit.Param{{Name: requestParam, Type: &emit.TypeRef{Spelling: "Query"}}},
				wantReturns: []*emit.Return{{Type: &emit.TypeRef{Spelling: getResponse}}},
			},
			{
				name: "returns the fields of a variadic, a keyword and an optional parameter",
				give: &emit.Method{
					Name: getName,
					Params: emit.SlotOf(
						&emit.Param{Name: keyParam, Type: &emit.TypeRef{Spelling: stringType}, Optional: true},
						&emit.Param{
							Name:     "rest",
							Type:     &emit.TypeRef{Spelling: int32Type},
							Variadic: symbol.VariadicPositional,
						},
						&emit.Param{
							Name:     "named",
							Type:     &emit.TypeRef{Spelling: int32Type},
							Variadic: symbol.VariadicKeyword,
						},
					),
					Returns: emit.SlotOf(&emit.Return{Type: &emit.TypeRef{Spelling: rowType}}),
				},
				want: []symbol.Symbol{&emit.Struct{Name: getRequest, Fields: emit.SlotOf(
					&emit.Field{Name: keyParam, Tags: tag1, Type: &emit.TypeRef{
						Spelling: "optional string",
						Form:     symbol.FormOptional,
						Elems:    []*emit.TypeRef{{Spelling: stringType}},
					}},
					&emit.Field{Name: "rest", Tags: tag2, Type: &emit.TypeRef{
						Spelling: "repeated int32",
						Form:     symbol.FormList,
						Elems:    []*emit.TypeRef{{Spelling: int32Type}},
					}},
					&emit.Field{Name: "named", Tags: tag3, Type: &emit.TypeRef{
						Spelling: "map<string, int32>", Form: symbol.FormMap,
						Elems: []*emit.TypeRef{{Spelling: stringType}, {Spelling: int32Type}},
					}},
				)}},
				wantParams:  []*emit.Param{{Name: requestParam, Type: &emit.TypeRef{Spelling: getRequest}}},
				wantReturns: []*emit.Return{{Type: &emit.TypeRef{Spelling: rowType}}},
			},
			{
				name: "returns the fields of unnamed parameters and results by their positions",
				give: &emit.Method{
					Name: getName,
					Params: emit.SlotOf(
						&emit.Param{
							Type: &emit.TypeRef{Spelling: stringType},
						},
						&emit.Param{Type: &emit.TypeRef{Spelling: int32Type}},
					),
					Returns: emit.SlotOf(
						&emit.Return{
							Type: &emit.TypeRef{Spelling: rowType},
						},
						&emit.Return{Name: "err", Type: &emit.TypeRef{Spelling: "Status"}},
					),
				},
				want: []symbol.Symbol{
					&emit.Struct{Name: getRequest, Fields: emit.SlotOf(
						&emit.Field{Name: "param_1", Type: &emit.TypeRef{Spelling: stringType}, Tags: tag1},
						&emit.Field{Name: "param_2", Type: &emit.TypeRef{Spelling: int32Type}, Tags: tag2},
					)},
					&emit.Struct{Name: getResponse, Fields: emit.SlotOf(
						&emit.Field{Name: "result_1", Type: &emit.TypeRef{Spelling: rowType}, Tags: tag1},
						&emit.Field{Name: "err", Type: &emit.TypeRef{Spelling: "Status"}, Tags: tag2},
					)},
				},
				wantParams:  []*emit.Param{{Name: requestParam, Type: &emit.TypeRef{Spelling: getRequest}}},
				wantReturns: []*emit.Return{{Type: &emit.TypeRef{Spelling: getResponse}}},
			},
			{
				name: "returns the field of one unnamed parameter without a position",
				give: &emit.Method{
					Name:    getName,
					Params:  emit.SlotOf(&emit.Param{Type: &emit.TypeRef{Spelling: stringType}}),
					Returns: emit.SlotOf(&emit.Return{Type: &emit.TypeRef{Spelling: rowType}}),
				},
				want: []symbol.Symbol{&emit.Struct{Name: getRequest, Fields: emit.SlotOf(
					&emit.Field{Name: "param", Type: &emit.TypeRef{Spelling: stringType}, Tags: tag1},
				)}},
				wantParams:  []*emit.Param{{Name: requestParam, Type: &emit.TypeRef{Spelling: getRequest}}},
				wantReturns: []*emit.Return{{Type: &emit.TypeRef{Spelling: rowType}}},
			},
			{
				name: "returns a request for one parameter of an enum",
				give: &emit.Method{
					Name: getName,
					Params: emit.SlotOf(&emit.Param{Name: "kind", Type: &emit.TypeRef{
						Spelling: "Kind",
						Target:   symbol.Identity{Lang: protobuf.Lang, Name: "Kind", Kind: symbol.KindEnum},
					}}),
					Returns: emit.SlotOf(&emit.Return{Type: &emit.TypeRef{Spelling: "google.protobuf.NullValue"}}),
				},
				want: []symbol.Symbol{
					&emit.Struct{
						Name: getRequest,
						Fields: emit.SlotOf(&emit.Field{Name: "kind", Tags: tag1, Type: &emit.TypeRef{
							Spelling: "Kind",
							Target:   symbol.Identity{Lang: protobuf.Lang, Name: "Kind", Kind: symbol.KindEnum},
						}}),
					},
					&emit.Struct{Name: getResponse, Fields: emit.SlotOf(
						&emit.Field{
							Name: "result",
							Type: &emit.TypeRef{Spelling: "google.protobuf.NullValue"},
							Tags: tag1,
						},
					)},
				},
				wantParams:  []*emit.Param{{Name: requestParam, Type: &emit.TypeRef{Spelling: getRequest}}},
				wantReturns: []*emit.Return{{Type: &emit.TypeRef{Spelling: getResponse}}},
			},
			{
				name: "returns a stream of a request and of a response for streams of scalars",
				give: &emit.Method{
					Name: getName,
					Params: emit.SlotOf(&emit.Param{Name: keyParam, Type: &emit.TypeRef{
						Spelling: "stream string",
						Form:     symbol.FormStream,
						Async:    true,
						Elems:    []*emit.TypeRef{{Spelling: stringType}},
					}}),
					Returns: emit.SlotOf(&emit.Return{Type: &emit.TypeRef{
						Spelling: "stream int32",
						Form:     symbol.FormStream,
						Async:    true,
						Elems:    []*emit.TypeRef{{Spelling: int32Type}},
					}}),
				},
				want: []symbol.Symbol{
					&emit.Struct{Name: getRequest, Fields: emit.SlotOf(
						&emit.Field{Name: "value", Type: &emit.TypeRef{Spelling: stringType}, Tags: tag1},
					)},
					&emit.Struct{Name: getResponse, Fields: emit.SlotOf(
						&emit.Field{Name: "value", Type: &emit.TypeRef{Spelling: int32Type}, Tags: tag1},
					)},
				},
				wantParams: []*emit.Param{{Name: keyParam, Type: &emit.TypeRef{
					Spelling: "stream " + getRequest, Form: symbol.FormStream, Async: true,
					Elems: []*emit.TypeRef{{Spelling: getRequest}},
				}}},
				wantReturns: []*emit.Return{{Type: &emit.TypeRef{
					Spelling: "stream " + getResponse, Form: symbol.FormStream, Async: true,
					Elems: []*emit.TypeRef{{Spelling: getResponse}},
				}}},
			},
			{
				name: "returns the wrapper of a stream of lists of lists inside its request",
				give: &emit.Method{
					Name: getName,
					Params: emit.SlotOf(&emit.Param{Name: keyParam, Type: &emit.TypeRef{
						Form:  symbol.FormStream,
						Async: true,
						Elems: []*emit.TypeRef{{Form: symbol.FormList, Elems: []*emit.TypeRef{
							{
								Spelling: "repeated string",
								Form:     symbol.FormList,
								Elems:    []*emit.TypeRef{{Spelling: stringType}},
							},
						}}},
					}}),
					Returns: emit.SlotOf(&emit.Return{Type: &emit.TypeRef{Spelling: rowType}}),
				},
				want: []symbol.Symbol{&emit.Struct{
					Name: getRequest,
					Fields: emit.SlotOf(&emit.Field{Name: "value", Tags: tag1, Type: &emit.TypeRef{
						Spelling: "repeated ValueItem",
						Form:     symbol.FormList,
						Elems:    []*emit.TypeRef{{Spelling: "ValueItem"}},
					}}),
					Types: emit.SlotOf[symbol.Symbol](&emit.Struct{Name: "ValueItem", Fields: emit.SlotOf(&emit.Field{
						Name: "values",
						Tags: tag1,
						Type: &emit.TypeRef{
							Spelling: "repeated string",
							Form:     symbol.FormList,
							Elems:    []*emit.TypeRef{{Spelling: stringType}},
						},
					})}),
				}},
				wantParams: []*emit.Param{{Name: keyParam, Type: &emit.TypeRef{
					Spelling: "stream " + getRequest, Form: symbol.FormStream, Async: true,
					Elems: []*emit.TypeRef{{Spelling: getRequest}},
				}}},
				wantReturns: []*emit.Return{{Type: &emit.TypeRef{Spelling: rowType}}},
			},
			{
				name: "returns the field of a stream without a child without a type",
				give: &emit.Method{
					Name: getName,
					Params: emit.SlotOf(
						&emit.Param{Name: keyParam, Type: &emit.TypeRef{Spelling: "stream", Form: symbol.FormStream}},
					),
					Returns: emit.SlotOf(&emit.Return{Type: &emit.TypeRef{Spelling: rowType}}),
				},
				want: []symbol.Symbol{&emit.Struct{Name: getRequest, Fields: emit.SlotOf(
					&emit.Field{Name: "value", Tags: tag1},
				)}},
				wantParams: []*emit.Param{{Name: keyParam, Type: &emit.TypeRef{
					Spelling: "stream " + getRequest,
					Form:     symbol.FormStream,
					Elems:    []*emit.TypeRef{{Spelling: getRequest}},
				}}},
				wantReturns: []*emit.Return{{Type: &emit.TypeRef{Spelling: rowType}}},
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				origin := symbol.Identity{
					Lang:    "golang",
					Package: "example.com/store",
					Name:    storeName,
					Kind:    symbol.KindInterface,
				}
				service := &emit.Interface{Origin: origin, Name: storeName, Methods: emit.SlotOf(tt.give)}
				out, err := backend.Lower(service)
				assert.NoError(t, err, "protobuf writes the rpc")
				for _, msg := range tt.want {
					msg.(*emit.Struct).Origin = origin
				}
				assert.Equal(t, out, append([]symbol.Symbol{service}, tt.want...),
					"the service and the messages that the lowering declares beside it", assert.EquateEmpty())
				assert.Equal(t, tt.give.Params.Items(), tt.wantParams, "the one parameter of the rpc")
				assert.Equal(t, tt.give.Returns.Items(), tt.wantReturns, "the one result of the rpc")
			})
		}

		refusals := []struct {
			name string
			give *emit.Interface
			want string
		}{
			{
				name: "returns an error for a service with fields",
				give: &emit.Interface{Name: storeName, Fields: emit.SlotOf(&emit.Field{Name: idField})},
				want: "protobuf: a service declares rpcs alone, and store declares fields",
			},
			{
				name: "returns an error for two rpcs of one name",
				give: &emit.Interface{
					Name:    storeName,
					Methods: emit.SlotOf(&emit.Method{Name: getName}, &emit.Method{Name: getName}),
				},
				want: "protobuf: a service declares each rpc once, and store declares get twice",
			},
			{
				name: "returns an error for a stream beside another parameter",
				give: &emit.Interface{Name: storeName, Methods: emit.SlotOf(&emit.Method{
					Name: getName,
					Params: emit.SlotOf(
						&emit.Param{Name: keyParam, Type: &emit.TypeRef{Spelling: stringType}},
						&emit.Param{Name: "rows", Type: &emit.TypeRef{
							Spelling: "stream Row",
							Form:     symbol.FormStream,
							Elems:    []*emit.TypeRef{{Spelling: rowType}},
						}},
					),
				})},
				want: "protobuf: a stream is the whole request of an rpc, and get streams rows beside another parameter",
			},
			{
				name: "returns an error for a stream beside another result",
				give: &emit.Interface{Name: storeName, Methods: emit.SlotOf(&emit.Method{
					Name:   getName,
					Params: emit.SlotOf(&emit.Param{Name: requestParam, Type: &emit.TypeRef{Spelling: "Query"}}),
					Returns: emit.SlotOf(
						&emit.Return{Type: &emit.TypeRef{Spelling: stringType}},
						&emit.Return{Type: &emit.TypeRef{Spelling: "stream Row", Form: symbol.FormStream}},
					),
				})},
				want: "protobuf: a stream is the whole response of an rpc, and get streams beside another result",
			},
			{
				name: "returns an error for a parameter with a label",
				give: &emit.Interface{Name: storeName, Methods: emit.SlotOf(&emit.Method{
					Name: getName,
					Params: emit.SlotOf(&emit.Param{
						Name: keyParam, Label: "by", Type: &emit.TypeRef{Spelling: stringType},
					}),
				})},
				want: "protobuf: a field of a request has one name, and the parameter key of get states the label by",
			},
			{
				name: "returns an error for two parameters of one name",
				give: &emit.Interface{Name: storeName, Methods: emit.SlotOf(&emit.Method{
					Name: getName,
					Params: emit.SlotOf(
						&emit.Param{Name: keyParam, Type: &emit.TypeRef{Spelling: stringType}},
						&emit.Param{Name: keyParam, Type: &emit.TypeRef{Spelling: int32Type}},
					),
				})},
				want: "protobuf: StoreGetRequest declares two members named key",
			},
			{
				name: "returns an error for two results of one name",
				give: &emit.Interface{Name: storeName, Methods: emit.SlotOf(&emit.Method{
					Name:   getName,
					Params: emit.SlotOf(&emit.Param{Name: requestParam, Type: &emit.TypeRef{Spelling: "Query"}}),
					Returns: emit.SlotOf(
						&emit.Return{Name: "row", Type: &emit.TypeRef{Spelling: rowType}},
						&emit.Return{Name: "row", Type: &emit.TypeRef{Spelling: rowType}},
					),
				})},
				want: "protobuf: StoreGetResponse declares two members named row",
			},
		}
		for _, tt := range refusals {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				out, err := backend.Lower(tt.give)
				assert.HasError(t, err, "protobuf has no form of the rpc")
				assert.Equal(t, err.Error(), tt.want, "the error states the rpc")
				assert.Nil(t, out, "a refusal returns no declaration")
			})
		}
	})
}
