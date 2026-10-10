---
adr: 0009
title: The carrier mark follows the brand
status: Accepted
date: 2026-09-30
supersedes: none
superseded-by: none
rfc: none
---

<!--
  ~ Copyright Dokimasia B.V. 2026
  ~ SPDX-License-Identifier: Apache-2.0
-->

# ADR-0009: The carrier mark follows the brand

## Status

Accepted

## Context

A carrier is the comment line that contains a directive. The
frontend kit finds a carrier line by its mark and strips the mark.
The one directive grammar parses the rest of the line. A product
reads a comment line as its own directive only when the line opens
with the product's mark.

Two forces pull against each other.

- Other tools write markers into the same comments. Kubernetes and
  kubebuilder mark Go declarations with lines such as
  `// +kubebuilder:object:root=true`, `// +groupName=example.com`,
  `// +k8s:deepcopy-gen=package` and `// +genclient`. On 2026-09-25
  the kit opened a carrier on `+` followed by a letter. Through the
  Go frontend, the first three lines each reported an Error, and
  `+genclient` attached an unclaimed directive.
- Two or more products built on the kernel can run in one
  repository. Each product is a consumer binary with a brand. The
  brand appears in its config file's name, `.<brand>.yaml`, in its
  state directory, `.<brand>/`, and as the owner in its provenance
  trailer, `<brand>:provenance`. With one mark for every product, each
  product reads the other products' directives. It reports every
  directive its own registry lacks as unclaimed.

The decision log made the ecosystem prefix permanent and never
configurable per plugin or per workspace, because a configurable
prefix forks the carrier grammar per workspace and breaks grep. A
brand is no workspace setting. A product compiles its brand in, so
the product's carriers have one spelling wherever it runs, and the
grammar after the mark is the same in every product.

The kernel's own names do not depend on the mark. Its metadata
namespace is `gen`, and its directive names, such as `meta`, `skip`
and `diag`, are bare names inside the payload.

## Decision

We will open every carrier with the composition's brand, as
`<brand>:` or `+<brand>:` to set a directive and `-<brand>:` to
negate one, because the brand already scopes a product's config,
state and outputs, and a carrier scoped the same way is read by its
own product alone.

- `Builder.Brand` declares the brand, and Build requires it. A brand
  is a lowercase letter followed by lowercase letters, digits and
  hyphens.
- The payload after the mark is the grammar's `name`, bare or
  `plugin:name`, followed by its arguments. A mark followed by
  anything but a letter opens no carrier.
- A line under another brand's mark is comment text, and so is a
  Kubernetes marker.
- The kernel's metadata keys remain `gen.*` in every product.

## Alternatives Considered

### Keep the mark of `+` followed by a letter

The kit read any comment line that opened with `+` and a letter as a
carrier. A directive then needs no brand and no prefix, which keeps
it as short as it can be. It lost because it reads other tools'
markers as directives: the Kubernetes and kubebuilder lines above
reported Errors or attached directives that no plugin registered.

### A fixed `+gen:` mark in every product

The specification wrote this mark, and the decision log made it
permanent. With one spelling in every product, one grep finds every
directive in a repository. Kubernetes markers are comment text under
this mark too. It lost because two products in one repository then
read each other's carriers. Neither product can tell its own
directives from the other product's.

### `+<brand>:` as the only mark

Grep and documentation then name one form per directive. Negation
remains the kernel's: `meta drop=` deletes a fact, and
`skip plugin=` opts a declaration out of a plugin. It lost on two
counts. Every carrier line shows in Go's documentation, because
go/ast omits the directive form `//name:value` from doc text and a
`+` before the name breaks that form. An author also has no way to
negate a plugin's directive in that plugin's own vocabulary.

### `+<brand>:` and `<brand>:`, without a negated form

The bare form is a second spelling of the set form. go/ast omits it
from doc text when the brand has no hyphen. It lost because it has
no negated form. An author opting one declaration out of one plugin
writes the kernel's `skip plugin=` with the plugin's name, and not
the plugin's own directive.

## Consequences

**Positive:**

- Two products built on the kernel run in one repository, and each
  reads only its own carriers.
- Kubernetes and kubebuilder markers are comment text in every
  product. The Go frontend keeps no list of other tools' `+` lines.
- Go's documentation omits a carrier written as `//<brand>:name`
  when the brand has no hyphen.

**Negative:**

- A carrier's spelling differs per product. Documentation for a
  plugin that more than one product composes writes the brand as a
  placeholder, and no single grep finds every product's directives.
- Two marks set a directive, so a search for one product's set
  directives matches two patterns.
- Renaming a product's brand rewrites every carrier its consumers
  wrote, as well as its config file, state directory and trailers.
- Build requires a brand from every composition, including one that
  declares no output.

**Neutral:**

- The grammar after the mark, the kernel's directive names and the
  `gen.*` metadata keys do not change.
- What a negated instance means for dispatch is a separate decision.

## References

| What | Where |
|---|---|
| D59, the refusal of a configurable prefix | ../architecture/21-decisions.md |
| D63, the provenance trailer | ../architecture/21-decisions.md |
| D70, discovery and state scoped by brand | ../architecture/21-decisions.md |
| go/ast, comment lines that doc text omits | https://pkg.go.dev/go/ast#CommentGroup.Text |
