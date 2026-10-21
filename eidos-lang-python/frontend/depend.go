// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend

import (
	"context"
	"fmt"
	"path"
	"strings"

	"go.dokimi.dev/eidos/lang/python"
	"go.dokimi.dev/eidos/lang/python/frontend/typeshed"
	"go.dokimi.dev/eidos/lang/python/internal/project"
	"go.dokimi.dev/eidos/sdk/plugin"
)

// venvDir is the directory of the virtual environment of a project, which
// uv and the venv module of Python create beside its project file.
const venvDir = ".venv"

// The reasons of a need that no step places, which the finding of the load
// quotes.
const (
	unplacedReason = "neither typeshed for Python 3.%d nor a site-packages directory has the module"
	noVenvReason   = "typeshed for Python 3.%d does not have the module, and the load reads no virtual environment"
)

// readVersions reads the table of versions of the typeshed store, and
// returns the empty table for a load without the store.
//
// Error modes: readVersions returns the error of a read that fails for
// another cause than an absent store, and the error of a table that does
// not parse.
func readVersions(r plugin.StoreReader) (typeshed.Versions, error) {
	data, err := r.Read(plugin.StorePath(TypeshedStore, typeshed.VersionsFile))
	if plugin.Absent(err) {
		return typeshed.Versions{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("frontend: %w", err)
	}
	return typeshed.ParseVersions(data)
}

// siteRoots returns the site-packages directories of the round: the root
// of the venv store where the load provides it, and the site-packages
// directory of the .venv beside each project file of the shared inputs,
// with the version of its pyvenv.cfg, below lib or Lib.
//
// Error modes: siteRoots returns the error of a read or a listing that
// fails for another cause than its absence.
func siteRoots(r plugin.StoreReader, shared []string) ([]string, error) {
	var roots []string
	venv := plugin.StorePath(VenvStore, "")
	if _, err := r.ReadDir(venv); err == nil {
		roots = append(roots, venv)
	} else if !plugin.Absent(err) {
		return nil, fmt.Errorf("frontend: %w", err)
	}
	for _, p := range shared {
		if path.Base(p) != project.File {
			continue
		}
		env := path.Join(path.Dir(p), venvDir)
		cfg, err := r.Read(path.Join(env, venvConfig))
		if plugin.Absent(err) {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("frontend: %w", err)
		}
		version, stated := venvVersion(cfg)
		if !stated {
			continue
		}
		for _, site := range [...]string{
			path.Join(env, libDir, pythonPrefix+version, sitePackages), path.Join(env, windowsLibDir, sitePackages),
		} {
			_, err := r.ReadDir(site)
			if err == nil {
				roots = append(roots, site)
				break
			}
			if !plugin.Absent(err) {
				return nil, fmt.Errorf("frontend: %w", err)
			}
		}
	}
	return roots, nil
}

// placeNeed returns the member of the unit that places one need, and
// reports false where no step places it.
//
// Error modes: placeNeed returns the error of a read that fails for another
// cause than an absent file.
func placeNeed(r plugin.StoreReader, need string, minor int, versions typeshed.Versions, sites []string) (
	string, bool, error,
) {
	var candidates []string
	if versions.Includes(need, minor) {
		module := join(plugin.StorePath(TypeshedStore, typeshed.StdlibDir), need)
		candidates = append(candidates, module+python.StubExtension, join(module, project.InitStub))
	}
	head, rest, nested := strings.Cut(need, python.PackageSep)
	for _, site := range sites {
		stubs := join(site, head+stubsSuffix)
		if nested {
			stubs = join(stubs, rest)
		}
		candidates = append(candidates, join(stubs, project.InitStub), stubs+python.StubExtension)
	}
	for _, site := range sites {
		module := join(site, need)
		candidates = append(candidates, join(module, project.InitStub), join(module, project.InitSource),
			module+python.StubExtension, module+python.Extension)
	}
	for _, c := range candidates {
		_, err := r.Read(c)
		switch {
		case err == nil:
			return c, true, nil
		case !plugin.Absent(err):
			return "", false, fmt.Errorf("frontend: %w", err)
		}
	}
	return "", false, nil
}

// join returns the path of a name below a site-packages directory, a root
// of a store or a directory of the workspace.
func join(dir, name string) string {
	if strings.HasSuffix(dir, python.PackageSep) {
		return dir + name
	}
	return dir + python.PackageSep + name
}

// dependencies is the dependency round of the Python frontend. The first
// round places builtins beside its needs, because every module reads the
// builtins. A need is a module that no unit of the workspace declares, and
// the round places it in the order of the module resolution of the typing
// specification:
//
//   - typeshed places a module that its VERSIONS lists, alone or by its
//     package, with a range that contains the version of the options.
//   - A package of stubs alone, <package>-stubs, in a site-packages
//     directory places its modules.
//   - A site-packages directory places the module, the stub before the
//     source, and a package directory before a module file.
//
// The site-packages directories are the venv store and the .venv
// directories beside the project files of the workspace, in that order. A
// dependency unit is one module, and the round reports each need that no
// step places, with its reason. The round does not report a need of the
// module of an output of the round, or of a package above that module,
// because the brand writes the module. An output has the module that the
// partition and the parse give a file of the workspace.
//
// Error modes: dependencies returns an error for options whose version is
// not a version of Python 3, an error that names a file of a store that
// does not read, the error of a VERSIONS file that does not parse, and the
// error of [outputModules].
func (f *pyFrontend) dependencies(
	_ context.Context, round *plugin.DependencyRound, r plugin.StoreReader,
) ([][]plugin.SourceRef, error) {
	minor, _, err := f.target()
	if err != nil {
		return nil, err
	}
	versions, err := readVersions(r)
	if err != nil {
		return nil, err
	}
	sites, err := siteRoots(r, round.Shared)
	if err != nil {
		return nil, err
	}
	reason := fmt.Sprintf(unplacedReason, minor)
	if len(sites) == 0 {
		reason = fmt.Sprintf(noVenvReason, minor)
	}
	var out [][]plugin.SourceRef
	placed := map[string]bool{}
	if round.Number == 1 {
		member, found, err := placeNeed(r, builtinsModule, minor, versions, sites)
		if err != nil {
			return nil, err
		}
		if found {
			placed[builtinsModule] = true
			out = append(out, []plugin.SourceRef{{Path: member}})
		}
	}
	var missing []string
	for _, n := range round.Needs {
		if placed[n.Path] {
			continue
		}
		member, found, err := placeNeed(r, n.Path, minor, versions, sites)
		switch {
		case err != nil:
			return nil, err
		case found:
			placed[n.Path] = true
			out = append(out, []plugin.SourceRef{{Path: member}})
		default:
			missing = append(missing, n.Path)
		}
	}
	if len(missing) == 0 {
		return out, nil
	}
	generated, failed := outputModules(r, round.Outputs)
	if failed != nil {
		return nil, failed
	}
	for _, need := range missing {
		if !generated[need] {
			round.Unplace(need, reason)
		}
	}
	return out, nil
}

// outputModules returns the module of each output of a round and every
// package above the module. [project.Place] places an output as it places
// a file of the workspace. The root of an output is a root that its project
// states, or the directory above the run of package directories that
// contain the output. A directory is a package directory where the door
// reads its __init__.py or its __init__.pyi.
//
// Error modes: outputModules returns the error of a read of the module of a
// package directory that fails for another cause than its absence.
func outputModules(r plugin.StoreReader, outputs []string) (map[string]bool, error) {
	modules := make(map[string]bool, len(outputs))
	files := projectFiles{r: r}
	var failed error
	isPackage := func(dir string) bool {
		for _, name := range [...]string{project.InitSource, project.InitStub} {
			switch _, err := r.Read(path.Join(dir, name)); {
			case err == nil:
				return true
			case !plugin.Absent(err):
				failed = err
				return false
			}
		}
		return false
	}
	for _, o := range outputs {
		_, proj := files.nearest(path.Dir(o))
		root, _ := project.Place(o, proj, isPackage)
		if failed != nil {
			return nil, fmt.Errorf("frontend: %w", failed)
		}
		for m := project.Package(o, root); m != ""; m = parentModule(m) {
			modules[m] = true
		}
	}
	return modules, nil
}
