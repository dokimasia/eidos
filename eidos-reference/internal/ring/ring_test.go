// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"archive/tar"
	"bytes"
	"context"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/expect"
	"go.dokimi.dev/assert/files"
)

// The prefix of the module paths of eidos and the directories of the
// modules of a checkout beside the kernel and the SDK facade. The test of the
// release in passing passes against the checkout. The test of the release in
// failing fails against it. The module in pseudo does not have a release.
// The proxy does not serve the module in missing. It fails each query of the
// module in broken. The path of each module is the prefix followed by the
// directory.
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
// packages of the kernel and of the facade that the tests of the releases
// import, and the files of those packages in a checkout.
const (
	kernelDir            = "core"
	facadeDir            = "sdk"
	unrelatedDir         = "unrelated"
	unrelatedModule      = "example.com/unrelated"
	versionPackage       = kernelModule + "/version"
	facadeVersionPackage = facadeModule + "/version"
	versionFile          = kernelDir + "/version/version.go"
	facadeVersionFile    = facadeDir + "/version/version.go"
)

// The sibling is a module that each release requires and replaces with a
// directory that the tree of the release does not have. The proxy serves
// its go.mod.
const (
	siblingName   = "sibling"
	siblingModule = modulePrefix + siblingName
)

// The go.mod of the nested module of the testdata of each release and the
// path of that module. A module zip does not contain the nested module.
const (
	fixtureMod    = "testdata/fixture/" + modFile
	fixtureModule = "example.com/fixture"
)

// The release of the published modules, a version without a release tag,
// the pseudo-version that the proxy returns for the module without a
// release, and the time of the versions.
const (
	release       = "v0.1.0"
	untagged      = "v0.2.0"
	pseudoVersion = "v0.0.0-20261010000000-abcdefabcdef"
	versionTime   = "2026-10-10T00:00:00Z"
)

// checkoutMajor is the major version of the kernel and the facade of a
// checkout. publishedMajor is the major version of the kernel and the facade
// of the release.
const (
	checkoutMajor  = 2
	publishedMajor = 1
)

// The files of a module in the module proxy protocol. Each path is relative
// to the directory of the module.
const (
	listFile   = "@v/list"
	latestFile = "@latest"
	versionDir = "@v/"
	infoExt    = ".info"
	modExt     = ".mod"
)

// The texts of the files of the tests. requiringFormat is the go.mod of a
// module that requires the kernel. infoFormat is an info file of the module
// proxy protocol. plainFormat is the go.mod of a module that requires
// nothing. unparsedMod is a go.mod that does not parse. versionSource is the
// package version of the kernel and of the facade. releaseMod, releaseSource
// and releaseTest are the go.mod, the source and the test of a release.
// useFormat is the use of a directory in a go.work.
const (
	requiringFormat = "module %s\n\ngo 1.27\n\nrequire " + kernelModule + " " + release + "\n"
	infoFormat      = `{"Version":"%s","Time":"` + versionTime + `"}` + "\n"
	plainFormat     = "module %s\n\ngo 1.27\n"
	unparsedMod     = "module\n"
	versionSource   = "// Package version states a major version.\npackage version\n\n" +
		"// Major is the major version.\nconst Major = %d\n"
	releaseMod = "module %s\n\ngo 1.27\n\nrequire (\n" +
		"\t" + kernelModule + " " + release + "\n" +
		"\t" + facadeModule + " " + release + "\n" +
		"\t" + siblingModule + " " + release + "\n)\n\n" +
		"replace " + kernelModule + " => " + parentPrefix + kernelDir + "\n\n" +
		"replace " + facadeModule + " => " + parentPrefix + facadeDir + "\n\n" +
		"replace " + siblingModule + " => " + parentPrefix + siblingName + "\n"
	releaseSource = "// Package %[1]s is a release of the tests of the ring.\npackage %[1]s\n"
	releaseTest   = "package %[1]s\n\nimport (\n\t\"os\"\n\t\"testing\"\n\n" +
		"\t\"" + versionPackage + "\"\n\tfacade \"" + facadeVersionPackage + "\"\n)\n\n" +
		"func TestMajor(t *testing.T) {\n" +
		"\tif version.Major != %[2]d {\n\t\tt.Fatalf(\"" + testFailure + "%%d\", version.Major)\n\t}\n" +
		"\tif facade.Major != %[2]d {\n\t\tt.Fatalf(\"" + facadeFailure + "%%d\", facade.Major)\n\t}\n" +
		"\tif _, err := os.Stat(\"" + fixtureMod + "\"); err != nil {\n\t\tt.Fatal(err)\n\t}\n}\n"
	useFormat = "\t%s\n"
)

