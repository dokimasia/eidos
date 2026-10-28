// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package protoc

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"strconv"
	"strings"
	"testing/fstest"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/pluginpb"

	"go.dokimi.dev/eidos/core/layout"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/workspace"
	"go.dokimi.dev/eidos/lang/protobuf/descriptor"
)

// The spellings of the parameter of a request.
const (
	// paramSep separates the entries of the parameter, as protoc joins the
	// options of a plugin.
	paramSep = ","
	// paramAssign separates the key of an entry from its value.
	paramAssign = "="
	// keySep separates the plugin of an option from the key of the option.
	keySep = "."
	// policyPrefix opens the key of an entry that selects the choice of a
	// policy.
	policyPrefix = "policy."
	// planPrefix opens the key of an entry that sets a field of a plan.
	planPrefix = "plan."
)

// The fields of a plan that the parameter sets, as a config file of
// eidos spells them.
const (
	planImportBase = "importBase"
	planDir        = "dir"
	planPolicy     = "policy"
	planDisabled   = "disabled"
)

// ErrParameter reports an entry of the parameter of a request that names
// no option, no policy and no field of a plan: an entry without "=", with
// an empty key, with a key that names no plugin, or with a field of a plan
// that the parameter does not set or a value that the field does not take.
// The response of the request contains the error.
var ErrParameter = errors.New(
	"protoc: the parameter has an entry that names no option, no policy and no field of a plan",
)

// configured sets the config that the parameter of a request states on a
// builder. It returns the builder unchanged for an empty parameter, and the
// error of [configOf] as text for a parameter that it refuses.
func configured(b *workspace.Builder, param string) (*workspace.Builder, string) {
	if param == "" {
		return b, ""
	}
	config, err := configOf(param)
	if err != nil {
		return b, err.Error()
	}
	return b.Config(config), ""
}

// configOf returns the config of the composition that a parameter of a
// request states. Each entry is key=value:
//
//   - <plugin>.<option>=<value> sets one option of one plugin. The option is
//     the key after the last dot, because the key of an option has no dot.
//   - policy.<key>=<choice> selects the choice of one policy.
//   - plan.<plan>.<field>=<value> sets one field of one plan: importBase,
//     dir, policy as inherit, alongside-source or centralised, or disabled
//     as true or false.
//
// A value is decoded as JSON where it is JSON, such as true, 3 or
// ["a","b"], and is a string otherwise.
//
// Error modes: an error that wraps [ErrParameter] for an entry without
// "=", with an empty key, with a key that names no plugin, and with a field
// of a plan that [planField] refuses, and so for an empty parameter.
func configOf(param string) (workspace.Config, error) {
	var c workspace.Config
	for _, entry := range entriesOf(param) {
		key, value, assigned := strings.Cut(entry, paramAssign)
		if !assigned || key == "" {
			return workspace.Config{}, fmt.Errorf("%w: %q", ErrParameter, entry)
		}
		if field, isPlan := strings.CutPrefix(key, planPrefix); isPlan {
			if err := planField(&c, field, value); err != nil {
				return workspace.Config{}, fmt.Errorf("%w: %q: %w", ErrParameter, entry, err)
			}
			continue
		}
		if policy, isPolicy := strings.CutPrefix(key, policyPrefix); isPolicy {
			if c.Policies == nil {
				c.Policies = map[plugin.PolicyKey]plugin.Choice{}
			}
			c.Policies[plugin.PolicyKey(policy)] = plugin.Choice(value)
			continue
		}
		at := strings.LastIndex(key, keySep)
		if at <= 0 || at == len(key)-1 {
			return workspace.Config{}, fmt.Errorf("%w: %q", ErrParameter, entry)
		}
		name, option := key[:at], key[at+1:]
		if c.Options == nil {
			c.Options = map[string]map[string]any{}
		}
		if c.Options[name] == nil {
			c.Options[name] = map[string]any{}
		}
		c.Options[name][option] = valueOf(value)
	}
	return c, nil
}

