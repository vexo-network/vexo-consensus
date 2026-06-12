# Production Readiness Guide

> Locale: ru · Русский
> Решения по безопасности и релизу подтверждаются английским источником и результатом release gate.

## Обзор

Этот документ объясняет, что нужно проверить перед тем, как считать сеть Vexo готовой к production.

Этот локализованный документ сохраняет команды, JSON-поля, RPC-методы, ключи конфигурации и имена пакетов без перевода, чтобы примеры можно было копировать в любой языковой версии.

## Почему это важно

Vexo combines BFT consensus, application modules, native accounting, optional EVM execution, validator economics, peer networking, and release evidence. A reader should be able to explain not just that a feature exists, but how to operate it safely and how to prove that it works on the target network.

## Что обязательно проверить

- **Protocol correctness**: `consensus`, `finality`, validator-set hash, vote sign bytes, timeout certificate, and three-chain finality must agree for the target validator set.
- **Runtime correctness**: `app`, `runtime`, `store`, and module writes must commit atomically, replay deterministically, and recover from crash boundaries.
- **Crypto custody**: BLS, VRF, remote signer, KMS/HSM, proof-of-possession, replay nonce, and double-sign guard evidence must match the release binary.
- **Networking safety**: `network_config.json` must bind chain ID, genesis hash, node ID, TLS/auth policy, durable replay path, peer scoring, ban, and backoff settings.
- **EVM/native accounting**: The EVM module uses the native Vexo coin as the balance asset; gas, base fee, blob base fee, receipts, proofs, and traces must pass external corpora.
- **Release evidence**: Release claims need signed artifacts, SBOM, evidence manifest, longrun, chaos, E2E, state sync, economics, governance, MEV, SDK, and EVM/Web3 evidence.

## Действия оператора

- **System view**: A Vexo network is safe only when protocol, runtime, operations, and evidence are ready together. Do not treat enabled code as a production claim.
- **Configuration review**: Review `config.json`, `module_config.json`, `network_config.json`, `consensus_config.json`, `mempool_config.json`, and `log_config.json` before `vexod start`.
- **Release decision**: Attach evidence from the exact binary, genesis, config schema, module set, and validator topology that will be released.

## Имена интерфейсов без перевода

- `vexod validate --home <home>`
- `vexod config audit --home <home> --strict`
- `/v1/status`
- `/v1/metrics`
- `/metrics/text`
- `/v1/diagnostics`
- `peer_count`
- `active_peer_count`
- `configured_peer_count`
- `scored_peer_count`
- `latest_height`
- `latest_finalized_height`
- `network_config.json`
- `consensus_config.json`
- `module_config.json`
- `mempool_config.json`
- `release gate`

## Частые ошибки

- Do not assume configured peers are connected peers; active sessions must be checked separately.
- Do not call BLS, VRF, EVM, state sync, or governance production-ready without release evidence.
- Do not use private operator shortcuts, managed hot keys, or local-only settings on public RPC/P2P listeners.
- Do not delete node data before collecting recovery reports, logs, and evidence when an incident happens.

## Нормативная ссылка

- [Нормативный источник](../en/production-readiness.md)

<!-- vexo-docs:technical-parity -->
## Приложение о техническом соответствии

Это приложение помогает убедиться, что перевод сохраняет исполняемые интерфейсы и ключевые разделы английского канонического документа. Команды, ключи конфигурации, методы RPC и имена пакетов остаются неизменными на всех языках.

