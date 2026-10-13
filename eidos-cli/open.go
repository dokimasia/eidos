// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"cmp"
	"errors"
	"fmt"
	"io"
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
	// source is the path of the config file in its faults, and data is the
	// content of the file. Both are empty when the workspace has no config
	// file.
	source string
	data   []byte
}

// Extension decodes the value of the extension name of the config file of
// m into v, with the strict rules of the file. It reports false where m has
// no config file or the file has no such extension. Extension decodes into
// the value of v, so a field that the extension does not set keeps its
// value. A member of a list reads the extensions of its own config file.
//
// Extension panics for a v that is nil or not a pointer, and for a name
// that is empty, is - or contains a comma.
//
// Error modes: Extension returns an error that wraps one *ConfigError for
// each key that v does not declare and for each value that does not decode.
// The file of each fault is relative to the directory of the config file
// that Open found.
func (m Member) Extension(name string, v any) (bool, error) {
	return config.Extension(m.source, m.data, name, v)
}

// configFile is the config file of one member.
type configFile struct {
	// path is the absolute path of the file, and empty for a workspace
	// without a config file.
	path string
	// source is the path of the file in its faults, relative to the
	// directory of the file that open found, in slash form.
	source string
	data   []byte
	doc    *config.Document
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
// not read or decode, for a plugin directory of the file that does not load,
// for a fault of Build, for a root of a list that does not exist, and for two
// roots of a list that nest. A fault of a file that does not decode names
// the file relative to the directory of the file that Open found, in slash
// form, as a finding names a file.
func Open(stdio IO, f Flags, compose Compose) ([]Member, error) {
	found, err := open(stdio, f, compose, nil)
	if err != nil {
		return nil, err
	}
	return found.members, nil
}

// open finds and builds the workspaces of [Open]. It returns every member
// that built beside the error that joins the faults of the other members,
// so a command such as doctor can continue with the members that built.
// The composition that names the brand becomes the workspace outside a
// list, and each member of a list builds a composition of its own. stream
// is the writer that each workspace writes its files to in place of the
// disk, and nil for the disk.
func open(stdio IO, f Flags, compose Compose, stream io.Writer) (workspaces, error) {
	stdio.check()
	b := compose()
	brand := b.BrandName()
	if !brand.Valid() {
		_, err := b.Build()
		return workspaces{}, err
	}
	dir, err := filepath.EvalSymlinks(stdio.Dir)
	if err != nil {
		return workspaces{}, fmt.Errorf("cli: resolve the working directory: %w", err)
	}
	name := ConfigName(brand)
	file, data, root, err := locate(dir, f.Config, name)
	if err != nil {
		return workspaces{dir: dir}, err
	}
	c := configFile{path: file, data: data}
	var decoded config.File
	if file != "" {
		c.source = relative(root, file)
		if decoded, err = config.Decode(c.source, data); err != nil {
			return workspaces{dir: dir}, err
		}
	}
	if decoded.List != nil {
		var members []Member
		members, err = list(compose, brand, root, name, decoded.List, stream)
		return workspaces{dir: dir, members: members}, err
	}
	c.doc = decoded.Document
	m, err := member(b, brand, "", root, c, stream)
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
// name is the file name of the config file of the brand. stream is the
// writer of the files of each member, nil for the disk, as [member] takes
// it. list returns the members that built beside the error that joins the
// faults of the others. It builds no member when a root does not exist or
// two roots nest.
func list(
	compose Compose, brand output.Brand, dir, name string, l *config.List, stream io.Writer,
) ([]Member, error) {
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
		c, err := entryConfig(roots[i], dir, name, e.Config)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		m, err := member(compose(), brand, path.Clean(e.Root), roots[i], c, stream)
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
// does not exist. A fault of the file names the file relative to dir.
func entryConfig(root, dir, name, given string) (configFile, error) {
	file := filepath.Join(root, name)
	if given != "" {
		file = resolve(dir, given)
	}
	data, err := os.ReadFile(file)
	if given == "" && errors.Is(err, fs.ErrNotExist) {
		return configFile{}, nil
	}
	if err != nil {
		return configFile{}, fmt.Errorf("cli: read the config file: %w", err)
	}
	c := configFile{path: file, source: relative(dir, file), data: data}
	decoded, err := config.Decode(c.source, data)
	if err != nil {
		return configFile{}, err
	}
	if decoded.List != nil {
		return configFile{}, fmt.Errorf("cli: the config file %s of a workspace in a list is a list", c.source)
	}
	c.doc = decoded.Document
	return c, nil
}

// member configures and builds one workspace from the composition b, which
// no other member uses. c is the config file of the workspace, whose
// document is nil when the workspace has none. Where stream is nil, the
// workspace writes its output into root and its state into the state
// directory of brand in root. Otherwise it writes its files to stream, each
// under a header that starts with the name of the member, and keeps no
// state.
func member(
	b *workspace.Builder, brand output.Brand, name, root string, c configFile, stream io.Writer,
) (Member, error) {
	if c.doc != nil {
		if err := c.doc.Apply(b, root); err != nil {
			return Member{}, err
		}
	}
	if stream == nil {
		// A run reads the sink and the ledger only when the error is nil.
		b.Output(func() (output.Sink, error) { return output.NewDisk(root, brand) })
		b.Ledger(func() (ledger.Ledger, error) { return ledger.OpenDir(root, brand) })
	} else {
		prefix := name
		if prefix != "" {
			prefix += "/"
		}
		b.Output(output.NewStream(stream, prefix).Open)
		b.Ledger(nil)
	}
	w, err := b.Build()
	if err != nil {
		return Member{}, err
	}
	return Member{Name: name, Root: root, Config: c.path, Workspace: w, source: c.source, data: c.data}, nil
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

// relative returns the path of file relative to the directory dir in slash
// form, the form in which a finding names a file of a workspace. A config
// error names its file this way, so its text does not depend on where the
// workspace is. relative returns file unchanged when file has no path
// relative to dir, such as a file on another volume of Windows.
func relative(dir, file string) string {
	rel, err := filepath.Rel(dir, file)
	if err != nil {
		return file
	}
	return filepath.ToSlash(rel)
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

// below returns the clean absolute path at relative to the directory root
// in slash form, and reports whether at is root or a path below it. It
// looks for root among the directories above at by file identity, so a
// path to root through a symbolic link, in another case on a file system
// that ignores case, or through a short name on Windows relates to root.
// below keeps the elements of at below root as written. They need not
// exist, so a pattern can end in its wildcard.
func below(root, at string) (string, bool) {
	rootInfo, err := os.Stat(root)
	if err != nil {
		return "", false
	}
	rest := ""
	for p := at; ; {
		if info, err := os.Stat(p); err == nil && os.SameFile(rootInfo, info) {
			return filepath.ToSlash(cmp.Or(rest, ".")), true
		}
		parent := filepath.Dir(p)
		if parent == p {
			return "", false
		}
		rest = filepath.Join(filepath.Base(p), rest)
		p = parent
	}
}
