// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package layout_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

	"go.dokimi.dev/eidos/core/layout"
	"go.dokimi.dev/eidos/core/plugin"
)

// The plan every configuration case checks under, and the generator no
// plan contains.
const (
	planName           = "services"
	ghost    plugin.ID = "ghost"
)

// checkAllocs is one check of a valid configuration over the fixture's
// generators, the same in each of 4 runs. Check sorts each map's keys
// through slices.Sorted over maps.Keys, which allocates the iterator
// and grows the list, and no other step allocates.
const checkAllocs = 12

// appendAllocs is one encoding of a configuration that refines three
// generators and three families into a buffer with room for it: the
// sorted keys of each of the two refinement maps, 48 and 96 bytes, which
// exceed the 32 bytes a list takes on the stack.
const appendAllocs = 2

// The import base the encoded configuration states, the generator one
// case moves a refinement to, the prefix the encoding appends after,
// and the number of encodings that repeat its bytes.
const (
	importBase           = "example.com/gen"
	movedGen   plugin.ID = "movedgen"
	prefix               = "fold:"
	encodings            = 32
)

// A configuration's contradictions are Build faults, so a misspelled
// generator or tag never waits for a run.
func TestConfig(t *testing.T) {
	t.Parallel()

	t.Run("Check", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name    string
			give    layout.Config
			outputs map[plugin.ID][]plugin.Output
			want    []string
		}{
			{
				name:    "returns no fault for the zero configuration of families beside their sources",
				outputs: sourceFamilies(),
			},
			{
				name:    "returns no fault for a plan directory a per-plan family reads",
				give:    layout.Config{Dir: genDir},
				outputs: families(),
			},
			{
				name: "returns no fault for a generator directory a centralised family reads",
				give: layout.Config{
					Plugins: refine(stubgen, layout.Refinement{Policy: layout.PolicyCentralised, Dir: genDir}),
				},
				outputs: sourceFamilies(),
			},
			{
				name:    "returns a fault for a plan policy outside the vocabulary",
				give:    layout.Config{Policy: layout.Policy(7)},
				outputs: sourceFamilies(),
				want:    []string{fault("Policy(7) is not a layout policy")},
			},
			{
				name:    "returns a fault for an output directory that leaves the workspace root",
				give:    layout.Config{Dir: "../gen"},
				outputs: sourceFamilies(),
				want:    []string{fault(`the output directory "../gen" is not a workspace-relative directory`)},
			},
			{
				name:    "returns a fault for an output directory with a backslash",
				give:    layout.Config{Dir: `gen\out`},
				outputs: sourceFamilies(),
				want:    []string{fault(`the output directory "gen\\out" is not a workspace-relative directory`)},
			},
			{
				name:    "returns a fault for an import base that is not a package path",
				give:    layout.Config{ImportBase: "/example.com/gen"},
				outputs: sourceFamilies(),
				want: []string{
					fault(`the import base "/example.com/gen" is not a slash-separated package path`),
				},
			},
			{
				name:    "returns a fault for a generator refinement of a generator outside the plan",
				give:    layout.Config{Plugins: refine(ghost, layout.Refinement{})},
				outputs: sourceFamilies(),
				want:    []string{fault("a refinement names ghost, which is no generator of the plan")},
			},
			{
				name:    "returns a fault for a generator refinement's policy outside the vocabulary",
				give:    layout.Config{Plugins: refine(stubgen, layout.Refinement{Policy: layout.Policy(9)})},
				outputs: sourceFamilies(),
				want:    []string{fault("stubgen refines to Policy(9), which is not a layout policy")},
			},
			{
				name:    "returns a fault for a generator refinement's filename of two path elements",
				give:    layout.Config{Plugins: refine(stubgen, layout.Refinement{File: "a/b.go"})},
				outputs: sourceFamilies(),
				want:    []string{fault(`stubgen refines its filename to "a/b.go", which is not one path element`)},
			},
			{
				name: "returns a fault for a family refinement of a generator outside the plan",
				give: layout.Config{Families: map[layout.Family]layout.Refinement{
					{Plugin: ghost}: {},
				}},
				outputs: sourceFamilies(),
				want:    []string{fault("a refinement names ghost, which is no generator of the plan")},
			},
			{
				name: "returns a fault for a family refinement of a tag its generator does not declare",
				give: layout.Config{Families: map[layout.Family]layout.Refinement{
					{Plugin: stubgen, Tag: "bogus"}: {},
				}},
				outputs: sourceFamilies(),
				want: []string{
					fault(`a refinement names family "bogus" of stubgen, which stubgen does not declare`),
				},
			},
			{
				name: "returns a fault for a family refinement's directory that leaves the workspace root",
				give: layout.Config{Families: map[layout.Family]layout.Refinement{
					{Plugin: stubgen, Tag: tagTest}: {Policy: layout.PolicyCentralised, Dir: "../x"},
				}},
				outputs: sourceFamilies(),
				want: []string{
					fault(`family "test" of stubgen refines its directory to "../x", ` +
						"which is not a workspace-relative directory"),
				},
			},
			{
				name:    "returns a fault for a per-plan family without a directory",
				outputs: families(),
				want: []string{
					fault(`family "plan" of stubgen writes under a directory, and the plan states none`),
				},
			},
			{
				name:    "returns a fault per family for a centralised policy without a directory",
				give:    layout.Config{Policy: layout.PolicyCentralised},
				outputs: sourceFamilies(),
				want: []string{
					fault(`family "" of docgen writes under a directory, and the plan states none`),
					fault(`family "" of stubgen writes under a directory, and the plan states none`),
					fault(`family "test" of stubgen writes under a directory, and the plan states none`),
				},
			},
			{
				name: "returns a fault for a family directory its policy leaves unread",
				give: layout.Config{Families: map[layout.Family]layout.Refinement{
					{Plugin: stubgen, Tag: tagTest}: {Dir: "x"},
				}},
				outputs: sourceFamilies(),
				want: []string{
					fault(`family "test" of stubgen is written beside its source, and its directory "x" goes unread`),
				},
			},
			{
				name:    "returns a fault for a generator directory no family reads",
				give:    layout.Config{Plugins: refine(stubgen, layout.Refinement{Dir: "x"})},
				outputs: sourceFamilies(),
				want: []string{
					fault(`no family of stubgen reads the generator's directory "x": ` +
						"each is written beside its source or under a directory of its own"),
				},
			},
			{
				name: "returns the faults of family refinements in plugin then tag order",
				give: layout.Config{Families: map[layout.Family]layout.Refinement{
					{Plugin: stubgen, Tag: "zeta"}:  {},
					{Plugin: stubgen, Tag: "alpha"}: {},
					{Plugin: docgen, Tag: "zeta"}:   {},
				}},
				outputs: sourceFamilies(),
				want: []string{
					fault(`a refinement names family "zeta" of docgen, which docgen does not declare`),
					fault(`a refinement names family "alpha" of stubgen, which stubgen does not declare`),
					fault(`a refinement names family "zeta" of stubgen, which stubgen does not declare`),
				},
			},
			{
				name: "returns every fault in configuration order",
				give: layout.Config{
					Policy:  layout.Policy(7),
					Plugins: refine(ghost, layout.Refinement{}),
					Families: map[layout.Family]layout.Refinement{
						{Plugin: stubgen, Tag: "bogus"}: {},
					},
				},
				outputs: families(),
				want: []string{
					fault("Policy(7) is not a layout policy"),
					fault("a refinement names ghost, which is no generator of the plan"),
					fault(`a refinement names family "bogus" of stubgen, which stubgen does not declare`),
					fault(`family "plan" of stubgen writes under a directory, and the plan states none`),
				},
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				got := []string{}
				for _, err := range tt.give.Check(planName, tt.outputs) {
					got = append(got, err.Error())
				}
				want := tt.want
				if want == nil {
					want = []string{}
				}
				assert.Equal(t, got, want, "the faults")
			})
		}
	})

	t.Run("AppendBinary", func(t *testing.T) {
		t.Parallel()

		t.Run("appends the same bytes on every call over refinement maps", func(t *testing.T) {
			t.Parallel()

			want := encoded(t, refined())
			for range encodings {
				assert.Equal(t, encoded(t, refined()), want, "the refinements encode in key order")
			}
		})

		t.Run("appends after the bytes of b", func(t *testing.T) {
			t.Parallel()

			got, err := refined().AppendBinary([]byte(prefix))
			assert.NoError(t, err, "the configuration encodes")
			assert.Equal(t, got, append([]byte(prefix), encoded(t, refined())...), "the encoding follows the prefix")
		})

		changes := []struct {
			name   string
			change func(*layout.Config)
		}{
			{
				name:   "appends other bytes for another policy",
				change: func(c *layout.Config) { c.Policy = layout.PolicyCentralised },
			},
			{
				name:   "appends other bytes for another directory",
				change: func(c *layout.Config) { c.Dir = tagTest },
			},
			{
				name:   "appends other bytes for another import base",
				change: func(c *layout.Config) { c.ImportBase = genDir },
			},
			{
				name: "appends other bytes for a directory that takes the first byte of the import base",
				change: func(c *layout.Config) {
					c.Dir, c.ImportBase = c.Dir+c.ImportBase[:1], c.ImportBase[1:]
				},
			},
			{
				name: "appends other bytes for a generator refinement under another generator",
				change: func(c *layout.Config) {
					r := c.Plugins[stubgen]
					delete(c.Plugins, stubgen)
					c.Plugins[movedGen] = r
				},
			},
			{
				name: "appends other bytes for another filename of a generator refinement",
				change: func(c *layout.Config) {
					r := c.Plugins[stubgen]
					r.File = tagPlan
					c.Plugins[stubgen] = r
				},
			},
			{
				name: "appends other bytes for a family refinement of another tag",
				change: func(c *layout.Config) {
					r := c.Families[layout.Family{Plugin: stubgen, Tag: tagTest}]
					delete(c.Families, layout.Family{Plugin: stubgen, Tag: tagTest})
					c.Families[layout.Family{Plugin: stubgen, Tag: tagPlan}] = r
				},
			},
			{
				name: "appends other bytes for another policy of a family refinement",
				change: func(c *layout.Config) {
					c.Families[layout.Family{Plugin: docgen}] = layout.Refinement{Policy: layout.PolicyCentralised}
				},
			},
			{
				name: "appends other bytes for a refinement moved from the generators to the families",
				change: func(c *layout.Config) {
					c.Families[layout.Family{Plugin: stubgen, Tag: tagPlan}] = c.Plugins[stubgen]
					delete(c.Plugins, stubgen)
				},
			},
		}
		for _, tt := range changes {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				changed := refined()
				tt.change(&changed)
				assert.NotEqual(t, encoded(t, changed), encoded(t, refined()), "the changed field changes the encoding")
			})
		}
	})
}

