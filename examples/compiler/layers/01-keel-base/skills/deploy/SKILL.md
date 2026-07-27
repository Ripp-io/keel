---
name: deploy
description: Deploy services to staging and production environments
license: MIT
allowed-tools: ["Bash", "Read"]
metadata:
  category: devops
  version: "2.0"
compatibility: "Claude Code 2.x, Cursor 2.x"
disable-model-invocation: false
paths: ["**/deploy*", "**/Dockerfile"]
---
Deploy the application using $ARGUMENTS for target environment.

Follow the deployment checklist in scripts/deploy.sh.
