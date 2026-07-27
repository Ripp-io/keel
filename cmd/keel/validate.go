package main

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"gitlab.com/ripp.io/oss/agentic/keel/pkg/validate"
)

func newValidateCmd() *cobra.Command {
	var agent string
	var agentSkillsCheck bool
	var lint bool
	var tokenBudget bool
	var agentToolsCheck bool
	var strict bool

	cmd := &cobra.Command{
		Use:   "validate [path]",
		Short: "Validate a config layer",
		Long: `Validate a configuration layer directory.

Checks that the layer has valid structure, keel.yml schema is correct,
and all referenced files exist. Without arguments, validates the current
directory.

Use --agent to include agent-specific compatibility checks.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			path := "."
			if len(args) > 0 {
				path = args[0]
			}

			absPath, err := filepath.Abs(path)
			if err != nil {
				return fmt.Errorf("resolving path: %w", err)
			}

			result := validate.Run(absPath, validate.Options{
				Agent:            agent,
				AgentSkillsCheck: agentSkillsCheck,
				Lint:             lint,
				TokenBudget:      tokenBudget,
				AgentToolsCheck:  agentToolsCheck,
			})
			printValidationResult(cmd, result)

			if result.ErrorCount > 0 {
				return fmt.Errorf("%d validation error(s) found", result.ErrorCount)
			}

			// Strict mode: treat warnings as errors.
			if strict && result.WarnCount > 0 {
				return fmt.Errorf("--strict: %d warning(s) treated as errors", result.WarnCount)
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&agent, "agent", "", "Target agent for compatibility checks (claude, cursor)")
	cmd.Flags().BoolVar(&agentSkillsCheck, "agent-skills-check", true, "Run Agent Skills format validation on skill directories")
	// Backward compat: keep old name as deprecated alias
	cmd.Flags().BoolVar(&agentSkillsCheck, "agentskills-check", true, "")
	_ = cmd.Flags().MarkHidden("agentskills-check")
	_ = cmd.Flags().MarkDeprecated("agentskills-check", "use --agent-skills-check instead")
	cmd.Flags().BoolVar(&lint, "lint", true, "Scan markdown bodies for non-portable agent-specific patterns")
	cmd.Flags().BoolVar(&tokenBudget, "token-budget", true, "Estimate token counts and warn about budget thresholds")
	cmd.Flags().BoolVar(&agentToolsCheck, "agent-tools-check", true, "Warn when agent tool lists exceed threshold")
	cmd.Flags().BoolVar(&strict, "strict", false, "Exit with error when any warnings are found (for CI)")

	return cmd
}

// printValidationResult formats and prints the validation results.
func printValidationResult(cmd *cobra.Command, result validate.Result) {
	w := cmd.OutOrStdout()

	fmt.Fprintf(w, "Validating %s ...\n\n", result.Path)

	// Format info.
	formatStr := string(result.Format)
	if result.Version != "" {
		formatStr += " (" + result.Version + ")"
	}
	fmt.Fprintf(w, "  Format:  %s\n", formatStr)

	if result.Name != "" {
		fmt.Fprintf(w, "  Project: %s\n", result.Name)
	}
	fmt.Fprintln(w)

	// Individual items.
	for _, item := range result.Items {
		var prefix string
		switch {
		case item.OK:
			prefix = "  ✓ "
		case item.Warn:
			prefix = "  ! "
		default:
			prefix = "  ✗ "
		}
		fmt.Fprintf(w, "%s%s\n", prefix, item.Message)
	}

	fmt.Fprintln(w)

	// Summary.
	var parts []string
	if result.ErrorCount > 0 {
		parts = append(parts, fmt.Sprintf("%d error(s)", result.ErrorCount))
	} else {
		parts = append(parts, "0 errors")
	}
	if result.WarnCount > 0 {
		parts = append(parts, fmt.Sprintf("%d warning(s)", result.WarnCount))
	} else {
		parts = append(parts, "0 warnings")
	}

	fmt.Fprintf(w, "  Result: %s\n", strings.Join(parts, ", "))
}
