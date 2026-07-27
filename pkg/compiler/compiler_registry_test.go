// compiler_registry_test.go — registry consistency tests verifying that
// every agent in config.AgentList has a compiler and reader registered,
// and vice versa.
package compiler

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"gitlab.com/ripp.io/oss/agentic/keel/pkg/config"
)

// --- Registry consistency ---

func TestCompilerRegistryMatchesAgents(t *testing.T) {
	t.Parallel()
	// Every agent in config.AgentList must have a compiler registered,
	// and every registered compiler must have an agent entry.
	for _, agent := range config.AgentList() {
		name := string(agent.Format)
		_, err := getCompiler(name)
		assert.NoError(t, err, "agent %q is in config.AgentList but has no compiler registered", name)
	}
	for name := range compilers {
		assert.True(t, config.IsAgentFormat(config.LayerFormat(name)),
			"compiler %q is registered but has no config.AgentList entry", name)
	}
}

func TestReaderRegistryMatchesCompilers(t *testing.T) {
	t.Parallel()
	// Every agent with a compiler should also have a reader registered,
	// and every registered reader should have a compiler.
	for name := range compilers {
		_, err := GetReader(name)
		assert.NoError(t, err, "compiler %q is registered but has no reader", name)
	}
	for _, name := range readerNames() {
		// "keel" is the DSL format reader — it reads keel.yml layers into
		// IR but does not have a per-agent compiler. The DSL compilation
		// pipeline is separate from agent-specific compilers.
		if name == string(config.FormatKeel) {
			continue
		}
		_, err := getCompiler(name)
		assert.NoError(t, err, "reader %q is registered but has no compiler", name)
	}
}