### Отслеживание разделов
- section: The Short Version — Этот раздел нужно проверять вместе с параметрами конфигурации, доказательствами проверки, условиями отказа и действиями оператора.
- section: How To Use This Guide — Этот раздел нужно проверять вместе с параметрами конфигурации, доказательствами проверки, условиями отказа и действиями оператора.
- section: Readiness Levels — Этот раздел нужно проверять вместе с параметрами конфигурации, доказательствами проверки, условиями отказа и действиями оператора.
- section: System Map — Этот раздел нужно проверять вместе с параметрами конфигурации, доказательствами проверки, условиями отказа и действиями оператора.
- section: Configuration Review Order — Этот раздел нужно проверять вместе с параметрами конфигурации, доказательствами проверки, условиями отказа и действиями оператора.
- section: Consensus and Finality Checklist — Этот раздел нужно проверять вместе с параметрами конфигурации, доказательствами проверки, условиями отказа и действиями оператора.
- section: Runtime and Storage Checklist — Этот раздел нужно проверять вместе с параметрами конфигурации, доказательствами проверки, условиями отказа и действиями оператора.
- section: EVM and Native Coin Checklist — Этот раздел нужно проверять вместе с параметрами конфигурации, доказательствами проверки, условиями отказа и действиями оператора.
- section: Crypto and Key Custody Checklist — Этот раздел нужно проверять вместе с параметрами конфигурации, доказательствами проверки, условиями отказа и действиями оператора.
- section: Networking Checklist — Этот раздел нужно проверять вместе с параметрами конфигурации, доказательствами проверки, условиями отказа и действиями оператора.
- section: Observability Checklist — Этот раздел нужно проверять вместе с параметрами конфигурации, доказательствами проверки, условиями отказа и действиями оператора.
- section: Release Evidence Checklist — Этот раздел нужно проверять вместе с параметрами конфигурации, доказательствами проверки, условиями отказа и действиями оператора.
- section: Common Failure Modes — Этот раздел нужно проверять вместе с параметрами конфигурации, доказательствами проверки, условиями отказа и действиями оператора.
- section: What This Guide Does Not Claim — Этот раздел нужно проверять вместе с параметрами конфигурации, доказательствами проверки, условиями отказа и действиями оператора.

