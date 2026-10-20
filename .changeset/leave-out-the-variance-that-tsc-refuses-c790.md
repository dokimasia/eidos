---
"go.dokimi.dev/eidos/lang/typescript": patch
---

Fix `backend.Lower` to leave out the variance of the type parameters of functions, methods and aliases of types that are not object types.
