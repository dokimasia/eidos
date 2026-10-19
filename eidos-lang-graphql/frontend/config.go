// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"path"
	"slices"

	"github.com/pelletier/go-toml/v2"
	"go.yaml.in/yaml/v3"

	"go.dokimi.dev/eidos/sdk/plugin"
)

// The names of the configuration files of graphql-config, in the order in
// which graphql-config searches a directory for them. A directory has the
// configuration of the first name that it contains. package.json counts
// only where it has the key graphql. The last four names are the legacy
// names, which graphql-config reads as well.
var graphqlConfigNames = [...]string{
	"graphql.config.ts", "graphql.config.cts", "graphql.config.mts", "graphql.config.js",
	"graphql.config.cjs", "graphql.config.mjs", "graphql.config.json", "graphql.config.yaml",
	"graphql.config.yml", "graphql.config.toml", ".graphqlrc", ".graphqlrc.ts", ".graphqlrc.cts",
	".graphqlrc.mts", ".graphqlrc.js", ".graphqlrc.cjs", ".graphqlrc.mjs", ".graphqlrc.json",
	".graphqlrc.yml", ".graphqlrc.yaml", ".graphqlrc.toml", packageJSON,
	".graphqlconfig", ".graphqlconfig.json", ".graphqlconfig.yaml", ".graphqlconfig.yml",
}

// The names of the configuration files of gqlgen, in the order in which
// gqlgen searches a directory for them.
var gqlgenNames = [...]string{".gqlgen.yml", "gqlgen.yml", "gqlgen.yaml"}

// The name and the extensions that decide the format of a configuration
// file. Every other configuration file is YAML, as graphql-config reads
// .graphqlrc and .graphqlconfig.
const (
	packageJSON = "package.json"
	jsonExt     = ".json"
	tomlExt     = ".toml"
)

// codeExts are the extensions of a configuration in TypeScript or in
// JavaScript, which graphql-config runs and the frontend does not.
var codeExts = [...]string{".ts", ".cts", ".mts", ".js", ".cjs", ".mjs"}

// The keys of a configuration that the frontend reads. graphqlKey is the
// key of package.json whose value is the configuration of graphql-config.
// The legacy keys are the keys of a legacy configuration, which
// graphql-config reads as the keys schema, include and exclude.
const (
	graphqlKey    = "graphql"
	schemaKey     = "schema"
	documentsKey  = "documents"
	includeKey    = "include"
	excludeKey    = "exclude"
	projectsKey   = "projects"
	schemaPathKey = "schemaPath"
	includesKey   = "includes"
	excludesKey   = "excludes"
)

// gqlgenSchema is the schema file of a configuration of gqlgen without the
// key schema, as gqlgen defaults it.
const gqlgenSchema = "schema.graphql"

// format is the encoding of one configuration file.
type format uint8

// The encodings of a configuration file.
const (
	// formatCode is TypeScript or JavaScript.
	formatCode format = iota + 1
	// formatJSON is JSON.
	formatJSON
	// formatYAML is YAML.
	formatYAML
	// formatTOML is TOML.
	formatTOML
	// formatPackage is package.json, whose key graphql is the
	// configuration.
	formatPackage
)

// formatOf returns the encoding of the configuration file named name.
func formatOf(name string) format {
	ext := path.Ext(name)
	switch {
	case name == packageJSON:
		return formatPackage
	case slices.Contains(codeExts[:], ext):
		return formatCode
	case ext == jsonExt:
		return formatJSON
	case ext == tomlExt:
		return formatTOML
	default:
		return formatYAML
	}
}

// valueKind is the kind of one decoded value of a configuration file.
type valueKind uint8

// The kinds of a decoded value.
const (
	// valueScalar is a string, a number, a bool or null, with its text.
	valueScalar valueKind = iota + 1
	// valueList is a list of values in their order.
	valueList
	// valueMap is a map, with its keys in the order of the document.
	valueMap
)

// value is one decoded value of a configuration file. A map has its keys
// in the order of the document, and the value of each key at the same
// index of items. TOML does not order the keys of a table, so a map of
// TOML has its keys in the order of their names.
type value struct {
	kind  valueKind
	text  string
	keys  []string
	items []value
}

