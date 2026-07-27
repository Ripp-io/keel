# Enterprise Policy Distribution with Keel

## Overview

Central teams (security, platform, architecture) publish governed configuration layers that downstream teams consume automatically. Keel's remote layers and force-locks provide the enforcement mechanism.

## Architecture

```
Central Policy Repo (git)
├── layers/
│   ├── security/          ← Security team's policy layer
│   │   ├── keel.yml
│   │   ├── instructions.md
│   │   ├── permissions.json
│   │   └── hooks.json
│   └── platform/          ← Platform team's standards
│       ├── keel.yml
│       └── instructions.md
│
Downstream Team Repos
├── layers/team/           ← Team's own config
├── .keel/config.yml       ← References central policy
└── ...
```

## Setup

### 1. Create a policy layer

Policy layers use `force: true` in the `files:` section to lock output files:

```yaml
# layers/security/keel.yml
schema: v1
name: security-policy
description: Organization security standards

instructions:
  - source: instructions.md

permissions:
  rules:
    - source: permissions.json
  policy:
    merge: additive

hooks:
  - source: hooks.json

files:
  "CLAUDE.md":
    force: true
  ".claude/permissions.json":
    force: true
  ".claude/hooks.json":
    force: true
```

### 2. Publish via Git

Host the policy layers in a Git repo accessible to all teams.

### 3. Consume in downstream projects

Teams reference the policy layer as a remote layer in their config:

```yaml
# .keel/config.yml
layers:
  - 'https://git.example.com/platform/keel-policies.git#main:layers/security'
  - 'https://git.example.com/platform/keel-policies.git#main:layers/platform'
  - ./layers/team
output:
  claude: .
```

### 4. Verify policy compliance

```bash
# Check if compiled output matches policy expectations
keel drift --policy .

# In CI: fail on policy violations
keel drift --policy . || exit 1
```

## Policy Enforcement

### Force-locks
Files marked `force: true` in the policy layer cannot be overridden by downstream layers. If a team layer tries to write the same file, the policy layer's version wins.

### Layer ordering
Policy layers should appear FIRST in the layer stack. This ensures they set the baseline that team layers build on top of.

### Drift detection
`keel drift --policy` specifically checks force-locked files for unauthorized modifications. Use this in CI to catch manual edits that bypass the policy.

## CI/CD Integration

Add policy compliance checks to your pipeline:

```yaml
# GitLab CI
policy-check:
  stage: validate
  script:
    - keel compile --layer ... --output claude=. --strict
    - keel drift --policy .
  only:
    - merge_requests
```

## Updating Policies

When the central team updates a policy layer:
1. Downstream teams run `keel compile` — the remote layer is re-fetched
2. Force-locked files are updated automatically
3. `keel drift --policy` confirms compliance

Use `keel cache clean` to force a fresh fetch of remote layers.
