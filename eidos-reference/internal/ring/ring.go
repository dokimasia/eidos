// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"archive/tar"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"golang.org/x/mod/modfile"
	"golang.org/x/mod/module"
	"golang.org/x/mod/semver"
)

// The kernel and the SDK facade. The ring replaces them with their
// directories in the checkout and does not test them.
const (
	kernelModule = "go.dokimi.dev/eidos/core"
	facadeModule = "go.dokimi.dev/eidos/sdk"
)

// The file of a workspace, the file of a module, and the modes of the
// directories and the files of the tree of a release.
const (
	workFile = "go.work"
	modFile  = "go.mod"
	dirMode  = 0o700
	fileMode = 0o600
)

// The go command, the environment entry that runs it without a workspace,
// the query of the latest version of a module, the format in which the ring
// reads the answer, and the pattern of every package of the main module.
const (
	goBinary     = "go"
	noWorkspace  = "GOWORK=off"
	latestQuery  = "@latest"
	answerFormat = `{{.Version}}{{"\n"}}{{with .Error}}{{.Err}}{{end}}`
	allPackages  = "./..."
)

// The git command, the prefix of the reference of a release tag, and the
// separator of a revision and a path in the object name of a file.
const (
	gitBinary = "git"
	tagRefs   = "refs/tags/"
	objectSep = ":"
)

// notServed are the status lines of the module proxy protocol for a module
// that the proxy does not serve: 404 Not Found and 410 Gone. The go command
// writes the status line into the error of a query.
var notServed = []string{
	strconv.Itoa(http.StatusNotFound) + " " + http.StatusText(http.StatusNotFound),
	strconv.Itoa(http.StatusGone) + " " + http.StatusText(http.StatusGone),
}

// checkout is the workspace of a checkout. The ring reads it from the
// go.work of the checkout.
type checkout struct {
	// root is the absolute directory of the go.work. It is the root of the
	// git repository whose release tags the ring reads.
	root string
	// kernel and facade are the absolute directories of the kernel and of
	// the SDK facade.
	kernel, facade string
	// ring are the modules that require the kernel in the order of their
	// paths. The kernel and the facade are not among them.
	ring []member
	// releases maps the kernel and the facade to the version that the
	// checkout releases. That version is the version of the replace
	// directive of the module in the go.work, raised by the highest bump of
	// the module in the pending changesets. A module without a versioned
	// replace directive has no entry.
	releases map[string]string
}

// member is a module of the ring.
type member struct {
	// path is the module path.
	path string
	// dir is the directory of the module relative to the root, with slashes
	// as the separators. It is "." for the module at the root.
	dir string
}

// read returns the checkout whose go.work is in root. read resolves a
// relative root against the absolute directory dir. It reads the go.work,
// the changesets and the go.mod of each directory that the go.work uses
// through an os.Root of root. The root keeps each read inside the checkout. A
// module that has the kernel among its requirements is a module of the ring.
// The directories of the kernel and of the facade are absolute, because the
// tree of a release in another directory replaces the kernel and the facade
// with them. The version that the checkout releases of the kernel and of the
// facade is the version of their replace directives in the go.work, raised by
// the pending bumps of the changesets. A release commit has the new versions
// in its go.work and no pending changeset.
//
// Error modes:
//   - the error of a root that does not open
//   - the error of a go.work or a go.mod that does not read or parse
//   - the error of the go.mod of a directory outside the checkout
//   - an error for a go.mod without a module line
//   - an error for a go.work that does not use the kernel, the SDK facade
//     or a module of the ring
//   - the errors of pending, and the error of raise for the version of a
//     replace directive of the kernel or the facade
func read(dir, root string) (checkout, error) {
	if !filepath.IsAbs(root) {
		root = filepath.Join(dir, root)
	}
	r, err := os.OpenRoot(root)
	if err != nil {
		return checkout{}, fmt.Errorf("%s: %w", commandName, err)
	}
	defer func() { _ = r.Close() }()
	data, err := r.ReadFile(workFile)
	if err != nil {
		return checkout{}, fmt.Errorf("%s: %w", commandName, err)
	}
	work, err := modfile.ParseWork(workFile, data, nil)
	if err != nil {
		return checkout{}, fmt.Errorf("%s: %w", commandName, err)
	}
	bumps, err := pending(r.FS())
	if err != nil {
		return checkout{}, err
	}
	c := checkout{root: root, releases: map[string]string{}}
	for _, rep := range work.Replace {
		if rep.Old.Version == "" || (rep.Old.Path != kernelModule && rep.Old.Path != facadeModule) {
			continue
		}
		if c.releases[rep.Old.Path], err = raise(rep.Old.Version, bumps[rep.Old.Path]); err != nil {
			return checkout{}, err
		}
	}
	for _, use := range work.Use {
		dir := filepath.Clean(filepath.FromSlash(use.Path))
		name := filepath.Join(dir, modFile)
		data, err := r.ReadFile(name)
		if err != nil {
			return checkout{}, fmt.Errorf("%s: %w", commandName, err)
		}
		mod, err := modfile.Parse(name, data, nil)
		if err != nil {
			return checkout{}, fmt.Errorf("%s: %w", commandName, err)
		}
		if mod.Module == nil {
			return checkout{}, fmt.Errorf("%s: %s does not have a module line", commandName, name)
		}
		switch modPath := mod.Module.Mod.Path; {
		case modPath == kernelModule:
			c.kernel = filepath.Join(root, dir)
		case modPath == facadeModule:
			c.facade = filepath.Join(root, dir)
		case slices.ContainsFunc(mod.Require, func(r *modfile.Require) bool { return r.Mod.Path == kernelModule }):
			c.ring = append(c.ring, member{path: modPath, dir: filepath.ToSlash(dir)})
		}
	}
	if c.kernel == "" {
		return checkout{}, fmt.Errorf("%s: the go.work of the checkout does not use the kernel %s",
			commandName, kernelModule)
	}
	if c.facade == "" {
		return checkout{}, fmt.Errorf("%s: the go.work of the checkout does not use the SDK facade %s",
			commandName, facadeModule)
	}
	if len(c.ring) == 0 {
		return checkout{}, fmt.Errorf("%s: the go.work of the checkout does not use a module that requires the kernel",
			commandName)
	}
	slices.SortFunc(c.ring, func(a, b member) int { return strings.Compare(a.path, b.path) })
	return c, nil
}

