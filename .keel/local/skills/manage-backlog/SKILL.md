---
name: manage-backlog
description: Manage the project backlog via GitLab issues. Create, update, close, and triage issues. Always present issues for human review before creating. Use when capturing new work, triaging ideas, or reviewing open issues.
---

# Manage Backlog

The project backlog lives in GitLab issues.

**Project:** `ripp.io/oss/agentic/keel` on `gitlab.com`

## Labels

| Label       | Color | Use for |
|-------------|-------|---------|
| `compiler`  | blue | Compiler, CLI, DSL, adapters |
| `container` | teal | Container images, runtime, delivery |
| `docs`      | green | Documentation site, guides, references |
| `tier-1`    | red | High value |
| `tier-2`    | orange | Medium value |
| `tier-3`    | orange | Medium value |

Every issue gets one area label (`compiler`, `container`, or `docs`) and one priority label (`tier-1` or `tier-2`).

## Golden rule

**Never create issues without human review.** Draft the issue title, description, and labels, then present them for approval before calling the GitLab API.

## Creating issues

1. Draft the issue with a clear title (imperative form, under 70 chars)
2. Write a description with:
   - One sentence explaining the value
   - A **Needs** section listing what's required
   - A **Current state** section if partially done
3. Assign area + priority labels
4. Present to the human for review
5. Only after approval: call `create_issue` via the GitLab MCP tool

Example draft format:

```
**Title:** CI runner examples (GitLab CI + GitHub Actions)
**Labels:** container, tier-1

The runner image exists but has zero adoption docs.

**Needs:** Write example pipeline YAML files. No code changes.
**Deliverable:** `examples/ci/gitlab-ci.yml` and `examples/ci/github-actions.yml`.
```

## Reviewing open issues

Use the GitLab MCP tool to list issues:

```
list_issues(project_id="ripp.io/oss/agentic/keel", state="opened")
```

Filter by label:

```
list_issues(project_id="ripp.io/oss/agentic/keel", labels=["container", "tier-1"])
```

When reviewing, check:
- Is the issue still relevant?
- Has the status changed? (partially done → fully done)
- Should the priority be adjusted?

## Closing issues

When work ships, close the issue with a comment noting what was delivered:

```
update_issue(project_id="ripp.io/oss/agentic/keel", issue_iid="1", state_event="close")
```

Add a comment with the commit or MR reference.

## Capturing ideas during work

When a new idea, improvement, or concern comes up during development:

1. Note it briefly
2. At scope-check time (not mid-build), draft it as an issue
3. Present to the human for review
4. Create if approved, discard if not

Don't let idea capture interrupt the build loop.

## Triaging deferred items

Items that are questionable or premature go to `local/backlog.md` (gitignored), not GitLab issues.
Only create issues for work that's clearly worth doing — just not right now.
