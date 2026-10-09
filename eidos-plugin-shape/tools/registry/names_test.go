// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package registry_test

import (
	"testing"
	"testing/fstest"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/plugin/shape/tools/specfront"
	"go.dokimi.dev/eidos/sdk/diag"
)

// The generator joins the words of a name into the Go identifiers of a
// spec, and reports the identifiers that two declarations would share.
func TestNames(t *testing.T) {
	t.Parallel()

	t.Run("New", func(t *testing.T) {
		t.Parallel()

		t.Run("joins the words of a name with each initialism in capitals", func(t *testing.T) {
			t.Parallel()

			want := []string{
				"XSSSafe",
				"XSSSafeLimit",
				"XSSSafeParams",
				"XSSSafeOf",
				"xssSafeMixinKey",
				"xssSafeLimitKey",
			}
			assert.Equal(t, keptOf(declaredNames(t, registryOf(t)), want), want,
				"the exported identifiers have XSS in capitals, and the unexported ones open in lowercase")
		})

		t.Run("declares the handle of an id for a contract alone", func(t *testing.T) {
			t.Parallel()

			want := []string{"txProbeIDKey", "probeReaderIDKey", "xssSafeIDKey"}
			assert.Equal(t, keptOf(declaredNames(t, registryOf(t)), want), want[:1],
				"the contract has the handle of its id, and the shape and the mixin have none")
		})

		t.Run("reports a Go identifier that one spec gives twice", func(t *testing.T) {
			t.Parallel()

			_, findings := generated(t, fstest.MapFS{
				writerPath: {
					Data: []byte("name: writer\n" + shapeHead + "params:\n  - {key: of, type: string, doc: a value}\n"),
				},
			})
			expectFindings(t, findings, []diag.Code{specfront.SpecDuplicate},
				[]string{"the spec writer gives the Go identifier WriterOf twice"})
		})

		t.Run("reports two specs of one name at each spec", func(t *testing.T) {
			t.Parallel()

			_, findings := generated(t, fstest.MapFS{
				writerPath: {Data: []byte("name: writer\n" + shapeHead)},
				aliasPath:  {Data: []byte("name: writer\n" + mixinHead)},
			})
			expectFindings(t, findings, []diag.Code{specfront.SpecDuplicate, specfront.SpecDuplicate},
				[]string{"two specs have the name writer", "two specs have the name writer"})
		})

		t.Run("reports two specs that give one Go identifier at each spec", func(t *testing.T) {
			t.Parallel()

			_, findings := generated(t, fstest.MapFS{
				idPath: {Data: []byte("name: id\n" + mixinHead)},
				iDPath: {Data: []byte("name: i-d\n" + mixinHead)},
			})
			codes := make([]diag.Code, 6)
			for i := range codes {
				codes[i] = specfront.SpecDuplicate
			}
			expectFindings(t, findings, codes, []string{
				"the specs i-d and id both give the Go identifier ID",
				"the specs i-d and id both give the Go identifier ID",
				"the specs i-d and id both give the Go identifier IDParams",
				"the specs i-d and id both give the Go identifier IDParams",
				"the specs i-d and id both give the Go identifier IDOf",
				"the specs i-d and id both give the Go identifier IDOf",
			})
		})
	})
}
