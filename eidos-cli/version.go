// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"cmp"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"fmt"
	"os"
	"runtime/debug"
	"strings"

	"go.dokimi.dev/eidos/core/workspace"
)

// The command version prints the versions of the modules with these paths.
const (
	coreModule = "go.dokimi.dev/eidos/core"
	cliModule  = "go.dokimi.dev/eidos/cli"
)

// The event of version, the roles of the plugins that only version lists,
// and the prefix of a digest.
const (
	eventVersion  = "version"
	roleGenerator = "generator"
	roleBackend   = "backend"
	digestPrefix  = "sha256:"
)

// moduleOf is the JSON form of the main module of the binary.
type moduleOf struct {
	Path    string `json:"path"`
	Version string `json:"version"`
}

// versionOf is the JSON form of the version event.
type versionOf struct {
	Module moduleOf `json:"module"`
	Core   string   `json:"core"`
	CLI    string   `json:"cli"`
	// Plugins are the plugins of the composition, each once.
	Plugins []component `json:"plugins"`
	// Composition is the SHA-256 of the fingerprint of the composition.
	// Executable is the SHA-256 of the running executable.
	Composition string `json:"composition"`
	Executable  string `json:"executable"`
}

// versionCommand returns the command version, which prints the versions of
// the binary, of eidos and of the plugins, and the digests of the
// composition and of the executable.
func versionCommand(compose Compose) *kernel {
	return &kernel{
		name:     "version",
		synopsis: "Prints the versions of the binary, of eidos and of the plugins of each workspace.",
		compose:  compose,
		define: func(*flag.FlagSet) runner {
			return func(_ context.Context, x *invocation) int { return x.version() }
		},
	}
}

// version renders one version event for each workspace. The sealed state
// records both digests, and the next run runs cold when either digest
// differs.
//
// version returns [StatusUsage] for a config error, [StatusFailed] for an
// executable that does not read, and [StatusOK] otherwise.
func (x *invocation) version() int {
	found, err := open(x.stdio, x.flags, x.k.compose)
	if err != nil {
		x.r.Error(err)
		return StatusUsage
	}
	exe, err := executable()
	if err != nil {
		x.r.Error(err)
		return StatusFailed
	}
	v := versionOf{Executable: exe}
	if info, ok := debug.ReadBuildInfo(); ok {
		versions := map[string]string{info.Main.Path: info.Main.Version}
		for _, d := range info.Deps {
			versions[d.Path] = d.Version
		}
		v.Module = moduleOf{Path: info.Main.Path, Version: info.Main.Version}
		v.Core, v.CLI = versions[coreModule], versions[cliModule]
	}
	for _, m := range found.members {
		if m.Name != "" {
			x.r.Workspace(m)
		}
		d := m.Workspace.Describe()
		sum := sha256.Sum256(d.Fingerprint)
		v.Composition = digestPrefix + hex.EncodeToString(sum[:])
		v.Plugins = pluginsOf(d)
		lines := []string{
			"module " + v.Module.Path + " " + v.Module.Version,
			"core " + v.Core,
			"cli " + v.CLI,
		}
		for _, p := range v.Plugins {
			lines = append(lines, strings.TrimSpace(fmt.Sprintf("%s %s %s", p.Role, p.Name, p.Version)))
		}
		lines = append(lines, "composition "+v.Composition, "executable "+v.Executable)
		x.r.Event(eventVersion, &v, strings.Join(lines, "\n"))
	}
	return StatusOK
}

// pluginsOf returns each plugin of a description once. The frontends come
// first, then the annotators, then the generators and the backend of each
// plan, and then the checks.
func pluginsOf(d workspace.Description) []component {
	var out []component
	seen := map[string]bool{}
	add := func(role string, c workspace.Component) {
		if !seen[string(c.Name)] {
			seen[string(c.Name)] = true
			out = append(out, component{Role: role, Name: string(c.Name), Version: c.Version})
		}
	}
	for _, c := range d.Frontends {
		add(roleFrontend, c)
	}
	for _, c := range d.Annotate {
		add(roleAnnotator, c)
	}
	for _, p := range d.Plans {
		for _, c := range p.Generate {
			add(roleGenerator, c)
		}
		add(roleBackend, p.Backend)
	}
	for _, c := range d.Checks {
		add(roleCheck, c.Component)
	}
	return out
}

// executable returns the SHA-256 of the running executable, as "sha256:"
// and the hex digest.
//
// Error modes: executable returns an error for an executable whose path
// does not resolve, or whose file does not read.
func executable() (string, error) {
	path, err := os.Executable()
	data, rerr := os.ReadFile(path)
	if err = cmp.Or(err, rerr); err != nil {
		return "", fmt.Errorf("cli: read the executable: %w", err)
	}
	sum := sha256.Sum256(data)
	return digestPrefix + hex.EncodeToString(sum[:]), nil
}