// A valid configuration checks, and a refined one encodes, within its
// ceiling in the ordinary run, which runs no benchmark.
func TestConfigAllocs(t *testing.T) {
	cfg, outputs := layout.Config{Dir: genDir}, families()
	var faults []error
	assert.MaxAllocs(t, func() { faults = cfg.Check(planName, outputs) }, checkAllocs,
		"Check allocates the sorted generators")
	assert.Empty(t, faults, "Check returns no fault for a valid configuration")

	full, room := refined(), make([]byte, 0, 256)
	var got []byte
	assert.MaxAllocs(t, func() { got, _ = full.AppendBinary(room) }, appendAllocs,
		"AppendBinary allocates the sorted keys of each refinement map")
	assert.Equal(t, got, encoded(t, full), "AppendBinary returns the encoding")
}

// BenchmarkConfig measures the check a Build runs over each plan's
// configuration, and the encoding the composition's fingerprint folds.
func BenchmarkConfig(b *testing.B) {
	b.Run("Check", func(b *testing.B) {
		b.Run("a valid configuration", func(b *testing.B) {
			cfg, outputs := layout.Config{Dir: genDir}, families()
			c := bench.Start(b).MaxAllocs(checkAllocs)
			defer c.End()
			var faults []error
			for c.Loop() {
				faults = cfg.Check(planName, outputs)
			}
			assert.Empty(b, faults, "Check returns no fault for a valid configuration")
		})
	})

	b.Run("AppendBinary", func(b *testing.B) {
		b.Run("a configuration that refines generators and families", func(b *testing.B) {
			full, room := refined(), make([]byte, 0, 256)
			want, err := full.AppendBinary(nil)
			assert.NoError(b, err, "the configuration encodes")
			c := bench.Start(b).MaxAllocs(appendAllocs)
			defer c.End()
			var got []byte
			for c.Loop() {
				got, err = full.AppendBinary(room)
			}
			assert.NoError(b, err, "AppendBinary returns no error")
			assert.Equal(b, got, want, "AppendBinary returns the encoding")
		})
	})
}

