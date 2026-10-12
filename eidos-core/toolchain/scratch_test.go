// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package toolchain_test

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/files"

	"go.dokimi.dev/eidos/core/toolchain"
)

// scratchPattern is the pattern of the scratch directories that the cases
// create.
const scratchPattern = "eidos-toolchain-*"

// The project file that the scratch cases pass, its content, and the
// content that an output states at its path.
const (
	projectFile = "build.cfg"
	projectBody = "project\n"
	statedBody  = "stated\n"
)

// The paths that the cases write outside the directory: one relative path
// that starts with "..", the name that it writes, and an absolute path.
const (
	escapePath   = "../escape.s"
	escapeName   = "escape.s"
	absolutePath = "/absolute.s"
)

// The directory and the file that the cases nest, and the directory that
// a skip names.
const (
	nestedDir  = "gen"
	skippedDir = "build"
)

// The probe that the probe cases write, and its content.
const (
	probeName = "probe/zz_probe.s"
	probeBody = "probe\n"
)

// The modes that the scratch project has.
const (
	privateFile = 0o600
	privateDir  = 0o700
)

// errCheck is the error of the check that a probe case fails.
var errCheck = errors.New("toolchain_test: the check fails")

// TestScratch checks that Scratch, WriteFile and Probe write only under
// their directory, and that Sources lists the files of a project.
func TestScratch(t *testing.T) {
	t.Parallel()

	t.Run("Scratch", func(t *testing.T) {
		t.Parallel()

		t.Run("writes every file of the output", func(t *testing.T) {
			t.Parallel()

			dir := scratched(t, output(), nil)
			files.HasContent(t, filepath.Join(dir, filepath.FromSlash(generatedFile)), generatedBody,
				"the file of the output has its own bytes")
		})

		t.Run("writes a project file where the output has none", func(t *testing.T) {
			t.Parallel()

			dir := scratched(t, output(), map[string][]byte{projectFile: []byte(projectBody)})
			files.HasContent(t, filepath.Join(dir, projectFile), projectBody, "the project file is in the directory")
		})

		t.Run("keeps the file of the output at the path of a project file", func(t *testing.T) {
			t.Parallel()

			g := output()
			g.Files[projectFile] = []byte(statedBody)
			dir := scratched(t, g, map[string][]byte{projectFile: []byte(projectBody)})
			files.HasContent(t, filepath.Join(dir, projectFile), statedBody, "the file of the output is kept")
		})

		t.Run("writes each file with the mode 0o600", func(t *testing.T) {
			t.Parallel()

			if runtime.GOOS == "windows" {
				t.Skip("Windows records no permission bits")
			}
			dir := scratched(t, output(), nil)
			files.HasMode(t, filepath.Join(dir, filepath.FromSlash(generatedFile)), privateFile,
				"only the owner reads the file")
		})

		t.Run("creates each directory with the mode 0o700", func(t *testing.T) {
			t.Parallel()

			if runtime.GOOS == "windows" {
				t.Skip("Windows records no permission bits")
			}
			dir := scratched(t, output(), nil)
			files.HasMode(t, filepath.Join(dir, nestedDir), privateDir, "only the owner opens the directory")
		})

		t.Run("returns an error for an output path that leaves the directory", func(t *testing.T) {
			t.Parallel()

			g := toolchain.Generated{Files: map[string][]byte{escapePath: []byte(generatedBody)}}
			_, err := toolchain.Scratch(scratchPattern, g, nil)
			assert.HasError(t, err, "Scratch writes no file outside the directory")
			assert.Contains(t, err.Error(), "leaves the directory", "the error contains the reason")
		})

		t.Run("returns an error for a project path that leaves the directory", func(t *testing.T) {
			t.Parallel()

			_, err := toolchain.Scratch(scratchPattern, output(), map[string][]byte{escapePath: []byte(projectBody)})
			assert.HasError(t, err, "Scratch writes no project file outside the directory")
			assert.Contains(t, err.Error(), "leaves the directory", "the error contains the reason")
		})
	})

	t.Run("WriteFile", func(t *testing.T) {
		t.Parallel()

		t.Run("writes the body at a nested path", func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()
			err := toolchain.WriteFile(dir, generatedFile, []byte(generatedBody))
			assert.NoError(t, err, "WriteFile writes the file")
			files.HasContent(t, filepath.Join(dir, filepath.FromSlash(generatedFile)), generatedBody,
				"the file has the body")
		})

		t.Run("returns an error for a path that leaves the directory", func(t *testing.T) {
			t.Parallel()

			base := t.TempDir()
			dir := filepath.Join(base, nestedDir)
			assert.NoError(t, os.Mkdir(dir, privateDir), "the directory is created")
			err := toolchain.WriteFile(dir, escapePath, []byte(generatedBody))
			assert.HasError(t, err, "WriteFile returns an error")
			assert.Contains(t, err.Error(), "leaves the directory", "the error contains the reason")
			files.Absent(t, filepath.Join(base, escapeName), "WriteFile writes nothing outside the directory")
		})

		t.Run("returns an error for an absolute path", func(t *testing.T) {
			t.Parallel()

			err := toolchain.WriteFile(t.TempDir(), absolutePath, []byte(generatedBody))
			assert.HasError(t, err, "WriteFile returns an error")
			assert.Contains(t, err.Error(), "leaves the directory", "the error contains the reason")
		})

		t.Run("returns an error for a path below a file", func(t *testing.T) {
			t.Parallel()

			dir := files.Workspace(t, files.Tree{nestedDir: files.Text(generatedBody)})
			err := toolchain.WriteFile(dir, generatedFile, []byte(generatedBody))
			assert.HasError(t, err, "WriteFile cannot create a directory over a file")
			assert.HasPrefix(t, err.Error(), "toolchain: ", "the error starts with the name of the package")
		})

		t.Run("returns an error for a path of a directory", func(t *testing.T) {
			t.Parallel()

			dir := files.Workspace(t, files.Tree{generatedFile: files.Text(generatedBody)})
			err := toolchain.WriteFile(dir, nestedDir, []byte(generatedBody))
			assert.HasError(t, err, "WriteFile cannot write a file over a directory")
			assert.HasPrefix(t, err.Error(), "toolchain: ", "the error starts with the name of the package")
		})
	})

	t.Run("Sources", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the files with one of the extensions in lexical order", func(t *testing.T) {
			t.Parallel()

			dir := files.Workspace(t, files.Tree{
				"b.go":   files.Text(generatedBody),
				"a/x.go": files.Text(generatedBody),
				"a/y.ts": files.Text(generatedBody),
				"c.txt":  files.Text(generatedBody),
			})
			got, err := toolchain.Sources(dir, []string{".go", ".ts"})
			assert.NoError(t, err, "Sources walks the directory")
			assert.Equal(t, got, []string{
				filepath.Join(dir, "a", "x.go"),
				filepath.Join(dir, "a", "y.ts"),
				filepath.Join(dir, "b.go"),
			}, "Sources returns the Go and TypeScript files in lexical order")
		})

		t.Run("passes over a directory that skip names", func(t *testing.T) {
			t.Parallel()

			dir := files.Workspace(t, files.Tree{
				skippedDir + "/x.go": files.Text(generatedBody),
				"x.go":               files.Text(generatedBody),
			})
			got, err := toolchain.Sources(dir, []string{".go"}, skippedDir)
			assert.NoError(t, err, "Sources walks the directory")
			want := []string{filepath.Join(dir, "x.go")}
			assert.Equal(t, got, want, "Sources returns no file of the skipped directory")
		})

		t.Run("returns the files of dir when skip names dir", func(t *testing.T) {
			t.Parallel()

			dir := files.Workspace(t, files.Tree{skippedDir + "/x.go": files.Text(generatedBody)})
			root := filepath.Join(dir, skippedDir)
			got, err := toolchain.Sources(root, []string{".go"}, skippedDir)
			assert.NoError(t, err, "Sources walks the directory")
			assert.Equal(t, got, []string{filepath.Join(root, "x.go")}, "Sources skips only directories below dir")
		})

		t.Run("returns the files that end in a suffix with two dots", func(t *testing.T) {
			t.Parallel()

			dir := files.Workspace(t, files.Tree{
				"row.test.js": files.Text(generatedBody),
				"row.js":      files.Text(generatedBody),
			})
			got, err := toolchain.Sources(dir, []string{".test.js"})
			assert.NoError(t, err, "Sources walks the directory")
			assert.Equal(t, got, []string{filepath.Join(dir, "row.test.js")}, "Sources returns the test module alone")
		})

		t.Run("returns no paths for a directory without a match", func(t *testing.T) {
			t.Parallel()

			dir := files.Workspace(t, files.Tree{"c.txt": files.Text(generatedBody)})
			got, err := toolchain.Sources(dir, []string{".go"})
			assert.NoError(t, err, "Sources walks the directory")
			assert.Empty(t, got, "Sources returns no path")
		})

		t.Run("returns an error that wraps ErrNotExist for a directory that does not exist", func(t *testing.T) {
			t.Parallel()

			_, err := toolchain.Sources(filepath.Join(t.TempDir(), missingDir), []string{".go"})
			assert.ErrorIs(t, err, fs.ErrNotExist, "the error wraps the error of the file system")
		})
	})

	t.Run("Probe", func(t *testing.T) {
		t.Parallel()

		t.Run("writes the probe before the check", func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()
			path := filepath.Join(dir, filepath.FromSlash(probeName))
			err := toolchain.Probe(dir, probeName, probeBody, func() error {
				files.HasContent(t, path, probeBody, "the check reads the probe")
				return nil
			})
			assert.NoError(t, err, "Probe returns no error")
		})

		t.Run("removes the probe after the check", func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()
			assert.NoError(t, toolchain.Probe(dir, probeName, probeBody, func() error { return nil }),
				"Probe returns no error")
			files.Absent(t, filepath.Join(dir, filepath.FromSlash(probeName)), "the probe is removed")
		})

		t.Run("returns the error of the check", func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()
			err := toolchain.Probe(dir, probeName, probeBody, func() error { return errCheck })
			assert.ErrorIs(t, err, errCheck, "Probe returns the error of the check")
			probe := filepath.Join(dir, filepath.FromSlash(probeName))
			files.Absent(t, probe, "the probe is removed after a failed check")
		})

		t.Run("returns an error for a name that leaves the directory", func(t *testing.T) {
			t.Parallel()

			err := toolchain.Probe(t.TempDir(), escapePath, probeBody, func() error { return nil })
			assert.HasError(t, err, "Probe writes no probe outside the directory")
			assert.Contains(t, err.Error(), "leaves the directory", "the error contains the reason")
		})

		t.Run("does not call the check for a name that leaves the directory", func(t *testing.T) {
			t.Parallel()

			called := false
			err := toolchain.Probe(t.TempDir(), escapePath, probeBody, func() error {
				called = true
				return nil
			})
			assert.HasError(t, err, "Probe returns the error of the write")
			assert.False(t, called, "Probe does not run the check without its probe")
		})

		t.Run("returns the error of a removal that fails", func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()
			path := filepath.Join(dir, filepath.FromSlash(probeName))
			err := toolchain.Probe(dir, probeName, probeBody, func() error { return os.Remove(path) })
			assert.ErrorIs(t, err, fs.ErrNotExist, "Probe returns the error of the removal of a missing probe")
		})
	})
}

