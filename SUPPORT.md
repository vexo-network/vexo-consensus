# Support

This project is an open-source consensus framework, not a managed service.

## Questions

Use GitHub Discussions if enabled. Otherwise, open a GitHub issue with the `question` label and include:

- what you are trying to build
- the command you ran
- expected behavior
- actual behavior
- OS, Go version, commit/tag, and relevant config

## Bugs

For non-security bugs, open an issue and include:

- reproduction steps
- minimal config or command
- logs or error output
- whether the bug affects consensus, storage, RPC, networking, keys, or modules
- whether it happens after restart, replay, pruning, or upgrade

## Security Reports

Do not open public issues for vulnerabilities. Follow [SECURITY.md](./SECURITY.md).

## Production Deployments

The maintainers cannot certify a deployment as production-safe from a GitHub issue alone. Before running a public network, collect the evidence described in:

- [Security Audit Readiness](./docs/security/audit-readiness.md)
- [Launch Runbook](./docs/release/launch-runbook.md)
- [Release Pipeline](./docs/release/release-pipeline.md)
