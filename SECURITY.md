# Security Policy

## Reporting a Vulnerability

Please report security vulnerabilities via [GitHub Security Advisories](https://github.com/fairyhunter13/community-waste-collection-system/security/advisories/new) rather than opening a public issue. This keeps the details private until a fix is ready.

When reporting, include:
- A description of the vulnerability and its potential impact
- Steps to reproduce
- Any suggested mitigations you are aware of

We aim to respond to reports within 72 hours and to publish a fix within 14 days for critical issues.

## Supported Versions

Only the `main` branch receives security fixes.

## Scope

This service manages community waste-collection workflows. Report vulnerabilities that could expose household data, allow unauthenticated access to admin endpoints, or enable privilege escalation between tenants. Theoretical denial-of-service against the development docker-compose stack is out of scope.