// The go.work of a checkout opens with workStart. The use of each directory
// follows it. workEnd closes the file. localUse opens the path of each
// directory.
const (
	workStart = "go %s\n\nuse (\n"
	workEnd   = ")\n"
	localUse  = "./"
)

// testFailure and facadeFailure open the messages of the test of a release
// whose kernel or facade has another major version. passedPrefix opens the
// line of go test for a package whose tests pass.
const (
	testFailure   = "the kernel has the major version "
	facadeFailure = "the facade has the major version "
	passedPrefix  = "ok  \t"
)

// The answer of the proxy that fails the queries of the broken module, and
// the status line of that answer.
const (
	brokenAnswer = "the proxy fails"
	brokenStatus = "500 Internal Server Error"
)

// The file below which a test asks for a scratch directory, the scratch
// directory, and the name of an entry outside it.
const (
	blockingFile = "blocking"
	scratchDir   = "scratch"
	escapeName   = "escape"
)

// The prefix of the variables of git, the message of the commit of a
// release, and the author of that commit. A git hook sets variables of git
// for the repository that runs it.
const (
	gitPrefix      = "GIT_"
	releaseMessage = "release"
	gitAuthor      = "ring"
	gitEmail       = "ring@ring.invalid"
)

// The directories of the kernel and of the facade of a checkout of the cases
// of rewire and extract. The go command reads a replacement path that starts
// with a slash as a directory on every system.
const (
	checkoutKernel = "/checkout/" + kernelDir
	checkoutFacade = "/checkout/" + facadeDir
)

// The entries of the archives of the cases of extract. archivedDirMode and
// archivedFileMode are the modes of a directory and of a file in an archive
// of git. sourceName and sourceText are a source file and its text. linkName
// is the name of a symbolic link. git archive writes the commit into the
// record commentRecord of its global header. parentPrefix opens a name in the
// parent directory. notArchive is text that is not a tar archive. blockSize
// is the size of a block of a tar archive.
const (
	archivedDirMode  = 0o775
	archivedFileMode = 0o664
	sourceName       = passingName + "/" + passingName + ".go"
	sourceText       = "package " + passingName + "\n"
	linkName         = passingName + "/link"
	commentRecord    = "comment"
	commitHash       = "1aacd53f6f8e8094c30f03efa480a79fdbff875c"
	parentPrefix     = "../"
	notArchive       = "This text is not a tar archive. "
	blockSize        = 512
)

// goVersion is the language version of the go command that runs the tests.
// The go line of each go.work of the tests has this version.
var goVersion = strings.TrimPrefix(runtime.Version(), "go")

// ringNames are the names of the modules of the ring of a checkout.
// ringMembers are those modules in the order of their paths.
var (
	ringNames   = []string{brokenName, failingName, missingName, passingName, pseudoName}
	ringMembers = []member{
		{path: modulePrefix + brokenName, dir: brokenName},
		{path: modulePrefix + failingName, dir: failingName},
		{path: modulePrefix + missingName, dir: missingName},
		{path: modulePrefix + passingName, dir: passingName},
		{path: modulePrefix + pseudoName, dir: pseudoName},
	}
)

// gitEnv makes git ignore the configuration of the system and of the user.
// It turns the automatic maintenance of git off and sets the author and the
// committer of a commit of a test.
var gitEnv = []string{
	"GIT_CONFIG_NOSYSTEM=1",
	"GIT_CONFIG_COUNT=1", "GIT_CONFIG_KEY_0=maintenance.auto", "GIT_CONFIG_VALUE_0=false",
	"GIT_AUTHOR_NAME=" + gitAuthor, "GIT_AUTHOR_EMAIL=" + gitEmail,
	"GIT_COMMITTER_NAME=" + gitAuthor, "GIT_COMMITTER_EMAIL=" + gitEmail,
}

