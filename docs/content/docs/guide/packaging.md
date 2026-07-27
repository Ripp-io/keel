---
title: Packaging
weight: 10
---

# Packaging

Package compiled output as archives or produce distributable layers that carry governance metadata forward through a compilation chain.

## Quick Examples

### Archive compiled output

```bash
keel compile --layer ./my-layer --output claude=./out --package tar.gz
```

Produces both the output directory and a `tar.gz` archive.

### Create a distributable layer

```bash
keel compile \
  --layer ./company-standards \
  --output claude=./distributable \
  --distributable "company-standards-v2"
```

The output preserves governance metadata (`_keel` keys, `force` frontmatter) so it can be used as an input layer for further compilation.

## Distributable Layers

Normally, Keel strips all governance metadata from compiled output — the agent sees clean, native config.
With `--distributable`, metadata is preserved.

### Using a distributable as a layer

```bash
keel compile \
  --layer ./distributable \
  --layer ./team-overrides \
  --output claude=./final
```

Force-locked values from the distributable carry through.
A team layer cannot override what the company layer locked.

### Supply chain

Distributables enable a governance supply chain:

```
Company layer → compile --distributable → Company distributable
                                             ↓
Team overrides + Company distributable → compile → Project output
```

Each level can lock values for downstream consumers.
See [Governance]({{< relref "governance" >}}) for details on force-locks.

## Archive Formats

### During compilation

```bash
keel compile --layer ./my-layer --output claude=./out --package tar.gz
keel compile --layer ./my-layer --output claude=./out --package zip
```

### From an existing directory

```bash
keel package --from ./out/claude
keel package --from ./out/claude --format zip
keel package --from ./out/claude --to /tmp/config.tar.gz
```

Supported formats: `tar.gz`, `tgz`, `zip`.
Symlinks are skipped.

## Lockfile

The `.keel.json` manifest records SHA-256 hashes of all layer content used during compilation.
Use `--locked` to verify that layers haven't changed since the last compile:

```bash
# First compile generates .keel.json with content hashes
keel compile --layer ./my-layer --output claude=.

# Subsequent compiles verify against the hashes
keel compile --layer ./my-layer --output claude=. --locked
```

If any layer file has changed, `--locked` fails with an error.
This is useful in CI pipelines to ensure deterministic builds.

## What's Next

- **[Governance]({{< relref "governance" >}})** — force-locks in distributable layers.
- **[Output Lifecycle]({{< relref "output-lifecycle" >}})** — pruning, cleaning, manifest tracking.
- **[CLI Reference]({{< relref "../reference/cli" >}})** — `--distributable`, `--package`, `--locked` flags.
