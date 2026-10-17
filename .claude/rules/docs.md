---
paths:
  - "docs/**"
  - "overrides/**"
  - "mkdocs.yml"
  - "README.md"
---

# Docs

- The site is MkDocs Material 1.x (2.0 drops theme overrides). A new page
  goes in `mkdocs.yml`'s `nav`. Build with
  `mkdocs build --strict -d /tmp/limabean-site`: strict fails on broken links
  and anchors, and the output must never land in the repo.
- Keep these in step when commands change: `docs/cli.md` (every command and
  setting), the README's command tables, and the command list on the
  landing page (`overrides/home.html`).
- `overrides/home.html` is the landing page (`docs/index.md` only selects
  it); its styles are `docs/assets/site/landing.css`, the palette in
  `docs/assets/src/brand.css`.
- Write plainly: short sentences, the reader's task first, commands they can
  paste. Say what something does, not that it's powerful or seamless. No
  claims the code doesn't back, such as timings nobody measured.
- Link the page that covers a topic instead of repeating it; anchors are the
  heading text, lower-cased with dashes.
- `docs/design.md` records decisions (D-numbered) and milestones; add to it
  when a change makes a design choice.
