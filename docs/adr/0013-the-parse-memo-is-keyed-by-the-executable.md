---
adr: 0013
title: The parse memo is keyed by the executable and can be shared
status: Accepted
date: 2026-10-02
supersedes: none
superseded-by: none
rfc: RFC-0020
---

<!--
  ~ Copyright Dokimasia B.V. 2026
  ~ SPDX-License-Identifier: Apache-2.0
-->

# ADR-0013: The parse memo is keyed by the executable and can be shared

## Status

Accepted

## Context

The parse memo keeps the regions of units that earlier runs parsed, so a
branch switch or a restored cache restores a unit without parsing it. It was
specified with a key of the unit's fingerprint and the frontend's declared
version, in the brand's state directory, capped and evicted by least recent
use at the commit. The unit's fingerprint folded the composition's
fingerprint.

Three facts conflict with that specification:

- The key trusts the frontend's declared version. A frontend whose code
  changes without a new version serves regions the new code would not
  produce.
- A region depends on the frontend and the unit's inputs alone. Folding the
  composition's fingerprint makes every entry miss after any change to the
  composition, such as a new annotator.
- Two worktrees of one repository each have a state directory, so each
  parses every unit the other already parsed.

A sealed generation is checked against the SHA-256 of the running
executable, and the memo can use the same digest. Two builds of unchanged
source produced byte-identical executables under Go 1.27.1, so a rebuild
that changes no code keeps the digest.

## Decision

We will key a memo entry by the digests of the unit's inputs and the digest
of the running executable, and keep the entries in a ledger the composition
chooses, because an entry must equal a fresh parse without trusting a
declared version, and an entry addressed by its content is safe to share.

- The default ledger is the composition's own, under `memo/`.
  `ledger.OpenAt` opens a directory that more than one workspace shares.
- A run writes its entries without a sync. An entry ends in a CRC-32C, so
  an entry a machine crash tore decodes as a miss and is removed.
- A hit touches the entry. The commit evicts the oldest entries by
  modification time, down to 90% of the cap, when its size total exceeds
  the cap or when the last full listing is more than a day old.

## Alternatives Considered

### Keep the composition fingerprint in the unit key

Every unit key folds the composition's fingerprint, and the memo keys on
the unit key. No entry then serves a run after the composition that
produced it changes. It
lost because a region does not depend on the composition: a frontend's
classification stamps are raw data that the run interprets after the load.
The fold makes every entry miss after any change to the composition, and the
sealed generation already checks the composition.

### A memo in the state directory only

The memo is always under the workspace's own state directory, and every
workspace keeps its own. No two processes then write one memo. It lost
because two worktrees of one repository each parse every unit the other
already parsed, and entries addressed by their content are correct without
isolation.

## Consequences

**Positive:**

- An entry serves any run of the build that wrote it, after a branch switch,
  a cold run or a change to the composition.
- A frontend changed without a new version cannot serve a stale region.
- Worktrees that open one memo ledger restore each other's units.

**Negative:**

- A build that changes the executable stops the earlier build's entries from
  serving, and they age out by eviction.
- A shared memo's size total can lose updates between concurrent commits,
  so the memo exceeds its cap by at most one day of writes.
- A composition that shares a memo chooses and opens its location.

**Neutral:**

- Eviction by least recent use at the commit, and the absence of a remote
  cache protocol, are unchanged.

## References

| What | Where |
|---|---|
| D77, the parse memo that this decision replaces | ../architecture/21-decisions.md |
| Go, the build cache's daily trim | https://github.com/golang/go/blob/67c1d421161d3d1ae9f5fd005e84c29fd0d9f896/src/cmd/go/internal/cache/cache.go |
| ccache, eviction by modification time | https://ccache.dev/manual/latest.html#_cache_size_management |
