// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend

import (
	"encoding/json"
	"fmt"
	"path"
	"slices"
	"strings"

	"github.com/tailscale/hujson"

	"go.dokimi.dev/eidos/lang/typescript"
)

// The names of a tsconfig chain: the file of the configuration of a
// project, the suffix that TypeScript appends to an extends path without a
// file, the wildcard that a paths pattern and its substitutions share, and
// the root directory where a probe ends.
const (
	tsconfigName  = "tsconfig.json"
	jsonExtension = ".json"
	pathsWildcard = "*"
	rootDir       = "."
)

// resolution is the module resolution of a tsconfig chain, relative to the
// workspace: the baseUrl, the directory that the paths substitutions are
// relative to, and the paths patterns sorted by key. The zero resolution
// resolves through neither.
type resolution struct {
	baseURL   string
	pathsBase string
	patterns  []pathPattern
}

// pathPattern is one key of the paths option: the spelling before and
// after its wildcard, whether it has one, and its substitutions in
// order.
type pathPattern struct {
	key      string
	prefix   string
	suffix   string
	wildcard bool
	targets  []string
}

// substitutions returns the workspace paths that the paths pattern of a
// specifier substitutes, in the order of the pattern. It selects the
// pattern as TypeScript does: a key equal to the specifier, and otherwise
// the matching wildcard key with the longest prefix, the first by key on
// a tie. It returns nothing for a specifier that no key matches.
func (r resolution) substitutions(specifier string) []string {
	var best *pathPattern
	star := ""
	for i := range r.patterns {
		p := &r.patterns[i]
		if !p.wildcard {
			if p.key == specifier {
				return r.substitute(p, "")
			}
			continue
		}
		if len(specifier) < len(p.prefix)+len(p.suffix) ||
			!strings.HasPrefix(specifier, p.prefix) || !strings.HasSuffix(specifier, p.suffix) {

			continue
		}
		if best == nil || len(p.prefix) > len(best.prefix) {
			best = p
			star = specifier[len(p.prefix) : len(specifier)-len(p.suffix)]
		}
	}
	if best == nil {
		return nil
	}
	return r.substitute(best, star)
}

// substitute returns the substitutions of a pattern with the wildcard
// replaced, each joined to the directory that the paths are relative to.
func (r resolution) substitute(p *pathPattern, star string) []string {
	out := make([]string, 0, len(p.targets))
	for _, t := range p.targets {
		out = append(out, path.Join(r.pathsBase, strings.Replace(t, pathsWildcard, star, 1)))
	}
	return out
}

// tsconfig is the part of a tsconfig.json that the frontend reads: what it
// extends, a path or a list of paths, and the options of module
// resolution.
type tsconfig struct {
	Extends         json.RawMessage `json:"extends"`
	CompilerOptions struct {
		BaseURL *string             `json:"baseUrl"`
		Paths   map[string][]string `json:"paths"`
	} `json:"compilerOptions"`
}

// reader is the door that a chain reads through: the recorded reader of
// the partition, or the jailed door of a unit.
type reader func(path string) ([]byte, error)

// readChain reads a tsconfig.json and every configuration that its extends
// names, depth first, and returns their paths in that order and the
// resolution that they state together. A configuration overrides what it
// extends, and a later entry of an extends list overrides an earlier one.
// readChain does not read a configuration twice, which stops a cycle. It
// returns the paths read before an error, and the error of a configuration
// that does not read or parse, or whose extends refers to no
// configuration.
func readChain(read reader, file string) ([]string, resolution, error) {
	c := &chain{read: read, seen: map[string]bool{}}
	res, err := c.visit(file)
	return c.paths, res, err
}

// chain is the state of one read of a chain: the door, the paths read in
// order, and the set of paths already read.
type chain struct {
	read  reader
	paths []string
	seen  map[string]bool
}

