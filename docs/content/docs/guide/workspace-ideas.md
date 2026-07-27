---
title: "Workspace Ideas"
weight: 20
---

# Workspace Ideas

Pre-built workspace images that combine **skills + tooling** for specific personas.
These go beyond "just add instructions" — they bake in the right tools, MCP servers, visualization libraries, and agent behaviors for a domain.

Each image extends a base Keel container image and adds domain-specific layers.
None of these exist yet — they're ideas for the community to build.
If you create one, [open an issue](https://gitlab.com/ripp.io/oss/keel/-/issues/new) and we'll link it here.

---

## Development Workspaces

### Spring Boot Java

**Persona:** Backend Java developer, lead engineer onboarding team to project

**What's in the image:**
- **Skills:** Maven build commands, Spring Boot debugging, database migration review, API endpoint scaffolding
- **Tooling:** JDK 21, Maven, Docker CLI (for testcontainers), PostgreSQL client
- **MCP servers:** GitLab, SonarQube

**Example session:** `make run` → 3 Spring Boot services cloned, Maven deps cached, agent knows the service architecture.

### Next.js / React

**Persona:** Frontend developer, full-stack developer

**What's in the image:**
- **Skills:** Component generation (PatternFly/Radix/Tailwind), ESLint fix commands, accessibility review, Storybook story generation
- **Tooling:** Node 22, pnpm, Playwright, Storybook CLI
- **MCP servers:** Figma (design tokens), GitLab

**Example session:** `make run` → Next.js app cloned, deps installed, agent knows the design system and component library.

### Python Data Engineering

**Persona:** Data engineer, analytics engineer

**What's in the image:**
- **Skills:** dbt model review, Airflow DAG validation, SQL optimization, data quality checks
- **Tooling:** Python 3.12, dbt-core, Airflow CLI, DuckDB, Great Expectations
- **MCP servers:** Snowflake, GitLab

**Example session:** `make run` → dbt project cloned, profiles configured, agent understands the data model and pipeline dependencies.

### Go Microservices

**Persona:** Backend Go developer, platform engineer

**What's in the image:**
- **Skills:** Go module management, protobuf/gRPC scaffolding, Kubernetes manifest review, performance profiling
- **Tooling:** Go 1.22+, protoc, kubectl, k9s, golangci-lint
- **MCP servers:** GitLab, Kubernetes API

**Example session:** `make run` → 5 Go services cloned, proto files compiled, agent knows the service mesh topology.

---

## Non-Engineering Workspaces

### Scrum Master / Product Manager

**Persona:** Scrum master, product manager, agile coach

**What's in the image:**
- **Skills:** Epic summarization, sprint velocity visualization, ticket triage, release notes generation, dependency mapping across teams
- **Tooling:** Python 3.12 (for chart generation), Mermaid CLI, pandoc (report export)
- **MCP servers:** Jira/Confluence (Atlassian MCP), GitLab (MR activity)

**Example session:** `make run` → Agent connected to Jira, can summarize sprint progress, generate burndown charts, draft release notes, identify blocked tickets across teams.

### Technical Writer

**Persona:** Technical writer, documentation engineer

**What's in the image:**
- **Skills:** API doc generation from OpenAPI specs, README scaffolding, changelog drafting, style guide enforcement, cross-repo doc consistency checking
- **Tooling:** Node (for Redocly/Swagger), Vale (prose linter), pandoc, mkdocs
- **MCP servers:** GitLab, Confluence

**Example session:** `make run` → All service repos cloned, agent knows the API specs, can generate and update docs across services.

---

## Security & Compliance Workspaces

### Security Review

**Persona:** Security engineer, AppSec engineer

**What's in the image:**
- **Skills:** Dependency vulnerability scanning, SAST findings triage, secrets detection, security policy enforcement review
- **Tooling:** Trivy, Semgrep, truffleHog, OWASP dependency-check
- **MCP servers:** SonarQube, GitLab (MR review), Snyk

**Example session:** `make run` → Repos cloned, agent runs security scans, generates findings report, suggests fixes for critical vulnerabilities.

### Compliance Audit

**Persona:** Compliance officer, enterprise architect

**What's in the image:**
- **Skills:** Policy documentation review, license compliance checking, data flow mapping, audit report generation
- **Tooling:** License Finder, FOSSA CLI
- **MCP servers:** Jira (compliance tickets), Confluence (policy docs)

**Example session:** `make run` → Agent audits all repos for license compliance, generates a report of third-party dependencies and their licenses.

---

## CI/CD & Operations Workspaces

### MR Review Runner

**Persona:** DevOps engineer, QA engineer

**What's in the image:**
- **Skills:** Code review checklist, test coverage analysis, breaking change detection, performance regression flagging
- **Tooling:** Language-specific linters, coverage tools
- **MCP servers:** GitLab (MR API), SonarQube

**Example session:** Triggered by MR webhook → agent reviews changes, posts comments, approves or requests changes.

### Incident Response

**Persona:** SRE, on-call engineer

**What's in the image:**
- **Skills:** Log analysis, runbook execution, incident timeline construction, post-mortem drafting
- **Tooling:** kubectl, stern (log tailing), jq, Prometheus CLI
- **MCP servers:** PagerDuty, Grafana, Kubernetes API

**Example session:** `make run` → Agent connected to monitoring stack, can query logs, check deployment status, correlate events, draft incident summary.

---

## Data & ML Workspaces

### Data Science Notebook

**Persona:** Data scientist, data analyst

**What's in the image:**
- **Skills:** EDA guidance, feature engineering suggestions, model evaluation, visualization best practices
- **Tooling:** Python 3.12, JupyterLab, pandas, scikit-learn, matplotlib, seaborn
- **MCP servers:** MLflow, S3/GCS

**Example session:** `make run` → Jupyter server running, data repos cloned, agent understands the dataset schema and can suggest analysis approaches.

### ML Pipeline

**Persona:** ML engineer

**What's in the image:**
- **Skills:** Model training pipeline review, hyperparameter tuning, model serving config, A/B test setup
- **Tooling:** Python 3.12, PyTorch/TensorFlow, MLflow, DVC, Docker CLI
- **MCP servers:** MLflow, Weights & Biases, Kubernetes API

**Example session:** `make run` → Training pipeline repo cloned, agent knows the model architecture, can review training configs and suggest optimizations.

---

## Building a Workspace Image

Each workspace is a Containerfile that extends a base Keel container image.
Skills are Keel config layers — instructions + commands + MCP config.
The image bakes the skills layer into `/etc/config/project/`.
Users can still mount their own config to customize or override.

The tooling is the real differentiator — skills alone are just markdown files.
The combination of skills + pre-installed tools + MCP servers + workspace init is the value.
