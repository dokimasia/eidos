// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package layout

import (
	"cmp"
	"encoding"
	"encoding/binary"
	"fmt"
	"io/fs"
	"maps"
	"slices"
	"strings"

	"go.dokimi.dev/eidos/core/plugin"
)

// Family names one declared output family: a generator and a tag,
// the empty tag naming the primary family.
type Family struct {
	Plugin plugin.ID
	Tag    string
}

// compare orders two families by plugin, then tag.
func (f Family) compare(o Family) int {
	return cmp.Or(cmp.Compare(f.Plugin, o.Plugin), cmp.Compare(f.Tag, o.Tag))
}

// Refinement overrides a plan's routing for one generator or one
// family. A zero field inherits from the enclosing scope. Build
// refuses a Dir for a per-source or per-package family under an
// alongside policy, because the run writes such a file beside its
// source and reads no directory for it.
type Refinement struct {
	Policy Policy
	// Dir is the output directory a centralised family writes under,
	// and the directory a per-plan family's file is written in.
	Dir string
	// File is the filename every file of the family takes, in place of
	// the target's spelling.
	File string
}

// check validates one refinement's own fields, naming the plan and
// the scope the refinement applies to.
func (r Refinement) check(plan, scope string) []error {
	var faults []error
	if !r.Policy.Valid() {
		faults = append(faults, fmt.Errorf("layout: plan %q: %s refines to %s, which is not a layout policy",
			plan, scope, r.Policy))
	}
	if r.Dir != "" && !directory(r.Dir) {
		faults = append(faults, fmt.Errorf("layout: plan %q: %s refines its directory to %q, "+
			"which is not a workspace-relative directory", plan, scope, r.Dir))
	}
	if r.File != "" && !filename(r.File) {
		faults = append(faults, fmt.Errorf("layout: plan %q: %s refines its filename to %q, "+
			"which is not one path element", plan, scope, r.File))
	}
	return faults
}

// appendBinary appends the refinement's policy, directory and filename
// to b, each string behind its length, and returns the extended buffer.
// It allocates only where b lacks the room.
func (r Refinement) appendBinary(b []byte) []byte {
	b = append(b, byte(r.Policy))
	b = appendString(b, r.Dir)
	return appendString(b, r.File)
}

// resolved is one family's routing configuration after the
// refinements: the family's own, then the generator's, then the
// plan's, field by field.
type resolved struct {
	policy Policy
	dir    string
	file   string
}

// Config is one plan's routing configuration. The zero Config places
// every file beside its source. [Config.Check] is the validation
// Build runs: the policies are valid, every directory is
// workspace-relative and inside the workspace root, every refinement
// names a generator of the plan and a family that generator declares,
// and every family that needs a directory has one.
type Config struct {
	// Policy is the plan's default placement.
	Policy Policy
	// Dir is the output directory, workspace-relative and
	// slash-separated: where a centralised policy writes, and where a
	// per-plan family's file is written.
	Dir string
	// ImportBase is the package path a target joins Dir's
	// subdirectories onto, for a directory no loaded module contains.
	ImportBase string
	// Plugins refines every family of one generator.
	Plugins map[plugin.ID]Refinement
	// Families refines one family of one generator, and takes
	// precedence over Plugins field by field.
	Families map[Family]Refinement
}

var _ encoding.BinaryAppender = Config{}

// Check validates the configuration against the families each
// generator of a plan declares, keyed by generator, and returns one
// fault per problem, each naming the plan. A generator that declares
// no family is a key with no families. Faults arrive in a fixed order:
// the plan's own fields, the generator refinements, the family
// refinements, then each generator's declared families.
func (c Config) Check(plan string, outputs map[plugin.ID][]plugin.Output) []error {
	var faults []error
	fault := func(format string, a ...any) {
		faults = append(faults, fmt.Errorf("layout: plan %q: "+format, append([]any{plan}, a...)...))
	}
	if !c.Policy.Valid() {
		fault("%s is not a layout policy", c.Policy)
	}
	if c.Dir != "" && !directory(c.Dir) {
		fault("the output directory %q is not a workspace-relative directory", c.Dir)
	}
	if c.ImportBase != "" && !fs.ValidPath(c.ImportBase) {
		fault("the import base %q is not a slash-separated package path", c.ImportBase)
	}
	for _, p := range slices.Sorted(maps.Keys(c.Plugins)) {
		if _, listed := outputs[p]; !listed {
			fault("a refinement names %s, which is no generator of the plan", p)
			continue
		}
		faults = append(faults, c.Plugins[p].check(plan, string(p))...)
	}
	for _, f := range slices.SortedFunc(maps.Keys(c.Families), Family.compare) {
		declared, listed := outputs[f.Plugin]
		switch {
		case !listed:
			fault("a refinement names %s, which is no generator of the plan", f.Plugin)
			continue
		case !slices.ContainsFunc(declared, func(o plugin.Output) bool { return o.Tag == f.Tag }):
			fault("a refinement names family %q of %s, which %s does not declare", f.Tag, f.Plugin, f.Plugin)
			continue
		}
		faults = append(faults, c.Families[f].check(plan, fmt.Sprintf("family %q of %s", f.Tag, f.Plugin))...)
	}
	for _, p := range slices.Sorted(maps.Keys(outputs)) {
		faults = append(faults, c.checkGenerator(plan, p, outputs[p])...)
	}
	return faults
}

