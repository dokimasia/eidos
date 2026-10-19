// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend

import (
	"fmt"
	"path"
	"regexp"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
)

// The spellings of the paths of a configuration. A configuration at the
// root of the workspace is in the directory ".", and its schema has the
// package of the root, which has no path.
const (
	rootDir  = "."
	parent   = ".."
	pathSep  = "/"
	negation = "!"
)

// The parts of a pattern of gqlgen with a walk. gqlgen walks the directory
// before the first ** and matches the rest of the path below it against the
// expression of the pattern after it, in which * matches every character.
const (
	gqlgenWalk     = "**"
	gqlgenWalkSep  = `\`
	gqlgenAnchored = "$"
)

// gqlgenExpression rewrites the rest of a pattern of gqlgen after its ** into
// the expression that gqlgen matches, as gqlgen rewrites it.
var gqlgenExpression = strings.NewReplacer(`.`, `\.`, `*`, `.+`, `\`, `[\\/]`, `/`, `[\\/]`)

// pattern is one entry of a key of a configuration. A pattern of
// graphql-config is a glob of minimatch, with ** for any number of
// directories and braces for alternatives, and a pattern that opens with !
// removes the files that it matches from the entries of its key. A pattern
// of gqlgen is a glob of Go, or a walk where it has **.
type pattern struct {
	glob    string
	negated bool
	gqlgen  bool
	// walk is the expression of a pattern of gqlgen with **, which matches
	// a path below prefix, and nil for every other pattern.
	walk   *regexp.Regexp
	prefix string
}

// graphqlConfigPattern returns the pattern of an entry of graphql-config. It
// cleans the path of the entry, as graphql-config normalizes it.
func graphqlConfigPattern(entry string) pattern {
	glob, negated := strings.CutPrefix(entry, negation)
	return pattern{glob: path.Clean(glob), negated: negated}
}

// gqlgenPattern returns the pattern of an entry of the key schema of gqlgen.
// An entry with ** walks the directory before it. gqlgenPattern returns an
// error for an entry that gqlgen refuses: a glob that does not parse, a walk
// without a directory before its **, and a walk whose expression does not
// compile.
func gqlgenPattern(entry string) (pattern, error) {
	before, after, walks := strings.Cut(entry, gqlgenWalk)
	if !walks {
		glob := path.Clean(entry)
		if _, err := path.Match(glob, ""); err != nil {
			return pattern{}, fmt.Errorf("frontend: gqlgen refuses the glob %q: %w", entry, err)
		}
		return pattern{glob: glob, gqlgen: true}, nil
	}
	if before == "" {
		return pattern{}, fmt.Errorf("frontend: gqlgen refuses the walk %q, which names no directory before **", entry)
	}
	rest := strings.TrimPrefix(strings.TrimPrefix(after, gqlgenWalkSep), pathSep)
	walk, err := regexp.Compile(gqlgenExpression.Replace(rest) + gqlgenAnchored)
	if err != nil {
		return pattern{}, fmt.Errorf("frontend: gqlgen refuses the walk %q: %w", entry, err)
	}
	return pattern{glob: entry, gqlgen: true, walk: walk, prefix: path.Clean(before)}, nil
}

// appendPatterns appends the patterns of graphql-config of the key of v to
// dst.
func appendPatterns(dst []pattern, v value, key string) []pattern {
	stated, held := v.get(key)
	if !held {
		return dst
	}
	for _, entry := range stated.pointers() {
		dst = append(dst, graphqlConfigPattern(entry))
	}
	return dst
}

// match reports whether the pattern matches the path rel, which is
// relative to the directory of the configuration. A glob of graphql-config
// that does not parse matches no path.
func (p pattern) match(rel string) bool {
	switch {
	case p.walk != nil:
		if p.prefix == rootDir {
			return p.walk.MatchString(rel)
		}
		rest, below := strings.CutPrefix(rel, p.prefix+pathSep)
		return below && p.walk.MatchString(rest)
	case p.gqlgen:
		matched, err := path.Match(p.glob, rel)
		return err == nil && matched
	default:
		matched, err := doublestar.Match(p.glob, rel)
		return err == nil && matched
	}
}

// schema is one schema that a configuration defines: a project of
// graphql-config, the one project of a configuration without the key
// projects, or the schema of a configuration of gqlgen.
type schema struct {
	// config is the path of the configuration file, and dir is its
	// directory. Every pattern is relative to dir.
	config, dir string
	// pkg is the package of the unit of the schema: the directory of the
	// configuration, or the directory joined with the name of a project.
	pkg string
	// entries are the patterns of the keys schema and documents. include
	// and exclude are the patterns of the keys include and exclude.
	entries, include, exclude []pattern
}

// graphqlConfigSchemas returns the schemas that the configuration of
// graphql-config at p defines. v is its decoded value.
//
// A configuration with the key projects defines one schema for each
// project, in the order of the projects. Each has the package of the
// directory of the configuration joined with the name of the project. A
// configuration without the key defines one schema for its keys schema and
// documents, with the package of its directory. A legacy configuration
// states the keys schemaPath, includes and excludes, which graphql-config
// reads as schema, include and exclude. A configuration without a schema
// defines none.
func graphqlConfigSchemas(p string, v value) []schema {
	dir := path.Dir(p)
	if projects, held := v.get(projectsKey); held && projects.kind == valueMap {
		out := make([]schema, 0, len(projects.keys))
		for i, name := range projects.keys {
			out = append(out, project(p, dir, packageOf(path.Join(dir, name)), projects.items[i]))
		}
		return out
	}
	_, single := v.get(schemaKey)
	_, legacy := v.get(schemaPathKey)
	if !single && !legacy {
		return nil
	}
	return []schema{project(p, dir, packageOf(dir), v)}
}

// gqlgenSchemas returns the schema that the configuration of gqlgen at p
// defines. v is its decoded value. The schema has the entries of the key
// schema, which is schema.graphql where the configuration does not state
// it, and the package of the directory of the configuration.
// gqlgenSchemas returns the error of the first entry that gqlgen refuses,
// and no schema, because gqlgen does not load such a configuration.
func gqlgenSchemas(p string, v value) ([]schema, error) {
	dir := path.Dir(p)
	entries := []string{gqlgenSchema}
	if stated, held := v.get(schemaKey); held {
		entries = stated.pointers()
	}
	s := schema{config: p, dir: dir, pkg: packageOf(dir)}
	for _, e := range entries {
		pt, err := gqlgenPattern(e)
		if err != nil {
			return nil, err
		}
		s.entries = append(s.entries, pt)
	}
	return []schema{s}, nil
}

// project returns the schema of one project of graphql-config at the
// package pkg. v is the value of the project.
func project(config, dir, pkg string, v value) schema {
	s := schema{config: config, dir: dir, pkg: pkg}
	for _, key := range [...]string{schemaKey, documentsKey, schemaPathKey} {
		s.entries = appendPatterns(s.entries, v, key)
	}
	for _, key := range [...]string{includeKey, includesKey} {
		s.include = appendPatterns(s.include, v, key)
	}
	for _, key := range [...]string{excludeKey, excludesKey} {
		s.exclude = appendPatterns(s.exclude, v, key)
	}
	return s
}

// matches reports whether the schema claims the file at the workspace path
// p, as graphql-config decides it. The entries claim a file that they
// match. A file that the entries do not match is claimed where include
// matches it and exclude does not.
func (s *schema) matches(p string) bool {
	rel := relative(s.dir, p)
	if claims(s.entries, rel) {
		return true
	}
	if claims(s.exclude, rel) {
		return false
	}
	return claims(s.include, rel)
}

// claims reports whether the patterns ps claim the path rel: a pattern that
// is not negated matches it, and no negated pattern does.
func claims(ps []pattern, rel string) bool {
	claimed := false
	for _, p := range ps {
		if !p.match(rel) {
			continue
		}
		if p.negated {
			return false
		}
		claimed = true
	}
	return claimed
}

// relative returns the slash path p of the workspace relative to the
// directory dir of the workspace. A path outside dir starts with one ..
// for each directory of dir that it leaves.
func relative(dir, p string) string {
	if dir == rootDir {
		return p
	}
	dirs := strings.Split(dir, pathSep)
	parts := strings.Split(p, pathSep)
	common := 0
	for common < len(dirs) && common < len(parts)-1 && dirs[common] == parts[common] {
		common++
	}
	out := make([]string, 0, len(dirs)-common+len(parts)-common)
	for range len(dirs) - common {
		out = append(out, parent)
	}
	return strings.Join(append(out, parts[common:]...), pathSep)
}

// packageOf returns the package of a schema in the directory dir. The
// package of the root of the workspace has no path.
func packageOf(dir string) string {
	if dir == rootDir {
		return ""
	}
	return dir
}
