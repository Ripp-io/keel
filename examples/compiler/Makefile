# Comprehensive E2E test fixture compilation
#
# Compiles all 6 test layers (3 bases + 3 overrides) for both Claude and Cursor.
# Exercises: DSL compilation, native transpilation, merge strategies, governance,
# carve-outs, overlay shadows, and portability warnings.
#
# Usage:
#   make              # compile for both agents
#   make clean        # remove compiled output

KEEL_ROOT := ../..
KEEL      := $(KEEL_ROOT)/dist/keel
LAYERS    := layers

.DEFAULT_GOAL := run

$(KEEL):
	$(MAKE) -C $(KEEL_ROOT) build-cli

.PHONY: run clean

run: $(KEEL)
	$(KEEL) compile \
		--layer "$(LAYERS)/*" \
		--output claude=output/claude \
		--output cursor=output/cursor \
		--summary

clean:
	rm -rf output/
