// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package protoc_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/expect"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"
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
			{
				name:      "returns ErrParameter in the response for a field that no plan has",
				giveParam: "plan.lists.colour=red",
			},
			{name: "returns ErrParameter in the response for a plan field without a plan", giveParam: "plan.dir=out"},
			{
				name:      "returns ErrParameter in the response for a placement that the layout does not know",
				giveParam: "plan.lists.policy=sideways",
			},
			{
				name:      "returns ErrParameter in the response for a disabled that is no bool",
				giveParam: "plan.lists.disabled=maybe",
			},
		}
		for _, tt := range refused {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				resp, _ := answered(t, request(t, tt.giveParam), compose)
				expect.That(t, resp.GetError()).
					Contains("the parameter has an entry that names no option, no policy and no field of a plan",
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

		commas := []struct {
			name      string
			giveParam string
			want      string
		}{
			{
				name:      "keeps a comma inside a JSON string in its value",
				giveParam: `lister.suffix="Is,Here"`,
				want:      "type RowIs,Here struct{}",
			},
			{
				name:      "keeps a comma after an escaped quote inside a JSON string",
				giveParam: `lister.suffix="A\",B"`,
				want:      `type RowA",B struct{}`,
			},
			{
				name:      "splits at a comma after a JSON string",
				giveParam: `lister.suffix="One,Two",lister.suffix="Last"`,
				want:      "type RowLast struct{}",
			},
		}
		for _, tt := range commas {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				resp, _ := answered(t, request(t, tt.giveParam), compose)
				assert.Equal(t, resp.GetError(), "", "the parameter is accepted")
				assert.Length(t, resp.GetFile(), 1, "the file is generated")
				assert.That(t, resp.GetFile()[0].GetContent()).Contains(tt.want, "the value keeps its commas")
			})
		}

		t.Run("keeps the commas of a JSON list in its value", func(t *testing.T) {
			t.Parallel()

			err := runErr(t, request(t, `lister.suffix=[1,{"a":2}]`))
			assert.HasError(t, err, "the list is no string")
			expect.That(t, err.Error()).Contains("cannot unmarshal array", "the whole list reaches the option")
		})

		t.Run("takes a file to generate from source_file_descriptors", func(t *testing.T) {
			t.Parallel()

			req := request(t, "")
			for _, fd := range req.GetProtoFile() {
				if fd.GetName() == rowsFile {
					source := proto.CloneOf(fd)
					source.MessageType[0].Name = new("Sourced")
					req.SourceFileDescriptors = []*descriptorpb.FileDescriptorProto{source}
				}
			}
			resp, _ := answered(t, req, compose)
			assert.Length(t, resp.GetFile(), 1, "the file is generated")
			assert.That(t, resp.GetFile()[0].GetContent()).
				Contains("type SourcedList struct{}", "the descriptor with the source options is read")
		})

		t.Run("leaves out a plan that the parameter disables", func(t *testing.T) {
			t.Parallel()

			resp, _ := answered(t, request(t, "plan.lists.disabled=true"), compose)
			assert.Equal(t, resp.GetError(), "", "the run succeeds")
			assert.Empty(t, resp.GetFile(), "the disabled plan writes nothing")
		})

		t.Run("places the files of a plan under the directory and the placement of the parameter", func(t *testing.T) {
			t.Parallel()

			resp, _ := answered(t, request(t, "plan.lists.policy=centralised,plan.lists.dir=out"), compose)
			assert.Length(t, resp.GetFile(), 1, "the file is generated")
			expect.Equal(t, resp.GetFile()[0].GetName(), "out/"+rowsList, "the file is under the directory")
		})

		t.Run("returns the error of the composition for a plan that it does not declare", func(t *testing.T) {
			t.Parallel()

			assert.HasError(t, runErr(t, request(t, "plan.missing.dir=out")), "the composition refuses the plan")
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
