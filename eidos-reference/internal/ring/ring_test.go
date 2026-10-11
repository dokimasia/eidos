// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"context"
	"fmt"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/expect"
	"go.dokimi.dev/assert/files"
	"golang.org/x/mod/module"
	modzip "golang.org/x/mod/zip"
)

// The modules of the checkouts of the tests beside the kernel and the SDK
// facade, by the name of their directory: a release whose test passes
// against the checkout, a release whose test fails against it, a module
// without a release, a module that the proxy does not serve, and a module
// whose queries the proxy fails. Each module path is the name after the
// prefix of the modules of eidos.
const (
	modulePrefix = "go.dokimi.dev/eidos/"
	passingName  = "passing"
	failingName  = "failing"
	pseudoName   = "pseudo"
	missingName  = "missing"
	brokenName   = "broken"
)

// The directories of the kernel, of the SDK facade and of a module that
// does not require the kernel in a checkout, the path of that module, the
// package of the kernel that the tests of the releases import, and the file
// of that package in a checkout.
const (
	kernelDir       = "core"
	facadeDir       = "sdk"
	unrelatedDir    = "unrelated"
	unrelatedModule = "example.com/unrelated"
	versionPackage  = kernelModule + "/version"
	versionFile     = kernelDir + "/version/version.go"
)

// The release of the published modules, the pseudo-version that the proxy
// returns for the module without a release, and the time of both.
const (
	release       = "v0.1.0"
	pseudoVersion = "v0.0.0-20261010000000-abcdefabcdef"
	versionTime   = "2026-10-10T00:00:00Z"
)

// The major version of the kernel of a checkout, and the major version of
// the kernel that the failing release was published with.
const (
	checkoutMajor  = 2
	publishedMajor = 1
)

// The files of a module in the module proxy protocol, relative to the
// directory of the module.
const (
	listFile   = "@v/list"
	latestFile = "@latest"
	versionDir = "@v/"
	infoExt    = ".info"
	modExt     = ".mod"
	zipExt     = ".zip"
)

// The texts of the files of the tests: the go.mod of a module that requires
// the kernel, an info file of the module proxy protocol, the go.mod of a
// module that requires nothing, the package of the kernel, the source and
// the test of a release, and the use of a directory in a go.work.
const (
	requiringFormat = "module %s\n\ngo 1.27\n\nrequire " + kernelModule + " " + release + "\n"
	infoFormat      = `{"Version":"%s","Time":"` + versionTime + `"}` + "\n"
	plainFormat     = "module %s\n\ngo 1.27\n"
	kernelSource    = "// Package version states the major version of the kernel.\npackage version\n\n" +
		"// Major is the major version of the kernel.\nconst Major = %d\n"
	releaseSource = "// Package %[1]s is a release of the tests of the ring.\npackage %[1]s\n"
	releaseTest   = "package %s\n\nimport (\n\t\"testing\"\n\n\t\"" + versionPackage + "\"\n)\n\n" +
		"func TestMajor(t *testing.T) {\n\tif version.Major != %d {\n" +
		"\t\tt.Fatalf(\"" + testFailure + "%%d\", version.Major)\n\t}\n}\n"
	useFormat = "\t%s\n"
)

// The go.work of a checkout opens with this text, followed by the use of
// each directory and workEnd.
const (
	workStart = "go %s\n\nuse (\n"
	workEnd   = ")\n"
	localUse  = "./"
)

// testFailure is the message of the test of a release whose kernel has
// another major version.
const testFailure = "the kernel has the major version "

// The answer of the proxy that fails the queries of the broken module, and
// the status line of that answer.
const (
	brokenAnswer = "the proxy fails"
	brokenStatus = "500 Internal Server Error"
)

// The file below which a test asks for a scratch module, and the directory
// of the scratch module.
const (
	blockingFile = "blocking"
	scratchDir   = "scratch"
)

// goVersion is the language version of the go command that runs the tests,
// which the go line of each checkout states.
var goVersion = strings.TrimPrefix(runtime.Version(), "go")

// ringNames are the names of the modules of the ring of a checkout, and
// ringModules are their paths, in the order of the paths.
var (
	ringNames   = []string{brokenName, failingName, missingName, passingName, pseudoName}
	ringModules = []string{
		modulePrefix + brokenName, modulePrefix + failingName, modulePrefix + missingName,
		modulePrefix + passingName, modulePrefix + pseudoName,
	}
)

