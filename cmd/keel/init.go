package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/spf13/cobra"
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/config"
	"gopkg.in/yaml.v3"
)

// validMCPName matches names containing only alphanumeric characters, hyphens,
// underscores, and dots. It rejects empty strings and names that start or end
// with a dot (which could cause hidden-file or extension-stripping issues).
var validMCPName = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._-]*$`)

// sanitizeMCPName validates that an MCP server name is safe for use as a
// filename component. It rejects path separators, ".." traversal sequences,
// and any characters outside the allowed set.
func sanitizeMCPName(name string) error {
	if name == "" {
		return fmt.Errorf("MCP server name must not be empty")
	}
	if strings.Contains(name, "..") {
		return fmt.Errorf("invalid MCP server name %q: must not contain '..'", name)
	}
	if strings.ContainsAny(name, `/\`) {
		return fmt.Errorf("invalid MCP server name %q: must not contain path separators", name)
	}
	if !validMCPName.MatchString(name) {
		return fmt.Errorf("invalid MCP server name %q: must contain only alphanumeric characters, hyphens, underscores, and dots", name)
	}
	return nil
}

// initConfig holds the collected answers from the init questionnaire.
type initConfig struct {
	Name  string
	Agent string
	Repos []initRepo
	MCP   []initMCP
}

type initRepo struct {
	Name string
	URL  string
}

type initMCP struct {
	Name    string
	Command string
}

func newInitCmd() *cobra.Command {
	var (
		name  string
		agent string
	)

	cmd := &cobra.Command{
		Use:   "init [dir]",
		Short: "Scaffold a new Keel project",
		Long: `Interactively create a keel.yml and instructions.md for a new project.

Asks a few questions and generates starter config files ready for compilation.
Use flags (--name, --agent) to skip prompts for scripted usage.

