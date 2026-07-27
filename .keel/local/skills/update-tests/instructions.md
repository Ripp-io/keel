When implementing new features, fixing bugs, or changing compilation behavior, update tests as part of the work — not as a separate step.

Use the SKILL.md in this directory as the reference for:
- Where tests go (fixture vs unit test)
- Which source files map to which test files
- Fixture layer conventions and naming
- Cross-agent coverage requirements (Claude + Cursor)

Key rules:
- Every feature that changes compiled output needs a fixture assertion
- Every fixture assertion for Claude needs a Cursor counterpart (unless agent-specific)
- Schema changes need both validation tests AND fixture coverage
- New defaults in `keel.default.yml` need coverage in fixture layer 08 (convention-defaults)
- Update `buildFixtureLayers()` count when adding layers
- Run `go test ./...` before reporting work as complete
