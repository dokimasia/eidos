---
"go.dokimi.dev/eidos/core": minor
"go.dokimi.dev/eidos/sdk": minor
---

**Breaking:** Seal the slots of every value in the store of a plan until the settle, so a direct `Slot.Append` panics there, and add `Slot.Replace`, `emit.AppendSealed` and `emit.Seal`.