// earlier returns the reason why version of the module m is on an earlier
// compatibility line than the checkout c releases. It reads the go.mod of
// the release from its tag in the git repository of c. earlier checks the
// requirement of the kernel first and the requirement of the facade second.
// The reason contains the first requirement on a line before the line of its
// version in c.releases. earlier returns an empty reason for a release on
// the line of c. A requirement of a module without an entry in c.releases
// does not make a release earlier.
//
// Error modes: the error of git for a tag or a file that it does not show,
// and the error of a go.mod that does not parse.
func earlier(ctx context.Context, env []string, c checkout, m member, version string) (string, error) {
	data, err := command(ctx, env, c.root, gitBinary, "show",
		tagRefs+path.Join(m.dir, version)+objectSep+path.Join(m.dir, modFile))
	if err != nil {
		return "", err
	}
	f, err := modfile.Parse(modFile, data, nil)
	if err != nil {
		return "", fmt.Errorf("%s: the go.mod of the release: %w", commandName, err)
	}
	for _, mod := range []string{kernelModule, facadeModule} {
		released, known := c.releases[mod]
		at := slices.IndexFunc(f.Require, func(r *modfile.Require) bool { return r.Mod.Path == mod })
		if !known || at < 0 {
			continue
		}
		if required := f.Require[at].Mod.Version; semver.Compare(lineOf(required), lineOf(released)) < 0 {
			return fmt.Sprintf("the release requires %s %s, and the checkout releases %s", mod, required, released),
				nil
		}
	}
	return "", nil
}

// latest returns the answer of the module proxy to the query of the latest
// version of mod. The answer is a version. Where the proxy does not return a
// version, the answer is the refusal of the query. latest reads the answer
// as go list -m -e <mod>@latest reports it. The go command runs in dir with
// the environment env.
//
// Error modes: the error of a go command that does not run or does not
// succeed.
func latest(ctx context.Context, env []string, dir, mod string) (version, refusal string, err error) {
	out, err := command(ctx, env, dir, goBinary, "list", "-m", "-e", "-f", answerFormat, mod+latestQuery)
	version, refusal, _ = strings.Cut(strings.TrimSuffix(string(out), "\n"), "\n")
	return version, refusal, err
}

