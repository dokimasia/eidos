// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend

import (
	"encoding/json"
	"fmt"
	"path"
	"slices"
	"strings"

	"github.com/tailscale/hujson"
)

// The names a tsconfig chain is spelled with: the file a project's
// configuration is in, the suffix TypeScript appends to an extends path
// that names no file, the wildcard a paths pattern and its
// substitutions share, and the root directory a probe ends at.
const (
	tsconfigName  = "tsconfig.json"
	jsonExtension = ".json"
	pathsWildcard = "*"
	rootDir       = "."
)

// resolution is a tsconfig chain's module resolution, workspace
// relative: the baseUrl, the directory the paths substitutions are
// relative to, and the paths patterns, sorted by key. The zero
// resolution resolves through neither.
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

// substitutions returns the workspace paths a specifier's paths pattern
// substitutes, in the pattern's order, the way TypeScript selects the
// pattern: a key that spells the specifier exactly, and otherwise the
// matching wildcard key with the longest prefix, the first by key on a
// tie. It returns nothing for a specifier no key matches.
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

// substitute returns a pattern's substitutions with the wildcard
// replaced, each joined to the directory the paths are relative to.
func (r resolution) substitute(p *pathPattern, star string) []string {
	out := make([]string, 0, len(p.targets))
	for _, t := range p.targets {
		out = append(out, path.Join(r.pathsBase, strings.Replace(t, pathsWildcard, star, 1)))
	}
	return out
}

// tsconfig is the part of a tsconfig.json the frontend reads: what it
// extends, a path or a list of paths, and the module resolution
// options.
type tsconfig struct {
	Extends         json.RawMessage `json:"extends"`
	CompilerOptions struct {
		BaseURL *string             `json:"baseUrl"`
		Paths   map[string][]string `json:"paths"`
	} `json:"compilerOptions"`
}

// reader is the door a chain reads through: the partition's recorded
// reader, or a unit's jailed one.
type reader func(path string) ([]byte, error)

// governing returns the tsconfig.json nearest above a directory, and
// false where no directory up to the root has one. A probe that misses
// reads nothing.
func governing(read reader, dir string) (string, bool) {
	for at := dir; ; at = path.Dir(at) {
		candidate := path.Join(at, tsconfigName)
		if _, err := read(candidate); err == nil {
			return candidate, true
		}
		if at == rootDir {
			return "", false
		}
	}
}

// readChain reads a tsconfig.json and every configuration its extends
// names, depth first, and returns their paths in that order and the
// resolution they state together. A configuration overrides what it
// extends, and a later entry of an extends list overrides an earlier
// one. A configuration the chain already read is not read again, which
// stops a cycle. It returns the paths read before an error, and the
// error of a configuration that does not read or parse, or whose
// extends names no configuration.
func readChain(read reader, file string) ([]string, resolution, error) {
	c := &chain{read: read, seen: map[string]bool{}}
	res, err := c.visit(file)
	return c.paths, res, err
}

// chain is one chain read's state: the door, the paths read in order,
// and the paths already read.
type chain struct {
	read  reader
	paths []string
	seen  map[string]bool
}

// visit reads one configuration and what it extends, and returns the
// resolution the configuration states over its bases'.
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

// locate returns the configuration an extends entry names: a relative
// path from the extending file's directory, with .json appended where
// the path names no file, or a package path under the nearest
// node_modules directory that has it, as Node resolves a package.
func (c *chain) locate(from, spec string) (string, bool) {
	if relative(spec) {
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

// readable reports whether a path reads through the chain's door.
func (c *chain) readable(p string) bool {
	_, err := c.read(p)
	return err == nil
}

// extendsOf returns the extends option's entries: none for an absent
// option, one for a string, and each of a list's.
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

// overlay returns a base resolution with a later base's options over
// it: the later base's baseUrl and paths where it states them.
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

// own returns the resolution a configuration states over its bases'.
// Its baseUrl is relative to its own directory. Its paths are relative
// to the baseUrl in effect, and to its own directory where none is.
// Paths it inherits follow a baseUrl it sets.
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

// patternsOf returns the paths option's patterns, sorted by key.
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