// AppendBinary implements [encoding.BinaryAppender]: it appends the
// configuration's canonical encoding to b and returns the extended
// buffer. The encoding spells the policy, the directory and the import
// base, then each generator refinement in generator order and each
// family refinement in family order, every string behind its length and
// every map behind its count. Two configurations append equal bytes
// exactly where their fields are equal, so a composition folds the
// encoding into its fingerprint. The error is always nil.
//
// # Allocation contract
//
// AppendBinary sorts the keys of each refinement map in a list of their
// own. A list of up to 32 bytes stays on the stack, so the generator
// keys allocate once beyond two generators, and the family keys once
// beyond one family. AppendBinary also grows b where b lacks the room.
func (c Config) AppendBinary(b []byte) ([]byte, error) {
	b = append(b, byte(c.Policy))
	b = appendString(b, c.Dir)
	b = appendString(b, c.ImportBase)
	b = binary.AppendUvarint(b, uint64(len(c.Plugins)))
	if len(c.Plugins) > 0 {
		generators := make([]plugin.ID, 0, len(c.Plugins))
		for p := range c.Plugins {
			generators = append(generators, p)
		}
		slices.Sort(generators)
		for _, p := range generators {
			b = appendString(b, string(p))
			b = c.Plugins[p].appendBinary(b)
		}
	}
	b = binary.AppendUvarint(b, uint64(len(c.Families)))
	if len(c.Families) > 0 {
		families := make([]Family, 0, len(c.Families))
		for f := range c.Families {
			families = append(families, f)
		}
		slices.SortFunc(families, Family.compare)
		for _, f := range families {
			b = appendString(b, string(f.Plugin))
			b = appendString(b, f.Tag)
			b = c.Families[f].appendBinary(b)
		}
	}
	return b, nil
}

// checkGenerator validates the routing each declared family of one
// generator resolves to: a family that writes under a directory has
// one, and a refinement's directory is read by a family it applies to.
func (c Config) checkGenerator(plan string, p plugin.ID, declared []plugin.Output) []error {
	var faults []error
	fault := func(format string, a ...any) {
		faults = append(faults, fmt.Errorf("layout: plan %q: "+format, append([]any{plan}, a...)...))
	}
	generatorDirRead := false
	for _, o := range declared {
		f := Family{Plugin: p, Tag: o.Tag}
		r := c.resolve(f)
		own := c.Families[f]
		needsDir := o.Per == plugin.PerPlan || r.policy == PolicyCentralised
		switch {
		case needsDir && r.dir == "":
			fault("family %q of %s writes under a directory, and the plan states none", o.Tag, p)
		case !needsDir && own.Dir != "":
			fault("family %q of %s is written beside its source, and its directory %q goes unread",
				o.Tag, p, own.Dir)
		}
		if needsDir && own.Dir == "" {
			generatorDirRead = true
		}
	}
	if gen, refined := c.Plugins[p]; refined && gen.Dir != "" && !generatorDirRead {
		fault("no family of %s reads the generator's directory %q: each is written beside its source "+
			"or under a directory of its own", p, gen.Dir)
	}
	return faults
}

// resolve returns one family's routing configuration.
func (c Config) resolve(f Family) resolved {
	own, gen := c.Families[f], c.Plugins[f.Plugin]
	return resolved{
		policy: own.Policy.or(gen.Policy.or(c.Policy.or(PolicyAlongside))),
		dir:    cmp.Or(own.Dir, gen.Dir, c.Dir),
		file:   cmp.Or(own.File, gen.File),
	}
}

// directory reports whether a configured directory is
// workspace-relative, slash-separated and inside the workspace root.
func directory(dir string) bool {
	return fs.ValidPath(dir) && !strings.Contains(dir, `\`)
}

// filename reports whether a configured filename is one path element.
func filename(name string) bool {
	return fs.ValidPath(name) && name != "." && !strings.ContainsAny(name, `/\`)
}

// appendString appends s behind its length, an unsigned varint, and
// returns the extended buffer. It allocates only where b lacks the room.
func appendString(b []byte, s string) []byte {
	b = binary.AppendUvarint(b, uint64(len(s)))
	return append(b, s...)
}
