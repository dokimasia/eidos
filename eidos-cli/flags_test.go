// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package cli_test

import (
	"flag"
	"io"
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/cli"
)

// Every command accepts the same six flags.
func TestFlags(t *testing.T) {
	t.Parallel()

	t.Run("Format", func(t *testing.T) {
		t.Parallel()

		t.Run("Valid", func(t *testing.T) {
			t.Parallel()

			tests := []struct {
				name string
				give cli.Format
				want bool
			}{
				{name: "reports true for FormatText", give: cli.FormatText, want: true},
				{name: "reports true for FormatJSON", give: cli.FormatJSON, want: true},
				{name: "reports false for a value above FormatJSON", give: cli.FormatJSON + 1, want: false},
			}
			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					t.Parallel()

					assert.Equal(t, tt.give.Valid(), tt.want, "Valid accepts the two formats")
				})
			}
		})

		t.Run("String", func(t *testing.T) {
			t.Parallel()

			tests := []struct {
				name string
				give cli.Format
				want string
			}{
				{name: "returns text for FormatText", give: cli.FormatText, want: "text"},
				{name: "returns json for FormatJSON", give: cli.FormatJSON, want: "json"},
				{name: "returns the number for a value above FormatJSON", give: 3, want: "Format(3)"},
			}
			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					t.Parallel()

					assert.Equal(t, tt.give.String(), tt.want, "String returns the name of the format")
				})
			}
		})

		t.Run("Set", func(t *testing.T) {
			t.Parallel()

			tests := []struct {
				name string
				give string
				want cli.Format
			}{
				{name: "sets FormatText for text", give: "text", want: cli.FormatText},
				{name: "sets FormatJSON for json", give: "json", want: cli.FormatJSON},
			}
			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					t.Parallel()

					var got cli.Format
					assert.NoError(t, got.Set(tt.give), "the name of a format sets the format")
					assert.Equal(t, got, tt.want, "the format has the name")
				})
			}

			t.Run("returns an error that wraps ErrFormat for a name of no format", func(t *testing.T) {
				t.Parallel()

				var got cli.Format
				assert.ErrorIs(t, got.Set("yaml"), cli.ErrFormat, "yaml is no output format")
			})

			t.Run("returns an error that lists the formats", func(t *testing.T) {
				t.Parallel()

				var got cli.Format
				err := got.Set("yaml")
				assert.HasError(t, err, "yaml is no output format")
				assert.Equal(t, err.Error(), `cli: unknown output format "yaml": use text or json`,
					"the error names the value and the formats")
			})
		})
	})

	t.Run("Register", func(t *testing.T) {
		t.Parallel()

		t.Run("defines the flags that set each field", func(t *testing.T) {
			t.Parallel()

			var got cli.Flags
			fs := flag.NewFlagSet("run", flag.ContinueOnError)
			got.Register(fs)
			err := fs.Parse([]string{
				"--format=json", "--config", "ci.yaml", "--strict", "--verbose", "--quiet", "--no-color",
			})
			assert.NoError(t, err, "the flags parse")
			assert.Equal(t, got, cli.Flags{
				Format: cli.FormatJSON, Config: "ci.yaml", Strict: true, Verbose: true, Quiet: true, NoColor: true,
			}, "each flag sets its field")
		})

		t.Run("leaves each field at its zero value without flags", func(t *testing.T) {
			t.Parallel()

			var got cli.Flags
			fs := flag.NewFlagSet("run", flag.ContinueOnError)
			got.Register(fs)
			assert.NoError(t, fs.Parse(nil), "no flags parse")
			assert.Equal(t, got, cli.Flags{}, "every field is zero")
		})

		t.Run("defines a format flag that returns an error for an unknown format", func(t *testing.T) {
			t.Parallel()

			var got cli.Flags
			fs := flag.NewFlagSet("run", flag.ContinueOnError)
			fs.SetOutput(io.Discard)
			got.Register(fs)
			assert.HasError(t, fs.Parse([]string{"--format=yaml"}), "the flag returns the error of Set")
		})
	})
}
