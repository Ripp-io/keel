# Contributing

## Prerequisites

- Go 1.26+
- [Hugo extended](https://gohugo.io/installation/) (for docs only)

## Project Structure

```
cmd/keel/           CLI entry point
pkg/                Go packages
  compiler/         Compilation pipeline, adapters, IR, portability
  config/           Layer resolution, compile config, agent registry
  globaldeps/       Global dependency manifest (~/.keel/bin/)
  merge/            Merge strategies, frontmatter, variables, snippets
  policy/           Merge strategy types
  remote/           Remote layer fetching (git, HTTP, cache)
  schema/           DSL schema, defaults, validation
  skills/           Agent Skills spec validation
  validate/         Layer validation and linting
  pathutil/         Path safety, file size limits
  cli/              Logging
  archive/          Packaging
adapters/           Container adapter scripts (per-agent)
containers/         Containerfile, runtime config, shared libs
docs/               Hugo documentation site
examples/           Demo layers + compiler test fixtures
layers/             Distributable Keel layers (e.g., authoring)
```

## Build & Test

```bash
make build-cli          # Build to dist/keel
make test               # Full test suite with coverage → dist/coverage.out
make test-short         # Fast tests, no coverage
make demo               # Compile the 5-layer demo
make docs               # Local docs server at localhost:1313
make clean              # Remove dist/
```

## Running Tests

```bash
go test ./...                              # All tests
go test ./pkg/compiler/ -run TestFixture   # Fixture tests only
go test ./... -short                       # Skip long-running tests
```

The fixture test suite in `examples/compiler/` compiles an 8-layer stack and asserts against the output.
It's the primary integration test — run it after any compilation pipeline changes.

## Adding an Agent Adapter

Each agent needs a compiler adapter (Go) and optionally a container adapter (shell scripts).
The project ships with skills that guide the process:

- `/add-compiler-adapter` — scaffolds a new compiler adapter
- `/config-surface-audit` — audits the agent's config surface and adds it to the compatibility matrix

The adapter pattern is: define an `AgentProfile` struct, register via `init()`, let `CompileWithProfile()` handle the rest.
See `pkg/compiler/claude/` and `pkg/compiler/cursor/` for examples.

## Code Conventions

- **Error handling**: `fmt.Errorf` with `%w` wrapping. No panics in library code. No `log.Fatal` in packages.
- **Testing**: Table-driven tests, `t.TempDir()` for isolation, `t.Helper()` on helpers. Golden files in `testdata/expected/`.
- **Naming**: Packages are single-word (`merge`, `compiler`, `remote`). Types use Go conventions.
- **Dependencies**: Minimal. Currently cobra, yaml.v3, x/text. Justify any addition.

## Documentation

The docs site is a Hugo site in `docs/`.

```bash
make docs               # Serve locally
hugo -s docs --minify   # Build
```

When code changes affect user-facing behavior, update the docs.
The `/update-docs` skill guides which pages to update for a given code change.

## Branch Workflow

- Feature branches for all work
- Meaningful commit messages — describe the *why*
- CI runs `go build` + `go test -short` on every push
- Docs deploy to GitLab Pages on merge to main

## Skills

The project uses Keel to manage its own agent configuration.
Skills in `.claude/skills/` guide common development tasks:

| Skill | Purpose |
|-------|---------|
| `/config-surface-audit` | Audit agent config surface, add new agents |
| `/add-compiler-adapter` | Build a new compiler adapter |
| `/add-container-adapter` | Build a new container adapter |
| `/add-product-comparison` | Write a comparison doc |
| `/manage-backlog` | Manage GitLab issues |
| `/update-docs` | Update docs after code changes |
| `/update-tests` | Update tests after code changes |

## Backlog

Feature backlog is in [GitLab Issues](https://gitlab.com/ripp.io/oss/agentic/keel/-/issues).
Labels: `compiler`, `container`, `docs`, `tier-1`, `tier-2`.