// fixture is a checkout and the environment of a go command that asks the
// module proxy of the tests.
type fixture struct {
	// root is the directory of the go.work of the checkout.
	root string
	// env is the environment of the go command and of git. It sets the
	// module proxy of the tests, a module cache of the test and the local
	// toolchain. It turns the checksum database off. git ignores the
	// configuration of the system and of the user.
	env []string
}

// entry is an entry of a tar archive of a test. It has a header and the
// content of a regular file.
type entry struct {
	header  tar.Header
	content string
}

// The ring reads the workspace of a checkout, reads the answers of the
// module proxy, writes the tree of a release tag, and tests a release
// against the kernel of the checkout.
func TestRing(t *testing.T) {
	t.Parallel()

	t.Run("read", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the modules that require the kernel in the order of their paths", func(t *testing.T) {
			t.Parallel()

			root := files.Workspace(t, checkoutTree(ringNames...))
			got, err := read(t.TempDir(), root)
			assert.NoError(t, err, "the checkout reads")
			expect.Equal(t, got.ring, ringMembers, "the ring has each module that requires the kernel")
			expect.Equal(t, got.root, root, "the root is the directory of the go.work")
			expect.Equal(t, got.kernel, filepath.Join(root, kernelDir), "the kernel is in its directory")
			expect.Equal(t, got.facade, filepath.Join(root, facadeDir), "the SDK facade is in its directory")
		})

		t.Run("returns the directories of a relative root as absolute directories", func(t *testing.T) {
			t.Parallel()

			root := files.Workspace(t, checkoutTree(passingName))
			got, err := read(filepath.Dir(root), filepath.Base(root))
			assert.NoError(t, err, "the checkout reads")
			expect.Equal(t, got.root, root, "the root is absolute")
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
				name:   "returns the error of a used directory without a go.mod",
				give:   files.Tree{workFile: files.Text(kernelOnly)},
				wantIs: fs.ErrNotExist,
			},
			{
				name: "returns the error of a go.mod that does not parse",
				give: files.Tree{
					workFile:                  files.Text(kernelOnly),
					kernelDir + "/" + modFile: files.Text(unparsedMod),
				},
				want: modFile,
			},
			{
				name: "returns an error for a go.mod without a module line",
				give: files.Tree{
					workFile:                  files.Text(kernelOnly),
					kernelDir + "/" + modFile: files.Text("go 1.27\n"),
				},
				want: "does not have a module line",
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
		passing := member{path: modulePrefix + passingName, dir: passingName}

		t.Run("returns no error for a release whose tests pass against the checkout", func(t *testing.T) {
			t.Parallel()

			out, err := test(t.Context(), f.env, c, filepath.Join(t.TempDir(), scratchDir), passing, release)
			assert.NoError(t, err, "go test passes: "+string(out))
			expect.Contains(t, string(out), passedPrefix+passing.path+"\t", "go test runs the tests of the release")
		})

		t.Run("returns the error of go test for a release whose tests fail against the checkout", func(t *testing.T) {
			t.Parallel()

			failing := member{path: modulePrefix + failingName, dir: failingName}
			out, err := test(t.Context(), f.env, c, filepath.Join(t.TempDir(), scratchDir), failing, release)
			assert.HasError(t, err, "go test fails")
			expect.Contains(t, string(out), testFailure+strconv.Itoa(checkoutMajor),
				"the output has the failure of the test")
		})

		t.Run("returns the error of git for a version without a release tag", func(t *testing.T) {
			t.Parallel()

			_, err := test(t.Context(), f.env, c, filepath.Join(t.TempDir(), scratchDir), passing, untagged)
			assert.HasError(t, err, "git does not archive the tag")
			expect.Contains(t, err.Error(), tagRefs+passingName+"/"+untagged, "the error names the tag")
		})
	})

	t.Run("extract", func(t *testing.T) {
		t.Parallel()

		c := checkout{kernel: checkoutKernel, facade: checkoutFacade}
		goMod := passingName + "/" + modFile
		pkg := entry{header: tar.Header{Typeflag: tar.TypeDir, Name: passingName + "/", Mode: archivedDirMode}}
		source := entry{
			header:  tar.Header{Typeflag: tar.TypeReg, Name: sourceName, Mode: archivedFileMode},
			content: sourceText,
		}

		t.Run("writes the directories and the regular files of the archive", func(t *testing.T) {
			t.Parallel()

			dir := filepath.Join(t.TempDir(), scratchDir)
			global := entry{header: tar.Header{
				Typeflag:   tar.TypeXGlobalHeader,
				PAXRecords: map[string]string{commentRecord: commitHash},
			}}
			assert.NoError(t, extract(bytes.NewReader(archived(t, global, pkg, source)), dir, goMod, c),
				"the archive extracts")
			files.Equal(t, os.DirFS(dir), files.Tree{sourceName: files.Text(sourceText)},
				"the directory has the directory and the file of the archive alone")
		})

		t.Run("writes the go.mod of the module with the replace directives of the checkout", func(t *testing.T) {
			t.Parallel()

			dir := filepath.Join(t.TempDir(), scratchDir)
			published := fmt.Sprintf(releaseMod, modulePrefix+passingName)
			mod := entry{
				header:  tar.Header{Typeflag: tar.TypeReg, Name: goMod, Mode: archivedFileMode},
				content: published,
			}
			assert.NoError(t, extract(bytes.NewReader(archived(t, pkg, mod)), dir, goMod, c), "the archive extracts")
			want, err := rewire(goMod, []byte(published), c)
			assert.NoError(t, err, "the go.mod of the release rewires")
			files.HasContent(t, filepath.Join(dir, passingName, modFile), string(want),
				"the go.mod is the go.mod of the release against the checkout")
		})

		t.Run("returns an error for an entry that is not a directory or a regular file", func(t *testing.T) {
			t.Parallel()

			link := entry{header: tar.Header{Typeflag: tar.TypeSymlink, Name: linkName, Linkname: sourceName}}
			err := extract(bytes.NewReader(archived(t, pkg, link)), filepath.Join(t.TempDir(), scratchDir), goMod, c)
			assert.HasError(t, err, "the archive does not extract")
			expect.Contains(t, err.Error(), linkName, "the error names the entry")
		})

		t.Run("returns the error of an archive that does not read", func(t *testing.T) {
			t.Parallel()

			garbage := strings.NewReader(strings.Repeat(notArchive, blockSize))
			err := extract(garbage, filepath.Join(t.TempDir(), scratchDir), goMod, c)
			assert.ErrorIs(t, err, tar.ErrHeader, "the error wraps the error of the tar reader")
		})

		t.Run("returns the error of a file whose content ends before its size", func(t *testing.T) {
			t.Parallel()

			cut := archived(t, source)[:blockSize+1]
			err := extract(bytes.NewReader(cut), filepath.Join(t.TempDir(), scratchDir), goMod, c)
			assert.ErrorIs(t, err, io.ErrUnexpectedEOF, "the error wraps the end of the archive")
		})

		t.Run("returns the error of a file outside the directory", func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			outside := entry{
				header:  tar.Header{Typeflag: tar.TypeReg, Name: parentPrefix + escapeName, Mode: archivedFileMode},
				content: sourceText,
			}
			err := extract(bytes.NewReader(archived(t, outside)), filepath.Join(root, scratchDir), goMod, c)
			assert.HasError(t, err, "the file does not extract")
			files.Absent(t, filepath.Join(root, escapeName), "the archive writes nothing outside the directory")
		})

		t.Run("returns the error of a directory outside the directory", func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			outside := entry{
				header: tar.Header{Typeflag: tar.TypeDir, Name: parentPrefix + escapeName + "/", Mode: archivedDirMode},
			}
			err := extract(bytes.NewReader(archived(t, outside)), filepath.Join(root, scratchDir), goMod, c)
			assert.HasError(t, err, "the directory does not extract")
			files.Absent(t, filepath.Join(root, escapeName), "the archive writes nothing outside the directory")
		})

		t.Run("returns the error of a directory that cannot be created", func(t *testing.T) {
			t.Parallel()

			root := files.Workspace(t, files.Tree{blockingFile: files.Text("")})
			err := extract(bytes.NewReader(archived(t)), filepath.Join(root, blockingFile, scratchDir), goMod, c)
			assert.HasError(t, err, "the directory is below a regular file")
		})
	})

	t.Run("rewire", func(t *testing.T) {
		t.Parallel()

		c := checkout{kernel: checkoutKernel, facade: checkoutFacade}
		published := []byte(fmt.Sprintf(releaseMod, modulePrefix+passingName))

		t.Run("replaces the kernel and the SDK facade with their directories in the checkout", func(t *testing.T) {
			t.Parallel()

			got, err := rewire(modFile, published, c)
			assert.NoError(t, err, "the go.mod rewires")
			expect.Contains(t, string(got), kernelModule+" => "+checkoutKernel,
				"the go.mod replaces the kernel with its directory in the checkout")
			expect.Contains(t, string(got), facadeModule+" => "+checkoutFacade,
				"the go.mod replaces the facade with its directory in the checkout")
		})

		t.Run("drops the other replace directives of the release", func(t *testing.T) {
			t.Parallel()

			got, err := rewire(modFile, published, c)
			assert.NoError(t, err, "the go.mod rewires")
			expect.NotContains(t, string(got), siblingModule+" =>", "the go.mod does not replace the sibling")
		})

		t.Run("returns the error of a go.mod that does not parse", func(t *testing.T) {
			t.Parallel()

			_, err := rewire(modFile, []byte(unparsedMod), c)
			assert.HasError(t, err, "the go.mod does not parse")
			expect.Contains(t, err.Error(), modFile, "the error names the file")
		})
	})
}

