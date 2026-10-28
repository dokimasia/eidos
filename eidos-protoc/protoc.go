// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package protoc

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/pluginpb"

	"go.dokimi.dev/eidos/core/output"
	"go.dokimi.dev/eidos/core/workspace"
	"go.dokimi.dev/eidos/lang/protobuf/descriptor"
)

// Compose returns the builder of the composition of a plugin binary: the
// brand, the rules, the plans, the generators and the backends. It
// registers no frontend, no output and no ledger. It is the function that
// the binary of a command line passes to cli.New, so both binaries
// register the generators once.
type Compose func() *workspace.Builder

// Main runs the composition that compose returns as a protoc plugin. It
// reads a CodeGeneratorRequest from standard input, writes a
// CodeGeneratorResponse to standard output, and writes every Warning and
// Info of the run to standard error. It returns for a request that it
// answers, also with an error in the response, and exits with status 1
// and the error on standard error for a request that does not decode, for
// a composition that does not build, and for a run that fails for a
// reason other than its findings.
func Main(compose Compose) {
	if err := Run(context.Background(), os.Stdin, os.Stdout, os.Stderr, compose); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// Run runs the composition that compose returns over the request that r
// contains, writes the response to w, and writes every Warning and Info of
// the run to log.
//
// The run is cold. Its tree contains the files of the request to
// generate, its store [descriptor.Store] contains every other file of the
// request, and the parameter of the request is the config of the
// composition. It writes into memory and keeps no ledger. The response
// contains every file of the run, or the Error findings of the run in its
// error field and no file.
//
// Error modes: an error for a request that does not read or decode, for a
// composition that does not build, for a run that fails for a reason other
// than its findings, and for a response that does not encode or write.
func Run(ctx context.Context, r io.Reader, w, log io.Writer, compose Compose) error {
	b, err := io.ReadAll(r)
	if err != nil {
		return fmt.Errorf("protoc: read the request: %w", err)
	}
	req := &pluginpb.CodeGeneratorRequest{}
	if err = proto.Unmarshal(b, req); err != nil {
		return fmt.Errorf("protoc: decode the request: %w", err)
	}
	resp, err := answer(ctx, req, log, compose)
	if err != nil {
		return err
	}
	out, err := proto.Marshal(resp)
	if err != nil {
		return fmt.Errorf("protoc: encode the response: %w", err)
	}
	if _, err := w.Write(out); err != nil {
		return fmt.Errorf("protoc: write the response: %w", err)
	}
	return nil
}

// answer runs the composition over one request and returns its response.
//
// Error modes: the errors of [Run] other than those of the request and the
// response.
func answer(
	ctx context.Context,
	req *pluginpb.CodeGeneratorRequest,
	log io.Writer,
	compose Compose,
) (*pluginpb.CodeGeneratorResponse, error) {
	resp := newResponse()
	builder, refused := configured(compose(), req.GetParameter())
	if refused != "" {
		resp.Error = &refused
		return resp, nil
	}
	in, err := inputOf(req)
	if err != nil {
		return nil, err
	}
	var sinks []*output.Mem
	ws, err := builder.
		Frontends(descriptor.New()).
		Output(func() (output.Sink, error) {
			m := output.NewMem()
			sinks = append(sinks, m)
			return m, nil
		}).
		Build()
	if err != nil {
		return nil, fmt.Errorf("protoc: build the composition: %w", err)
	}
	report, err := ws.Run(ctx, in)
	if err != nil && !errors.Is(err, workspace.ErrRunFailed) {
		return nil, fmt.Errorf("protoc: run the composition: %w", err)
	}
	logFindings(log, report.Sink)
	if failed := errorText(report.Sink); failed != "" {
		resp.Error = new(failed)
		return resp, nil
	}
	resp.File = filesOf(sinks)
	return resp, nil
}
