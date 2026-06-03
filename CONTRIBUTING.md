# Contributing to vexo-consensus

Thanks for taking the time to improve `vexo-consensus`.

This repository contains consensus, storage, cryptography boundaries, slashing, networking, and release tooling. Changes should optimize for correctness, reviewability, and safety over cleverness.

## Project Maturity

`vexo-consensus` is pre-production software. Do not weaken production gates, security checks, audit requirements, or maturity warnings to make examples easier to run.

## Development Setup

Prerequisites:

- Go 1.26 or newer
- `make`

Useful commands:

```bash
make build
make check
make fuzz-smoke
make ops-verify
```

## Contribution Guidelines

- Keep changes small, focused, and testable.
- Prefer explicit interfaces over hidden global state.
- Keep deterministic crypto clearly development-only.
- Do not add production crypto claims without an audited implementation and evidence.
- Preserve height/version semantics for consensus, validator sets, finality proofs, storage, and upgrades.
- Update documentation when CLI behavior, protocol behavior, storage schema, release flow, or extension interfaces change.

## Testing Expectations

At minimum, run:

```bash
make check
```

For security, consensus, networking, storage, release, or operational changes, also run the relevant checks:

```bash
make fuzz-smoke
make ops-verify
go run ./cmd/vexod consensus adversarial --json
go run ./cmd/vexod release readiness --json
```

Add or update tests for:

- consensus safety and finality verification
- slashing evidence validation and false-slashing prevention
- validator-set history and hash binding
- crash/restart recovery paths
- mempool replay, nonce, fee, and DoS behavior
- networking handshake, scoring, rate limits, and bans
- SDK/module extension behavior

## Pull Request Checklist

Before opening a PR:

- [ ] The change is scoped to one logical topic.
- [ ] `make check` passes.
- [ ] Relevant fuzz, adversarial, ops, or release checks pass when applicable.
- [ ] Public APIs, CLI behavior, or wire/storage formats are documented.
- [ ] New production claims are backed by code, tests, and evidence.
- [ ] Security-sensitive behavior is fail-closed.

## Documentation

Update `README.md` or the relevant file under `docs/` when a change affects:

- CLI usage
- protocol behavior
- storage schema
- transaction format
- validator lifecycle
- finality proof format
- RPC API compatibility
- app module development
- crypto, storage, or transport extension points
- release, audit, or operations flow

## Commit Messages

Use concise conventional-style messages:

```text
feat: add validator set recovery check
fix: reject malformed finality signer bitmap
refactor: split rpc response helpers
docs: clarify release gate evidence
test: cover slashing restart reconciliation
```

## Security Issues

Do not open public issues for suspected vulnerabilities. Follow [SECURITY.md](./SECURITY.md).
