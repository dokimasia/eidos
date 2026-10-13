// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package cli_test

import (
	"errors"
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/cli"
)

// Run exits with the status of a handler, and renders nothing for it.
func TestCommand(t *testing.T) {
	t.Parallel()

	t.Run("ExitError", func(t *testing.T) {
		t.Parallel()

		t.Run("Error", func(t *testing.T) {
			t.Parallel()

			t.Run("returns the empty string", func(t *testing.T) {
				t.Parallel()

				assert.Equal(t, cli.ExitError{Code: cli.StatusUsage}.Error(), "", "the handler rendered its failure")
			})
		})

		t.Run("ExitCode", func(t *testing.T) {
			t.Parallel()

			t.Run("returns the code", func(t *testing.T) {
				t.Parallel()

				assert.Equal(t, cli.ExitError{Code: cli.StatusUsage}.ExitCode(), cli.StatusUsage,
					"a program exits with the code")
			})
		})
	})

	t.Run("UsageError", func(t *testing.T) {
		t.Parallel()

		cause := errors.New("cli_test: unknown flag --x")

		t.Run("Error", func(t *testing.T) {
			t.Parallel()

			t.Run("returns the text of the error that it wraps", func(t *testing.T) {
				t.Parallel()

				assert.Equal(t, (&cli.UsageError{Err: cause}).Error(), cause.Error(),
					"the text is the text of the cause")
			})
		})

		t.Run("Unwrap", func(t *testing.T) {
			t.Parallel()

			t.Run("returns the error that it wraps", func(t *testing.T) {
				t.Parallel()

				assert.Equal(t, (&cli.UsageError{Err: cause}).Unwrap(), cause, "the cause", assert.ByIdentity())
			})
		})
	})

	t.Run("Exit", func(t *testing.T) {
		t.Parallel()

		t.Run("returns nil for StatusOK", func(t *testing.T) {
			t.Parallel()

			assert.NoError(t, cli.Exit(cli.StatusOK), "Run exits with 0 for no error")
		})

		tests := []struct {
			name string
			give int
			want int
		}{
			{name: "returns an ExitError with code 1 for StatusFailed", give: cli.StatusFailed, want: 1},
			{name: "returns an ExitError with code 64 for StatusUsage", give: cli.StatusUsage, want: 64},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				exit := assert.ErrorAs[cli.ExitError](t, cli.Exit(tt.give), "the status is an ExitError")
				assert.Equal(t, exit.Code, tt.want, "the error has the status as its code")
			})
		}
	})
}
