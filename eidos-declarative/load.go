// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package declarative

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	eidos "go.dokimi.dev/eidos/core"
	"go.dokimi.dev/eidos/core/directive"
	"go.dokimi.dev/eidos/core/plugin"
)

// The names in a plugin directory that the loader reads: the manifest, and
// the directory of the template trees.
const (
	manifestName = "plugin.yaml"
	templatesDir = "templates"
)

// versionSep separates the version of the manifest from the digest of the
// plugin directory, as build metadata does in Semantic Versioning.
const versionSep = "+"

// templateFile is one file of a template tree.
type templateFile struct {
	// path is the slash-separated path of the file below templates, which
	// starts with the name of its tree, as in golang/method.tmpl.
	path string
	// sum is the SHA-256 of the file's bytes.
	sum [sha256.Size]byte
}

// templates is what the loader read below the templates directory of a
// plugin directory: the targets of the template trees, and each file of
// each tree with the SHA-256 of its bytes.
type templates struct {
	// trees are the names of the directories directly below templates,
	// which are the targets of the trees, in lexical order.
	trees []string
	// files are the files of the trees, in the lexical order of their
	// slash-separated paths below templates.
	files []templateFile
}

// scan reads the template trees of the plugin directory dir through
// [rooted]. A plugin without a templates directory has no tree. scan
// returns an Error without a line for a file directly below templates, for
// a directory that does not read, and for a file that does not read, such
// as a symbolic link that leaves the plugin directory.
func scan(dir string) (templates, []*Error) {
	fsys := rooted{dir: dir}
	entries, err := fs.ReadDir(fsys, templatesDir)
	if errors.Is(err, fs.ErrNotExist) {
		return templates{}, nil
	}
	if err != nil {
		return templates{}, []*Error{{File: filepath.Join(dir, templatesDir), Msg: err.Error()}}
	}
	var t templates
	var faults []*Error
	for _, entry := range entries {
		root := path.Join(templatesDir, entry.Name())
		if !entry.IsDir() {
			faults = append(faults, &Error{
				File: filepath.Join(dir, filepath.FromSlash(root)),
				Msg:  "the file is directly below " + templatesDir + ", and each file belongs to the tree of a target",
			})
			continue
		}
		t.trees = append(t.trees, entry.Name())
		// The callback returns no error, so the walk does not fail.
		_ = fs.WalkDir(fsys, root, func(p string, d fs.DirEntry, err error) error {
			if err == nil && !d.IsDir() {
				var body []byte
				if body, err = fs.ReadFile(fsys, p); err == nil {
					t.files = append(t.files, templateFile{
						path: strings.TrimPrefix(p, templatesDir+"/"), sum: sha256.Sum256(body),
					})
				}
			}
			if err != nil {
				faults = append(faults, &Error{File: filepath.Join(dir, filepath.FromSlash(p)), Msg: err.Error()})
			}
			return nil
		})
	}
	slices.SortFunc(t.files, func(a, b templateFile) int { return strings.Compare(a.path, b.path) })
	return t, faults
}

// contains reports whether the tree of target contains the file at the
// slash-separated path name. It searches the sorted files, in O(log n).
func (t templates) contains(target, name string) bool {
	_, found := slices.BinarySearchFunc(t.files, target+"/"+name, func(f templateFile, p string) int {
		return strings.Compare(f.path, p)
	})
	return found
}

// check returns an Error at line in file for a template path that is no
// slash-separated path inside a tree, for a path without a tree, and for
// each tree that does not contain the template. what describes the template
// in the message, such as "the rule's template". An empty path is no
// template, and check returns nothing for it.
func (t templates) check(file string, line int, what, name string) []*Error {
	if name == "" {
		return nil
	}
	if !fs.ValidPath(name) {
		return []*Error{{
			File: file, Line: line, Msg: fmt.Sprintf("%s %q is not a slash-separated path inside a tree", what, name),
		}}
	}
	if len(t.trees) == 0 {
		return []*Error{
			{File: file, Line: line, Msg: fmt.Sprintf("the plugin has no template tree for %s %q", what, name)},
		}
	}
	var out []*Error
	for _, tree := range t.trees {
		if !t.contains(tree, name) {
			out = append(out, &Error{
				File: file, Line: line, Msg: fmt.Sprintf("the tree %s does not contain %s %q", tree, what, name),
			})
		}
	}
	return out
}

// digest returns the hexadecimal SHA-256 of the fold of a plugin directory:
// the bytes of the manifest, then the path and the SHA-256 of each template
// file in order, each part behind its length.
func (t templates) digest(manifest []byte) string {
	fold := appendPart(nil, manifest)
	for _, f := range t.files {
		fold = appendPart(fold, f.path)
		fold = appendPart(fold, f.sum[:])
	}
	sum := sha256.Sum256(fold)
	return hex.EncodeToString(sum[:])
}

