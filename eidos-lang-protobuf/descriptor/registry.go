// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package descriptor

import (
	"bytes"
	"fmt"
	"slices"
	"sync"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/dynamicpb"
)

// registry is the extensions that the files of [Registry] declare,
// resolved once for each content of the registry and shared by every unit
// of one frontend. A run reads one registry, so the frontend builds the
// files of the registry once, where each unit with a custom option would
// build them again.
//
// # Concurrency
//
// The units of a load parse at once, and typesOf is safe for concurrent
// use: the first unit builds the types, and the others wait for them.
type registry struct {
	mu    sync.Mutex
	held  []byte
	types *dynamicpb.Types
	err   error
}

// typesOf returns the extension types of the registry whose content is b,
// and builds them where the registry differs from the last one.
//
// Error modes: the error of the decoding of the registry, and of the
// building of its files.
func (r *registry) typesOf(b []byte) (*dynamicpb.Types, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.held != nil && bytes.Equal(r.held, b) {
		return r.types, r.err
	}
	r.held, r.types, r.err = slices.Clone(b), nil, nil
	set := &descriptorpb.FileDescriptorSet{}
	if err := proto.Unmarshal(b, set); err != nil {
		r.err = fmt.Errorf("the registry: %w", err)
		return nil, r.err
	}
	files, err := protodesc.NewFiles(set)
	if err != nil {
		r.err = fmt.Errorf("the files of the registry: %w", err)
		return nil, r.err
	}
	r.types = dynamicpb.NewTypes(files)
	return r.types, nil
}
