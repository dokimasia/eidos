---
adr: 0110
title: An acronym keeps the s of its plural
status: Accepted
date: 2026-10-27
supersedes: none
superseded-by: none
rfc: none
---

<!--
  ~ Copyright Dokimasia B.V. 2026
  ~ SPDX-License-Identifier: Apache-2.0
-->

# ADR-0110: An acronym keeps the s of its plural

## Status

Accepted

## Context

Every satellite spells its names and its filenames through `naming`. The Go backend writes each
declared name through `naming.Pascal` or `naming.Camel` with the initialisms of `naming.Default`.
The other satellites write through a caser of `naming.New()`, which recognises no initialism.

- `naming.Words` split an acronym run before its last capital where a lower-case rune followed it.
  Issue 92 reports that the rule split the plural `IDs` into `I` and `Ds`. `Kebab("UserIDs")` was
  `user-i-ds`, `Snake("ListURLs")` was `list_ur_ls`, and `Camel("IDs")` was `iDs`.
- A title style title-cases a word that is not an initialism and not an acronym run. With the split
  of the issue alone, `Pascal("ListURLs")` would have become `ListUrls` and `Pascal("PDFs")`
  `Pdfs`. A generated method would then have lost the name of the method that it implements.
- `Pascal("user_ids")` wrote `UserIds`, although the plural of a recognised initialism keeps it
  upper-case in Go, as the field `URIs` of `crypto/x509.Certificate` does.
- `naming.Default` recognises a list of 38 initialisms. The naming package and the protobuf bridge
  of the previous eidos kept the same list.

## Decision

We will make each recognised initialism and every other acronym run keep the s of its plural.

- `Words` keeps a lone lower-case s after an acronym run with the run where the s ends the input
  or a rune other than a lower-case letter follows it. `UserIDs` splits into `User` and `IDs`, and
  `URLsToFetch` into `URLs`, `To` and `Fetch`. `IDsort` splits into `I` and `Dsort` as before,
  because a lower-case rune follows its s.
- `Pascal` and `Camel` keep an acronym run with its plural s as written, where they keep an
  acronym run, with every caser. `Camel` writes it in lower case as the first word, so `IDsToFetch`
  becomes `idsToFetch`.
- A word of a recognised initialism and a final lower-case s spells the initialism and the s in
  `Pascal` and `Camel`. `user_ids` and `UserIds` become `UserIDs` through `naming.Default`. The
  initialism itself takes precedence, so `https_proxy` becomes `HTTPSProxy`.
- The list of initialisms stays `naming.CommonInitialisms`.
- The backends whose spelling of a plural run changes have new versions: Go 0.13.0, Java 0.9.0,
  Rust 0.11.0, TypeScript 0.11.0, and the GraphQL, protobuf and Python backends 0.3.0.

## Alternatives Considered

### The split alone

The issue requests the rule of `Words`. The title styles then title-case the words `IDs` and `URLs`.
`Pascal("ListURLs")` measured `ListUrls`, and the Go backend would have renamed every declared name
with a plural acronym.

### The plurals as initialisms

`WithInitialisms` refuses a lower-case letter, so `IDs` cannot enter the list. A list of plurals also
leaves an acronym run outside the list, such as `PDFs`, split into `PD` and `Fs`.

### A plural rule for recognised initialisms alone

`PDFs` and every other acronym run that no list names would keep the split of the issue. A
satellite on `naming.New()` recognises no initialism, so its names would not change at all.

## Consequences

**Positive:**

- `Kebab("ReturnsDistinctIDs")` is `returns-distinct-ids`, the identity that the suite generator of
  dokimi takes for a check.
- Rust and Python write `ListURLs` as `list_urls`, and every title style keeps `ListURLs` and
  `userIDs` as written.
- Go writes the plural of an initialism in one case, as it writes the initialism.

**Negative:**

- Go writes a name that a source spells `userIds` as `userIDs`, as it writes `userId` as `userID`.
- A run of capitals before an s that starts a word, such as `IDsort`, still splits before its
  last capital.
- `Pascal` and `Camel` look up a second initialism for a word that ends in s.
- In interleaved runs, `Words` measured 6% to 8% slower for an identifier of 19 bytes without a
  plural, about 8 ns, at the same allocations. A function that this decision does not change moved
  4% in the same runs.

## References

| What | Where |
|---|---|
| D228, the row of this decision | ../architecture/21-decisions.md |
| D172, the casers without initialisms | ../architecture/21-decisions.md |
| Names across the boundary | ../architecture/10-cross-language.md |
| The Go wiki, initialisms | <https://go.dev/wiki/CodeReviewComments#initialisms> |
| Issue 92, Words splits a plural initialism such as IDs into I and Ds | <https://github.com/dokimasia/eidos/issues/92> |
