package compiler

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"strings"
	"time"
)

// verifyInstalled runs a verify command with a 30-second timeout and returns
// true if the command exits 0. Uses a sanitized environment.
func verifyInstalled(verifyCmd, targetDir string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "sh", "-c", verifyCmd)
	cmd.Env = prependPath(sanitizeEnv(os.Environ()), targetDir)
	return cmd.Run() == nil
}

// executeInstall runs an install command via sh -c with a sanitized
// environment that has PATH prepended and KEEL_DEP_DIR / KEEL_DEP_NAME
// injected so the command knows where Keel expects the binary to end up.
// Sensitive environment variables (API keys, tokens, credentials) are
// stripped to prevent exfiltration by layer-controlled commands.
func executeInstall(name, installCmd, targetDir string) error {
	slog.Info("executing dep install command", "dep", name, "command", installCmd)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	cmd := exec.CommandContext(ctx, "sh", "-c", installCmd)
	cmd.Env = prependPath(sanitizeEnv(os.Environ()), targetDir)
	cmd.Env = append(cmd.Env, "KEEL_DEP_DIR="+targetDir, "KEEL_DEP_NAME="+name)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("install command failed: %w\noutput: %s", err, string(output))
	}
	slog.Info("dep installed via command", "dep", name)
	return nil
}

// prependPath returns a copy of env with targetDir prepended to PATH.
func prependPath(env []string, targetDir string) []string {
	result := make([]string, 0, len(env))
	found := false
	for _, e := range env {
		if strings.HasPrefix(e, "PATH=") {
			result = append(result, "PATH="+targetDir+string(os.PathListSeparator)+e[5:])
			found = true
		} else {
			result = append(result, e)
		}
	}
	if !found {
		result = append(result, "PATH="+targetDir)
	}
	return result
}
