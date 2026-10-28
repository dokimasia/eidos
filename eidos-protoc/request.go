// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package protoc

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"strings"
	"testing/fstest"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/pluginpb"

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
)

// ErrParameter reports an entry of the parameter of a request that names
// no option and no policy: an entry without "=", with an empty key, or with
// a key that names no plugin. The response of the request contains the
// error.
var ErrParameter = errors.New("protoc: the parameter has an entry that names no option and no policy")

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
//
// A value is decoded as JSON where it is JSON, such as true, 3 or
// ["a","b"], and is a string otherwise.
//
// Error modes: an error that wraps [ErrParameter] for an entry without
// "=", with an empty key, or with a key that names no plugin, and so for an
// empty parameter.
func configOf(param string) (workspace.Config, error) {
	var c workspace.Config
	for entry := range strings.SplitSeq(param, paramSep) {
		key, value, assigned := strings.Cut(entry, paramAssign)
		if !assigned || key == "" {
			return workspace.Config{}, fmt.Errorf("%w: %q", ErrParameter, entry)
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
	registry, err := proto.Marshal(&descriptorpb.FileDescriptorSet{File: req.GetProtoFile()})
	if err != nil {
		return workspace.Input{}, fmt.Errorf("protoc: encode the registry: %w", err)
	}
	tree[descriptor.Registry] = &fstest.MapFile{Data: registry}
	return workspace.Input{Tree: tree, Stores: map[string]fs.FS{descriptor.Store: store}}, nil
}