// test runs go test over the packages of version of the module m against
// the kernel and the SDK facade of the checkout c. It returns the output of
// go test.
//
// A module zip leaves out each directory that has a go.mod and each file
// outside the module. A fixture under testdata can be such a directory. test
// reads the tree of the release from its tag in the git repository of c
// instead. It writes the tree into dir. By the rule of the go
// command, the tag of a module in a subdirectory is <dir>/<version>. The tag
// of the module at the root is the version. git archives the tree with the
// line endings with which the go command archives a module version.
//
// In the go.mod of m, test drops each replace directive and replaces the
// kernel and the facade with their directories in c. go test runs in the
// directory of m with the environment env and without a workspace. It adds
// the requirements and the sums that the packages need. The requirements of
// the kernel of c apply, because the main module requires the kernel.
//
// Error modes: the error of git for a tag that it does not archive, the
// errors of extract, and the error of go test. The error of go test contains
// the errors of the go command and of the tests that fail.
func test(ctx context.Context, env []string, c checkout, dir string, m member, version string) ([]byte, error) {
	archive, err := command(ctx, env, c.root, gitBinary, "-c", "core.autocrlf=input", "-c", "core.eol=lf",
		"archive", "--format=tar", tagRefs+path.Join(m.dir, version))
	if err == nil {
		err = extract(bytes.NewReader(archive), dir, path.Join(m.dir, modFile), c)
	}
	if err != nil {
		return nil, err
	}
	return command(ctx, env, filepath.Join(dir, filepath.FromSlash(m.dir)), goBinary, "test", "-mod=mod",
		"-count=1", allPackages)
}

// extract creates dir. It writes the directories and the regular files of
// the tar archive that r reads into dir. goMod is a name of the archive.
// extract writes that file as rewire returns it for c. It writes each file
// with the mode fileMode. The go command does not write an executable file
// into the module cache either. extract skips the global header in which
// git archive records the commit. It writes through an os.Root of dir. The
// root refuses an entry outside dir.
//
// Error modes:
//   - the error of a dir that cannot be created
//   - the error of an archive that does not read
//   - the error of a file that ends before its size
//   - the error of an entry outside dir
//   - the error of a directory or a file that cannot be written
//   - the error of a goMod that does not parse
//   - an error for a symbolic link or another entry that is not a directory
//     or a regular file
func extract(r io.Reader, dir, goMod string, c checkout) error {
	err := os.MkdirAll(dir, dirMode)
	var root *os.Root
	if err == nil {
		root, err = os.OpenRoot(dir)
	}
	if err != nil {
		return fmt.Errorf("%s: %w", commandName, err)
	}
	defer func() { _ = root.Close() }()
	archive := tar.NewReader(r)
	for {
		h, err := archive.Next()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("%s: %w", commandName, err)
		}
		name := filepath.Clean(filepath.FromSlash(h.Name))
		switch h.Typeflag {
		case tar.TypeXGlobalHeader:
			// The header does not have a file.
		case tar.TypeDir:
			err = root.MkdirAll(name, dirMode)
		case tar.TypeReg:
			var data []byte
			data, err = io.ReadAll(archive)
			if err == nil && h.Name == goMod {
				data, err = rewire(h.Name, data, c)
			}
			if err == nil {
				err = root.WriteFile(name, data, fileMode)
			}
		default:
			err = fmt.Errorf("the archive has the entry %s of the type %q instead of a directory or a regular file",
				h.Name, h.Typeflag)
		}
		if err != nil {
			return fmt.Errorf("%s: %w", commandName, err)
		}
	}
}

// rewire returns the go.mod data of a release without its replace
// directives. In their place, the go.mod replaces the kernel and the SDK
// facade with their directories in c. name is the file name of data. The
// errors of the parser contain it.
//
// Each replace directive of the release applies to the build, because the
// release is the main module of go test. A consumer of the release does not
// read the directives. A directive into the tree of the release builds
// against the kernel of the release.
//
// Error modes: the error of data that does not parse. The edits of modfile
// do not return an error for the directories of a checkout.
func rewire(name string, data []byte, c checkout) ([]byte, error) {
	f, err := modfile.Parse(name, data, nil)
	if err != nil {
		return nil, fmt.Errorf("the go.mod of the release: %w", err)
	}
	// DropReplace clears every directive of one old version. The loop reads
	// the old versions before the first drop.
	olds := make([]module.Version, 0, len(f.Replace))
	for _, r := range f.Replace {
		olds = append(olds, r.Old)
	}
	for _, old := range olds {
		err = errors.Join(err, f.DropReplace(old.Path, old.Version))
	}
	err = errors.Join(err, f.AddReplace(kernelModule, "", c.kernel, ""), f.AddReplace(facadeModule, "", c.facade, ""))
	f.Cleanup()
	return modfile.Format(f.Syntax), err
}

// command runs program with args in dir under ctx. The program runs with
// the environment env and without a workspace. command returns what the
// program writes to its standard output. The error of a program that does
// not succeed contains what the program writes to its standard error.
func command(ctx context.Context, env []string, dir, program string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, program, args...)
	cmd.Dir = dir
	cmd.Env = append(slices.Clip(env), noWorkspace)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return out, fmt.Errorf("%s: %s %s: %w\n%s", commandName, program, strings.Join(args, " "), err,
			stderr.Bytes())
	}
	return out, nil
}