// decodeConfig decodes the bytes b of a configuration file named name into
// its value. It returns an error for a configuration in code, and the
// error of the decoder for bytes that do not decode. A package.json
// decodes to the value of its key graphql, and reports false for a
// package.json without the key, which is no configuration.
func decodeConfig(name string, b []byte) (value, bool, error) {
	switch formatOf(name) {
	case formatCode:
		return value{}, true, errors.New("frontend: the configuration is code, and the frontend does not run it")
	case formatPackage:
		v, err := decodeJSON(b)
		if err != nil {
			return value{}, true, err
		}
		inner, held := v.get(graphqlKey)
		return inner, held, nil
	case formatJSON:
		v, err := decodeJSON(b)
		return v, true, err
	case formatTOML:
		v, err := decodeTOML(b)
		return v, true, err
	default:
		v, err := decodeYAML(b)
		return v, true, err
	}
}

// decodeJSON decodes one JSON document, with the keys of each object in the
// order of the document.
func decodeJSON(b []byte) (value, error) {
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.UseNumber()
	v, err := jsonValue(dec)
	if err != nil {
		return value{}, err
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return value{}, errors.New("frontend: decode JSON: the document continues after its value")
	}
	return v, nil
}

// jsonValue decodes the next value of dec.
func jsonValue(dec *json.Decoder) (value, error) {
	tok, err := dec.Token()
	if err != nil {
		return value{}, fmt.Errorf("frontend: decode JSON: %w", err)
	}
	switch t := tok.(type) {
	case json.Delim:
		if t == '[' {
			v := value{kind: valueList}
			for dec.More() {
				item, err := jsonValue(dec)
				if err != nil {
					return value{}, err
				}
				v.items = append(v.items, item)
			}
			if _, err := dec.Token(); err != nil {
				return value{}, fmt.Errorf("frontend: decode JSON: %w", err)
			}
			return v, nil
		}
		v := value{kind: valueMap}
		for dec.More() {
			// The decoder returns the key of an object as a string, and
			// fails on any other token in its place.
			key, err := dec.Token()
			if err != nil {
				return value{}, fmt.Errorf("frontend: decode JSON: %w", err)
			}
			item, err := jsonValue(dec)
			if err != nil {
				return value{}, err
			}
			name, _ := key.(string)
			v.keys = append(v.keys, name)
			v.items = append(v.items, item)
		}
		if _, err := dec.Token(); err != nil {
			return value{}, fmt.Errorf("frontend: decode JSON: %w", err)
		}
		return v, nil
	case nil:
		return value{kind: valueScalar}, nil
	default:
		return value{kind: valueScalar, text: fmt.Sprint(t)}, nil
	}
}

// decodeYAML decodes one YAML document, with the keys of each mapping in the
// order of the document. An empty document decodes to an empty map.
func decodeYAML(b []byte) (value, error) {
	var doc yaml.Node
	if err := yaml.Unmarshal(b, &doc); err != nil {
		return value{}, fmt.Errorf("frontend: decode YAML: %w", err)
	}
	if len(doc.Content) == 0 {
		return value{kind: valueMap}, nil
	}
	return yamlValue(doc.Content[0]), nil
}

// yamlValue converts one node of YAML, and follows an alias to its anchor.
func yamlValue(n *yaml.Node) value {
	switch n.Kind {
	case yaml.MappingNode:
		v := value{kind: valueMap}
		for i := 0; i+1 < len(n.Content); i += 2 {
			v.keys = append(v.keys, n.Content[i].Value)
			v.items = append(v.items, yamlValue(n.Content[i+1]))
		}
		return v
	case yaml.SequenceNode:
		v := value{kind: valueList}
		for _, item := range n.Content {
			v.items = append(v.items, yamlValue(item))
		}
		return v
	case yaml.AliasNode:
		return yamlValue(n.Alias)
	default:
		return value{kind: valueScalar, text: n.Value}
	}
}

