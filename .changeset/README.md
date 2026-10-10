<!--
  ~ Copyright Dokimasia B.V. 2026
  ~ SPDX-License-Identifier: Apache-2.0
-->

# Changesets

A change that a release contains adds a changeset to this directory. A changeset is a Markdown
file: its front matter names each package that the change releases with its bump, `major`,
`minor` or `patch`, and its body is the entry in each package's changelog.

```md
---
"package-name": minor
---

Add the option that the entry describes.
```

The release flow turns the changesets on `main` into one version pull request. Merging that pull
request publishes the packages. `config.json` configures the flow.
