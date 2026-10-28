// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package protoc_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/expect"
)

func TestRequest(t *testing.T) {
	t.Parallel()

	t.Run("Run", func(t *testing.T) {
		t.Parallel()

		refused := []struct {
			name      string
			giveParam string
		}{
			{name: "returns ErrParameter in the response for an entry without a value", giveParam: "lister.suffix"},
			{name: "returns ErrParameter in the response for an empty key", giveParam: "=Table"},
			{name: "returns ErrParameter in the response for a key that names no plugin", giveParam: "suffix=Table"},
		}
		for _, tt := range refused {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				resp, _ := answered(t, request(t, tt.giveParam), compose)
				expect.That(t, resp.GetError()).
					Contains("the parameter has an entry that names no option and no policy",
						"the response reports the entry")
				expect.Empty(t, resp.GetFile(), "no file is written")
			})
		}

		t.Run("returns the error of the composition for an option that no plugin declares", func(t *testing.T) {
			t.Parallel()

			in := request(t, "lister.missing=1")
			assert.HasError(t, runErr(t, in), "the composition refuses the option")
		})

		t.Run("returns the error of the composition for a policy that no backend declares", func(t *testing.T) {
			t.Parallel()

			assert.HasError(t, runErr(t, request(t, "policy.golang.grpc=none")), "the composition refuses the policy")
		})

		t.Run("decodes a value that is no JSON as a string", func(t *testing.T) {
			t.Parallel()

			resp, _ := answered(t, request(t, "lister.suffix=Sheet"), compose)
			assert.Length(t, resp.GetFile(), 1, "the file is generated")
			assert.That(t, resp.GetFile()[0].GetContent()).Contains("type RowSheet struct{}", "the value is the string")
		})

		t.Run("decodes a value that is JSON", func(t *testing.T) {
			t.Parallel()

			resp, _ := answered(t, request(t, `lister.suffix="Quoted"`), compose)
			assert.Length(t, resp.GetFile(), 1, "the file is generated")
			assert.That(t, resp.GetFile()[0].GetContent()).
				Contains("type RowQuoted struct{}", "the JSON string is decoded")
		})
	})
}