// proxied returns a checkout beside a module proxy. The go.work of the
// checkout uses the kernel, the SDK facade, a module that does not require
// the kernel, and the modules of names.
//
// The checkout is a git repository with one commit, the release. The release
// has the kernel and the facade of the major version publishedMajor. It also
// has the sources of the passing and the failing module, at which the tags
// passing/v0.1.0 and failing/v0.1.0 point. The go.mod of each release
// replaces the kernel, the facade and the sibling with directories of the
// tree. The tree does not have a directory of the sibling. The test of each
// release checks the major versions of the kernel and of the facade. It also
// reads the nested module of its testdata. The test of the passing release
// expects checkoutMajor and passes against the checkout alone. The test of
// the failing release expects publishedMajor. The working tree raises the
// kernel and the facade to checkoutMajor. It also fixes the test of the
// failing module.
//
// The proxy serves the releases of the passing and the failing module, the
// go.mod of the sibling, and the pseudo-version of the module without a
// release. It answers 404 Not Found for the missing module. It fails each
// query of the broken module. It stops when the test ends.
func proxied(t *testing.T, names ...string) fixture {
	t.Helper()

	releases := []struct {
		name  string
		major int
	}{{name: passingName, major: checkoutMajor}, {name: failingName, major: publishedMajor}}
	served := files.Tree{}
	tree := checkoutTree(names...)
	for _, r := range releases {
		mod := modulePrefix + r.name
		goMod := fmt.Sprintf(releaseMod, mod)
		served[mod+"/"+listFile] = files.Text(release + "\n")
		served[mod+"/"+versionDir+release+infoExt] = files.Text(fmt.Sprintf(infoFormat, release))
		served[mod+"/"+versionDir+release+modExt] = files.Text(goMod)
		tree[r.name+"/"+modFile] = files.Text(goMod)
		tree[r.name+"/"+r.name+".go"] = files.Text(fmt.Sprintf(releaseSource, r.name))
		tree[r.name+"/"+r.name+"_test.go"] = files.Text(fmt.Sprintf(releaseTest, r.name, r.major))
		tree[r.name+"/"+fixtureMod] = files.Text(fmt.Sprintf(plainFormat, fixtureModule))
	}
	served[siblingModule+"/"+versionDir+release+infoExt] = files.Text(fmt.Sprintf(infoFormat, release))
	served[siblingModule+"/"+versionDir+release+modExt] = files.Text(fmt.Sprintf(plainFormat, siblingModule))
	pseudo := modulePrefix + pseudoName
	served[pseudo+"/"+listFile] = files.Text("")
	served[pseudo+"/"+latestFile] = files.Text(fmt.Sprintf(infoFormat, pseudoVersion))
	served[pseudo+"/"+versionDir+pseudoVersion+infoExt] = files.Text(fmt.Sprintf(infoFormat, pseudoVersion))
	served[pseudo+"/"+versionDir+pseudoVersion+modExt] = files.Text(fmt.Sprintf(requiringFormat, pseudo))
	tree[versionFile] = files.Text(fmt.Sprintf(versionSource, publishedMajor))
	tree[facadeVersionFile] = files.Text(fmt.Sprintf(versionSource, publishedMajor))

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
	env := append(slices.DeleteFunc(os.Environ(), func(v string) bool { return strings.HasPrefix(v, gitPrefix) }),
		"GOPROXY="+srv.URL, "GOSUMDB=off", "GONOSUMDB=", "GOPRIVATE=", "GONOPROXY=", "GOINSECURE=",
		"GOMODCACHE="+t.TempDir(), "GOFLAGS=-modcacherw", "GOTOOLCHAIN=local",
		"GIT_CONFIG_GLOBAL="+filepath.Join(t.TempDir(), missingDir),
	)
	env = append(env, gitEnv...)

	root := files.Workspace(t, tree)
	commands := [][]string{
		{"init", "-q"},
		{"add", "--all"},
		{"commit", "-q", "-m", releaseMessage},
		{"tag", passingName + "/" + release},
		{"tag", failingName + "/" + release},
	}
	for _, args := range commands {
		cmd := exec.CommandContext(t.Context(), gitBinary, args...)
		cmd.Dir = root
		cmd.Env = env
		out, err := cmd.CombinedOutput()
		assert.NoError(t, err, "git "+strings.Join(args, " ")+" succeeds: "+string(out))
	}
	files.Write(t, root, files.Tree{
		versionFile:       files.Text(fmt.Sprintf(versionSource, checkoutMajor)),
		facadeVersionFile: files.Text(fmt.Sprintf(versionSource, checkoutMajor)),
		failingName + "/" + failingName + "_test.go": files.Text(
			fmt.Sprintf(releaseTest, failingName, checkoutMajor)),
	})
	return fixture{root: root, env: env}
}

