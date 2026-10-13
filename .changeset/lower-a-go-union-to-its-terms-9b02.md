---
"go.dokimi.dev/eidos/lang/go": minor
---

Lower a Go union constraint such as `~int | time.Duration` to a `Union` of its terms, and import the package of each term.
