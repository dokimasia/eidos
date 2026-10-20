// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend

import (
	"context"
	"path"
	"slices"
	"strings"
	"sync"

	"go.dokimi.dev/eidos/lang/python"
	"go.dokimi.dev/eidos/lang/python/frontend/typeshed"
	"go.dokimi.dev/eidos/lang/python/internal/project"
	"go.dokimi.dev/eidos/sdk/meta"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/position"
)

// The directory of the installed distributions of an environment, and the
// suffix of the name of a package of stubs alone, as PEP 561 names it.
const (
	sitePackages = "site-packages"
	stubsSuffix  = "-stubs"
)

// place is where one unit is in its workspace: the package of its module,
// whether the module is the module of a package directory, and the project
// that governs it, with the module and the root of the project.
type place struct {
	pkg     string
	init    bool
	project *project.Project
	name    string
}

// dependencyPlace returns the place of a member of a dependency below the
// root root: the package of its module, without the suffix -stubs of the
// first name of a stubs package.
func dependencyPlace(file, root string) place {
	pkg := project.Package(file, root)
	head, rest, nested := strings.Cut(pkg, python.PackageSep)
	if base, stubs := strings.CutSuffix(head, stubsSuffix); stubs {
		pkg = base
		if nested {
			pkg = base + python.PackageSep + rest
		}
	}
	return place{pkg: pkg, init: strings.TrimSuffix(path.Base(file), path.Ext(file)) == python.InitModule}
}

// projectEntry is the decode of one content of a project file: the
// content, the project, and the error of the decode.
type projectEntry struct {
	data    string
	project project.Project
	err     error
}

// projectMemo keeps the decode of each project file for the content that
// the last parse read, so the units of one project decode its file once. It
// keeps one content of each path, and decodes a content again after the
// file changes. The zero memo is empty and ready for use.
//
// # Concurrency
//
// A memo is safe for concurrent use. A decode runs under its lock.
type projectMemo struct {
	mu      sync.Mutex
	entries map[string]projectEntry
}

// read returns the project of the content data of the project file at p,
// and the error of its decode.
//
// Error modes: read returns the error of [project.Read] for a content that
// does not decode.
//
// # Allocation contract
//
// A content that the memo keeps allocates nothing. Any other content
// allocates its decode, the copy of the content and the entry.
func (m *projectMemo) read(p string, data []byte) (project.Project, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if e, kept := m.entries[p]; kept && e.data == string(data) {
		return e.project, e.err
	}
	proj, err := project.Read(p, data)
	if m.entries == nil {
		m.entries = map[string]projectEntry{}
	}
	m.entries[p] = projectEntry{data: string(data), project: proj, err: err}
	return proj, err
}

// parse loads one unit, a module, into the builder of the unit. The unit
// lowers its stub where it has one, and its source otherwise. The source of
// a pair loads as a File without declarations, stamped python.stubbed. A
// syntax error is a problem of the source: every ERROR and MISSING node
// reports at its position, and every declaration outside them loads. The
// package of every unit has the module of its project, which is the name of
// the project in its project file.
//
// Error modes: parse returns the error of a member that does not read, the
// errors of [pyFrontend.placement], the error of the grammar for a context
// that ended, and an error for options whose version is not a version of
// Python 3.
func (f *pyFrontend) parse(ctx context.Context, u *plugin.SourceUnit) error {
	minor, platform, err := f.target()
	if err != nil {
		return err
	}
	files := u.Files()
	lowered := files[len(files)-1]
	at, err := f.placement(u, lowered)
	if err != nil {
		return err
	}
	gb := u.Graph()
	p := gb.Package(at.pkg)
	if at.pkg != "" {
		p.Name = path.Base(at.pkg)
	}
	for _, ref := range files[:len(files)-1] {
		source := &node.File{Path: ref.Path, Pos: position.Pos{File: ref.Path, Line: 1, Col: 1}}
		p.Files = append(p.Files, source)
		gb.Stamp(source, meta.RawStamp{Key: python.StubbedKey, Value: true, Pos: source.Pos})
	}
	src, err := u.Read(lowered.Path)
	if err != nil {
		return err
	}
	l := newLowering(ctx, u, f.v, lowered.Path, at, minor, platform)
	l.parsed(src, l.lower)
	if l.failed != nil {
		return l.failed
	}
	if at.project != nil {
		gb.StampModule(p, at.name, at.project.Dir)
	}
	return nil
}

// placement returns the place of a unit whose member ref the unit lowers.
// The members of a unit share their shared inputs, and the first of them is
// the project file where the partition found one. The place reads the
// project file again through the door of the unit, so the read folds into
// the key of the unit, and the memo of the frontend decodes it. A project
// file that does not read or decode reports under [UnreadProject], and its
// files take the root of the run of their package directories, which the
// remaining shared inputs list. A member of a store or below a
// site-packages directory is a dependency, whose root is the root of its
// store or the site-packages directory, and whose stubs package names the
// package of its module.
//
// Error modes: placement returns the error of a read of the module of a
// package directory that fails for another cause than its absence.
func (f *pyFrontend) placement(u *plugin.SourceUnit, ref plugin.SourceRef) (place, error) {
	file := ref.Path
	if store, inner, qualified := plugin.CutStorePath(file); qualified {
		root := "."
		if store == TypeshedStore {
			root = typeshed.StdlibDir
		}
		return dependencyPlace(inner, root), nil
	}
	offset := 0
	for segment := range strings.SplitSeq(file, python.PackageSep) {
		offset += len(segment)
		if segment == sitePackages {
			return dependencyPlace(file, file[:offset]), nil
		}
		offset += len(python.PackageSep)
	}
	var at place
	shared := ref.Shared
	if len(shared) > 0 && path.Base(shared[0]) == project.File {
		data, err := u.Read(shared[0])
		var p project.Project
		if err == nil {
			p, err = f.projects.read(shared[0], data)
		}
		if err != nil {
			u.Warnf(UnreadProject, position.Pos{File: shared[0], Line: 1, Col: 1},
				"%v. Its files take the root of the run of their package directories", err)
		} else {
			at.project = &p
			at.name = p.Name
		}
		shared = shared[1:]
	}
	var failed error
	root, _ := project.Place(file, at.project, func(dir string) bool {
		for _, name := range [...]string{project.InitSource, project.InitStub} {
			probe := path.Join(dir, name)
			if !slices.Contains(shared, probe) {
				continue
			}
			switch _, err := u.Read(probe); {
			case err == nil:
				return true
			case !plugin.Absent(err):
				failed = err
				return false
			}
		}
		return false
	})
	if failed != nil {
		return place{}, failed
	}
	at.pkg = project.Package(file, root)
	at.init = strings.TrimSuffix(path.Base(file), path.Ext(file)) == python.InitModule
	return at, nil
}
