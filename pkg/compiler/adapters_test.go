// adapters_test.go uses the external test package so it can blank-import
// agent sub-packages without creating an import cycle. The init() functions
// in the sub-packages register compilers, readers, and writers that the
// internal test files (package compiler) rely on.
package compiler_test

import (
	_ "gitlab.com/ripp.io/oss/agentic/keel/pkg/compiler/agentplugins"
	_ "gitlab.com/ripp.io/oss/agentic/keel/pkg/compiler/agentsmd"
	_ "gitlab.com/ripp.io/oss/agentic/keel/pkg/compiler/aider"
	_ "gitlab.com/ripp.io/oss/agentic/keel/pkg/compiler/amp"
	_ "gitlab.com/ripp.io/oss/agentic/keel/pkg/compiler/antigravity"
	_ "gitlab.com/ripp.io/oss/agentic/keel/pkg/compiler/claude"
	_ "gitlab.com/ripp.io/oss/agentic/keel/pkg/compiler/cline"
	_ "gitlab.com/ripp.io/oss/agentic/keel/pkg/compiler/codex"
	_ "gitlab.com/ripp.io/oss/agentic/keel/pkg/compiler/copilot"
	_ "gitlab.com/ripp.io/oss/agentic/keel/pkg/compiler/cursor"
	_ "gitlab.com/ripp.io/oss/agentic/keel/pkg/compiler/goose"
	_ "gitlab.com/ripp.io/oss/agentic/keel/pkg/compiler/junie"
	_ "gitlab.com/ripp.io/oss/agentic/keel/pkg/compiler/kilo"
	_ "gitlab.com/ripp.io/oss/agentic/keel/pkg/compiler/kiro"
	_ "gitlab.com/ripp.io/oss/agentic/keel/pkg/compiler/opencode"
	_ "gitlab.com/ripp.io/oss/agentic/keel/pkg/compiler/openhands"
	_ "gitlab.com/ripp.io/oss/agentic/keel/pkg/compiler/qwen"
	_ "gitlab.com/ripp.io/oss/agentic/keel/pkg/compiler/vibe"
)
