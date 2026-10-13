// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"context"

	gocli "go.stealthscale.io/cli"
	"go.stealthscale.io/cli/yaml"

	"go.dokimi.dev/eidos/cli/internal/config"
	"go.dokimi.dev/eidos/core/output"
)

// appRenderer is the renderer of the go-cli app that [Prepare] prepares. It
// writes the help and the table of --flag-debug as gocli.Plain does. It
// contains the global options of the run, so [Run] renders a failure of the
// run in the format and the colour of the run.
type appRenderer struct {
	gocli.Plain
	global Global
}

// New returns a go-cli app for the command line of a binary, with the help
// text help. The name of the app is the brand of the compositions that
// compose returns, so the brand acme reads the variable ACME_STRICT. New
// prepares the app with [Prepare], and adds the kernel commands run, plan,
// explain, prune, doctor, watch, version and schema at its root. A binary
// adds its own commands to the app, and runs it with [Main].
func New(help string, compose Compose) *gocli.App[Global] {
	app := gocli.New[Global](string(compose().BrandName()), help)
	Prepare(app, compose)
	k := kernels{compose: compose}
	app.Command(runName, runHelp, k.run)
	app.Command(planName, planHelp, k.plan)
	app.Command(explainName, explainHelp, k.explain)
	app.Command(pruneName, pruneHelp, k.prune)
	app.Command(doctorName, doctorHelp, k.doctor)
	app.Command(watchName, watchHelp, k.watch)
	app.Command(versionName, versionHelp, k.version)
	app.Command(schemaName, schemaHelp, k.schema)
	return app
}

// Prepare registers on app what a binary built with eidos needs from
// go-cli, for an app that a binary creates itself:
//
//   - the YAML format of go.stealthscale.io/cli/yaml for the config file.
//   - the keys of the workspace document, which go-cli skips in the config
//     file, because [Open] decodes them.
//   - the search for the config file of the brand of compose, as Open
//     searches.
//   - the renderer that [Run] reads the format of a run from.
//   - a hook that checks the working directory of each run, and makes a
//     relative Config absolute against the working directory of the
//     process, where go-cli read the file. Open then reads the same file
//     under Chdir.
//
// Prepare calls compose once, for the brand.
func Prepare(app *gocli.App[Global], compose Compose) {
	brand := compose().BrandName()
	yaml.Register(app)
	app.IgnoreKeys(config.Keys()...)
	app.LocateConfig(func(global *Global, _ *gocli.Env) (string, error) { return located(global.Chdir, brand) })
	app.RenderFor(func(global *Global) gocli.Renderer { return appRenderer{global: *global} })
	app.Use(absolute)
}

// Mount adds the eight kernel commands as subcommands of parent, over
// compose. A binary whose command gen is parent then runs acme gen run. The
// app of parent needs the registrations of [Prepare].
func Mount[C any](parent *gocli.Command[Global, C], compose Compose) {
	k := kernels{compose: compose}
	parent.Command(runName, runHelp, k.run)
	parent.Command(planName, planHelp, k.plan)
	parent.Command(explainName, explainHelp, k.explain)
	parent.Command(pruneName, pruneHelp, k.prune)
	parent.Command(doctorName, doctorHelp, k.doctor)
	parent.Command(watchName, watchHelp, k.watch)
	parent.Command(versionName, versionHelp, k.version)
	parent.Command(schemaName, schemaHelp, k.schema)
}

// located returns the path of the config file of brand that the search of
// [Open] finds for a command whose Chdir is chdir, and the empty path when
// the search does not find a file or the brand is not valid. Prepare
// registers it as the search of go-cli.
//
// Error modes: located returns an error that wraps a *UsageError for a
// working directory that does not resolve, and for a config file that does
// not read.
func located(chdir string, brand output.Brand) (string, error) {
	if !brand.Valid() {
		return "", nil
	}
	_, dir, err := workdir(chdir)
	if err != nil {
		return "", err
	}
	file, _, _, err := search(dir, ConfigName(brand))
	if err != nil {
		return "", &UsageError{Err: err}
	}
	return file, nil
}

// absolute is the hook that [Prepare] registers. It checks the working
// directory of the run, makes a relative Config absolute against the
// working directory of the process, and calls next.
//
// Error modes: absolute returns the error of next, and the error of
// workdir without calling next.
func absolute(ctx context.Context, run *gocli.Run[Global, Global], next func(context.Context) error) error {
	global := run.Global()
	cwd, _, err := workdir(global.Chdir)
	if err != nil {
		return err
	}
	if global.Config != "" {
		global.Config = gocli.ConfigFile(resolve(cwd, string(global.Config)))
	}
	return next(ctx)
}