// sourceFamilies returns generators whose families are all written
// beside their sources: the families no zero configuration refuses.
func sourceFamilies() map[plugin.ID][]plugin.Output {
	all := families()
	return map[plugin.ID][]plugin.Output{
		stubgen: all[stubgen][:2],
		docgen:  all[docgen],
	}
}

// fault returns a fault's text under the fixture plan.
func fault(text string) string { return `layout: plan "services": ` + text }

// refine returns a generator refinement map of one entry.
func refine(p plugin.ID, r layout.Refinement) map[plugin.ID]layout.Refinement {
	return map[plugin.ID]layout.Refinement{p: r}
}

// refined returns a configuration that sets every field: a policy, a
// directory and an import base, refinements of three generators, and
// refinements of three families.
func refined() layout.Config {
	return layout.Config{
		Policy:     layout.PolicyAlongside,
		Dir:        genDir,
		ImportBase: importBase,
		Plugins: map[plugin.ID]layout.Refinement{
			stubgen: {Policy: layout.PolicyCentralised, Dir: genDir, File: tagTest},
			docgen:  {File: tagPlan},
			ghost:   {Dir: tagTest},
		},
		Families: map[layout.Family]layout.Refinement{
			{Plugin: stubgen, Tag: tagTest}: {Policy: layout.PolicyCentralised, Dir: tagTest},
			{Plugin: stubgen}:               {Dir: genDir},
			{Plugin: docgen}:                {File: tagPlan},
		},
	}
}

// encoded returns a configuration's encoding, and fails the test where
// AppendBinary returns an error.
func encoded(t *testing.T, c layout.Config) []byte {
	t.Helper()

	got, err := c.AppendBinary(nil)
	assert.NoError(t, err, "the configuration encodes")
	return got
}
