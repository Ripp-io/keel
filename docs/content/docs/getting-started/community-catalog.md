---
title: Community Catalog
weight: 2
---

# Community Catalog

Popular community config repos that work as Keel layers out of the box.
These repos have `.claude/`, `CLAUDE.md`, or other native config at their root — Keel auto-detects the format.

Add any of these to your `keel compile` command:

```bash
keel compile \
  --layer 'https://github.com/<owner>/<repo>.git#main' \
  --layer ./my-tools \
  --output claude=~ --scope user
```

See [Personal Toolkit]({{< relref "../guide/personal-toolkit" >}}) for the full guide on assembling your global config.

---

{{< community-catalog >}}

---

## Using Multiple Sources

Pick the sources you want and stack them in one command.
Later layers override earlier ones, so put your personal tools last:

```bash
keel compile \
  --layer 'https://github.com/JuliusBrussee/caveman.git#main' \
  --layer 'https://github.com/affaan-m/everything-claude-code.git#main' \
  --layer ./my-tools \
  --output claude=~ --scope user
```

## Adding to This List

Know a community repo that works well as a Keel layer?
Add an entry to `data/community_layers.yaml` and open an MR.

## What's Next

- **[Personal Toolkit]({{< relref "../guide/personal-toolkit" >}})** — full guide on assembling your global config.
- **[Layers]({{< relref "../guide/layers#remote-layers" >}})** — remote layer URL format, caching, and offline mode.
