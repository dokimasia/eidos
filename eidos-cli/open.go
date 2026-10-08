// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"

	"go.dokimi.dev/eidos/cli/internal/config"
	"go.dokimi.dev/eidos/core/ledger"
	"go.dokimi.dev/eidos/core/output"
	"go.dokimi.dev/eidos/core/workspace"
)

// configExt is the extension of a config file. The name of the config file
// of a brand is the name of its state directory and the extension, such as
// .acme.yaml.
const configExt = ".yaml"

// markers contains the names that mark the root of a repository of a
// version control system.
var markers = [...]string{".git", ".hg", ".jj", ".svn"}

// Member is one workspace that [Open] built.
type Member struct {
	// Name is the root of a member of a list, as the list writes it, with
	// slashes and without a leading ./, such as tools/gen. It is empty for a
	// workspace outside a list.
	Name string
	// Root is the absolute workspace root, with symbolic links resolved.
	Root string
	// Config is the absolute path of the config file of the workspace, and
	// empty when the workspace has none.
	Config string
	// Workspace is the configured and built composition.
	Workspace *workspace.Workspace
}

// workspaces is what open found for a working directory.
type workspaces struct {
	// dir is the working directory, with symbolic links resolved.
	dir string
	// members are the members that built, in list order.
	members []Member
}

// Open finds the config file for stdio.Dir, or reads f.Config when it is not
// empty. It applies the file to the compositions that compose returns, and
// builds one member for each workspace that the file covers. A list covers
// its entries in list order, and any other file covers one workspace. The
// workspace of each member writes its output and its state directory below
// its root.
//
// Without --config, Open searches stdio.Dir and then each parent directory
// for the config file of the brand, such as .acme.yaml. The directory of the
// file is the root. The search stops after the first directory that
// contains a version control marker: .git, .hg, .jj or .svn. Without a file,
// the root is the directory with the marker, or stdio.Dir outside a
// repository.
//
// Open panics unless stdio has both output streams, Getenv and an absolute
// Dir.
//
// Error modes: every error is a config error, which a command reports with
// [StatusUsage]. The error joins the faults of every member. Open returns an
// error for a working directory that does not resolve, for a file that does
// not read or decode, for a fault of Build, for a root of a list that does
// not exist, and for two roots of a list that nest.
func Open(stdio IO, f Flags, compose Compose) ([]Member, error) {
	found, err := open(stdio, f, compose)
	if err != nil {
		return nil, err
	}
	return found.members, nil
}

// open finds and builds the workspaces of [Open]. It returns every member
// that built beside the error that joins the faults of the other members,
// so a command such as doctor can continue with the members that built.
func open(stdio IO, f Flags, compose Compose) (workspaces, error) {
	stdio.check()
	brand := compose().BrandName()
	if !brand.Valid() {
		_, err := compose().Build()
		return workspaces{}, err
	}
	dir, err := filepath.EvalSymlinks(stdio.Dir)
	if err != nil {
		return workspaces{}, fmt.Errorf("cli: resolve the working directory: %w", err)
	}
	name := ledger.StateDir(brand) + configExt
	file, data, root, err := locate(dir, f.Config, name)
	if err != nil {
		return workspaces{dir: dir}, err
	}
	var decoded config.File
	if file != "" {
		if decoded, err = config.Decode(file, data); err != nil {
			return workspaces{dir: dir}, err
		}
	}
	if decoded.List != nil {
		var members []Member
		members, err = list(compose, brand, root, name, decoded.List)
		return workspaces{dir: dir, members: members}, err
	}
	m, err := member(compose, brand, "", root, file, decoded.Document)
	if err != nil {
		return workspaces{dir: dir}, err
	}
	return workspaces{dir: dir, members: []Member{m}}, nil
}

// locate returns the absolute path and the bytes of the config file, and
// the workspace root. dir is the working directory, and given is the path
// that --config sets, relative to dir. name is the file name of the config
// file of the brand. The path is empty when the search finds no file.
func locate(dir, given, name string) (string, []byte, string, error) {
	if given != "" {
		file, err := filepath.EvalSymlinks(resolve(dir, given))
		if err != nil {
			return "", nil, "", fmt.Errorf("cli: read the config file: %w", err)
		}
		data, err := os.ReadFile(file)
		if err != nil {
			return "", nil, "", fmt.Errorf("cli: read the config file: %w", err)
		}
		return file, data, filepath.Dir(file), nil
	}
	for d := dir; ; d = filepath.Dir(d) {
		file := filepath.Join(d, name)
		data, err := os.ReadFile(file)
		if err == nil {
			return file, data, d, nil
		}
		if !errors.Is(err, fs.ErrNotExist) {
			return "", nil, "", fmt.Errorf("cli: read the config file: %w", err)
		}
		if marked(d) {
			return "", nil, d, nil
		}
		if filepath.Dir(d) == d {
			return "", nil, dir, nil
		}
	}
}

