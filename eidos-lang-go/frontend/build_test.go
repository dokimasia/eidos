// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend_test

import (
	"io/fs"
	"testing"
	"testing/fstest"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/lang/go/frontend"
	"go.dokimi.dev/eidos/sdk/plugin"
)

// The fixture's second workspace module, which requires a later
// version of the library than the first, and that version's tree.
const (
	svcGoMod    = "svc/go.mod"
	svcGoSum    = "svc/go.sum"
	laterLib    = "v1.3.0"
	laterTree   = libModule + "@" + laterLib
	laterHash   = "h1:later="
	laterRecord = "cache/download/" + libModule + "/@v/" + laterLib + ".ziphash"
)

// The build list decides which version of a module a need reads, so the
// selection across go.mod files and the replacements are pinned.
func TestBuild(t *testing.T) {
	t.Parallel()

	t.Run("readBuild", func(t *testing.T) {
		t.Parallel()

		t.Run("selects the highest version any go.mod requires", func(t *testing.T) {
			t.Parallel()

			tree := depWorkspace()
			tree[svcGoMod] = &fstest.MapFile{Data: []byte("module example.test/svc\n\nrequire " +
				libModule + " " + laterLib + "\n")}
			tree[svcGoSum] = &fstest.MapFile{Data: []byte(libModule + " " + laterLib + " " + laterHash + "\n")}
			stores := depStores()
			cache := depCache()
			cache[laterTree+"/pkg/pkg.go"] = &fstest.MapFile{Data: []byte(libSource)}
			cache[laterRecord] = &fstest.MapFile{Data: []byte(laterHash)}
			stores[frontend.ModCacheStore] = cache
			units, err := runRound(t, tree, stores, libPackage)
			assert.NoError(t, err, "the round places the need")
			assert.Equal(t, units[0][0].Path, cached(laterTree+"/pkg/pkg.go"), "the later version is the build's")
			assert.Equal(t, units[0][0].Shared, []string{svcGoMod}, "the go.mod that requires it selects it")
		})

		t.Run("returns an error for a go.mod that does not parse", func(t *testing.T) {
			t.Parallel()

			tree := depWorkspace()
			tree[appGoMod] = &fstest.MapFile{Data: []byte("module " + appModule + "\n\nrequire (\n")}
			assert.Contains(t, refused(t, tree, depStores(), libPackage), appGoMod, "the error names the go.mod")
		})

		t.Run("returns a go.mod read's own error", func(t *testing.T) {
			t.Parallel()

			dependent, _ := frontend.New(nil).(plugin.Dependent)
			_, err := dependent.Dependencies(t.Context(), &plugin.DependencyRound{
				Number: 1, Needs: []plugin.Need{{Path: libPackage}}, Shared: []string{svcGoMod},
			}, roundReader{storeTree{depWorkspace(), depStores()}})
			assert.ErrorIs(t, err, fs.ErrNotExist, "a go.mod the round lists and cannot read fails it")
		})

		t.Run("returns a go.sum read's own error", func(t *testing.T) {
			t.Parallel()

			tree := depWorkspace()
			tree[appGoSum] = &fstest.MapFile{Mode: fs.ModeDir}
			_, err := runRound(t, tree, depStores(), libPackage)
			assert.HasError(t, err, "a go.sum that exists and does not read fails the round")
		})
	})

	t.Run("selectionOf", func(t *testing.T) {
		t.Parallel()

		const fork, forkVersion, pinned = "example.com/fork", "v1.3.0", "example.com/pinned"
		forked := func(replaces string) fstest.MapFS {
			tree := depWorkspace()
			tree[appGoMod] = &fstest.MapFile{Data: append(tree[appGoMod].Data, replaces...)}
			tree[appGoSum] = &fstest.MapFile{Data: []byte(fork + " " + forkVersion + " " + libHash + "\n" +
				pinned + " " + forkVersion + " " + libHash + "\n" +
				libModule + " " + libVersion + " " + libHash + "\n")}
			return tree
		}
		forkStores := func() map[string]fs.FS {
			stores := depStores()
			stores[frontend.ModCacheStore] = fstest.MapFS{
				fork + "@" + forkVersion + "/pkg/pkg.go":                       {Data: []byte(libSource)},
				"cache/download/" + fork + "/@v/" + forkVersion + ".ziphash":   {Data: []byte(libHash)},
				pinned + "@" + forkVersion + "/pkg/pkg.go":                     {Data: []byte(libSource)},
				"cache/download/" + pinned + "/@v/" + forkVersion + ".ziphash": {Data: []byte(libHash)},
			}
			return stores
		}

		t.Run("applies a replacement of every version", func(t *testing.T) {
			t.Parallel()

			tree := forked("replace " + libModule + " => " + fork + " " + forkVersion + "\n")
			assert.Equal(t, placed(t, tree, forkStores(), libPackage),
				[][]string{{cached(fork + "@" + forkVersion + "/pkg/pkg.go")}}, "the replacement's tree")
		})

		orders := []struct {
			name     string
			replaces string
		}{
			{
				name: "applies a replacement of the required version stated after one of every version",
				replaces: "replace " + libModule + " => " + fork + " " + forkVersion + "\n" +
					"replace " + libModule + " " + libVersion + " => " + pinned + " " + forkVersion + "\n",
			},
			{
				name: "applies a replacement of the required version stated before one of every version",
				replaces: "replace " + libModule + " " + libVersion + " => " + pinned + " " + forkVersion + "\n" +
					"replace " + libModule + " => " + fork + " " + forkVersion + "\n",
			},
		}
		for _, tt := range orders {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.Equal(t, placed(t, forked(tt.replaces), forkStores(), libPackage),
					[][]string{{cached(pinned + "@" + forkVersion + "/pkg/pkg.go")}}, "the version's own replacement")
			})
		}

		t.Run("applies no replacement of another version", func(t *testing.T) {
			t.Parallel()

			tree := forked("replace " + libModule + " v1.0.0 => " + fork + " " + forkVersion + "\n")
			assert.Equal(t, placed(t, tree, depStores(), libPackage),
				[][]string{{cached(libDir + "/pkg.go")}}, "the required version is not replaced")
		})
	})

	t.Run("moduleOf", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the selected module whose path is the longest prefix", func(t *testing.T) {
			t.Parallel()

			const nested, nestedVersion = libModule + "/pkg", "v0.1.0"
			tree := depWorkspace()
			tree[appGoMod] = &fstest.MapFile{Data: append(tree[appGoMod].Data,
				"require "+nested+" "+nestedVersion+"\n"...)}
			tree[appGoSum] = &fstest.MapFile{Data: append(tree[appGoSum].Data,
				nested+" "+nestedVersion+" "+libHash+"\n"...)}
			stores := depStores()
			cache := depCache()
			cache[nested+"@"+nestedVersion+"/pkg.go"] = &fstest.MapFile{Data: []byte(libSource)}
			cache["cache/download/"+nested+"/@v/"+nestedVersion+".ziphash"] = &fstest.MapFile{Data: []byte(libHash)}
			stores[frontend.ModCacheStore] = cache
			assert.Equal(t, placed(t, tree, stores, libPackage),
				[][]string{{cached(nested + "@" + nestedVersion + "/pkg.go")}},
				"the nested module provides the package, as in the go command")
		})
	})
}
