// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Package fixture declares the composition, the app and the search of the
// binaries that the tests of the acceptance kit build. One binary meets the
// contract, and each other binary breaks one rule of it.
package fixture

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"time"

	gocli "go.stealthscale.io/cli"

	"go.dokimi.dev/eidos/cli"
	eidos "go.dokimi.dev/eidos/core"
	"go.dokimi.dev/eidos/core/backend"
	"go.dokimi.dev/eidos/core/backend/render"
	"go.dokimi.dev/eidos/core/emit"
	"go.dokimi.dev/eidos/core/frontend/frontendtest"
	"go.dokimi.dev/eidos/core/output"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/symbol"
	"go.dokimi.dev/eidos/core/workspace"
)

// Brand is the brand of the composition of every binary.
const Brand output.Brand = "acme"

// The command of the binaries that panics, and its help text.
const (
	CrashName = "crash"
	crashHelp = "Panics."
)

// Help is the help text of the app of every binary.
const Help = "Generates the code of the acme platform."

// The composition renders each struct of the scripted language into the
// text file of its package, through one plan.
const (
	target   plugin.Target = "text"
	planName               = "mirrors"
)

// App returns the app of the binaries over compose: the kernel commands at
// its root, and the command crash, which panics.
func App(compose cli.Compose) *gocli.App[cli.Global] {
	app := cli.New(Help, compose)
	app.Command(CrashName, crashHelp, crash)
	return app
}

// Compose returns the composition of the binaries: the scripted frontend
// and one plan that mirrors each struct into the file mirror.txt of its
// package.
func Compose() *workspace.Builder {
	return composed(func(name string) string { return "For" + name })
}

// ComposeClock returns the composition of Compose with another mirror. The
// mirror puts the time of the run into the name of each struct, so no two
// runs write the same bytes.
func ComposeClock() *workspace.Builder {
	return composed(func(name string) string {
		return "For" + name + strconv.FormatInt(time.Now().UnixNano(), 10)
	})
}

// ConfigAbove returns the path of the config file of the brand in dir, or in
// the nearest parent directory of dir that has the file. The search does not
// stop at a version control marker. ConfigAbove reports false when no
// directory has the file.
func ConfigAbove(dir string) (string, bool) {
	name := cli.ConfigName(Brand)
	for d := dir; ; d = filepath.Dir(d) {
		file := filepath.Join(d, name)
		if _, err := os.Stat(file); err == nil {
			return file, true
		}
		if filepath.Dir(d) == d {
			return "", false
		}
	}
}

// crash is the handler of the command crash. It panics.
func crash(context.Context, *gocli.Run[cli.Global, struct{}]) error {
	panic("fixture: the crash command panics")
}

// composed returns the composition of the binaries. Its mirror gives the
// mirror of each struct the name that name returns for the struct.
func composed(name func(string) string) *workspace.Builder {
	mirror := eidos.NewPlugin("mirror").
		Output(plugin.Output{Per: plugin.PerPackage, Word: "mirror"}).
		Handle(eidos.OnStruct(func(m *eidos.StructMatch, e *eidos.Emitter) error {
			e.PackageFile().Append(&emit.Struct{Origin: m.Struct.Identity(), Name: name(m.Struct.Name)})
			return nil
		})).
		Generator()
	coverage := map[symbol.Fact]render.Verdict{}
	for _, f := range symbol.Facts() {
		coverage[f] = render.Renders
	}
	printer := backend.New("printer", target, plugin.CommentSyntax{Line: []string{"//"}}).
		KindTemplates(map[symbol.Kind]string{symbol.KindStruct: "type {{.Name}} struct{}\n"}).
		Naming(func(u plugin.Unit) string { return u.Word + ".txt" }).
		Scaffold(func(emit.Stmt, *render.ImportSet) ([]byte, error) {
			return nil, errors.New("fixture: the printer writes no statements")
		}).
		Imports(func(*render.ImportSet) string { return "" }).
		Finalise(func(src []byte) ([]byte, error) { return src, nil }).
		Coverage(render.Coverage{Facts: coverage}).
		Build()
	return workspace.New().
		Brand(Brand).
		Frontends(frontendtest.NewScripted()).
		Targets(target).
		Plans(workspace.Plan{Name: planName, Generators: []plugin.Generator{mirror}, Backend: printer})
}
