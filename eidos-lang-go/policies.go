// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package golang

import "go.dokimi.dev/eidos/sdk/plugin"

// GRPC is the lowering policy of the Go target that chooses the gRPC binding
// of the code that a plan writes against a protobuf service. Config selects
// a choice for every plan or for one plan, and the param grpc of the golang
// directive overrides the choice for one declaration.
const GRPC plugin.PolicyKey = "golang.grpc"

// The choices of [GRPC]. Under [GRPCNone] a reference to a protobuf
// declaration spells as eidos translates the declaration. Under [GRPCGo] and
// [GRPCConnect] it spells as protoc-gen-go names the declaration, because
// grpc-go and connect-go both use the messages of protoc-gen-go.
const (
	GRPCNone    plugin.Choice = "none"
	GRPCGo      plugin.Choice = "grpc-go"
	GRPCConnect plugin.Choice = "connect-go"
)

// grpcDoc documents the key of [GRPC] and the param of the golang directive
// that overrides it.
const grpcDoc = "the gRPC binding of the code that a plan writes against a protobuf service"

// Policies returns the spec of the Go policy [GRPC], whose default is
// [GRPCNone]. The backend declares it through the backend kit.
//
// # Allocation contract
//
// Policies allocates the list of specs and the list of choices: two
// allocations.
func Policies() []plugin.PolicySpec {
	return []plugin.PolicySpec{
		{Key: GRPC, Choices: []plugin.Choice{GRPCNone, GRPCGo, GRPCConnect}, Default: GRPCNone, Doc: grpcDoc},
	}
}
