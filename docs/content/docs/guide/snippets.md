---
title: Snippets
weight: 5
---

# Snippets

Define a text block once, include it in any instruction file across any layer.
Keel injects the content at compile time.

## Quick Examples

### Basic include

A snippet file in your layer:

```
my-layer/
├── keel.yml
├── instructions.md
└── snippets/
    └── reasoning-protocol.md
```

`snippets/reasoning-protocol.md`:

```markdown
Before each significant action:
1. State what you're about to do
2. Explain why
3. Identify risks
```

Reference it in any instruction file:

```markdown
# Project Instructions

## Reasoning Protocol

<!-- keel:include reasoning-protocol -->
```

Compiled output:

```markdown
# Project Instructions

## Reasoning Protocol

Before each significant action:
1. State what you're about to do
2. Explain why
3. Identify risks
```

### Parameterized include

Snippets accept key-value parameters for compile-time customization.

`snippets/domain-rule.md`:

```markdown
In the {{domain}} domain, always {{action}}.
```

Reference with parameters:

```markdown
<!-- keel:include domain-rule domain="security" action="validate user input" -->
```

Compiled output:

```markdown
In the security domain, always validate user input.
```

Unreferenced placeholders are left as-is — this avoids conflicts with other template syntaxes like Cursor's `{{...}}` variables.

## How It Works

Add a `snippets/` directory to your layer and reference it in `keel.yml`:

```yaml
snippets:
  - source: snippets/
```

Each file in the directory is a named snippet.
The filename without extension is the snippet name — `snippets/reasoning-protocol.md` becomes `reasoning-protocol`.

Include markers use the syntax `<!-- keel:include snippet-name -->`.
Markers inside fenced code blocks are treated as literal text, not as directives.

The snippet content is injected with the indentation of the include marker preserved.
If the marker is indented, the injected content is indented to match.

## Sharing Across Layers

Snippets accumulate across the layer stack.
A higher-trust layer can define snippets that any downstream layer references.

```
layers/
├── 01-company/
│   ├── keel.yml          # defines snippets
│   └── snippets/
│       └── reasoning-protocol.md
└── 04-project/
    ├── keel.yml
    └── instructions.md   # uses <!-- keel:include reasoning-protocol -->
```

The company defines the snippet once.
Every project layer can use it without duplicating the content.

If multiple layers define a snippet with the same name, the last layer wins (overlay behavior).

## Snippets vs Conditionals vs Variables

These three features are complementary:

| Feature | Purpose | Syntax |
|---------|---------|--------|
| Snippets | Reuse text blocks across instruction files | `<!-- keel:include name -->` |
| [Conditionals]({{< relref "conditionals" >}}) | Include different text per agent in the same file | `<!-- keel-if: expr -->` |
| [Variables]({{< relref "conditionals" >}}) | Substitute layer-level values into instructions | `${keel:name}` |

Snippets are resolved first, then conditionals are evaluated, then variables are substituted at finalization.
You can combine all three — a snippet can contain variables and conditional blocks.

## What's Next

- **[Conditionals & Variables]({{< relref "conditionals" >}})** — agent-specific content and layer-level substitution.
- **[DSL Reference]({{< relref "dsl" >}})** — the `snippets` field in `keel.yml`.
- **[Governance]({{< relref "governance" >}})** — force-lock snippets so downstream layers can't override them.
