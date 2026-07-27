# --- base ---

# Container Environment

You are in a containerized AI development environment.
Workspace is at /keel/workspace.


# --- company ---

# Company Standards

All code must follow company coding standards.
Security reviews required for all changes touching auth or data access.


# --- org ---

# Payments Organization

Part of the payments organization. Follow PCI-DSS compliance requirements.
Sensitive data handling requires encryption at rest and in transit.


# --- solution ---

# Customer MDM Solution

Working on the Customer Master Data Management solution.
Services follow domain-driven design with bounded contexts.


# --- team ---

# Platform Team

Part of the platform engineering team. We own infrastructure, CI/CD, and shared libraries.
Use conventional commits. PRs require at least one approval.


# --- project ---

# Demo Project

A microservices application with three components:
- `gateway/` — API gateway
- `ui/` — Frontend web application  
- `service/` — Core backend service

## Build Order

1. service (npm install && npm run build)
2. gateway (npm install && npm run build)
3. ui (npm install && npm run build)
