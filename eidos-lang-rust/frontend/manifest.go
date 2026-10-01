// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend

import (
	"maps"
	"path"
	"slices"
	"strings"

	"github.com/BurntSushi/toml"
)

// The names Cargo's layout is spelled with: the manifest, the
// directories its targets are in, the crate root files, and the build
// script.
const (
	manifestName  = "Cargo.toml"
	srcDir        = "src"
	binDir        = "src/bin"
	testsDir      = "tests"
	examplesDir   = "examples"
	benchesDir    = "benches"
	libRoot       = "src/lib.rs"
	mainRoot      = "src/main.rs"
	mainFile      = "main.rs"
	modFile       = "mod.rs"
	buildScript   = "build.rs"
	buildCrate    = "build_script_build"
	rustExtension = ".rs"
)

// role is what a crate target is to Cargo.
type role uint8

// The crate targets Cargo builds, and the shared modules under a test,
// example or bench directory that no target roots, which load as units
// of their own.
const (
	roleLib     role = 1
	roleBin     role = 2
	roleTest    role = 3
	roleExample role = 4
	roleBench   role = 5
	roleBuild   role = 6
	roleShared  role = 7
)

// declared is one target a manifest declares: its name, and the path of
// its root relative to the manifest's directory.
type declared struct {
	Name string `toml:"name"`
	Path string `toml:"path"`
}

// target is one crate a package builds: its role, its crate name, and
// the path of its root relative to the manifest's directory.
type target struct {
	role role
	name string
	root string
}

// tests reports whether every file of a target is a test file: an
// integration test's, and a shared module's under the tests directory.
func (t target) tests() bool {
	return t.role == roleTest || t.role == roleShared && strings.HasPrefix(t.root, testsDir+"/")
}

// manifest is the part of a Cargo.toml the frontend reads: the package's
// name and the targets it declares explicitly.
type manifest struct {
	Package struct {
		Name string `toml:"name"`
	} `toml:"package"`
	Lib     *declared  `toml:"lib"`
	Bin     []declared `toml:"bin"`
	Test    []declared `toml:"test"`
	Example []declared `toml:"example"`
	Bench   []declared `toml:"bench"`
}

// parseManifest decodes a Cargo.toml.
func parseManifest(data []byte) (*manifest, error) {
	var m manifest
	if err := toml.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	return &m, nil
}

// crateName returns the package's name as a crate name: hyphens replaced
// by underscores, which is Cargo's rule.
func (m *manifest) crateName() string {
	return strings.ReplaceAll(m.Package.Name, "-", "_")
}

// targets returns the crate targets of a package whose files, relative
// to the manifest's directory, are rel: the library, at [lib] path or
// src/lib.rs, the binaries, tests, examples and benches the manifest
// declares at a path and the ones Cargo's layout places, and the build
// script. A library takes [lib] name or the crate name, and every other
// target its own name: a declared one's, a file's stem, or its
// directory's for a main.rs. A target the manifest declares without a
// path is at the root Cargo's layout places under its name. A root the
// manifest declares comes before the same root Cargo's layout places,
// so its declared name is the one [owner] finds.
func (m *manifest) targets(rel map[string]bool) []target {
	var out []target
	add := func(t target) {
		if rel[t.root] {
			out = append(out, t)
		}
	}
	lib := target{role: roleLib, name: m.crateName(), root: libRoot}
	if m.Lib != nil {
		if m.Lib.Path != "" {
			lib.root = path.Clean(m.Lib.Path)
		}
		if m.Lib.Name != "" {
			lib.name = m.Lib.Name
		}
	}
	add(lib)
	for _, kind := range []struct {
		role     role
		dir      string
		declared []declared
	}{
		{roleBin, binDir, m.Bin},
		{roleTest, testsDir, m.Test},
		{roleExample, examplesDir, m.Example},
		{roleBench, benchesDir, m.Bench},
	} {
		for _, d := range kind.declared {
			if d.Path != "" {
				add(target{role: kind.role, name: d.Name, root: path.Clean(d.Path)})
			}
		}
		if kind.role == roleBin {
			add(target{role: roleBin, name: m.crateName(), root: mainRoot})
		}
		for _, f := range slices.Sorted(maps.Keys(rel)) {
			if name, auto := autoTarget(kind.dir, f); auto {
				add(target{role: kind.role, name: name, root: f})
			}
		}
	}
	add(target{role: roleBuild, name: buildCrate, root: buildScript})
	return out
}

// autoTarget reports whether a file is a target root Cargo's layout
// places in a target directory: dir/<name>.rs, or dir/<name>/main.rs,
// and returns the target's name.
func autoTarget(dir, f string) (string, bool) {
	rest, under := strings.CutPrefix(f, dir+"/")
	if !under {
		return "", false
	}
	if name, file := strings.CutSuffix(rest, rustExtension); file && !strings.Contains(name, "/") {
		return name, true
	}
	if name, main := strings.CutSuffix(rest, "/"+mainFile); main && !strings.Contains(name, "/") {
		return name, true
	}
	return "", false
}

// owner returns the target a package file belongs to, in this order: the
// target it roots, the target whose main.rs is in a directory outside src
// that contains the file, as under src/bin and tests, a shared module for
// a file under a test, example or bench directory, named for the directory
// below that one, and the target [home] places it in. Any other file is a
// target of its own, named for its path.
func owner(targets []target, crate, f string) target {
	for _, t := range targets {
		if t.root == f {
			return t
		}
	}
	for _, t := range targets {
		dir := path.Dir(t.root)
		if path.Base(t.root) == mainFile && dir != srcDir && strings.HasPrefix(f, dir+"/") {
			return t
		}
	}
	for _, kind := range []string{testsDir, examplesDir, benchesDir} {
		if rest, under := strings.CutPrefix(f, kind+"/"); under {
			dir, _, _ := strings.Cut(rest, "/")
			name := path.Join(kind, dir)
			return target{role: roleShared, name: path.Join(crate, name), root: name}
		}
	}
	if t, placed := home(targets, f); placed {
		return t
	}
	name := strings.TrimSuffix(f, rustExtension)
	return target{role: roleShared, name: path.Join(crate, name), root: f}
}

// home returns the target whose module tree Cargo's layout places a file
// in, because Cargo finds a crate's modules below its root's directory:
// the library for a file below the directory of its root, src in Cargo's
// layout, and the src/main.rs binary for a file under src. Where both
// directories contain the file, the deeper one's target takes it, and the
// library takes a tie. Neither takes a file under src/bin, which Cargo's
// layout reserves for binaries.
func home(targets []target, f string) (target, bool) {
	if strings.HasPrefix(f, binDir+"/") {
		return target{}, false
	}
	var found target
	depth := -1
	for _, t := range targets {
		if t.role != roleLib && t.root != mainRoot {
			continue
		}
		dir := path.Dir(t.root)
		if dir == rootDir {
			dir = ""
		}
		// Two directories that both contain the file nest, so the longer
		// one is the deeper, and the package's own directory the
		// shallowest.
		if (dir == "" || strings.HasPrefix(f, dir+"/")) && len(dir) > depth {
			found, depth = t, len(dir)
		}
	}
	return found, depth >= 0
}
