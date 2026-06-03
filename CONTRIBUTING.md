# Contributing to vexo-consensus

Thanks for helping improve `vexo-consensus`.

This project is a pre-production consensus framework, so changes should optimize for clarity, reviewability, and safety over cleverness.

## Development Rules

- Keep changes small and testable.
- Prefer explicit interfaces over hidden global state.
- Do not weaken production gates to make examples pass.
- Do not add production crypto claims without an audited implementation and evidence.
- Keep deterministic crypto clearly development-only.
- Add or update tests for consensus safety, storage recovery, slashing, governance, networking, and release tooling changes.

## Before Opening a PR

Run:

```bash
make check
```

For security, release, or operational changes, also run the relevant command:

```bash
make fuzz-smoke
make ops-verify
go run ./cmd/vexod consensus adversarial --json
go run ./cmd/vexod release readiness --json
```

## Documentation

Update `README.md` or the relevant file under `docs/` when a change affects:

- CLI usage
- protocol behavior
- storage schema
- validator lifecycle
- transaction format
- release, audit, or operations flow
- extension interfaces

## Security Issues

Do not open public issues for suspected vulnerabilities. Use the security guidance in `docs/security/audit-readiness.md` and contact the maintainers privately.