// checkoutTree returns the tree of a checkout whose go.work uses the kernel,
// the SDK facade, a module that does not require the kernel, and the
// modules of names. Each module is in the directory of its name. The kernel
// and the facade declare the package version. Its constant Major is
// checkoutMajor.
func checkoutTree(names ...string) files.Tree {
	tree := workspaceOf(append([]string{kernelDir, facadeDir, unrelatedDir}, names...)...)
	tree[versionFile] = files.Text(fmt.Sprintf(versionSource, checkoutMajor))
	tree[facadeVersionFile] = files.Text(fmt.Sprintf(versionSource, checkoutMajor))
	for _, n := range names {
		tree[n+"/"+modFile] = files.Text(fmt.Sprintf(requiringFormat, modulePrefix+n))
	}
	return tree
}

// workspaceOf returns the tree of a checkout whose go.work uses the
// directories dirs. The tree has the go.mod of the kernel, of the SDK facade
// and of the module that does not require the kernel when dirs contains
// their directories.
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

// archived returns the tar archive of entries. The size of the header of
// each entry is the length of its content.
func archived(t *testing.T, entries ...entry) []byte {
	t.Helper()

	var b bytes.Buffer
	w := tar.NewWriter(&b)
	for _, e := range entries {
		e.header.Size = int64(len(e.content))
		assert.NoError(t, w.WriteHeader(&e.header), "the archive writes the header of "+e.header.Name)
		_, err := io.WriteString(w, e.content)
		assert.NoError(t, err, "the archive writes the content of "+e.header.Name)
	}
	assert.NoError(t, w.Close(), "the archive closes")
	return b.Bytes()
}
