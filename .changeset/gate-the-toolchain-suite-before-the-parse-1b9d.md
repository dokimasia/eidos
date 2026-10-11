---
"go.dokimi.dev/eidos/core": patch
"go.dokimi.dev/eidos/sdk": patch
"go.dokimi.dev/eidos/lang/typescript": patch
---

Gate all three checks of `RunToolchainSuite` on the toolchain, and return the error of a `tsc` that does not run from the TypeScript `Parse`.
