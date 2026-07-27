---
name: add-product-comparison
description: Write a product comparison document showing how Keel relates to another tool in the AI agent infrastructure space. Use when evaluating a new product, framework, or standard against Keel.
---

# Add Product Comparison

Write a comparison document explaining how Keel relates to another product, framework, or standard.
The goal is clarity about overlap, complement, and how the two systems can work together.

## Step 1: Read existing comparisons for format and style

Read `docs/content/docs/ecosystem/comparisons.md` for the index and overlap matrix.
Read at least two existing comparison docs to internalize the format:

- `docs/content/docs/ecosystem/vs-devcontainers.md` -- comparison with DevContainers (complementary, partial overlap)
- `docs/content/docs/ecosystem/vs-claude-managed-settings.md` -- comparison with Claude's own managed settings (overlapping governance)

Note the consistent structure:
1. Opening paragraph: what the product is and its relationship to Keel
2. Stack diagram: where each system sits in the architecture
3. Comparison table: what each system does across shared concerns
4. "Where X is sufficient" section: when you don't need Keel
5. "Where Keel adds value" section: what Keel brings beyond X
6. Optional: "Using them together" section

Style rules:
- One sentence per line in markdown (the renderer joins them into paragraphs).
- Use tables for structured comparisons.
- Use stack diagrams (ASCII art in code blocks) to show architectural layering.
- Be honest about where the other product is sufficient without Keel.
- No marketing language -- state facts and trade-offs.

## Step 2: Research the product

Gather information about the target product:

- Official documentation and website
- GitHub repository (if open source)
- Architecture and design philosophy
- Configuration model and file formats
- Governance capabilities (if any)
- Container/delivery story (if any)
- Agent support (if applicable)

Identify the primary relationship to Keel:
- **Overlapping**: Both solve parts of the same problem (e.g., DevContainers)
- **Complementary**: Solves a different problem that can integrate (e.g., OPA)
- **Subset**: Keel fully subsumes the product's capabilities
- **Superset**: The product does everything Keel does and more (be honest)

## Step 3: Create the stack diagram

Show where each system sits in the architecture.
Use a vertical stack diagram in a code block:

```
┌───────────────────────────────────────────────┐
│  <Layer that Product handles>                 │  Product
├───────────────────────────────────────────────┤
│  <Shared layer>                               │  Both
├───────────────────────────────────────────────┤
│  <Layer that Keel handles>                    │  Keel only
└───────────────────────────────────────────────┘
```

The diagram should make the relationship immediately clear.

## Step 4: Write the comparison table

Create a table comparing what each system does across shared concerns.
Use the same column structure as existing comparisons:

```markdown
| Concern | <Product> | Keel |
|---|---|---|
| Primary purpose | ... | ... |
| Config governance | ... | ... |
| Agent awareness | ... | ... |
| ... | ... | ... |
```

Choose concerns that are relevant to both systems.
Include rows where the product is stronger than Keel, not just where Keel wins.

## Step 5: Write the "sufficient without Keel" section

Describe scenarios where the product alone is enough.
This builds credibility and helps readers decide if they need Keel at all.

## Step 6: Write the "Keel adds value" section

Describe what Keel brings beyond the product.
Organize by capability, not feature list.
Each subsection should explain the gap and how Keel fills it.

## Step 7: Write the optional integration section

If the two systems can be used together, describe how.
Give concrete examples of combined usage.

## Step 8: Create the file

Create `docs/content/docs/ecosystem/vs-<product-name>.md`.
Use kebab-case for the filename, prefixed with `vs-`.

Add Hugo frontmatter with `bookHidden: true` so the page is accessible via URL
but hidden from the sidebar (linked from the comparisons index instead):

Document structure:

```markdown
---
title: "Keel + <Product Name>"
bookHidden: true
---

# Keel + <Product Name>

<Opening paragraph: what the product is and why it's being compared to Keel.>

## Where each system sits

<Stack diagram>

## What each system does

<Comparison table>

## Where <Product> is sufficient

<Scenarios where you don't need Keel>

## Where Keel adds value

### <Capability 1>
<Explanation>

### <Capability 2>
<Explanation>

## Using them together (optional)

<Integration description>
```

## Step 9: Update the comparisons index

Edit `docs/content/docs/ecosystem/comparisons.md`.

Add the new product to the appropriate section:
- **Overlap matrix** table if there is config/governance/delivery overlap.
- **Complementary** table if the product solves a different problem.

Add a summary paragraph and link using `{{</* relref "vs-<product-name>" */>}}`.
Fill in all columns of the overlap matrix accurately.

## Step 10: Review

Verify:
- One sentence per line throughout
- Stack diagram accurately shows the relationship
- Comparison table is balanced (shows product strengths too)
- No marketing language or superlatives
- "Sufficient without Keel" section is honest
- README index is updated

## Checklist

- [ ] Read existing comparisons for format and style
- [ ] Target product researched (docs, architecture, capabilities)
- [ ] Relationship to Keel identified (overlapping, complementary, subset, superset)
- [ ] Stack diagram created showing architectural layering
- [ ] Comparison table written with balanced assessment
- [ ] "Where X is sufficient" section written honestly
- [ ] "Where Keel adds value" section written with concrete capabilities
- [ ] File created at `docs/content/docs/ecosystem/vs-<product-name>.md` with `bookHidden: true`
- [ ] `docs/content/docs/ecosystem/comparisons.md` index updated
- [ ] One-sentence-per-line style used throughout
