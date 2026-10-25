// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package layout

import (
	"errors"
	"fmt"
	"path"

	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/symbol"
)

// errPlacerInput is the reason of each preview of a placer whose input has
// no index or no filename speller, which [Route] refuses as well.
var errPlacerInput = errors.New("layout: a preview of a route needs an index and a filename speller")

// placer is the placer of the layout of one plan, which [NewPlacer]
// returns. It keeps the input of the plan and the paths of the kernel out
// directives on its packages, and each preview routes on a router of its
// own.
type placer struct {
	in       Input
	pkgPaths map[symbol.Identity]override
}

var _ plugin.Placer = (*placer)(nil)

// NewPlacer returns the placer of the layout of one plan over the routing
// of in. A preview applies the steps of [Route] to one declaration that
// takes the routing of a subject: the override on the subject and on its
// package, the destination of the family under the configuration, the
// filename of the unit, and the package rule of the target over the
// placement of the file. The placer reads the index, the configuration,
// the families, the directives, the speller, the packager, the residents
// and the modules of in. Route reports an override whose path is a file
// for two or more families of the subject. A preview does not report it,
// because the ambiguity depends on every family into which the plugin
// places the declarations of the subject.
//
// # Concurrency
//
// The placer is safe for concurrent use where the halves of the target
// and the residents of in are. Each preview routes on a router of its own,
// and the placer reads in and the paths of the packages alone.
//
// # Allocation contract
//
// NewPlacer allocates the placer and the map of the paths of the kernel
// out directives on the packages of in, which is nil where no package
// states one. A preview allocates the joined directory of an override and
// of a centralised family, the filename that the speller spells, the path
// of the file, and what the residents and the package rule allocate. A
// refusal allocates its error.
func NewPlacer(in Input) plugin.Placer {
	p := &placer{in: in}
	if in.Index != nil {
		p.pkgPaths = packagePaths(in.Index)
	}
	return p
}

// Place implements [plugin.Placer]. It routes one declaration of u that
// takes the routing of subject as [Route] routes it, and names the file of
// u without its declarations. A unit with an origin in [plugin.Unit.Inside]
// places its file without the mark of a family that tests from outside. A
// placer without a packager gives the file the package of the unit, as
// Route does.
//
// Error modes: [errPlacerInput] for an input without an index or a
// speller, an error for a family that the plugin does not declare, an
// error with the message of the finding that Route reports where the route
// derives no destination, an error for a filename that is not one path
// element, and the error of the package rule.
func (p *placer) Place(subject symbol.Identity, u plugin.Unit) (plugin.Placed, error) {
	placed := plugin.Placed{Origin: subject}
	if p.in.Index == nil || p.in.Speller == nil {
		return placed, errPlacerInput
	}
	own, declared := familyOf(p.in.Outputs[u.Plugin], u.Tag)
	if !declared {
		return placed, fmt.Errorf("layout: %s places into family %q, which it does not declare", u.Plugin, u.Tag)
	}
	r := router{in: p.in, pkgPaths: p.pkgPaths, previewing: true}
	described, _ := p.in.Index.Lookup(subject)
	dest, routed := r.destinationOf(&u, described, own, subject)
	if !routed {
		return placed, errors.New("layout: " + r.found.Msg)
	}
	placed.Dir = dest.dir
	if dest.per == plugin.PerPackage {
		placed.Files = dest.pkg
	}
	part := u
	part.Tag, part.Per, part.Word, part.Template = dest.tag, dest.per, dest.word, dest.template
	part.Key, part.Pkg = dest.key, dest.pkg
	name, err := r.fileNameOf(&dest, part)
	if err != nil {
		return placed, err
	}
	if p.in.Packager == nil {
		placed.Package = dest.pkg
		return placed, nil
	}
	fam, _ := familyOf(p.in.Outputs[u.Plugin], dest.tag)
	at := r.placementOf(path.Join(dest.dir, name), dest.pkg, nil, fam.External && len(u.Inside) == 0)
	pkg, err := p.in.Packager.PackageAt(at)
	if err != nil {
		return placed, err
	}
	placed.Package = pkg
	return placed, nil
}

// destinationOf returns the destination of one declaration of u that takes
// the routing of origin: the destination that the override on origin and
// on its package names, and the file of the family of u for its key
// otherwise. d names the declaration in a finding. It reports false, with
// the finding, where no destination derives.
func (r *router) destinationOf(
	u *plugin.Unit, d symbol.Symbol, own plugin.Output, origin symbol.Identity,
) (destination, bool) {
	ov := r.overrideOf(origin, u.Plugin)
	if ov == (override{}) {
		dest, _, found := r.target(u.Plugin, own, u.Key, u.Pkg)
		if !found {
			r.sourceless(u, d)
		}
		return dest, found
	}
	ov.origin = origin
	dest, _, found := r.destine(u, d, own, ov)
	return dest, found
}
