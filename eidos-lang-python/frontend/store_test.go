// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend_test

import (
	"io/fs"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/expect"
	"go.dokimi.dev/assert/files"

	"go.dokimi.dev/eidos/lang/python/frontend"
	"go.dokimi.dev/eidos/lang/python/frontend/typeshed"
)

// The variable that names a virtual environment, the module of its
// site-packages directory that the cases read, the configuration of an
// environment, and a configuration that states the version of Python 3.12.
const (
	envVirtualEnv = "VIRTUAL_ENV"
	siteModule    = "six.py"
	venvConfig    = "pyvenv.cfg"
	version312    = "home = /usr/bin\nversion = 3.12.4\n"
)

// TestStore pins the stores that the load passes to the Python frontend.
func TestStore(t *testing.T) {
	t.Parallel()

	t.Run("Stores", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the typeshed store alone where VIRTUAL_ENV is unset", func(t *testing.T) {
			t.Parallel()

			stores, err := frontend.Stores(func(string) string { return "" })
			assert.NoError(t, err, "a load without an environment reads typeshed alone")
			assert.Length(t, stores, 1, "the stores of the load")
			_, err = fs.Stat(stores[frontend.TypeshedStore], typeshed.VersionsFile)
			assert.NoError(t, err, "the store is the embedded tree of typeshed")
		})

		found := []struct {
			name string
			give files.Tree
		}{
			{
				name: "returns the site-packages directory of the version of pyvenv.cfg",
				give: files.Tree{
					venvConfig: files.Text(version312), "lib/python3.12/site-packages/" + siteModule: files.Text(""),
				},
			},
			{
				name: "returns the site-packages directory of the version_info of pyvenv.cfg",
				give: files.Tree{
					venvConfig: files.Text("version_info = 3.13.0.final.0\n"),
					"lib/python3.13/site-packages/" + siteModule: files.Text(""),
				},
			},
			{
				name: "returns the site-packages directory of an environment of Windows",
				give: files.Tree{venvConfig: files.Text(version312), "Lib/site-packages/" + siteModule: files.Text("")},
			},
		}
		for _, tt := range found {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				env := files.Workspace(t, tt.give)
				stores, err := frontend.Stores(func(key string) string {
					if key == envVirtualEnv {
						return env
					}
					return ""
				})
				assert.NoError(t, err, "the environment resolves")
				_, err = fs.Stat(stores[frontend.VenvStore], siteModule)
				assert.NoError(t, err, "the venv store is the site-packages directory")
			})
		}

		refused := []struct {
			name string
			give files.Tree
			want string
		}{
			{
				name: "returns an error for an environment without pyvenv.cfg",
				give: files.Tree{"bin": files.Dir()},
				want: venvConfig,
			},
			{
				name: "returns an error for a pyvenv.cfg without a version",
				give: files.Tree{venvConfig: files.Text("home = /usr/bin\n")},
				want: "states no version",
			},
			{
				name: "returns an error for a version without a minor version",
				give: files.Tree{venvConfig: files.Text("version = 3\n")},
				want: "states no version",
			},
			{
				name: "returns an error for an environment without a site-packages directory",
				give: files.Tree{venvConfig: files.Text(version312)},
				want: "is a directory",
			},
		}
		for _, tt := range refused {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				env := files.Workspace(t, tt.give)
				_, err := frontend.Stores(func(key string) string {
					if key == envVirtualEnv {
						return env
					}
					return ""
				})
				assert.HasError(t, err, "the environment does not resolve")
				expect.Contains(t, err.Error(), envVirtualEnv, "the error names the variable to set")
				expect.Contains(t, err.Error(), tt.want, "the error states its reason")
			})
		}
	})
}
