# Runtime Adapters

This directory contains **runtime adapters** for container-based deployment of Keel-managed agent configurations.

## What are runtime adapters?

Runtime adapters are backend services that enable agent-native tooling to run in containerized environments (web UI, API endpoints, cloud execution). They handle:

- Container lifecycle management
- Backend configuration and initialization
- User onboarding flows
- Runtime environment setup

## Distinction from compile-time adapters

**Runtime adapters** (this directory) are separate from **compile-time adapters** located in `pkg/compiler/{claude,cursor,agentsmd,...}/`.

- **Compile-time adapters**: Transform Keel DSL layers into agent-native configuration formats (readers and writers for each target agent)
- **Runtime adapters**: Provide backend infrastructure for running agents in production environments

The compile-time adapters produce the configuration; the runtime adapters execute it.

## Base image

The default Containerfile uses `node:22-bookworm-slim` (Debian bookworm).
All adapters work on this base — pip-based agents install Python via apt during their `install.sh`.