Examples:
  keel init
  keel init ./my-layer
  keel init ./my-layer --name my-project --agent claude`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := "."
			if len(args) > 0 {
				dir = args[0]
			}
			return runInit(cmd.InOrStdin(), cmd.OutOrStdout(), dir, name, agent)
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "Project name (default: current directory name)")
	cmd.Flags().StringVar(&agent, "agent", "", "Target agent: claude, cursor, agentsmd, cline, copilot, aider, all, or both")

	return cmd
}

func runInit(stdin io.Reader, stdout io.Writer, outputDir, flagName, flagAgent string) error {
	scanner := bufio.NewScanner(stdin)
	cfg := initConfig{}

	// Resolve output directory.
	absOutput, err := filepath.Abs(outputDir)
	if err != nil {
		return fmt.Errorf("resolving output directory: %w", err)
	}

	// 1. Project name
	if flagName != "" {
		cfg.Name = flagName
	} else {
		defaultName := filepath.Base(absOutput)
		cfg.Name = prompt(scanner, stdout, fmt.Sprintf("Project name [%s]: ", defaultName))
		if cfg.Name == "" {
			cfg.Name = defaultName
		}
	}

	// 2. Agent
	if flagAgent != "" {
		if err := validateAgent(flagAgent); err != nil {
			return err
		}
		cfg.Agent = flagAgent
	} else {
		agentNames := config.AgentNames()
		promptText := fmt.Sprintf("Agent (%s, all, both) [claude]: ", strings.Join(agentNames, ", "))
		cfg.Agent = prompt(scanner, stdout, promptText)
		if cfg.Agent == "" {
			cfg.Agent = "claude"
		}
		if err := validateAgent(cfg.Agent); err != nil {
			return err
		}
	}

	// 3. Repos and MCP servers (skip in non-interactive if no flags for them)
	if flagName == "" || flagAgent == "" {
		cfg.Repos = collectRepos(scanner, stdout)

		var mcpErr error
		cfg.MCP, mcpErr = collectMCPServers(scanner, stdout)
		if mcpErr != nil {
			return mcpErr
		}
	}

	// Generate files
	if err := os.MkdirAll(absOutput, 0o755); err != nil {
		return fmt.Errorf("creating output directory: %w", err)
	}

	keelPath := filepath.Join(absOutput, "keel.yml")
	if err := writeKeelYML(keelPath, cfg); err != nil {
		return err
	}

	permPath := filepath.Join(absOutput, "permissions.json")
	if err := writePermissionsFile(permPath); err != nil {
		return err
	}

	if len(cfg.MCP) > 0 {
		if err := writeMCPFiles(absOutput, cfg.MCP); err != nil {
			return err
		}
	}

	instrPath := filepath.Join(absOutput, "instructions.md")
	if err := writeInstructionsMD(instrPath, cfg.Name); err != nil {
		return err
	}

	// Print summary
	fmt.Fprintln(stdout)
	fmt.Fprintln(stdout, "Keel project initialized:")
	fmt.Fprintf(stdout, "  %s\n", keelPath)
	fmt.Fprintf(stdout, "  %s\n", instrPath)
	fmt.Fprintf(stdout, "  %s\n", permPath)
	fmt.Fprintln(stdout)

	// Build compile command suggestion based on agent choice
	if cfg.Agent == "both" {
		fmt.Fprintln(stdout, "Next: edit instructions.md, then run 'keel compile --output claude=. --output cursor=.'")
	} else if cfg.Agent == "all" {
		agentNames := config.AgentNames()
		var outputs []string
		for _, a := range agentNames {
			outputs = append(outputs, fmt.Sprintf("--output %s=.", a))
		}
		fmt.Fprintf(stdout, "Next: edit instructions.md, then run 'keel compile %s'\n", strings.Join(outputs, " "))
	} else {
		fmt.Fprintf(stdout, "Next: edit instructions.md, then run 'keel compile --output %s=.'\n", cfg.Agent)
	}

	return nil
}

// collectRepos interactively asks the user to add repository entries.
func collectRepos(scanner *bufio.Scanner, stdout io.Writer) []initRepo {
	addRepos := prompt(scanner, stdout, "Add repos? [y/N]: ")
	if !strings.EqualFold(addRepos, "y") && !strings.EqualFold(addRepos, "yes") {
		return nil
	}
	var repos []initRepo
	for {
		repoName := prompt(scanner, stdout, "  Repo name (empty to finish): ")
		if repoName == "" {
			break
		}
		repoURL := prompt(scanner, stdout, "  Repo URL: ")
		if repoURL == "" {
			continue
		}
		repos = append(repos, initRepo{Name: repoName, URL: repoURL})
	}
	return repos
}

// collectMCPServers interactively asks the user to add MCP server entries.
// Returns an error if a server name fails sanitization.
func collectMCPServers(scanner *bufio.Scanner, stdout io.Writer) ([]initMCP, error) {
	addMCP := prompt(scanner, stdout, "Add MCP servers? [y/N]: ")
	if !strings.EqualFold(addMCP, "y") && !strings.EqualFold(addMCP, "yes") {
		return nil, nil
	}
	var servers []initMCP
	for {
		mcpName := prompt(scanner, stdout, "  MCP server name (empty to finish): ")
		if mcpName == "" {
			break
		}
		if err := sanitizeMCPName(mcpName); err != nil {
			return nil, err
		}
		mcpCmd := prompt(scanner, stdout, "  MCP command: ")
		if mcpCmd == "" {
			continue
		}
		servers = append(servers, initMCP{Name: mcpName, Command: mcpCmd})
	}
	return servers, nil
}

// prompt prints the prompt text and reads a line from the scanner.
// Returns empty string on EOF or scan error.
func prompt(scanner *bufio.Scanner, stdout io.Writer, text string) string {
	fmt.Fprint(stdout, text)
	if !scanner.Scan() {
		return ""
	}
	return strings.TrimSpace(scanner.Text())
}

func validateAgent(agent string) error {
	if agent == "both" || agent == "all" {
		return nil
	}
	// Check if agent is in the registered agents list
	for _, a := range config.AgentNames() {
		if agent == a {
			return nil
		}
	}
	agentList := append(config.AgentNames(), "both", "all")
	return fmt.Errorf("invalid agent %q: must be one of %s", agent, strings.Join(agentList, ", "))
}

// writeKeelYML writes the keel.yml configuration file. It only writes the
// keel.yml itself — companion files like permissions.json and MCP JSON files
// are written by their own dedicated functions.
//
// All user-provided values are marshalled through the YAML library to ensure
// proper escaping of special characters (colons, quotes, etc.).
func writeKeelYML(path string, cfg initConfig) error {
	// Build the full config as a map so all values are properly escaped.
	data := map[string]any{
		"schema": "v1",
		"name":   cfg.Name,
		"instructions": []map[string]string{
			{"source": "instructions.md"},
		},
		"permissions": map[string]any{
			"rules": []map[string]string{
				{"source": "permissions.json"},
			},
		},
	}

	if len(cfg.Repos) > 0 {
		repos := make(map[string]map[string]any, len(cfg.Repos))
		for _, r := range cfg.Repos {
			repos[r.Name] = map[string]any{
				"url":      r.URL,
				"branch":   "main",
				"required": true,
			}
		}
		data["repos"] = repos
	}

	if len(cfg.MCP) > 0 {
		mcpEntries := make([]map[string]string, 0, len(cfg.MCP))
		for _, m := range cfg.MCP {
			mcpEntries = append(mcpEntries, map[string]string{
				"source": "mcp/" + m.Name + ".json",
			})
		}
		data["mcp"] = map[string]any{
			"servers": mcpEntries,
		}
	}

	out, err := yaml.Marshal(data)
	if err != nil {
		return fmt.Errorf("marshaling keel.yml: %w", err)
	}

	// Append commented-out repos template when no repos are provided,
	// as a hint for users editing the file.
	if len(cfg.Repos) == 0 {
		out = append(out, []byte("\n# repos:\n#   my-service:\n#     url: git@example.com:org/my-service.git\n#     branch: main\n#     required: true\n")...)
	}

	return os.WriteFile(path, out, 0o644)
}

// writePermissionsFile creates a permissions.json file with default allow rules.
func writePermissionsFile(path string) error {
	content := `{
  "allow": ["Read(*)", "Edit(*)", "Write(*)", "Bash(git:*)"]
}
`
	return os.WriteFile(path, []byte(content), 0o644)
}

// writeMCPFiles creates individual MCP server JSON configuration files in
// an mcp/ subdirectory under outputDir.
func writeMCPFiles(outputDir string, servers []initMCP) error {
	mcpDir := filepath.Join(outputDir, "mcp")
	if err := os.MkdirAll(mcpDir, 0o755); err != nil {
		return fmt.Errorf("creating mcp directory: %w", err)
	}
	for _, m := range servers {
		if err := sanitizeMCPName(m.Name); err != nil {
			return err
		}
		mcpFile := filepath.Join(mcpDir, m.Name+".json")
		mcpConfig := map[string]any{
			m.Name: map[string]any{
				"command": m.Command,
				"args":    []string{},
			},
		}
		content, err := json.MarshalIndent(mcpConfig, "", "  ")
		if err != nil {
			return fmt.Errorf("marshaling MCP config for %s: %w", m.Name, err)
		}
		if err := os.WriteFile(mcpFile, append(content, '\n'), 0o644); err != nil {
			return fmt.Errorf("writing MCP config %s: %w", mcpFile, err)
		}
	}
	return nil
}

func writeInstructionsMD(path, projectName string) error {
	content := fmt.Sprintf(`# %s

## Project Context
<!-- Describe what this project does -->

## Tech Stack
<!-- List languages, frameworks, and key dependencies -->

## Build Commands
<!-- How to build, test, and run the project -->

## Code Conventions
<!-- Style rules, naming conventions, patterns to follow -->
`, projectName)

	return os.WriteFile(path, []byte(content), 0o644)
}
