package validate

// This file imports all adapter packages during tests to ensure their init()
// functions run and register agents. Without this, tests in pkg/validate that
// detect native agent layers would fail because the agent registry is empty.

import (
	_ "gitlab.com/ripp.io/oss/agentic/keel/pkg/compiler/aider"
	_ "gitlab.com/ripp.io/oss/agentic/keel/pkg/compiler/agentsmd"
	_ "gitlab.com/ripp.io/oss/agentic/keel/pkg/compiler/claude"
	_ "gitlab.com/ripp.io/oss/agentic/keel/pkg/compiler/cline"
	_ "gitlab.com/ripp.io/oss/agentic/keel/pkg/compiler/copilot"
	_ "gitlab.com/ripp.io/oss/agentic/keel/pkg/compiler/cursor"
)
