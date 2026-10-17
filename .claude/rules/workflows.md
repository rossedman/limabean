---
paths:
  - ".github/**"
  - ".goreleaser.yaml"
---

# Workflows and releases

- Pin every action by full commit SHA with the version in a comment:
  `uses: actions/checkout@<sha> # v7.0.1`.
- Give each workflow the least `permissions` it needs, starting from
  `contents: read`.
- Run `actionlint` after any change here.
- Downloaded tools are pinned by version and checked by SHA-256, as Lima is
  in `lint.yml`.
- Releases are GoReleaser (`.goreleaser.yaml`): one universal macOS binary.
  Check changes with `goreleaser check` if it's installed; CI runs it.