// Load reads the plugin directory at the absolute path dir. It reads the
// manifest plugin.yaml and the template trees below templates, and returns
// the generator that [eidos.NewPlugin] builds from the manifest. Each call
// returns a new generator, because a plugin instance belongs to one
// workspace.
//
// Load opens every file through [os.OpenInRoot], with dir as the root. The
// generator reads its templates the same way at each run, so an edit of a
// template takes effect at the next run, and the run after the edit runs
// cold. The version of the generator is the version of the manifest, a plus
// sign, and the hexadecimal SHA-256 of the manifest and of every template
// file, so an edit of either changes the fingerprint of the composition.
//
// Error modes: Load returns an error that joins one *[Error] for each fault
// of the directory, sorted by file and line. A directory without
// plugin.yaml is one Error at the directory, and so is a relative path. A
// manifest that does not decode returns the faults of the decoder alone.
// Otherwise Load returns every fault of the manifest's checks and of the
// template trees together.
func Load(dir string) (plugin.Generator, error) {
	if !filepath.IsAbs(dir) {
		return nil, &Error{File: dir, Msg: "the plugin directory is not an absolute path"}
	}
	file := filepath.Join(dir, manifestName)
	data, err := fs.ReadFile(rooted{dir: dir}, manifestName)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, &Error{File: dir, Msg: "the directory has no " + manifestName}
	}
	if err != nil {
		return nil, &Error{File: file, Msg: err.Error()}
	}
	m, faults := decode(file, data)
	if len(faults) > 0 {
		return nil, joined(faults)
	}
	tpl, faults := scan(dir)
	faults = append(faults, m.check(file, tpl)...)
	if len(faults) > 0 {
		return nil, joined(faults)
	}
	return m.build(dir, m.Version+versionSep+tpl.digest(data), tpl.trees), nil
}

// appendPart appends one part to dst behind its length, eight bytes in
// little-endian order, so two sequences of parts cannot trade bytes, and
// returns the extended buffer. The workspace folds a template tree the
// same way. appendPart allocates only where dst lacks the room.
func appendPart[B ~string | ~[]byte](dst []byte, part B) []byte {
	dst = binary.LittleEndian.AppendUint64(dst, uint64(len(part)))
	return append(dst, part...)
}

// build returns the generator of the manifest, which the authoring surface
// builds. dir is the plugin directory, version the version of the plugin
// with its digest, and trees the targets of the template trees. The
// manifest passed its checks, so Build does not panic on its declaration.
//
// The rules of one directive share one [eidos.Directive] wrapper, because
// the authoring surface registers each directive once. The bare rules come
// first, in the order of the manifest, then the wrappers in the order of
// the directives.
func (m *manifest) build(dir, version string, trees []string) plugin.Generator {
	b := eidos.NewPlugin(plugin.ID(m.Name)).
		Version(version).
		Priority(plugin.RoleGenerator, m.Priority).
		Provides(m.Provides...).
		Requires(m.Requires...)
	families := make(map[string]outputDecl, len(m.Outputs))
	for _, o := range m.Outputs {
		b.Output(plugin.Output{Tag: o.Tag, Per: plugin.Cardinality(o.Per), Word: o.Word, Template: o.Template})
		families[o.Tag] = o
	}
	opts := optionsOf(m.Options)
	if opts.IsValid() {
		b.Options(opts.Interface())
	}
	// The workspace populates the options struct when it builds, before
	// the first match, so the map has the values of the config.
	values := sync.OnceValue(func() map[string]any { return optionValues(opts, m.Options) })
	for _, target := range trees {
		b.For(plugin.Target(target), eidos.Templates(rooted{dir: dir, prefix: path.Join(templatesDir, target)}))
	}
	params := make(map[directive.Name][]directive.ParamKey, len(m.Directives))
	for _, d := range m.Directives {
		for _, p := range d.Params {
			params[d.Name] = append(params[d.Name], p.Key)
		}
	}
	var bare []eidos.Rule
	gated := make(map[directive.Name][]eidos.Rule, len(m.Directives))
	for _, r := range m.Rules {
		fam := families[r.Output]
		l := &lowered{
			word:     fam.Word,
			per:      plugin.Cardinality(fam.Per),
			tag:      eidos.Tag(r.Output),
			claimed:  fam.Template != "",
			template: r.Template,
			params:   params[r.Directive],
			options:  values,
		}
		if r.Directive == "" {
			bare = append(bare, r.lower(l))
			continue
		}
		gated[r.Directive] = append(gated[r.Directive], r.lower(l))
	}
	b.Handle(bare...)
	for _, d := range m.Directives {
		b.Handle(eidos.Directive(d.schema(m.Name), gated[d.Name]...))
	}
	// Build returns a generator for a plugin whose rules take the Emitter,
	// and every rule of a manifest takes it.
	g, _ := b.Build().(plugin.Generator)
	return g
}
