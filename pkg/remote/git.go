package remote

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"gitlab.com/ripp.io/oss/agentic/keel/pkg/pathutil"
)

// gitPathOnce caches the result of exec.LookPath("git") so it is resolved
// once per process lifetime rather than on every runGit invocation.
var (
	gitPathOnce sync.Once
	gitPath     string
	gitPathErr  error
)

// resetGitPathCache resets the cached git path lookup so subsequent calls to
// runGit will re-resolve the git binary. Used only by test code.
// Not safe for concurrent use — call only from test setup/teardown, never
// from production code.
func resetGitPathCache() {
	gitPathOnce = sync.Once{}
	gitPath = ""
	gitPathErr = nil
}

// FetchGit resolves a Git source to a local directory path.
// On first access it clones the repo as a mirror. On subsequent access it
// fetches new refs. Then it checks out the requested branch/ref into a worktree
// directory. Returns the local path to the layer directory (worktree root +
// subpath).
func FetchGit(src ParsedSource, cacheBase string, offline bool) (string, error) {
	// Defense-in-depth: Parse() already rejects ext:: URLs, but we check again
	// here in case FetchGit is ever called directly without going through Resolve.
	if strings.HasPrefix(src.RawURL, "ext::") {
		return "", fmt.Errorf("rejected dangerous git remote helper URL: %s", src.RawURL)
	}

	// 1. Compute cache key and set up directory structure.
	hash := CacheKey(src.RawURL)
	hashDir, err := EnsureCacheDir(cacheBase, "git", hash)
	if err != nil {
		return "", fmt.Errorf("preparing cache for %s: %w", src.RawURL, err)
	}

	// 2. Acquire advisory lock.
	lock, err := AcquireLock(hashDir)
	if err != nil {
		return "", fmt.Errorf("acquiring cache lock for %s: %w", src.RawURL, err)
	}
	defer ReleaseLock(lock)

	repoDir := filepath.Join(hashDir, "repo")

	// 3. Clone or fetch.
	cloneExists := gitMirrorExists(repoDir)

	if !cloneExists {
		if offline {
			return "", fmt.Errorf("no cached data for %s, run without --offline first", src.RawURL)
		}
		// Clone as mirror.
		if _, err := runGit("clone", "--mirror", src.RawURL, repoDir); err != nil {
			return "", fmt.Errorf("cloning %s: %w", src.RawURL, err)
		}
	} else if !offline {
		// Remove existing worktrees before fetching. Git refuses to fetch
		// into a branch that is checked out in a worktree, so we must clean
		// them up first. They are re-created from the updated mirror below.
		if err := removeAllWorktrees(repoDir, hashDir); err != nil {
			return "", fmt.Errorf("cleaning worktrees before fetch for %s: %w", src.RawURL, err)
		}
		// Fetch updates into existing mirror.
		if _, err := runGit("-C", repoDir, "fetch", "--all", "--prune"); err != nil {
			return "", fmt.Errorf("fetching updates for %s: %w", src.RawURL, err)
		}
	}
	// If clone exists and offline: skip fetch, use existing data.

	// 4. Resolve the target ref.
	branch := src.Branch
	if branch == "" {
		resolved, err := resolveDefaultBranch(repoDir)
		if err != nil {
			return "", fmt.Errorf("resolving default branch for %s: %w", src.RawURL, err)
		}
		branch = resolved
	}

	// 5. Check out worktree.
	sanitized := sanitizeBranch(branch)
	worktreeDir := filepath.Join(hashDir, "worktrees", sanitized)

	if err := ensureWorktree(repoDir, worktreeDir, branch); err != nil {
		return "", fmt.Errorf("checking out %s@%s: %w", src.RawURL, branch, err)
	}

	// 6. Apply subpath.
	resultDir := worktreeDir
	if src.SubPath != "" {
		resultDir = filepath.Join(worktreeDir, src.SubPath)
		if !pathutil.ContainsPath(worktreeDir, resultDir) {
			return "", fmt.Errorf("git subpath %q escapes worktree directory", src.SubPath)
		}
		info, err := os.Stat(resultDir)
		if err != nil {
			if os.IsNotExist(err) {
				return "", fmt.Errorf("path %q not found in %s", src.SubPath, src.RawURL)
			}
			return "", fmt.Errorf("checking subpath %q in %s: %w", src.SubPath, src.RawURL, err)
		}
		if !info.IsDir() {
			return "", fmt.Errorf("path %q in %s is a file, not a directory", src.SubPath, src.RawURL)
		}
	}

	// 7. Update meta.json.
	metaPath := filepath.Join(hashDir, "meta.json")
	meta := GitMeta{
		URL:         src.RawURL,
		LastFetched: time.Now().UTC(),
	}
	if err := WriteMeta(metaPath, meta); err != nil {
		return "", fmt.Errorf("writing meta for %s: %w", src.RawURL, err)
	}

	return resultDir, nil
}

