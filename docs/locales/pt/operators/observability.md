# Guia de observabilidade

> Locale: pt · Português
> Decisões de segurança e release devem ser confirmadas pela fonte inglesa e pelo release gate.

## Visão geral

Este documento explica como avaliar a saúde de um nó Vexo usando status, métricas, logs e alertas.

Este guia localizado mantém comandos, campos JSON, métodos RPC, chaves de configuração e nomes de pacotes inalterados para que os exemplos continuem copiáveis entre idiomas.

## Por que importa

Este guia explica como saber se um nó Vexo está saudável com base em RPC, métricas, logs e evidências de release.

## O que verificar

- **Height and finality**: `latest_height`, `latest_finalized_height`, height rate, and finality proof availability show whether consensus and execution are progressing.
- **Peer health**: `peer_count` is compatibility summary; prefer `active_peer_count`, `configured_peer_count`, and `scored_peer_count` to separate live sessions from configured addresses.
- **Latency and timeout**: `round_timeouts`, proposal latency, vote latency, and commit latency show whether timeout values still fit the real network.
- **Execution pressure**: `mempool_size`, gas/base-fee behavior, tx count, and commit p95/p99 show whether block capacity and storage are under pressure.
- **Recovery readiness**: `snapshot_healthy`, `replay_healthy`, recovery reports, and state-root checks show whether a node can safely restart or sync.
- **Custody and safety**: `validator_signing_failures`, remote signer logs, ban spikes, and reconciliation failures require immediate operator review.

## Ações do operador

- **Status flow**: Start with `/v1/status`, then compare `/v1/metrics`, `/metrics/text`, `/v1/diagnostics`, `/v1/finality/latest`, and recovery reports.
- **Alert flow**: Alert on stalled height, stalled finality, zero active peers, timeout spikes, high commit latency, mempool pressure, replay failure, and signer failures.
- **Incident flow**: Preserve logs, metrics, configs, genesis, binary hash, and evidence files before deleting data or restarting repeatedly.

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

- [Fonte normativa](../../en/operators/observability.md)

<!-- vexo-docs:technical-parity -->
## Apêndice de paridade técnica

Este apêndice garante que a tradução preserve as interfaces executáveis e as seções principais do documento canônico em inglês. Comandos, chaves de configuração, métodos RPC e nomes de pacotes permanecem inalterados em todos os idiomas.

### Rastreamento de seções
- section: Core Endpoints — Esta seção deve ser revisada junto com valores de configuração, evidências de verificação, condições de falha e ações do operador.
- section: Reading `/v1/status` — Esta seção deve ser revisada junto com valores de configuração, evidências de verificação, condições de falha e ações do operador.
- section: Prometheus Metrics — Esta seção deve ser revisada junto com valores de configuração, evidências de verificação, condições de falha e ações do operador.
- section: Suggested Alert Rules — Esta seção deve ser revisada junto com valores de configuração, evidências de verificação, condições de falha e ações do operador.
- section: Suggested Starting Thresholds — Esta seção deve ser revisada junto com valores de configuração, evidências de verificação, condições de falha e ações do operador.
- section: Incident Triage Matrix — Esta seção deve ser revisada junto com valores de configuração, evidências de verificação, condições de falha e ações do operador.
- section: Log Events to Keep — Esta seção deve ser revisada junto com valores de configuração, evidências de verificação, condições de falha e ações do operador.
- section: First Response Playbook — Esta seção deve ser revisada junto com valores de configuração, evidências de verificação, condições de falha e ações do operador.
- section: Dashboard Layout — Esta seção deve ser revisada junto com valores de configuração, evidências de verificação, condições de falha e ações do operador.
- section: Release Evidence From Observability — Esta seção deve ser revisada junto com valores de configuração, evidências de verificação, condições de falha e ações do operador.

### Interfaces mantidas sem alteração
- `/v1/status` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `/v1/metrics` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `/metrics/text` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `/v1/diagnostics` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `/v1/finality/latest` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `/v1/state/latest` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `/v1/recovery/report` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `/v1/snapshot` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `latest_height` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `latest_finalized_height` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `latest_app_hash` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `peer_count` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `active_peer_count` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `configured_peer_count` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `scored_peer_count` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `banned_peers` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `banned_peers=0` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `vexo_node_running` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `vexo_latest_height` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `vexo_peer_count` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `vexo_active_peer_count` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `vexo_configured_peer_count` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `vexo_scored_peer_count` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `vexo_banned_peers` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `vexo_height_rate_per_minute` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `vexo_round_timeouts` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `vexo_proposal_latency_p95_nanos` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `vexo_vote_latency_p95_nanos` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `vexo_commit_latency_p95_nanos` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `vexo_mempool_size` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `vexo_snapshot_healthy` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `vexo_replay_healthy` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `vexo_validator_signing_failures` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `vexo_post_commit_reconciliation_failures` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `vexo_node_running == 0` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `vexo_active_peer_count == 0` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `vexo_snapshot_healthy == 0` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `vexo_replay_healthy == 0` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `vexo_validator_signing_failures > 0` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `vexo_post_commit_reconciliation_failures > 0` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `timeout_propose` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `max_txs` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `node_running` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `rpc_listening` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `p2p_listening` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `peer_configured` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `peer_connected` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `peer_disconnected` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `peer_dial_failed` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `peer_banned` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `consensus_loop_running` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `block_committed` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `round_timeout` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `validator_signing_failure` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `evidence_received` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `evidence_applied` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `snapshot_exported` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `replay_checked` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `upgrade_halt` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `upgrade_applied` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `dist/` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