// fixture is a checkout and the environment of a go command that asks the
// module proxy of the tests.
type fixture struct {
	// root is the directory of the go.work of the checkout.
	root string
	// env is the environment of the go command: the module proxy of the
	// tests, a module cache of the test, no checksum database, and the
	// local toolchain.
	env []string
}

// The ring reads the workspace of a checkout, reads the answers of the
// module proxy, and tests a release against the kernel of the checkout.
func TestRing(t *testing.T) {
	t.Parallel()

	t.Run("read", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the modules that require the kernel in the order of their paths", func(t *testing.T) {
			t.Parallel()

			f := proxied(t, ringNames...)
			got, err := read(t.TempDir(), f.root)
			assert.NoError(t, err, "the checkout reads")
			expect.Equal(t, got.ring, ringModules, "the ring has each module that requires the kernel")
			expect.Equal(t, got.kernel, filepath.Join(f.root, kernelDir), "the kernel is in its directory")
			expect.Equal(t, got.facade, filepath.Join(f.root, facadeDir), "the SDK facade is in its directory")
			expect.Equal(t, got.goVersion, goVersion, "the version is the go line of the go.work")
		})

		t.Run("returns the directories of a relative root as absolute directories", func(t *testing.T) {
			t.Parallel()

			root := files.Workspace(t, checkoutTree(passingName))
			got, err := read(filepath.Dir(root), filepath.Base(root))
			assert.NoError(t, err, "the checkout reads")
			expect.Equal(t, got.kernel, filepath.Join(root, kernelDir), "the directory of the kernel is absolute")
			expect.Equal(t, got.facade, filepath.Join(root, facadeDir), "the directory of the facade is absolute")
		})

		t.Run("returns the error of a root that does not exist", func(t *testing.T) {
			t.Parallel()

			_, err := read(t.TempDir(), missingDir)
			assert.ErrorIs(t, err, fs.ErrNotExist, "the error wraps the error of the file system")
		})

		t.Run("returns the error of a directory that the go.work uses outside the checkout", func(t *testing.T) {
			t.Parallel()

			root := files.Workspace(t, checkoutTree(passingName))
			outside := files.Workspace(t, files.Tree{modFile: files.Text(fmt.Sprintf(plainFormat, kernelModule))})
			var work strings.Builder
			fmt.Fprintf(&work, workStart, goVersion)
			fmt.Fprintf(&work, useFormat, filepath.ToSlash(outside))
			fmt.Fprintf(&work, useFormat, localUse+facadeDir)
			fmt.Fprintf(&work, useFormat, localUse+passingName)
			work.WriteString(workEnd)
			files.Write(t, root, files.Tree{workFile: files.Text(work.String())})
			_, err := read(t.TempDir(), root)
			assert.HasError(t, err, "the checkout does not read")
			expect.HasPrefix(t, err.Error(), commandName+": ", "the error starts with the name of the command")
		})

		kernelOnly := fmt.Sprintf(workStart, goVersion) + fmt.Sprintf(useFormat, localUse+kernelDir) + workEnd
		errs := []struct {
			name   string
			give   files.Tree
			wantIs error
			want   string
		}{
			{
				name:   "returns the error of a root without a go.work",
				give:   files.Tree{modFile: files.Text(fmt.Sprintf(plainFormat, unrelatedModule))},
				wantIs: fs.ErrNotExist,
			},
			{
				name: "returns the error of a go.work that does not parse",
				give: files.Tree{workFile: files.Text("use (\n")},
				want: workFile,
			},
			{
				name: "returns an error for a go.work without a go line",
				give: files.Tree{workFile: files.Text("use " + localUse + kernelDir + "\n")},
				want: "has no go line",
			},
			{
				name:   "returns the error of a used directory without a go.mod",
				give:   files.Tree{workFile: files.Text(kernelOnly)},
				wantIs: fs.ErrNotExist,
			},
			{
				name: "returns the error of a go.mod that does not parse",
				give: files.Tree{
					workFile:                  files.Text(kernelOnly),
					kernelDir + "/" + modFile: files.Text("module\n"),
				},
				want: modFile,
			},
			{
				name: "returns an error for a go.mod without a module line",
				give: files.Tree{
					workFile:                  files.Text(kernelOnly),
					kernelDir + "/" + modFile: files.Text("go 1.27\n"),
				},
				want: "has no module line",
			},
			{
				name: "returns an error for a go.work that does not use the kernel",
				give: workspaceOf(facadeDir, unrelatedDir),
				want: "does not use the kernel",
			},
			{
				name: "returns an error for a go.work that does not use the SDK facade",
				give: workspaceOf(kernelDir, unrelatedDir),
				want: "does not use the SDK facade",
			},
			{
				name: "returns an error for a go.work that does not use a module that requires the kernel",
				give: workspaceOf(kernelDir, facadeDir, unrelatedDir),
				want: "does not use a module that requires the kernel",
			},
		}
		for _, tt := range errs {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				_, err := read(t.TempDir(), files.Workspace(t, tt.give))
				assert.HasError(t, err, "the checkout does not read")
				expect.HasPrefix(t, err.Error(), commandName+": ", "the error starts with the name of the command")
				if tt.wantIs != nil {
					expect.ErrorIs(t, err, tt.wantIs, "the error wraps the error of the file system")
					return
				}
				expect.Contains(t, err.Error(), tt.want, "the error names what the checkout lacks")
			})
		}
	})

	t.Run("latest", func(t *testing.T) {
		t.Parallel()

		f := proxied(t, ringNames...)
		answers := []struct {
			name        string
			give        string
			wantVersion string
			wantRefusal string
		}{
			{name: "returns the release that the proxy serves", give: passingName, wantVersion: release},
			{
				name:        "returns the pseudo-version of a module without a release",
				give:        pseudoName,
				wantVersion: pseudoVersion,
			},
			{
				name:        "returns the refusal of a module that the proxy does not serve",
				give:        missingName,
				wantRefusal: notServed[0],
			},
			{name: "returns the refusal of a proxy that fails", give: brokenName, wantRefusal: brokenStatus},
		}
		for _, tt := range answers {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				version, refusal, err := latest(t.Context(), f.env, t.TempDir(), modulePrefix+tt.give)
				assert.NoError(t, err, "the go command answers")
				if tt.wantRefusal != "" {
					expect.Contains(t, refusal, tt.wantRefusal, "the refusal has the status line of the proxy")
					return
				}
				expect.Equal(t, version, tt.wantVersion, "the version is the answer of the proxy")
				expect.Equal(t, refusal, "", "the answer has no refusal")
			})
		}

		t.Run("returns the error of a go command that its context stops", func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()
			assert.HonoursCancellation(t, func(ctx context.Context) error {
				_, _, err := latest(ctx, f.env, dir, modulePrefix+passingName)
				return err
			}, "latest returns the error of the context")
		})
	})

	t.Run("test", func(t *testing.T) {
		t.Parallel()

		f := proxied(t, ringNames...)
		c, err := read(t.TempDir(), f.root)
		assert.NoError(t, err, "the checkout reads")

		t.Run("returns no error for a release whose tests pass against the checkout", func(t *testing.T) {
			t.Parallel()

			out, err := test(t.Context(), f.env, c, filepath.Join(t.TempDir(), scratchDir), modulePrefix+passingName,
				release)
			assert.NoError(t, err, "go test passes: "+string(out))
		})

		t.Run("returns the error of go test for a release whose tests fail against the checkout", func(t *testing.T) {
			t.Parallel()

			out, err := test(t.Context(), f.env, c, filepath.Join(t.TempDir(), scratchDir), modulePrefix+failingName,
				release)
			assert.HasError(t, err, "go test fails")
			expect.Contains(t, string(out), testFailure+strconv.Itoa(checkoutMajor),
				"the output has the failure of the test")
		})

		t.Run("returns the error of a scratch module that cannot be written", func(t *testing.T) {
			t.Parallel()

			root := files.Workspace(t, files.Tree{blockingFile: files.Text("")})
			_, err := test(t.Context(), f.env, c, filepath.Join(root, blockingFile, scratchDir),
				modulePrefix+passingName, release)
			assert.HasError(t, err, "the directory of the scratch module is below a regular file")
		})
	})
}

