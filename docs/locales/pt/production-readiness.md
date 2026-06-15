# Guia de prontidão para produção

> Locale: pt · Português
> Decisões de segurança e release devem ser confirmadas pela fonte inglesa e pelo release gate.

## Visão geral

Este documento explica o que deve ser verificado antes de chamar uma rede Vexo de pronta para produção.

Este guia localizado mantém comandos, campos JSON, métodos RPC, chaves de configuração e nomes de pacotes inalterados para que os exemplos continuem copiáveis entre idiomas.

## Por que importa

O Vexo reúne consenso BFT, módulos de aplicação, contabilidade nativa, execução EVM opcional, economia de validadores, rede de peers e evidências de release. A pessoa leitora deve conseguir explicar não só que existe uma funcionalidade, mas como operá-la com segurança e como provar que ela funciona na rede alvo.

## O que verificar

- **Protocol correctness**: `consensus`, `finality`, validator-set hash, vote sign bytes, timeout certificate, and three-chain finality must agree for the target validator set.
- **Runtime correctness**: `app`, `runtime`, `store`, and module writes must commit atomically, replay deterministically, and recover from crash boundaries.
- **Crypto custody**: BLS, VRF, remote signer, KMS/HSM, proof-of-possession, replay nonce, and double-sign guard evidence must match the release binary.
- **Networking safety**: `network_config.json` must bind chain ID, genesis hash, node ID, TLS/auth policy, durable replay path, peer scoring, ban, and backoff settings.
- **EVM/native accounting**: The EVM module uses the native Vexo coin as the balance asset; gas, base fee, blob base fee, receipts, proofs, and traces must pass external corpora.
- **Release evidence**: Release claims need signed artifacts, SBOM, evidence manifest, longrun, chaos, E2E, state sync, economics, governance, MEV, SDK, and EVM/Web3 evidence.

## Ações do operador

- **System view**: A Vexo network is safe only when protocol, runtime, operations, and evidence are ready together. Do not treat enabled code as a production claim.
- **Configuration review**: Review `config.json`, `module_config.json`, `network_config.json`, `consensus_config.json`, `mempool_config.json`, and `log_config.json` before `vexod start`.
- **Release decision**: Attach evidence from the exact binary, genesis, config schema, module set, and validator topology that will be released.

## Nomes de interface preservados

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

## Erros comuns

- Não assuma que os peers configurados estão conectados; as sessões ativas precisam ser verificadas separadamente.
- Não chame BLS, VRF, EVM, state sync ou governança de prontos para produção sem evidências de release.
- Do not use private operator shortcuts, managed hot keys, or local-only settings on public RPC/P2P listeners.
- Do not delete node data before collecting recovery reports, logs, and evidence when an incident happens.

## Referência normativa

- [Fonte normativa](../en/production-readiness.md)

<!-- vexo-docs:technical-parity -->
## Apêndice de paridade técnica

Este apêndice garante que a tradução preserve as interfaces executáveis e as seções principais do documento canônico em inglês. Comandos, chaves de configuração, métodos RPC e nomes de pacotes permanecem inalterados em todos os idiomas.

### Rastreamento de seções
- section: The Short Version — Esta seção deve ser revisada junto com valores de configuração, evidências de verificação, condições de falha e ações do operador.
- section: How To Use This Guide — Esta seção deve ser revisada junto com valores de configuração, evidências de verificação, condições de falha e ações do operador.
- section: Readiness Levels — Esta seção deve ser revisada junto com valores de configuração, evidências de verificação, condições de falha e ações do operador.
- section: System Map — Esta seção deve ser revisada junto com valores de configuração, evidências de verificação, condições de falha e ações do operador.
- section: Configuration Review Order — Esta seção deve ser revisada junto com valores de configuração, evidências de verificação, condições de falha e ações do operador.
- section: Consensus and Finality Checklist — Esta seção deve ser revisada junto com valores de configuração, evidências de verificação, condições de falha e ações do operador.
- section: Runtime and Storage Checklist — Esta seção deve ser revisada junto com valores de configuração, evidências de verificação, condições de falha e ações do operador.
- section: EVM and Native Coin Checklist — Esta seção deve ser revisada junto com valores de configuração, evidências de verificação, condições de falha e ações do operador.
- section: Crypto and Key Custody Checklist — Esta seção deve ser revisada junto com valores de configuração, evidências de verificação, condições de falha e ações do operador.
- section: Networking Checklist — Esta seção deve ser revisada junto com valores de configuração, evidências de verificação, condições de falha e ações do operador.
- section: Observability Checklist — Esta seção deve ser revisada junto com valores de configuração, evidências de verificação, condições de falha e ações do operador.
- section: Release Evidence Checklist — Esta seção deve ser revisada junto com valores de configuração, evidências de verificação, condições de falha e ações do operador.
- section: Common Failure Modes — Esta seção deve ser revisada junto com valores de configuração, evidências de verificação, condições de falha e ações do operador.
- section: What This Guide Does Not Claim — Esta seção deve ser revisada junto com valores de configuração, evidências de verificação, condições de falha e ações do operador.

### Interfaces mantidas sem alteração
- `docs/specs/consensus-spec.md` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `docs/specs/finality-proof-format.md` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `modules/staking` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `docs/specs/validator-lifecycle.md` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `modules/*` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `docs/sdk/app-module-guide.md` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `docs/specs/storage-schema.md` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `modules/bank` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `docs/specs/tx-format.md` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `docs/specs/evm-native-accounting.md` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `modules/evm` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `docs/sdk/rpc-api-versioning.md` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `cmd/vexod keys` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `docs/sdk/custom-crypto-backend.md` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `docs/security/audit-readiness.md` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `docs/specs/networking-spec.md` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `docs/operators/node-initialization.md` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `docs/release/launch-runbook.md` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `cmd/vexod` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `docs/operators/observability.md` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `docs/release/release-pipeline.md` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `config.json` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `network_config.json` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `rpc.tls_cert_path` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `rpc.tls_key_path` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `rpc.tls_ca_path` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `consensus_config.json` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `module_config.json` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `mempool_config.json` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `log_config.json` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `vexod validate --home <home>` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `vexod config audit --home <home> --strict` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `execution_commit` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `allow_unsafe_qc_commit` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `timeout_propose` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `timeout_prevote` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `timeout_precommit` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `timeout_commit` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `create_empty_blocks` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `eth_getProof` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `go.mod` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `max_score` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `latest_height` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `make check` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `v1/status` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `active_peer_count` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `vexo_web3Capabilities` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