// visit reads one configuration and what it extends, and returns the
// resolution that the configuration states over the resolution of its
// bases.
func (c *chain) visit(file string) (resolution, error) {
	if c.seen[file] {
		return resolution{}, nil
	}
	c.seen[file] = true
	c.paths = append(c.paths, file)
	data, err := c.read(file)
	if err != nil {
		return resolution{}, err
	}
	standard, err := hujson.Standardize(data)
	if err != nil {
		return resolution{}, fmt.Errorf("frontend: %s: %w", file, err)
	}
	var cfg tsconfig
	if decodeErr := json.Unmarshal(standard, &cfg); decodeErr != nil {
		return resolution{}, fmt.Errorf("frontend: %s: %w", file, decodeErr)
	}
	bases, err := extendsOf(cfg.Extends)
	if err != nil {
		return resolution{}, fmt.Errorf("frontend: %s: %w", file, err)
	}
	var res resolution
	for _, spec := range bases {
		base, found := c.locate(file, spec)
		if !found {
			return res, fmt.Errorf("frontend: %s extends %s, which names no configuration", file, spec)
		}
		inherited, err := c.visit(base)
		if err != nil {
			return res, err
		}
		res = overlay(res, inherited)
	}
	return own(res, file, cfg), nil
}

// locate returns the configuration of an extends entry: a relative path
// from the directory of the extending file, with .json appended where the
// path has no file, or a package path under the nearest node_modules
// directory that has it, as Node resolves a package.
func (c *chain) locate(from, spec string) (string, bool) {
	if typescript.Relative(spec) {
		candidate := path.Join(path.Dir(from), spec)
		if c.readable(candidate) {
			return candidate, true
		}
		if !strings.HasSuffix(candidate, jsonExtension) && c.readable(candidate+jsonExtension) {
			return candidate + jsonExtension, true
		}
		return "", false
	}
	for at := path.Dir(from); ; at = path.Dir(at) {
		pkg := path.Join(at, nodeModules, spec)
		for _, candidate := range []string{pkg, pkg + jsonExtension, path.Join(pkg, tsconfigName)} {
			if c.readable(candidate) {
				return candidate, true
			}
		}
		if at == rootDir {
			return "", false
		}
	}
}

// readable reports whether a path reads through the door of the chain.
func (c *chain) readable(p string) bool {
	_, err := c.read(p)
	return err == nil
}

// extendsOf returns the entries of the extends option: none for an absent
// option, one for a string, and each entry of a list.
func extendsOf(raw json.RawMessage) ([]string, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	var one string
	if err := json.Unmarshal(raw, &one); err == nil {
		return []string{one}, nil
	}
	var many []string
	if err := json.Unmarshal(raw, &many); err != nil {
		return nil, fmt.Errorf("extends is neither a path nor a list of paths: %w", err)
	}
	return many, nil
}

// overlay returns a base resolution with the options of a later base over
// it: the baseUrl and the paths of the later base where it states them.
func overlay(under, over resolution) resolution {
	if over.baseURL != "" {
		under.baseURL = over.baseURL
	}
	if over.patterns != nil {
		under.patterns = over.patterns
		under.pathsBase = over.pathsBase
	}
	return under
}

// own returns the resolution that a configuration states over the
// resolution of its bases. Its baseUrl is relative to its own directory.
// Its paths are relative to the baseUrl in effect, and to its own
// directory where none is. The paths that it inherits follow a baseUrl
// that it sets.
func own(res resolution, file string, cfg tsconfig) resolution {
	dir := path.Dir(file)
	opts := cfg.CompilerOptions
	if opts.BaseURL != nil {
		res.baseURL = path.Join(dir, *opts.BaseURL)
		if res.patterns != nil {
			res.pathsBase = res.baseURL
		}
	}
	if opts.Paths != nil {
		res.patterns = patternsOf(opts.Paths)
		res.pathsBase = res.baseURL
		if res.pathsBase == "" {
			res.pathsBase = dir
		}
	}
	return res
}

// patternsOf returns the patterns of the paths option, sorted by key.
func patternsOf(paths map[string][]string) []pathPattern {
	out := make([]pathPattern, 0, len(paths))
	for key, targets := range paths {
		p := pathPattern{key: key, targets: targets}
		if before, after, wildcard := strings.Cut(key, pathsWildcard); wildcard {
			p.prefix, p.suffix, p.wildcard = before, after, true
		}
		out = append(out, p)
	}
	slices.SortFunc(out, func(a, b pathPattern) int { return strings.Compare(a.key, b.key) })
	return out
}