// proxied returns a checkout whose go.work uses the kernel, the SDK facade,
// a module that does not require the kernel, and the modules of names,
// beside a module proxy that serves the releases of the passing and the
// failing module and the pseudo-version of the module without a release.
// The proxy answers 404 Not Found for the missing module, and fails each
// query of the broken module. It stops when the test ends.
func proxied(t *testing.T, names ...string) fixture {
	t.Helper()

	served := files.Tree{}
	for _, r := range []struct {
		name  string
		major int
	}{{name: passingName, major: checkoutMajor}, {name: failingName, major: publishedMajor}} {
		mod := modulePrefix + r.name
		goMod := fmt.Sprintf(requiringFormat, mod)
		src := files.Workspace(t, files.Tree{
			modFile:             files.Text(goMod),
			r.name + ".go":      files.Text(fmt.Sprintf(releaseSource, r.name)),
			r.name + "_test.go": files.Text(fmt.Sprintf(releaseTest, r.name, r.major)),
		})
		var zipped bytes.Buffer
		assert.NoError(t, modzip.CreateFromDir(&zipped, module.Version{Path: mod, Version: release}, src),
			"the release of "+mod+" zips")
		served[mod+"/"+listFile] = files.Text(release + "\n")
		served[mod+"/"+versionDir+release+infoExt] = files.Text(fmt.Sprintf(infoFormat, release))
		served[mod+"/"+versionDir+release+modExt] = files.Text(goMod)
		served[mod+"/"+versionDir+release+zipExt] = files.Bytes(zipped.Bytes())
	}
	pseudo := modulePrefix + pseudoName
	served[pseudo+"/"+listFile] = files.Text("")
	served[pseudo+"/"+latestFile] = files.Text(fmt.Sprintf(infoFormat, pseudoVersion))
	served[pseudo+"/"+versionDir+pseudoVersion+infoExt] = files.Text(fmt.Sprintf(infoFormat, pseudoVersion))
	served[pseudo+"/"+versionDir+pseudoVersion+modExt] = files.Text(fmt.Sprintf(requiringFormat, pseudo))

	proxy := http.FileServerFS(os.DirFS(files.Workspace(t, served)))
	broken := "/" + modulePrefix + brokenName + "/"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, broken) {
			http.Error(w, brokenAnswer, http.StatusInternalServerError)
			return
		}
		proxy.ServeHTTP(w, r)
	}))
	t.Cleanup(srv.Close)
	return fixture{
		root: files.Workspace(t, checkoutTree(names...)),
		env: append(os.Environ(),
			"GOPROXY="+srv.URL, "GOSUMDB=off", "GONOSUMDB=", "GOPRIVATE=", "GONOPROXY=", "GOINSECURE=",
			"GOMODCACHE="+t.TempDir(), "GOFLAGS=-modcacherw", "GOTOOLCHAIN=local",
		),
	}
}

