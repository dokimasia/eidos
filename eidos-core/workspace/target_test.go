// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package workspace_test

import (
	"strconv"
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/frontend/frontendtest"
	"go.dokimi.dev/eidos/core/internal/coretest"
	"go.dokimi.dev/eidos/core/position"
	"go.dokimi.dev/eidos/core/symbol"
	"go.dokimi.dev/eidos/core/workspace"
)

// The spellings that the target cases parse: the name of a file that
// contains a colon, the declaration that an identity names, and a key that
// no composition registers.
const (
	colonedFile  = "c:row.zz"
	targetedDecl = "svc/store.Row"
	ghostKey     = "shape.ghost"
)

// An argument of explain is one of four forms, told apart by its shape.
func TestTarget(t *testing.T) {
	t.Parallel()

	t.Run("ParseTarget", func(t *testing.T) {
		t.Parallel()

		w := built(t, valid().Frontends(frontendtest.NewScripted()).Rules(native{}).Keys(contracted))
		code := workspace.UnmetContract.String()
		line := ":" + strconv.Itoa(rowAt.Line)

		tests := []struct {
			name string
			give string
			want workspace.Target
		}{
			{
				name: "returns a code at a line for a code before the @",
				give: code + "@" + sealedSource + line,
				want: workspace.Target{Code: workspace.UnmetContract, At: rowAt},
			},
			{
				name: "returns a code at a column for a position with a column",
				give: code + "@" + sealedSource + line + ":5",
				want: workspace.Target{
					Code: workspace.UnmetContract,
					At:   position.Pos{File: sealedSource, Line: rowAt.Line, Col: 5},
				},
			},
			{
				name: "returns a key at a line for a registered key before the @",
				give: string(contractKey) + "@" + sealedSource + line,
				want: workspace.Target{Key: contractKey, At: rowAt},
			},
			{
				name: "returns the whole name of a file that contains a colon",
				give: code + "@" + colonedFile + line,
				want: workspace.Target{
					Code: workspace.UnmetContract,
					At:   position.Pos{File: colonedFile, Line: rowAt.Line},
				},
			},
			{
				name: "returns a path for an identity under a language that the composition does not register",
				give: "rust:" + targetedDecl,
				want: workspace.Target{Path: "rust:" + targetedDecl},
			},
			{
				name: "returns a path for a file",
				give: sealedSource,
				want: workspace.Target{Path: sealedSource},
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				got, err := w.ParseTarget(tt.give)
				assert.NoError(t, err, "the argument parses")
				assert.Equal(t, got, tt.want, "the target has the argument's form")
			})
		}

		identities := []struct {
			name string
			give string
		}{
			{
				name: "returns an identity under the language of a frontend",
				give: string(frontendtest.ScriptedLang) + ":" + targetedDecl,
			},
			{
				name: "returns an identity under the language of a rules value",
				give: string(coretest.Lang) + ":" + targetedDecl,
			},
		}
		for _, tt := range identities {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				id, err := symbol.Parse(tt.give)
				assert.NoError(t, err, "the argument is an identity's spelling")
				got, err := w.ParseTarget(tt.give)
				assert.NoError(t, err, "the argument parses")
				assert.Equal(t, got, workspace.Target{Identity: id}, "the target is the parsed identity")
			})
		}

		refused := []struct {
			name   string
			before string
			after  string
		}{
			{name: "returns an error naming a position without a line", before: code, after: sealedSource},
			{name: "returns an error naming a position without a file", before: code, after: line},
			{name: "returns an error naming a line below one", before: code, after: sealedSource + ":0"},
			{name: "returns an error naming a column below one", before: code, after: sealedSource + line + ":0"},
			{
				name:   "returns an error naming a line below one before a column",
				before: code,
				after:  sealedSource + ":0:5",
			},
		}
		for _, tt := range refused {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				_, err := w.ParseTarget(tt.before + "@" + tt.after)
				assert.HasError(t, err, "the argument is no target")
				assert.Contains(t, err.Error(), strconv.Quote(tt.after), "the error names the position")
			})
		}

		t.Run("returns an error naming a key that the composition does not register", func(t *testing.T) {
			t.Parallel()

			_, err := w.ParseTarget(ghostKey + "@" + sealedSource + line)
			assert.HasError(t, err, "the argument is no target")
			assert.Contains(t, err.Error(), strconv.Quote(ghostKey), "the error names the key")
		})
	})
}
