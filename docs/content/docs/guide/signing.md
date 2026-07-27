---
title: Signing
weight: 12
---

# Signing

Keel signs layers and compiled output with Ed25519, proving who authored each layer and that the compiled config hasn't been tampered with.

## Quick Examples

### Sign a layer

```bash
# Generate a signing key
keel key generate

# Sign your layer
keel sign ./layers/company-policy
```

This writes `keel.sig` alongside `keel.yml`.

### Verify a layer

```bash
# Verify the signature
keel verify ./layers/company-policy
```

Keel checks that the signature is valid and that the layer content matches the signed hash.

### Compile with signing

```bash
# Compile with layer verification and output signing
keel compile --sign --verify-layers
```

Keel verifies all layer signatures before compiling, then signs the compiled manifest.

### Verify compiled output

```bash
# Verify the manifest signature
keel verify .
```

Keel checks that the compiled manifest signature is valid.

## How It Works

The signing chain: layer author signs → Keel verifies at compile time → Keel signs the output → consumer verifies.

**Layer signing** — the author signs the layer's content hash with their Ed25519 key.
The signature proves who authored the layer and that its content hasn't changed.

**Compilation verification** — `keel compile --verify-layers` checks every layer signature before processing.
If any signature is invalid or missing, compilation fails.

**Manifest signing** — `keel compile --sign` signs the compiled manifest with your key.
The signature proves who compiled the output and that the manifest hasn't been modified.

**Consumer verification** — `keel verify` checks signatures on layers or manifests.
`keel drift --verify` includes signature verification in drift checks.

## Key Management

Keel stores keys in `~/.keel/keys/`.
Private keys are PEM-encoded PKCS8 (mode 0600).
Public keys are PEM-encoded PKIX (mode 0644).

Key IDs are `keel:sha256:<hex>` fingerprints derived from the public key.

### Generate keys

```bash
# Generate a key named "default"
keel key generate

# Generate a key with a specific name
keel key generate --name work
```

### List keys

```bash
keel key list
```

Shows NAME, KEY ID, TYPE, and whether the private key is present.

### Export and share

```bash
# Export your public key
keel key export default > my-key.pub

# Share it with others
```

Publish your public key so others can verify your signatures.

### Trust keys from others

```bash
# Add a public key to your trusted keyring
keel key trust coworker-key.pub
```

Trusted keys live in `~/.keel/keys/trusted/`.

### Revoke trusted keys

```bash
# Remove a key from your trusted keyring
keel key revoke alice
keel key revoke keel:sha256:abc123...
```

Use the key name or key ID.

## Layer Signing

A layer signature is a `keel.sig` file alongside `keel.yml`:

```json
{
  "algorithm": "ed25519",
  "key_id": "keel:sha256:abc123...",
  "content_hash": "sha256:def456...",
  "signature": "base64-encoded-signature",
  "signed_at": "2026-06-18T12:00:00Z",
  "signer": "default"
}
```

The content hash covers all files in the layer directory except `keel.sig` itself.
This makes signing idempotent — you can re-sign without changing the hash.

### Sign a layer

```bash
cd ./layers/company-policy
keel sign .
```

Or from elsewhere:

```bash
keel sign ./layers/company-policy
```

Use a specific key:

```bash
keel sign ./layers/company-policy --key work
```

### How content hashing works

Keel hashes every file in the layer directory (sorted by path) to produce a deterministic content hash.
The `keel.sig` file is excluded from the hash — otherwise signing would change the hash, requiring another signature.

Changing any file in the layer invalidates the signature.

## Compilation Signing

`--sign` tells Keel to sign the compiled manifest.
`--verify-layers` tells Keel to verify all layer signatures before compiling.

```bash
keel compile --sign --verify-layers
```

### Verification flow

When `--verify-layers` is set:

1. Keel loads each layer
2. If a layer has `keel.sig`, Keel verifies the signature
3. If verification fails, compilation aborts
4. If the layer has no signature, compilation aborts (unless it's `.keel/local`)

The `.keel/local` layer is implicitly trusted — it's user-created content in the same repo, not a remote dependency.

### Signing flow

When `--sign` is set:

1. Keel compiles normally
2. After writing all output, Keel signs the manifest
3. The signature is written to `.keel.json` inside the agent directory (`.claude/`, `.cursor/`, etc.)

Use `--sign-key` to specify which key to use:

```bash
keel compile --sign --sign-key work
```

Default key name is `default`.

### Two-pass compilation

Signing requires a two-pass process:

1. **First pass** — compile all layers, write files, generate manifest
2. **Sign** — hash the manifest, sign it, write signature into manifest
3. **Rewrite** — write the updated manifest to disk

The manifest includes its own signature — the signature proves the manifest hasn't been modified after signing.

## Verification

Verify a layer:

```bash
keel verify ./layers/company-policy
```

Verify compiled output:

```bash
keel verify .
```

Keel auto-detects whether the path is a layer (has `keel.yml` and `keel.sig`) or compiled output (has `.keel.json` with a signature).

Include verification in drift checks:

```bash
keel drift --verify
```

This checks both content drift and signature validity.

## CI Pipeline Example

A typical CI workflow with signing:

```bash
# 1. Generate a signing key (one-time setup, store private key as CI secret)
keel key generate --name ci-bot

# 2. Sign all layers before committing
keel sign ./layers/company-policy
keel sign ./layers/team

# 3. Compile with verification and signing
keel compile --sign --sign-key ci-bot --verify-layers

# 4. Commit the compiled output (with signed manifest)
git add .claude/.keel.json
git commit -m "Update config"

# 5. In CI, verify the output hasn't been tampered with
keel verify .
keel drift --verify
```

Consumers can verify the signature before deploying the config.

## What Signing Does Not Do

Signing proves **provenance** and detects **tampering**, but it does not prevent runtime modification.

Agents read config files from disk.
If someone modifies `CLAUDE.md` after compilation, the agent will use the modified version.
Signing will detect the tampering (via `keel verify` or `keel drift --verify`), but it won't stop the agent from reading the file.

### The enforcement gap

True enforcement requires agent vendor cooperation.
The agent must:

1. Refuse to load config that doesn't pass signature verification
2. Seal config at load time so it can't be modified during execution

This is analogous to container image signing — Docker can verify a signed image, but it's the runtime that enforces the policy.

Keel provides the signing mechanism and verification tooling.
The agent is responsible for enforcement.

### What you get today

- **Tamper detection** — `keel drift --verify` catches unauthorized changes
- **Provenance tracking** — the manifest records who signed each layer and the final output
- **CI gating** — verify signatures before deploying config
- **Audit trails** — signed manifests prove what was compiled and by whom

This is sufficient for:
- Detecting supply chain attacks on config layers
- Proving compliance (who authorized this config?)
- Gating deployments (don't deploy unsigned config)
- Post-incident forensics (was this config tampered with?)

It does not prevent an attacker with filesystem access from modifying config after it's deployed.

## What's Next

- **[Governance]({{< relref "governance" >}})** — force-locks and supply chain security.
- **[Output Lifecycle]({{< relref "output-lifecycle" >}})** — manifest structure and provenance tracking.
- **[CLI Reference]({{< relref "../reference/cli#keel-key" >}})** — all key, sign, and verify commands.