// checkoutTree returns the tree of a checkout whose go.work uses the kernel,
// the SDK facade, a module that does not require the kernel, and the
// modules of names, each in the directory of its name. The kernel declares
// the package version, whose constant Major is checkoutMajor.
func checkoutTree(names ...string) files.Tree {
	tree := workspaceOf(append([]string{kernelDir, facadeDir, unrelatedDir}, names...)...)
	tree[versionFile] = files.Text(fmt.Sprintf(kernelSource, checkoutMajor))
	for _, n := range names {
		tree[n+"/"+modFile] = files.Text(fmt.Sprintf(requiringFormat, modulePrefix+n))
	}
	return tree
}

// workspaceOf returns the tree of a checkout whose go.work uses the
// directories dirs, with the go.mod of the kernel, of the SDK facade and of
// the module that does not require the kernel, where dirs has their
// directories.
func workspaceOf(dirs ...string) files.Tree {
	var work strings.Builder
	fmt.Fprintf(&work, workStart, goVersion)
	for _, d := range dirs {
		fmt.Fprintf(&work, useFormat, localUse+d)
	}
	work.WriteString(workEnd)
	tree := files.Tree{workFile: files.Text(work.String())}
	modules := map[string]string{
		kernelDir:    fmt.Sprintf(plainFormat, kernelModule),
		facadeDir:    fmt.Sprintf(requiringFormat, facadeModule),
		unrelatedDir: fmt.Sprintf(plainFormat, unrelatedModule),
	}
	for _, d := range dirs {
		if goMod, known := modules[d]; known {
			tree[d+"/"+modFile] = files.Text(goMod)
		}
	}
	return tree
}
