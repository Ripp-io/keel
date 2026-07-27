Systematic debugging for the demo project services.

## When to Use

Use this skill when a service fails to start, returns unexpected errors, or behaves incorrectly.

## Process

1. Reproduce the issue — get the exact error message and steps
2. Check logs: `docker compose logs <service>`
3. Trace the request path: gateway → service → database
4. Form a hypothesis about the root cause
5. Test one variable at a time
6. Fix and verify the fix addresses the root cause, not just the symptom
