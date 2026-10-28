// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package protoc

import (
	"fmt"
	"io"
	"maps"
	"slices"
	"strings"

	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/pluginpb"

	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/output"
	"go.dokimi.dev/eidos/core/position"
)

// The editions that a plugin of the host supports: from proto2 to the
// newest edition that the protobuf satellite loads.
const (
	minimumEdition = descriptorpb.Edition_EDITION_PROTO2
	maximumEdition = descriptorpb.Edition_EDITION_2026
)

// newResponse returns a response without a file, with the features of the
// host: the optional label of proto3 and the editions from
// [minimumEdition] to [maximumEdition].
func newResponse() *pluginpb.CodeGeneratorResponse {
	features := uint64(pluginpb.CodeGeneratorResponse_FEATURE_PROTO3_OPTIONAL) |
		uint64(pluginpb.CodeGeneratorResponse_FEATURE_SUPPORTS_EDITIONS)
	return &pluginpb.CodeGeneratorResponse{
		SupportedFeatures: new(features),
		MinimumEdition:    new(int32(minimumEdition)),
		MaximumEdition:    new(int32(maximumEdition)),
	}
}

// filesOf returns the files that the sinks of a run committed, in path
// order, as the files of a response.
func filesOf(sinks []*output.Mem) []*pluginpb.CodeGeneratorResponse_File {
	all := map[string][]byte{}
	for _, m := range sinks {
		maps.Copy(all, m.Files())
	}
	out := make([]*pluginpb.CodeGeneratorResponse_File, 0, len(all))
	for _, path := range slices.Sorted(maps.Keys(all)) {
		out = append(out, &pluginpb.CodeGeneratorResponse_File{
			Name:    new(path),
			Content: new(string(all[path])),
		})
	}
	return out
}

// errorText returns the Error findings of a sink, one line each in the form
// of eidos run, and the empty string for a sink without one.
func errorText(sink *diag.Sink) string {
	var lines []string
	for d := range sink.All() {
		if d.Severity == diag.SeverityError {
			lines = append(lines, lineOf(d))
		}
	}
	return strings.Join(lines, "\n")
}

// logFindings writes every Warning and Info of a sink to log, one line
// each in the form of eidos run.
func logFindings(log io.Writer, sink *diag.Sink) {
	for d := range sink.All() {
		if d.Severity != diag.SeverityError {
			fmt.Fprintln(log, lineOf(d))
		}
	}
}

// lineOf spells one finding as eidos run does: the position, the
// severity, the code, the message and the origin.
func lineOf(d diag.Diag) string {
	return fmt.Sprintf("%s: %s %s: %s (%s)", place(d.Pos), d.Severity, d.Code, d.Msg, d.Origin)
}

// place spells a position as file:line:column, without the parts that are
// zero.
func place(p position.Pos) string {
	switch {
	case p.Line == 0:
		return p.File
	case p.Col == 0:
		return fmt.Sprintf("%s:%d", p.File, p.Line)
	default:
		return fmt.Sprintf("%s:%d:%d", p.File, p.Line, p.Col)
	}
}