### Интерфейсы, сохраняемые без изменений
- `docs/specs/consensus-spec.md` — Это имя используется без изменений в исполняемых примерах и проверке конфигурации, поэтому его нельзя переводить.
- `docs/specs/finality-proof-format.md` — Это имя используется без изменений в исполняемых примерах и проверке конфигурации, поэтому его нельзя переводить.
- `modules/staking` — Это имя используется без изменений в исполняемых примерах и проверке конфигурации, поэтому его нельзя переводить.
- `docs/specs/validator-lifecycle.md` — Это имя используется без изменений в исполняемых примерах и проверке конфигурации, поэтому его нельзя переводить.
- `modules/*` — Это имя используется без изменений в исполняемых примерах и проверке конфигурации, поэтому его нельзя переводить.
- `docs/sdk/app-module-guide.md` — Это имя используется без изменений в исполняемых примерах и проверке конфигурации, поэтому его нельзя переводить.
- `docs/specs/storage-schema.md` — Это имя используется без изменений в исполняемых примерах и проверке конфигурации, поэтому его нельзя переводить.
- `modules/bank` — Это имя используется без изменений в исполняемых примерах и проверке конфигурации, поэтому его нельзя переводить.
- `docs/specs/tx-format.md` — Это имя используется без изменений в исполняемых примерах и проверке конфигурации, поэтому его нельзя переводить.
- `docs/specs/evm-native-accounting.md` — Это имя используется без изменений в исполняемых примерах и проверке конфигурации, поэтому его нельзя переводить.
- `modules/evm` — Это имя используется без изменений в исполняемых примерах и проверке конфигурации, поэтому его нельзя переводить.
- `docs/sdk/rpc-api-versioning.md` — Это имя используется без изменений в исполняемых примерах и проверке конфигурации, поэтому его нельзя переводить.
- `cmd/vexod keys` — Это имя используется без изменений в исполняемых примерах и проверке конфигурации, поэтому его нельзя переводить.
- `docs/sdk/custom-crypto-backend.md` — Это имя используется без изменений в исполняемых примерах и проверке конфигурации, поэтому его нельзя переводить.
- `docs/security/audit-readiness.md` — Это имя используется без изменений в исполняемых примерах и проверке конфигурации, поэтому его нельзя переводить.
- `docs/specs/networking-spec.md` — Это имя используется без изменений в исполняемых примерах и проверке конфигурации, поэтому его нельзя переводить.
- `docs/operators/node-initialization.md` — Это имя используется без изменений в исполняемых примерах и проверке конфигурации, поэтому его нельзя переводить.
- `docs/release/launch-runbook.md` — Это имя используется без изменений в исполняемых примерах и проверке конфигурации, поэтому его нельзя переводить.
- `cmd/vexod` — Это имя используется без изменений в исполняемых примерах и проверке конфигурации, поэтому его нельзя переводить.
- `docs/operators/observability.md` — Это имя используется без изменений в исполняемых примерах и проверке конфигурации, поэтому его нельзя переводить.
- `docs/release/release-pipeline.md` — Это имя используется без изменений в исполняемых примерах и проверке конфигурации, поэтому его нельзя переводить.
- `config.json` — Это имя используется без изменений в исполняемых примерах и проверке конфигурации, поэтому его нельзя переводить.
- `network_config.json` — Это имя используется без изменений в исполняемых примерах и проверке конфигурации, поэтому его нельзя переводить.
- `rpc.tls_cert_path` — Это имя используется без изменений в исполняемых примерах и проверке конфигурации, поэтому его нельзя переводить.
- `rpc.tls_key_path` — Это имя используется без изменений в исполняемых примерах и проверке конфигурации, поэтому его нельзя переводить.
- `rpc.tls_ca_path` — Это имя используется без изменений в исполняемых примерах и проверке конфигурации, поэтому его нельзя переводить.
- `consensus_config.json` — Это имя используется без изменений в исполняемых примерах и проверке конфигурации, поэтому его нельзя переводить.
- `module_config.json` — Это имя используется без изменений в исполняемых примерах и проверке конфигурации, поэтому его нельзя переводить.
- `mempool_config.json` — Это имя используется без изменений в исполняемых примерах и проверке конфигурации, поэтому его нельзя переводить.
- `log_config.json` — Это имя используется без изменений в исполняемых примерах и проверке конфигурации, поэтому его нельзя переводить.
- `vexod validate --home <home>` — Это имя используется без изменений в исполняемых примерах и проверке конфигурации, поэтому его нельзя переводить.
- `vexod config audit --home <home> --strict` — Это имя используется без изменений в исполняемых примерах и проверке конфигурации, поэтому его нельзя переводить.
- `execution_commit` — Это имя используется без изменений в исполняемых примерах и проверке конфигурации, поэтому его нельзя переводить.
- `allow_unsafe_qc_commit` — Это имя используется без изменений в исполняемых примерах и проверке конфигурации, поэтому его нельзя переводить.
- `timeout_propose` — Это имя используется без изменений в исполняемых примерах и проверке конфигурации, поэтому его нельзя переводить.
- `timeout_prevote` — Это имя используется без изменений в исполняемых примерах и проверке конфигурации, поэтому его нельзя переводить.
- `timeout_precommit` — Это имя используется без изменений в исполняемых примерах и проверке конфигурации, поэтому его нельзя переводить.
- `timeout_commit` — Это имя используется без изменений в исполняемых примерах и проверке конфигурации, поэтому его нельзя переводить.
- `create_empty_blocks` — Это имя используется без изменений в исполняемых примерах и проверке конфигурации, поэтому его нельзя переводить.
- `eth_getProof` — Это имя используется без изменений в исполняемых примерах и проверке конфигурации, поэтому его нельзя переводить.
- `go.mod` — Это имя используется без изменений в исполняемых примерах и проверке конфигурации, поэтому его нельзя переводить.
- `max_score` — Это имя используется без изменений в исполняемых примерах и проверке конфигурации, поэтому его нельзя переводить.
- `latest_height` — Это имя используется без изменений в исполняемых примерах и проверке конфигурации, поэтому его нельзя переводить.
- `make check` — Это имя используется без изменений в исполняемых примерах и проверке конфигурации, поэтому его нельзя переводить.
- `v1/status` — Это имя используется без изменений в исполняемых примерах и проверке конфигурации, поэтому его нельзя переводить.
- `active_peer_count` — Это имя используется без изменений в исполняемых примерах и проверке конфигурации, поэтому его нельзя переводить.
- `vexo_web3Capabilities` — Это имя используется без изменений в исполняемых примерах и проверке конфигурации, поэтому его нельзя переводить.
