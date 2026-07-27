# Demo Project

A multi-service project with a gateway, UI, and backend service.

## Architecture

- `gateway/` — API gateway, routes requests to backend services
- `ui/` — Frontend web application
- `service/` — Core backend service with PostgreSQL

## Development Workflow

<!-- keel:include reasoning-protocol -->

## Conventions

- Follow existing patterns in each service
- Gateway handles auth and routing — no business logic
- Service owns the data model and migrations
- UI communicates only through the gateway API
