# Signing Fixture Tests

Behavioral contract tests for Ed25519 signing and verification.

Tests exercise the full signing pipeline: key management, layer signing,
signed compilation, manifest verification, and tamper detection.

Layers in `layers/` are minimal fixtures — do not modify them during tests.
Tests that need to write keel.sig or modify content copy layers to temp dirs.
