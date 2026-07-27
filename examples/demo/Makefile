# Enterprise Demo: Mixed-format layer compilation
#
# 5 layers mixing Keel DSL, Claude native, and Cursor native formats.
# Demonstrates cross-format compilation and governance.
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
