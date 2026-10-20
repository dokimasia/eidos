---
"go.dokimi.dev/eidos/lang/rust": patch
---

Fix `backend.Speller` to use the module that the first segment of a path names, as `std::io` for `io::Error`, and no item for a path from a crate.
