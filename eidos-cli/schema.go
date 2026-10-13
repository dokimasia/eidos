// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"context"
	"encoding/json"
	"strings"

	gocli "go.stealthscale.io/cli"

	"go.dokimi.dev/eidos/cli/internal/config"
	"go.dokimi.dev/eidos/core/plugin"
)

// The name and the help text of the command schema.
const (
	schemaName = "schema"
	schemaHelp = "Prints the JSON Schema of the config file of the binary."
)

// eventSchema is the event of schema.
const eventSchema = "schema"

// schemaOf is the JSON form of the schema event.
type schemaOf struct {
	// Schema is the schema of the config file. The encoder of the event
	// writes it as compact JSON.
	Schema json.RawMessage `json:"schema"`
}

// schema is the handler of schema, which prints the JSON Schema of the
// config file of the binary.
func (k kernels) schema(ctx context.Context, run *gocli.Run[Global, struct{}]) error {
	return execute(ctx, k.compose, run, nil, func(_ context.Context, x *invocation) int { return x.schema() })
}

// schema renders the schema event and returns [StatusOK]. It decodes no
// workspace document. Text output writes the schema, and JSON output writes
// the event with the schema as compact JSON.
func (x *invocation) schema() int {
	s := Schema(x.compose)
	x.r.Event(eventSchema, &schemaOf{Schema: s}, strings.TrimSuffix(string(s), "\n"))
	return StatusOK
}

// Schema returns the JSON Schema of the config file of the binary that
// compose composes, draft 2020-12, as indented JSON that ends with a
// newline. It is the published schema with one property under options for
// each plugin of the composition that declares options, which
// [plugin.OptionsSchema] returns. Each such property is a closed object
// with one property for each option. The top level of the file admits the
// keys of the options of the command line, which go-cli checks.
//
// Schema reads the plugins through [workspace.Builder.Plugins], so it does
// not build the composition, and a composition with a required option has
// a schema without its config file. It leaves out a plugin whose options
// break the tag contract, and Build reports the fault. The options of a
// document admit any other plugin, because a plan directory adds
// declarative plugins.
func Schema(compose Compose) []byte {
	options := map[string]any{}
	for _, p := range compose().Plugins() {
		if s := plugin.OptionsSchema(p); s != nil {
			options[string(p.Name())] = s
		}
	}
	return config.Schema(options)
}
