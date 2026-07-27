---
title: Prompt A/B Testing
weight: 14
---

# Prompt A/B Testing

Systematically test which system prompts produce better agent output.
Keel's variables and conditionals turn your instruction files into a prompt testing framework — compile two variants, run both against the same task, compare results.

## The Pattern

1. Gate prompt sections behind a variable
2. Compile variant A and variant B with `--var`
3. Run both against the same task
4. Compare output (manually or with an eval framework)

## Example: Concise vs Detailed Code Review

### Layer setup

```yaml
# keel.yml
schema: v1
name: review-test
variables:
  review_style: detailed   # default variant
```

### Instruction file

```markdown
# Code Review Instructions

<!-- keel-if: review_style == "concise" -->
Review the diff. Flag bugs and security issues only. No style feedback.
Keep comments under 2 sentences each. Skip anything that's subjective.
<!-- keel-elif: review_style == "detailed" -->
Review the diff thoroughly:
- Check for bugs, security issues, and edge cases
- Evaluate naming, readability, and code organization
- Assess test coverage — are the changes well-tested?
- Note any architectural concerns or tech debt
- Suggest specific improvements with code examples

Be thorough but constructive. Explain the "why" behind each suggestion.
<!-- keel-endif -->
```

### Compile both variants

```bash
# Variant A: concise review style
keel compile --layer . --output claude=output/concise -v review_style=concise

# Variant B: detailed review style
keel compile --layer . --output claude=output/detailed -v review_style=detailed
```

### Verify what was compiled

```bash
keel compile --layer . --output claude=output/concise -v review_style=concise --show-vars
```

```
  Variables:
    keel.agent                     = claude
    keel.scope                     = project
    review_style                   = concise
    ...
```

### Run both against the same task

```bash
# Run concise variant
cd output/concise && claude "Review the changes on this branch"

# Run detailed variant
cd output/detailed && claude "Review the changes on this branch"
```

Compare the output side by side.

## Multi-Dimensional Testing

Variables compose — test multiple dimensions simultaneously:

```yaml
variables:
  review_style: detailed
  tone: professional
  language: english
```

```markdown
<!-- keel-if: tone == "direct" -->
Be blunt. Say exactly what's wrong. No hedging.
<!-- keel-elif: tone == "professional" -->
Be constructive and specific. Frame suggestions positively.
<!-- keel-elif: tone == "educational" -->
Explain the reasoning behind each suggestion. Link to documentation or standards.
<!-- keel-endif -->
```

```bash
# Test: concise + direct
keel compile --layer . --output claude=output/concise-direct \
  -v review_style=concise -v tone=direct

# Test: detailed + educational
keel compile --layer . --output claude=output/detailed-edu \
  -v review_style=detailed -v tone=educational
```

## Pairing with Eval Frameworks

Keel handles **compilation** — turning variables into concrete prompt variants.
The **evaluation** (scoring, ranking, statistical comparison) is a separate concern.

[Promptfoo](https://www.promptfoo.dev/) is a natural fit:

1. Keel compiles N variants into separate output directories
2. Each directory is a complete agent environment
3. Promptfoo (or any eval tool) runs the same task against each environment
4. Promptfoo scores and ranks the results

```bash
# Compile a matrix of variants
for style in concise detailed; do
  for tone in direct professional educational; do
    keel compile --layer . \
      --output claude=variants/${style}-${tone} \
      -v review_style=$style -v tone=$tone
  done
done
```

This produces 6 complete agent environments, each with different system prompts compiled from the same source layer.

## Variable Files for Test Matrices

For structured testing, use `--var-file` to load variable sets from files:

```bash
# variants/concise-direct.vars
review_style=concise
tone=direct

# variants/detailed-edu.vars
review_style=detailed
tone=educational
```

```bash
keel compile --layer . --output claude=output/v1 --var-file variants/concise-direct.vars
keel compile --layer . --output claude=output/v2 --var-file variants/detailed-edu.vars
```

## Snippets as Test Components

Combine with [snippets]({{< relref "snippets" >}}) to test modular instruction components:

```markdown
<!-- keel:include review-checklist style="security" -->
```

Where `review-checklist.md` uses parameters:

```markdown
<!-- keel-if: style == "security" -->
- [ ] No hardcoded credentials
- [ ] Input validation on all external data
- [ ] SQL injection protection
<!-- keel-elif: style == "performance" -->
- [ ] No N+1 queries
- [ ] Bounded memory usage
- [ ] Appropriate caching
<!-- keel-endif -->
```

## A/B Testing Agent Behaviors

The patterns above aren't just for prompt development — they're for answering operational questions about agent behavior.

### Which rules produce better code?

Compare a strict-lint variant against a flexible-lint variant across a week of real development:

```yaml
variables:
  lint_enforcement: strict
```

```markdown
<!-- keel-if: lint_enforcement == "strict" -->
Run the full linter on every file you touch. Fix all warnings before committing.
Never disable lint rules with inline comments.
<!-- keel-elif: lint_enforcement == "flexible" -->
Run the linter before committing. Fix errors. Warnings are advisory —
use your judgment on whether to fix them.
<!-- keel-endif -->
```

```bash
# Team A gets strict enforcement
keel compile --layer . --output claude=output/strict -v lint_enforcement=strict

# Team B gets flexible enforcement
keel compile --layer . --output claude=output/flexible -v lint_enforcement=flexible
```

Track code review feedback, build failures, and developer satisfaction across both groups.

### Does a reasoning protocol help?

Test whether explicit reasoning instructions improve agent output quality:

```yaml
variables:
  reasoning: enabled
```

```markdown
<!-- keel-if: reasoning == "enabled" -->
Before each significant action:
1. State what you're about to do
2. Explain why
3. Identify risks
4. Proceed only if risks are acceptable
<!-- keel-endif -->
```

Compile with and without, run the same set of tasks, compare results.

### Structuring an A/B test

1. **Isolate one dimension** — change only the variable under test
2. **Compile both variants** — `keel compile -v dimension=a` and `keel compile -v dimension=b`
3. **Run against the same tasks** — identical prompts, identical codebases
4. **Measure what matters** — code correctness, review cycles, time-to-merge, developer preference
5. **Run multiple trials** — LLM output is non-deterministic; single runs prove nothing

The key advantage of using Keel for A/B testing: both variants compile from the same source.
You know exactly what differs between them — the diff is in the variable, not in a manually edited file.

## Tips

- **One variable per dimension.** Don't overload a single variable with multiple concerns.
- **Name variants clearly.** `review_style=concise` beats `variant=a`.
- **Test against real tasks.** Synthetic tasks don't predict real-world performance.
- **Control for randomness.** Run each variant multiple times — LLM output varies.
- **Keep the non-prompt context identical.** Same permissions, same MCP servers, same tools. Only the instructions should differ.

## What's Next

- **[Conditionals & Variables]({{< relref "conditionals" >}})** — the full conditional syntax reference.
- **[Snippets]({{< relref "snippets" >}})** — reusable, parameterized content blocks.
- **[CLI Reference]({{< relref "../reference/cli" >}})** — `--var`, `--var-file`, and `--show-vars` flags.
