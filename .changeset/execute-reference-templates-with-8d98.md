---
"go.dokimi.dev/eidos/core": minor
---

**Breaking:** Execute each reference template with `missingkey=error`, so a template that reads a key that its data does not contain fails under `RefusedTemplate`.
