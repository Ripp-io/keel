# Compiler Examples and Fixture Tests

Curated layer examples used by fixture_test.go for behavioral contract testing.
Unlike the golden-file tests in pkg/compiler/testdata/, these tests assert on
behavioral properties (contains/not-contains) rather than exact file matches.

The layers/ directory contains 8 progressively layered configurations covering
cross-cutting concerns: force-locks, overlays, variable resolution, conditionals,
portability warnings, MCP filtering, snippets, and convention defaults.
