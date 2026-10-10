// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend_test

import (
	"io/fs"
	"testing"
	"testing/fstest"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/lang/go/frontend"
)

// The vendor fixture's list: the library its go.mod requires, marked
// explicit, and its one package, and a module the cases replace or
// list beside it.
const (
	vendorList     = "vendor/modules.txt"
	vendorLibLine  = "# " + libModule + " " + libVersion + "\n"
	vendorExplicit = "## explicit; go 1.27\n"
	vendorPackage  = libPackage + "\n"
	otherModule    = "example.com/other"
	forkModule     = "example.com/fork"
	forkVersion    = "v1.0.0"
)

// The vendor tree is the second source of a module, and the go
// command's consistency checks decide whether it may be one, so the
// list's reading and each check are pinned.
func TestVendor(t *testing.T) {
	t.Parallel()

	vendoredLib := [][]string{{"vendor/" + libPackage + "/pkg.go"}}

	t.Run("parseVendor", func(t *testing.T) {
		t.Parallel()

		t.Run("reads the annotations of a module across its lines", func(t *testing.T) {
			t.Parallel()

			tree := vendorTree("", vendorLibLine+"## go 1.27\n## explicit\n"+vendorPackage)
			assert.Equal(t, placed(t, tree, uncached(), libPackage), vendoredLib,
				"the second annotation line marks the module explicit")
		})

		t.Run("reads a replacement of every version by a directory", func(t *testing.T) {
			t.Parallel()

			tree := vendorTree("replace "+otherModule+" => ../other\n",
				vendorLibLine+vendorExplicit+vendorPackage+"# "+otherModule+" => ../other\n")
			assert.Equal(t, placed(t, tree, uncached(), libPackage), vendoredLib,
				"the list records the go.mod's replacement")
		})

		t.Run("reads a replacement of a version by a module", func(t *testing.T) {
			t.Parallel()

			tree := vendorTree("replace "+otherModule+" "+forkVersion+" => "+forkModule+" "+forkVersion+"\n",
				vendorLibLine+vendorExplicit+vendorPackage+
					"# "+otherModule+" "+forkVersion+" => "+forkModule+" "+forkVersion+"\n")
			assert.Equal(t, placed(t, tree, uncached(), libPackage), vendoredLib,
				"the list records the go.mod's replacement")
		})

		t.Run("leaves out a module line of another shape", func(t *testing.T) {
			t.Parallel()

			tree := vendorTree("", "# "+otherModule+" notaversion\n## explicit\n"+otherModule+"\n"+
				vendorLibLine+vendorExplicit+vendorPackage)
			assert.Equal(t, placed(t, tree, uncached(), libPackage), vendoredLib,
				"the go command reads the line as no module, so its packages belong to nothing")
		})

		t.Run("leaves out a module line without a version", func(t *testing.T) {
			t.Parallel()

			tree := vendorTree("", "# "+otherModule+"\n## explicit\n"+otherModule+"\n"+
				vendorLibLine+vendorExplicit+vendorPackage)
			assert.Equal(t, placed(t, tree, uncached(), libPackage), vendoredLib,
				"the go command reads the line as no module, so its packages belong to nothing")
		})

		t.Run("leaves out a replacement of another shape", func(t *testing.T) {
			t.Parallel()

			tree := vendorTree("", vendorLibLine+vendorExplicit+vendorPackage+
				"# "+otherModule+" "+forkVersion+" => a b c\n")
			assert.Equal(t, placed(t, tree, uncached(), libPackage), vendoredLib,
				"the go command reads the arrow's operands as no replacement")
		})
	})

	t.Run("checkVendor", func(t *testing.T) {
		t.Parallel()

		mismatches := []struct {
			name       string
			goModExtra string
			list       string
			want       string
		}{
			{
				name: "returns an error for a requirement the list does not mark explicit",
				list: vendorLibLine + vendorPackage,
				want: "is explicitly required in go.mod, but not marked as explicit",
			},
			{
				name:       "returns an error for a replacement the list does not record",
				goModExtra: "replace " + otherModule + " => " + forkModule + " " + forkVersion + "\n",
				list:       vendorLibLine + vendorExplicit + vendorPackage,
				want:       "is replaced in go.mod, but not marked as replaced",
			},
			{
				name:       "returns an error for a replacement the list records otherwise",
				goModExtra: "replace " + otherModule + " => " + forkModule + " " + forkVersion + "\n",
				list: vendorLibLine + vendorExplicit + vendorPackage +
					"# " + otherModule + " => " + forkModule + " v2.0.0\n",
				want: "is replaced by " + forkModule + "@" + forkVersion + " in go.mod",
			},
			{
				name: "returns an error for an explicit module the go.mod does not require",
				list: vendorLibLine + vendorExplicit + vendorPackage +
					"# " + otherModule + " " + forkVersion + "\n## explicit\n" + otherModule + "\n",
				want: "is marked as explicit in modules.txt, but not explicitly required",
			},
			{
				name: "returns an error for a replacement the go.mod does not state",
				list: vendorLibLine + vendorExplicit + vendorPackage +
					"# " + otherModule + " " + forkVersion + " => " + forkModule + " " + forkVersion + "\n",
				want: "is marked as replaced in modules.txt, but not replaced in go.mod",
			},
		}
		for _, tt := range mismatches {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.Contains(t, refused(t, vendorTree(tt.goModExtra, tt.list), uncached(), libPackage), tt.want,
					"the error names the mismatch the go command reports")
			})
		}

		t.Run("passes a listed replacement of a version the go.mod replaces for every version", func(t *testing.T) {
			t.Parallel()

			tree := vendorTree("replace "+otherModule+" => "+forkModule+" "+forkVersion+"\n",
				vendorLibLine+vendorExplicit+vendorPackage+
					"# "+otherModule+" => "+forkModule+" "+forkVersion+"\n"+
					"# "+otherModule+" "+forkVersion+" => "+forkModule+" "+forkVersion+"\n")
			assert.Equal(t, placed(t, tree, uncached(), libPackage), vendoredLib,
				"the go.mod's replacement of every version covers the listed version")
		})
	})
}

// vendorTree returns the workspace with a vendor tree whose list states
// the lines given and whose copy of the library's package is on disk.
func vendorTree(goModExtra, list string) fstest.MapFS {
	tree := depWorkspace()
	tree[appGoMod] = &fstest.MapFile{Data: append(tree[appGoMod].Data, goModExtra...)}
	tree[vendorList] = &fstest.MapFile{Data: []byte(list)}
	tree["vendor/"+libPackage+"/pkg.go"] = &fstest.MapFile{Data: []byte(libSource)}
	return tree
}

// uncached returns the stores with an empty module cache, so a module
// need falls back to the vendor tree.
func uncached() map[string]fs.FS {
	stores := depStores()
	stores[frontend.ModCacheStore] = fstest.MapFS{}
	return stores
}
