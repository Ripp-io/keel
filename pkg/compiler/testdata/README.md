# Compiler Test Data

Golden-file fixtures for compiler integration tests. Each subdirectory under
`layers/` is a named layer in a 7-layer enterprise stack. Tests compile
these layers and assert exact output matches against expected files.

Used by: compiler_claude_test.go, compiler_cursor_test.go, compiler_cross_test.go
