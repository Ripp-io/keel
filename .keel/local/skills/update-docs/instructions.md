When implementing new features, fixing bugs, or changing user-facing behavior, update documentation as part of the work — not as a separate step.

Use the SKILL.md in this directory as the reference for:
- Which source files map to which doc pages
- The doc site structure and page purposes
- The style guide rules (summary-first, examples before explanation, one sentence per line)
- What warrants a doc update vs what doesn't

Key rules:
- New DSL fields need entries in `guide/dsl.md` (quick reference table + section) and `features.md`
- New CLI commands need entries in `reference/cli.md` (command table + section) and `features.md`
- New CLI flags need entries in the relevant command's flags table in `reference/cli.md`
- New auto-injected variables need entries in `guide/carve-outs.md`
- Read the style guide at the path referenced in SKILL.md before writing any doc content
- Run `hugo --source docs --quiet` to verify no broken links
- Do NOT rewrite pages from scratch — make targeted updates that fit the existing structure
