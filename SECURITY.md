# Security Policy

`vexo-consensus` is pre-production consensus infrastructure. Security reports are taken seriously, especially issues affecting consensus safety, validator signing, slashing, state recovery, RPC/admin access, release artifacts, or cryptographic verification boundaries.

## Supported Versions

| Version | Security Support |
|---|---|
| `main` | Active review and fixes before release tagging |
| release candidates | Security fixes for release-gate blockers |
| tagged releases | Security fixes according to the release notes and attached gate evidence |

## Reporting a Vulnerability

Please do **not** open a public GitHub issue for suspected vulnerabilities.

Report privately through the maintainers' preferred private channel. If no channel is published for your deployment, contact the repository owner or organization administrators and include:

- affected commit, tag, or branch
- affected component
- impact summary
- reproduction steps or proof of concept
- whether the issue can cause fund loss, consensus safety failure, false slashing, key exposure, chain halt, or remote code execution
- suggested mitigation, if known

## Security Scope

In scope:

- consensus safety and finality verification
- validator-set hash binding and historical validator-set lookup
- signing domains, remote signer policy, and double-sign prevention
- slashing evidence validation and false-slashing resistance
- storage recovery, replay, snapshots, and rollback consistency
- P2P/RPC DoS resistance and admin authorization
- release artifacts, checksums, SBOM, and supply-chain integrity

Out of scope:

- social engineering
- denial-of-service reports without a concrete exploit path
- issues in unaudited third-party adapters that are not wired into this repository
- missing production readiness evidence that is already documented as a limitation

## Production Notice

Do not use this project to secure real funds or public validator infrastructure without:

- independent security audit
- audited production crypto backend
- remote signer/KMS operational evidence
- multi-machine long-run and chaos evidence
- release-gate evidence
- chain-specific economic and governance review

See [Security Audit Readiness](./docs/security/audit-readiness.md) for the audit checklist.
