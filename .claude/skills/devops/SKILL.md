---
name: devops
description: Run, build, deploy and operate the LMS with Docker, GitHub Actions, VPS, Nginx and HTTPS.
---

# DevOps

## Development
Preferred services:
- postgres: PostgreSQL 17
- backend: Go Gin
- frontend: Next.js

PostgreSQL should run in Docker.

## Production
Preferred topology:
Internet -> Nginx on VPS -> frontend/backend containers

Nginx stays on the host for easier multi-domain management unless there is a specific reason to containerize it.

## CI/CD
Typical pipeline:
1. checkout
2. test
3. build images
4. push to registry
5. SSH to VPS
6. pull new images
7. run migrations
8. restart services
9. health-check

## Rules
- never commit `.env`
- provide `.env.example`
- use GitHub Secrets for CI/CD credentials
- use restart policies
- use HTTPS
- back up PostgreSQL
- do not delete unrelated containers/images on shared VPS
- deployment must not break other domains
