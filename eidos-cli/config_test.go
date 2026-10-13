// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package cli_test

import (
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/cli"
)

// One function returns the name of the config file of a brand, and every
// command finds the file under that name.
func TestConfig(t *testing.T) {
	t.Parallel()

	t.Run("ConfigName", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the name of the state directory with the extension .yaml", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, cli.ConfigName(brand), confName, "the config file of acme is .acme.yaml")
		})
	})
}