// TestScratchEnv checks the cases that set the temporary directory of the
// process, which run apart from the parallel cases of TestScratch.
func TestScratchEnv(t *testing.T) {
	t.Run("Scratch", func(t *testing.T) {
		t.Run("removes the directory after a write that fails", func(t *testing.T) {
			tmp := t.TempDir()
			setTempDir(t, tmp)
			g := toolchain.Generated{Files: map[string][]byte{escapePath: []byte(generatedBody)}}
			_, err := toolchain.Scratch(scratchPattern, g, nil)
			assert.HasError(t, err, "Scratch returns the error of the write")
			entries, err := os.ReadDir(tmp)
			assert.NoError(t, err, "the temporary directory reads")
			assert.Empty(t, entries, "Scratch leaves no directory behind")
		})

		t.Run("returns an error for a temporary directory that does not exist", func(t *testing.T) {
			setTempDir(t, filepath.Join(t.TempDir(), missingDir))
			_, err := toolchain.Scratch(scratchPattern, output(), nil)
			assert.ErrorIs(t, err, fs.ErrNotExist, "Scratch returns the error of os.MkdirTemp")
		})
	})
}

// scratched writes g and project into a scratch directory through Scratch,
// and removes the directory when the test ends.
func scratched(t *testing.T, g toolchain.Generated, project map[string][]byte) string {
	t.Helper()

	dir, err := toolchain.Scratch(scratchPattern, g, project)
	assert.NoError(t, err, "Scratch writes the files")
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	return dir
}

// setTempDir makes dir the temporary directory that os.TempDir returns for
// the rest of the test, on every platform.
func setTempDir(t *testing.T, dir string) {
	t.Helper()

	t.Setenv("TMPDIR", dir)
	t.Setenv("TMP", dir)
	t.Setenv("TEMP", dir)
}
