---
"go.dokimi.dev/eidos/core": minor
---

Add `ledger.GroupWriter`, and commit the files of `output.Disk` and the blobs of `ledger.Dir.WriteAll` as groups through one order barrier and one durability barrier.