// list builds the members of a list. dir is the directory of the list, and
// name is the file name of the config file of the brand. list returns the
// members that built beside the error that joins the faults of the others.
// It builds no member when a root does not exist or two roots nest.
func list(compose Compose, brand output.Brand, dir, name string, l *config.List) ([]Member, error) {
	roots := make([]string, 0, len(l.Workspaces))
	var errs []error
	for i, e := range l.Workspaces {
		root, err := filepath.EvalSymlinks(resolve(dir, e.Root))
		roots = append(roots, root)
		if err != nil {
			errs = append(errs, fmt.Errorf("cli: the workspace root %s does not resolve: %w", e.Root, err))
			continue
		}
		for j, other := range roots[:i] {
			if other != "" && (inside(other, root) || inside(root, other)) {
				errs = append(errs, fmt.Errorf("cli: the workspace roots %s and %s nest", l.Workspaces[j].Root, e.Root))
			}
		}
	}
	if len(errs) > 0 {
		return nil, errors.Join(errs...)
	}
	members := make([]Member, 0, len(l.Workspaces))
	for i, e := range l.Workspaces {
		file, doc, err := entryConfig(roots[i], dir, name, e.Config)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		m, err := member(compose, brand, path.Clean(e.Root), roots[i], file, doc)
		if err != nil {
			errs = append(errs, fmt.Errorf("cli: the workspace %s does not build", e.Root), err)
			continue
		}
		members = append(members, m)
	}
	return members, errors.Join(errs...)
}

// entryConfig reads and decodes the config file of an entry of a list. root
// is the root of the entry, dir is the directory of the list, and given is
// the path that the entry sets. When given is empty, the file is the config
// file of the brand in the root, and the entry has no config when that file
// does not exist.
func entryConfig(root, dir, name, given string) (string, *config.Document, error) {
	file := filepath.Join(root, name)
	if given != "" {
		file = resolve(dir, given)
	}
	data, err := os.ReadFile(file)
	if given == "" && errors.Is(err, fs.ErrNotExist) {
		return "", nil, nil
	}
	if err != nil {
		return "", nil, fmt.Errorf("cli: read the config file: %w", err)
	}
	decoded, err := config.Decode(file, data)
	if err != nil {
		return "", nil, err
	}
	if decoded.List != nil {
		return "", nil, fmt.Errorf("cli: the config file %s of a workspace in a list is a list", file)
	}
	return file, decoded.Document, nil
}

// member configures and builds one workspace. doc is the document of the
// config file of the workspace, and nil when the workspace has none. The
// workspace writes its output into root and its state into the state
// directory of brand in root.
func member(compose Compose, brand output.Brand, name, root, file string, doc *config.Document) (Member, error) {
	b := compose()
	if doc != nil {
		doc.Apply(b, root)
	}
	// A run reads the sink and the ledger only when the error is nil.
	b.Output(func() (output.Sink, error) { return output.NewDisk(root, brand) })
	b.Ledger(func() (ledger.Ledger, error) { return ledger.OpenDir(root, brand) })
	w, err := b.Build()
	if err != nil {
		return Member{}, err
	}
	return Member{Name: name, Root: root, Config: file, Workspace: w}, nil
}

// resolve returns the path p of a config file or a command line as an
// absolute path of the system. A relative p is relative to dir.
func resolve(dir, p string) string {
	p = filepath.FromSlash(p)
	if filepath.IsAbs(p) {
		return filepath.Clean(p)
	}
	return filepath.Join(dir, p)
}

// marked reports whether dir contains a version control marker.
func marked(dir string) bool {
	for _, m := range markers {
		if _, err := os.Lstat(filepath.Join(dir, m)); err == nil {
			return true
		}
	}
	return false
}

// inside reports whether the path p is the directory dir or a path below
// it. Both paths are clean and absolute.
func inside(dir, p string) bool {
	sep := string(filepath.Separator)
	return p == dir || strings.HasPrefix(p, strings.TrimSuffix(dir, sep)+sep)
}