// planField sets one field of a plan on c from the key <plan>.<field> and
// its value. The plan is the key before the last dot.
//
// Error modes: an error for a key without a plan, for a field that the
// parameter does not set, for a policy that the layout does not know, and
// for a disabled that is no bool.
func planField(c *workspace.Config, key, value string) error {
	at := strings.LastIndex(key, keySep)
	if at <= 0 {
		return errors.New("protoc: the key names no plan")
	}
	name, field := key[:at], key[at+1:]
	if c.Plans == nil {
		c.Plans = map[string]workspace.PlanConfig{}
	}
	p := c.Plans[name]
	switch field {
	case planImportBase:
		p.ImportBase = value
	case planDir:
		p.Dir = value
	case planPolicy:
		policy, err := layout.ParsePolicy(value)
		if err != nil {
			return err
		}
		p.Policy = policy
	case planDisabled:
		disabled, err := strconv.ParseBool(value)
		if err != nil {
			return fmt.Errorf("protoc: disabled takes true or false: %w", err)
		}
		p.Disabled = disabled
	default:
		return fmt.Errorf("protoc: a plan has no field %q", field)
	}
	c.Plans[name] = p
	return nil
}

// entriesOf splits a parameter into its entries at each comma outside a
// JSON string, list or object, so a value such as "a,b" or [1,2] keeps
// its commas. A backslash inside a string escapes the next byte.
func entriesOf(param string) []string {
	var (
		out      []string
		start    int
		depth    int
		inString bool
		escaped  bool
	)
	for i := range len(param) {
		c := param[i]
		switch {
		case escaped:
			escaped = false
		case inString && c == '\\':
			escaped = true
		case c == '"':
			inString = !inString
		case inString:
		case c == '[' || c == '{':
			depth++
		case c == ']' || c == '}':
			depth--
		case c == paramSep[0] && depth <= 0:
			out = append(out, param[start:i])
			start = i + 1
		}
	}
	return append(out, param[start:])
}

// valueOf returns a value of the parameter decoded as JSON, and the value
// itself where it is no JSON.
func valueOf(text string) any {
	var v any
	if err := json.Unmarshal([]byte(text), &v); err != nil {
		return text
	}
	return v
}

// inputOf returns the input of the run of a request. Its tree contains
// each file to generate at its proto path, as a serialized
// FileDescriptorProto, and [descriptor.Registry] with every file of the
// request. The store [descriptor.Store] contains every other file. The
// descriptor of a file to generate is its entry of source_file_descriptors,
// which keeps the options of source retention, and its entry of proto_file
// where the request has none.
//
// Error modes: an error for a descriptor that does not encode.
func inputOf(req *pluginpb.CodeGeneratorRequest) (workspace.Input, error) {
	generate := map[string]bool{}
	for _, name := range req.GetFileToGenerate() {
		generate[name] = true
	}
	sources := map[string]*descriptorpb.FileDescriptorProto{}
	for _, fd := range req.GetSourceFileDescriptors() {
		sources[fd.GetName()] = fd
	}
	tree, store := fstest.MapFS{}, fstest.MapFS{}
	for _, fd := range req.GetProtoFile() {
		into := store
		if generate[fd.GetName()] {
			into = tree
			if source, held := sources[fd.GetName()]; held {
				fd = source
			}
		}
		b, err := proto.Marshal(fd)
		if err != nil {
			return workspace.Input{}, fmt.Errorf("protoc: encode %s: %w", fd.GetName(), err)
		}
		into[fd.GetName()] = &fstest.MapFile{Data: b}
	}
	// The registry serves the extensions of custom options, so it leaves out
	// the source code info, which is most of a descriptor of protoc.
	set := &descriptorpb.FileDescriptorSet{File: make([]*descriptorpb.FileDescriptorProto, 0, len(req.GetProtoFile()))}
	for _, fd := range req.GetProtoFile() {
		bare := proto.CloneOf(fd)
		bare.SourceCodeInfo = nil
		set.File = append(set.File, bare)
	}
	registry, err := proto.Marshal(set)
	if err != nil {
		return workspace.Input{}, fmt.Errorf("protoc: encode the registry: %w", err)
	}
	tree[descriptor.Registry] = &fstest.MapFile{Data: registry}
	return workspace.Input{Tree: tree, Stores: map[string]fs.FS{descriptor.Store: store}}, nil
}