// decodeTOML decodes one TOML document. TOML does not order the keys of a
// table, so each map has its keys in the order of their names.
func decodeTOML(b []byte) (value, error) {
	var doc map[string]any
	if err := toml.Unmarshal(b, &doc); err != nil {
		return value{}, fmt.Errorf("frontend: decode TOML: %w", err)
	}
	return tomlValue(doc), nil
}

// tomlValue converts one decoded value of TOML.
func tomlValue(x any) value {
	switch t := x.(type) {
	case map[string]any:
		v := value{kind: valueMap, keys: slices.Sorted(maps.Keys(t))}
		for _, k := range v.keys {
			v.items = append(v.items, tomlValue(t[k]))
		}
		return v
	case []any:
		v := value{kind: valueList}
		for _, item := range t {
			v.items = append(v.items, tomlValue(item))
		}
		return v
	default:
		return value{kind: valueScalar, text: fmt.Sprint(t)}
	}
}

// get returns the value of the key k of a map. It reports false for a
// value that is not a map, and for a map without the key.
func (v value) get(k string) (value, bool) {
	if i := slices.Index(v.keys, k); i >= 0 && v.kind == valueMap {
		return v.items[i], true
	}
	return value{}, false
}

// pointers returns the entries of a key of graphql-config: the text of a
// string, the entries of each item of a list, and the first key of a map,
// which is an entry with its own configuration. An entry that is not a
// path, such as a URL or a schema written in place, is a pattern that
// matches no file.
func (v value) pointers() []string {
	switch v.kind {
	case valueScalar:
		return []string{v.text}
	case valueList:
		var out []string
		for _, item := range v.items {
			out = append(out, item.pointers()...)
		}
		return out
	default:
		if len(v.keys) == 0 {
			return nil
		}
		return []string{v.keys[0]}
	}
}

// tool is the tool whose configuration a file is. The tools are in the
// order in which [probe] reads the configurations of a directory.
type tool uint8

// The tools of a configuration file.
const (
	// toolGraphQLConfig is graphql-config.
	toolGraphQLConfig tool = iota + 1
	// toolGqlgen is gqlgen.
	toolGqlgen
)

// config is one configuration file: its path, its tool and the schemas that
// it defines. A configuration that the frontend does not read has the
// reason in err and no schema. The partition records first, the first
// claimed file in path order in the directory of the configuration or below
// it.
type config struct {
	path    string
	tool    tool
	schemas []schema
	err     error
	first   string
}

// readConfig reads the configuration file at p from its bytes b. It reports
// false for a package.json without the key graphql, which is no
// configuration. A configuration in code, a configuration that does not
// decode, and a configuration of gqlgen with an entry that gqlgen refuses
// have the reason in err.
func readConfig(p string, b []byte) (config, bool) {
	name := path.Base(p)
	v, is, err := decodeConfig(name, b)
	if !is {
		return config{}, false
	}
	c := config{path: p, tool: toolGraphQLConfig, err: err}
	if slices.Contains(gqlgenNames[:], name) {
		c.tool = toolGqlgen
	}
	switch {
	case err != nil:
		// A configuration that does not decode defines no schema.
	case c.tool == toolGqlgen:
		c.schemas, c.err = gqlgenSchemas(p, v)
	default:
		c.schemas = graphqlConfigSchemas(p, v)
	}
	return c, true
}

// probe returns the configurations of one directory: the first
// configuration of graphql-config in the order of [graphqlConfigNames], and
// then the first configuration of gqlgen in the order of [gqlgenNames]. A
// package.json without the key graphql is no configuration, so the search
// continues after it. first is the claimed file whose directory chain
// reached the directory first. A name that does not read is not in the
// directory.
func probe(r plugin.FileReader, dir, first string) []config {
	var out []config
	for _, names := range [...][]string{graphqlConfigNames[:], gqlgenNames[:]} {
		for _, name := range names {
			p := path.Join(dir, name)
			b, err := r.Read(p)
			if err != nil {
				continue
			}
			c, is := readConfig(p, b)
			if !is {
				continue
			}
			c.first = first
			out = append(out, c)
			break
		}
	}
	return out
}
