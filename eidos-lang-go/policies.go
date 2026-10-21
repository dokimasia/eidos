// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package golang

import "go.dokimi.dev/eidos/sdk/plugin"

// The lowering policies of the Go target. Config selects a choice for every
// plan or for one plan. The param grpc of the golang directive overrides the
// choice of [GRPC] for one declaration. [Width] applies to every file of a
// plan, so the directive has no param for it.
const (
	// GRPC chooses the gRPC binding of the code that a plan writes against
	// a protobuf service.
	GRPC plugin.PolicyKey = "golang.grpc"
	// Width chooses the width in columns at which the backend wraps a line
	// of code, as golines wraps it. A tab counts four columns.
	Width plugin.PolicyKey = "golang.width"
)

// The choices of [GRPC]. Under [GRPCNone] a reference to a protobuf
// declaration spells as eidos translates the declaration. Under [GRPCGo] and
// [GRPCConnect] it spells as protoc-gen-go names the declaration, because
// grpc-go and connect-go both use the messages of protoc-gen-go.
const (
	GRPCNone    plugin.Choice = "none"
	GRPCGo      plugin.Choice = "grpc-go"
	GRPCConnect plugin.Choice = "connect-go"
)

// The choices of [Width]. [Width100] is the default width of golines and of
// golangci-lint. Under [WidthNone] the backend wraps no line.
const (
	Width80   plugin.Choice = "80"
	Width100  plugin.Choice = "100"
	Width120  plugin.Choice = "120"
	WidthNone plugin.Choice = "none"
)

// The docs of the policies. Each doc documents the key of its policy, and
// the doc of [GRPC] documents the param of the golang directive too.
const (
	grpcDoc  = "the gRPC binding of the code that a plan writes against a protobuf service"
	widthDoc = "the width in columns at which the written files wrap a line of code"
)

// Policies returns the specs of the Go policies [GRPC], whose default is
// [GRPCNone], and [Width], whose default is [Width100]. The width is a
// policy of the plan, because the render formats every file of a plan at
// one width. The backend declares the policies through the backend kit.
//
// # Allocation contract
//
// Policies allocates the list of specs and the list of choices of each
// spec: three allocations.
func Policies() []plugin.PolicySpec {
	return []plugin.PolicySpec{
		{Key: GRPC, Choices: []plugin.Choice{GRPCNone, GRPCGo, GRPCConnect}, Default: GRPCNone, Doc: grpcDoc},
		{
			Key:     Width,
			Choices: []plugin.Choice{Width80, Width100, Width120, WidthNone},
			Default: Width100, Doc: widthDoc, Plan: true,
		},
	}
}
