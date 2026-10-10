// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package load_test

import (
	"context"
	"strings"
	"testing"
	"testing/fstest"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/frontend/frontendtest"
	"go.dokimi.dev/eidos/core/frontend/load"
	"go.dokimi.dev/eidos/core/plugin"
)

// namePrefix opens the header line through which the naming language's
// file names its package.
const namePrefix = "// name "

// The paths of the tree whose target moves: the declaring package's
// second file, in the declaring unit's directory.
const movedFile = "a/left/m.zz"

// headed is the scripted language with a header that names a file's
// package: a first line "// name NAME" sets the name the file's unit
// spells for its packages, which the scripted language derives from
// the package's path otherwise. The header is source, so a unit's key
// moves with its name.
type headed struct {
	*frontendtest.Scripted
}

// Parse parses the scripted unit, then names its packages after the
// header of a member that has one.
func (f headed) Parse(ctx context.Context, u *plugin.SourceUnit) error {
	if err := f.Scripted.Parse(ctx, u); err != nil {
		return err
	}
	for _, ref := range u.Files() {
		b, err := u.Read(ref.Path)
		if err != nil {
			return err
		}
		first, _, _ := strings.Cut(string(b), "\n")
		if name, named := strings.CutPrefix(first, namePrefix); named {
			for _, p := range u.Graph().Packages() {
				p.Name = name
			}
		}
	}
	return nil
}

// Units of one package couple through the identities they declare and
// the names they spell, so a warm load parses the kept units an edit
// couples to and no others.
func TestCouple(t *testing.T) {
	t.Parallel()

	t.Run("Load", func(t *testing.T) {
		t.Parallel()

		t.Run("parses the other unit of a package where an edit adds an identity it declares", func(t *testing.T) {
			t.Parallel()

			warm, cold := warmCold(t,
				twoUnitTree("type Twin left\n", "type Other right\n"),
				twoUnitTree("type Twin left\n", "type Other right\ntype Twin right\n"))
			assert.Equal(t, fromOf(warm.report)[oneFile], load.FromParse, "the unit that declares Twin parses")
			assert.Equal(t, warm.report.Reparsed, 1, "though its key is the recorded one")
			assertSameLoad(t, warm, cold)
		})

		t.Run("keeps the other unit where an edit adds an identity it does not declare", func(t *testing.T) {
			t.Parallel()

			warm, cold := warmCold(t,
				twoUnitTree("type Twin left\n", "type Other right\n"),
				twoUnitTree("type Twin left\n", "type Other right\ntype Fresh int\n"))
			assert.Equal(t, fromOf(warm.report)[oneFile], load.FromGeneration, "the other unit is kept")
			assert.Equal(t, warm.report.Reparsed, 0, "and no unit parses again")
			assertSameLoad(t, warm, cold)
		})

		t.Run("parses every unit of a package that records a duplicate", func(t *testing.T) {
			t.Parallel()

			warm, cold := warmCold(t,
				twoUnitTree("type Twin left\n", "type Twin right\n"),
				twoUnitTree("type Twin left\n", "type Twin right\ntype Fresh int\n"))
			assert.Equal(t, fromOf(warm.report)[oneFile], load.FromParse, "the unit whose Twin is kept parses")
			assertSameLoad(t, warm, cold)
		})

		t.Run("parses the other unit of a package where the edit removes a duplicate", func(t *testing.T) {
			t.Parallel()

			warm, cold := warmCold(t,
				twoUnitTree("type Twin left\n", "type Twin right\n"),
				twoUnitTree("type Twin left\n", "type Other right\n"))
			assert.Equal(t, fromOf(warm.report)[oneFile], load.FromParse, "the unit whose Twin was kept parses")
			assertSameLoad(t, warm, cold)
		})

		t.Run("parses every unit of a package whose units spell two names after an edit", func(t *testing.T) {
			t.Parallel()

			named := with(headed{frontendtest.NewScripted()})
			warm, cold := warmCold(t,
				twoUnitTree("type A int\n", "type B int\n"),
				fstest.MapFS{
					oneFile: {Data: []byte("package " + sharedPath + "\ntype A int\n")},
					twoFile: {Data: []byte(namePrefix + "other\npackage " + sharedPath + "\ntype B int\n")},
				}, named)
			assert.Equal(t, fromOf(warm.report)[oneFile], load.FromParse, "the first name's unit parses")
			assertSameLoad(t, warm, cold)
		})

		t.Run("parses every unit of a package a new unit joins under another name", func(t *testing.T) {
			t.Parallel()

			named := with(headed{frontendtest.NewScripted()})
			before := fstest.MapFS{oneFile: {Data: []byte("package " + sharedPath + "\ntype A int\n")}}
			after := fstest.MapFS{
				oneFile: before[oneFile],
				twoFile: {Data: []byte(namePrefix + "other\npackage " + sharedPath + "\ntype B int\n")},
			}
			warm, cold := warmCold(t, before, after, named)
			assert.Equal(t, fromOf(warm.report)[oneFile], load.FromParse, "the unit already there parses")
			assertSameLoad(t, warm, cold)
		})

		t.Run("keeps every unit of a package a new unit joins under the same name", func(t *testing.T) {
			t.Parallel()

			before := fstest.MapFS{oneFile: {Data: []byte("package " + sharedPath + "\ntype A int\n")}}
			after := twoUnitTree("type A int\n", "type B int\n")
			warm, cold := warmCold(t, before, after)
			assert.Equal(t, fromOf(warm.report)[oneFile], load.FromGeneration, "the unit already there is kept")
			assertSameLoad(t, warm, cold)
		})

		t.Run("parses a kept unit of an importing language whose target moved to another file", func(t *testing.T) {
			t.Parallel()

			before := dualTree(leftPath)
			after := dualTree(leftPath)
			after["a/left/l.zz"] = &fstest.MapFile{Data: []byte("package a/left\ntype Other string\n")}
			after[movedFile] = &fstest.MapFile{Data: []byte("package a/left\ntype Thing string\n")}
			warm, cold := warmCold(t, before, after, with(&importing{frontendtest.NewScripted()}))
			assert.Equal(t, fromOf(warm.report)["svc/hold/h.zz"], load.FromParse,
				"the holder parses for the import of the moved target")
			assert.Equal(t, warm.report.Reparsed, 1, "the holder is the one kept unit that parses again")
			assertSameLoad(t, warm, cold)
		})

		t.Run("keeps a unit without the importer role whose target moved to another file", func(t *testing.T) {
			t.Parallel()

			before := dualTree(leftPath)
			after := dualTree(leftPath)
			after["a/left/l.zz"] = &fstest.MapFile{Data: []byte("package a/left\ntype Other string\n")}
			after[movedFile] = &fstest.MapFile{Data: []byte("package a/left\ntype Thing string\n")}
			warm, cold := warmCold(t, before, after)
			assert.Equal(t, fromOf(warm.report)["svc/hold/h.zz"], load.FromGeneration,
				"the holder's target keeps its identity")
			assertSameLoad(t, warm, cold)
		})
	})
}
