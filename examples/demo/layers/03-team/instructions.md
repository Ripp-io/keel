# Data Platform Team

We use GitLab for source control and CI/CD.
Feature branches follow: `feature/{ticket}-{description}`.
All merge requests require at least one approval.
Deploy to staging before production — never skip.

## Version Control Workflow

<!-- keel-if: keel.agent == "claude" -->
Use worktrees for parallel feature work.
Keep the main worktree on the default branch.
Create a new worktree for each feature branch.
<!-- keel-elif: keel.agent == "cursor" -->
Use branch switching for feature work.
Stash or commit changes before switching branches.
<!-- keel-endif -->

## Pre-Commit Checklist

<!-- keel:include security-checklist -->