// gitMirrorExists checks whether a mirror clone exists by looking for the HEAD
// file inside the repo directory.
func gitMirrorExists(repoDir string) bool {
	_, err := os.Stat(filepath.Join(repoDir, "HEAD"))
	return err == nil
}

// resolveDefaultBranch reads the default branch name from a mirror clone.
// It parses the output of `git symbolic-ref HEAD` which returns something like
// "refs/heads/main".
func resolveDefaultBranch(repoDir string) (string, error) {
	out, err := runGit("-C", repoDir, "symbolic-ref", "HEAD")
	if err != nil {
		return "", fmt.Errorf("reading HEAD: %w", err)
	}
	// Output is e.g. "refs/heads/main"
	ref := strings.TrimSpace(out)
	const prefix = "refs/heads/"
	if strings.HasPrefix(ref, prefix) {
		return ref[len(prefix):], nil
	}
	// Fallback: return the full ref.
	return ref, nil
}

// removeAllWorktrees removes all worktree directories and prunes worktree
// metadata from the mirror. This must be done before fetching because Git
// refuses to fetch into a branch that is checked out in a worktree.
func removeAllWorktrees(repoDir, hashDir string) error {
	worktreesDir := filepath.Join(hashDir, "worktrees")
	entries, err := os.ReadDir(worktreesDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("reading worktrees dir: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		p := filepath.Join(worktreesDir, entry.Name())
		if err := os.RemoveAll(p); err != nil {
			return fmt.Errorf("removing worktree %s: %w", p, err)
		}
	}

	// Prune stale worktree metadata in the mirror.
	_, _ = runGit("-C", repoDir, "worktree", "prune")
	return nil
}

// ensureWorktree creates or refreshes a worktree from a mirror repo.
// If the worktree already exists, it is removed and re-added to ensure it
// reflects the latest state of the mirror. This avoids the complexity of
// updating worktrees from bare/mirror repos.
func ensureWorktree(repoDir, worktreeDir, branch string) error {
	if _, err := os.Stat(worktreeDir); err == nil {
		// Worktree exists. Remove it and re-add to pick up any new refs.
		_ = os.RemoveAll(worktreeDir)
		// Prune stale worktree references in the mirror.
		_, _ = runGit("-C", repoDir, "worktree", "prune")
	}

	_, err := runGit("-C", repoDir, "worktree", "add", worktreeDir, branch)
	if err != nil {
		return fmt.Errorf("creating worktree for %s: %w", branch, err)
	}
	return nil
}

// unsafeBranchChars matches any character that is not alphanumeric, hyphen,
// underscore, or dot — characters that are safe for directory names.
var unsafeBranchChars = regexp.MustCompile(`[^a-zA-Z0-9._-]`)

// consecutiveUnderscores collapses runs of multiple underscores into one.
var consecutiveUnderscores = regexp.MustCompile(`_{2,}`)

// sanitizeBranch replaces characters that are unsafe for directory names.
// Slashes and any other non-alphanumeric/hyphen/underscore/dot characters are
// replaced with underscores, consecutive underscores are collapsed, and path
// traversal components (`.`, `..`, null bytes) are rejected.
func sanitizeBranch(branch string) string {
	// Reject path traversal and null bytes before any transformation.
	if branch == ".." || branch == "." || strings.Contains(branch, "\x00") {
		return "_invalid_branch_"
	}
	// Replace slashes first (preserves existing behavior).
	s := strings.ReplaceAll(branch, "/", "_")
	// Replace any remaining unsafe characters.
	s = unsafeBranchChars.ReplaceAllString(s, "_")
	// Collapse consecutive underscores.
	s = consecutiveUnderscores.ReplaceAllString(s, "_")
	return s
}

// runGit runs a git command and returns the combined stdout/stderr output.
// It sets environment variables for network timeout protection and inherits
// the current environment for SSH, proxy, and credential helper support.
// Returns a descriptive error if the git binary is not found or the command
// fails.
func runGit(args ...string) (string, error) {
	gitPathOnce.Do(func() {
		gitPath, gitPathErr = exec.LookPath("git")
	})
	if gitPathErr != nil {
		return "", errors.New("git is required for remote Git layer sources: install git and retry")
	}

	cmd := exec.Command(gitPath, args...)

	// Inherit environment and add timeout protection.
	cmd.Env = append(os.Environ(),
		"GIT_HTTP_LOW_SPEED_LIMIT=1000",
		"GIT_HTTP_LOW_SPEED_TIME=30",
	)

	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git %s: %w\n%s", strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return strings.TrimSpace(string(out)), nil
}
