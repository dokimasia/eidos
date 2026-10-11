// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"golang.org/x/mod/modfile"
)

// The kernel and the SDK facade, which the ring replaces with their
// directories in the checkout and does not test.
const (
	kernelModule = "go.dokimi.dev/eidos/core"
	facadeModule = "go.dokimi.dev/eidos/sdk"
)

// The file of a workspace, the file of a module, and the modes of the
// directory and the file of a scratch module.
const (
	workFile = "go.work"
	modFile  = "go.mod"
	dirMode  = 0o700
	fileMode = 0o600
)

// scratchModule is the module path of a scratch module. The top-level
// domain invalid is reserved, so no published module has the path.
const scratchModule = "ring.invalid/scratch"

// The go command, the environment entry that runs it without the workspace
// of the checkout, the query of the latest version of a module, the format
// in which the ring reads the answer, and the pattern of every package of a
// module.
const (
	goBinary     = "go"
	noWorkspace  = "GOWORK=off"
	latestQuery  = "@latest"
	answerFormat = `{{.Version}}{{"\n"}}{{with .Error}}{{.Err}}{{end}}`
	allPackages  = "/..."
)

// notServed are the status lines of the module proxy protocol for a module
// that the proxy does not serve: 404 Not Found and 410 Gone. The go command
// writes the status line into the error of a query.
var notServed = []string{
	strconv.Itoa(http.StatusNotFound) + " " + http.StatusText(http.StatusNotFound),
	strconv.Itoa(http.StatusGone) + " " + http.StatusText(http.StatusGone),
}

// checkout is the workspace of a checkout, as the ring reads it from the
// go.work of the checkout.
type checkout struct {
	// goVersion is the language version of the go.work, which each scratch
	// module states.
	goVersion string
	// kernel and facade are the directories of the kernel and of the SDK
	// facade.
	kernel, facade string
	// ring are the paths of the modules that require the kernel, without
	// the kernel and the facade, sorted.
	ring []string
}

// read returns the checkout whose go.work is in root. A root that is not
// absolute is relative to dir, an absolute directory such as the working
// directory of the command. read reads the go.work and the go.mod of each
// directory that the go.work uses through an os.Root of root, so it does
// not read a file outside the checkout. A module that has the kernel among
// its requirements is a module of the ring. The directories of the
// checkout are absolute, because a scratch module in another directory
// replaces the kernel and the facade with them.
//
// Error modes:
//   - the error of a root that does not open
//   - the error of a go.work or a go.mod that does not read or parse, such
//     as the go.mod of a directory outside the checkout
//   - an error for a go.work without a go line, and for a go.mod without a
//     module line
//   - an error for a go.work that does not use the kernel, the SDK facade
//     or a module of the ring
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
	if work.Go == nil {
		return checkout{}, fmt.Errorf("%s: the go.work of the checkout has no go line", commandName)
	}
	c := checkout{goVersion: work.Go.Version}
	for _, use := range work.Use {
		dir := filepath.FromSlash(use.Path)
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
			return checkout{}, fmt.Errorf("%s: %s has no module line", commandName, name)
		}
		switch path := mod.Module.Mod.Path; {
		case path == kernelModule:
			c.kernel = filepath.Join(root, dir)
		case path == facadeModule:
			c.facade = filepath.Join(root, dir)
		case slices.ContainsFunc(mod.Require, func(r *modfile.Require) bool { return r.Mod.Path == kernelModule }):
			c.ring = append(c.ring, path)
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
	slices.Sort(c.ring)
	return c, nil
}

// latest returns the answer of the module proxy to the query of the latest
// version of mod. The answer is a version, or the refusal of the query
// where the proxy does not return a version. latest reads the answer as go
// list -m -e <mod>@latest reports it, with the go command in dir and the
// environment env.
//
// Error modes: the error of a go command that does not run or does not
// succeed.
func latest(ctx context.Context, env []string, dir, mod string) (version, refusal string, err error) {
	out, err := goCommand(ctx, env, dir, "list", "-m", "-e", "-f", answerFormat, mod+latestQuery)
	version, refusal, _ = strings.Cut(strings.TrimSuffix(string(out), "\n"), "\n")
	return version, refusal, err
}

// test runs go test over the packages of version of mod against the kernel
// and the SDK facade of the checkout c, and returns the output of go test.
//
// test creates dir and writes a scratch module into it. The scratch module
// has the go line of c and requires version of mod. It replaces the kernel
// and the facade with their directories in c. The go command runs in dir
// with the environment env. It adds the requirements and the sums that the
// packages need.
//
// Error modes: the error of a scratch module that cannot be written, and
// the error of go test. The error of go test contains the errors of the go
// command and of the tests that fail.
func test(ctx context.Context, env []string, c checkout, dir, mod, version string) ([]byte, error) {
	f := new(modfile.File)
	// The arguments run in order, so the scratch module is complete when the
	// last argument writes it.
	err := errors.Join(
		f.AddModuleStmt(scratchModule),
		f.AddGoStmt(c.goVersion),
		f.AddRequire(mod, version),
		f.AddReplace(kernelModule, "", c.kernel, ""),
		f.AddReplace(facadeModule, "", c.facade, ""),
		os.MkdirAll(dir, dirMode),
		os.WriteFile(filepath.Join(dir, modFile), modfile.Format(f.Syntax), fileMode),
	)
	var out []byte
	if err == nil {
		out, err = goCommand(ctx, env, dir, "test", "-mod=mod", "-count=1", mod+allPackages)
	}
	return out, err
}

// goCommand runs the go command with args in dir, under ctx, with the
// environment env and without a workspace, and returns what the command
// writes to its standard output. The error of a command that does not
// succeed contains what the command writes to its standard error.
func goCommand(ctx context.Context, env []string, dir string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, goBinary, args...)
	cmd.Dir = dir
	cmd.Env = append(slices.Clip(env), noWorkspace)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return out, fmt.Errorf("%s: go %s: %w\n%s", commandName, strings.Join(args, " "), err, stderr.Bytes())
	}
	return out, nil
}
