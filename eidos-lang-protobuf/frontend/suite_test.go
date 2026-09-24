// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend_test

import (
	"os"
	"testing"

	"go.dokimi.dev/assert"

	protobuf "go.dokimi.dev/eidos/lang/protobuf"
	protofrontend "go.dokimi.dev/eidos/lang/protobuf/frontend"
	"go.dokimi.dev/eidos/sdk/directive"
	"go.dokimi.dev/eidos/sdk/frontendtest"
	"go.dokimi.dev/eidos/sdk/plugin"
)

// tableSchema is the directive the fixture schema's carrier writes.
func tableSchema() directive.Schema {
	return directive.Schema{
		Plugin: "gen", Name: "table",
		Params: []directive.ParamSpec{{
			Key: "name", Type: directive.TypeString, Required: true,
			Doc: "the table the message maps to",
		}},
		Doc: "maps a message onto a table",
	}
}

// The satellite meets the frontend conformance bar over its own
// schema tree, which is the same bar every other language's
// frontend meets.
func TestFrontendSuite(t *testing.T) {
	t.Parallel()

	frontendtest.RunFrontendSuite(t, func(assert.TB) (plugin.Frontend, *frontendtest.Fixture) {
		return protofrontend.New(), &frontendtest.Fixture{
			Sources: os.DirFS("testdata/schema"),
			// A schema states no bodies and no unexported names, so
			// a signature-only root loads what a full one does, and
			// the check proves it.
			Signatures: []string{"dep"},
			Schemas:    []directive.Schema{tableSchema()},
			Keys:       protobuf.Keys,
		}
	})
}
