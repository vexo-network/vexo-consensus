> Locale: ru · Русский

QUERY LENGTH LIMIT EXCEEDED. MAX ALLOWED QUERY : 500 CHARS

QUERY LENGTH LIMIT EXCEEDED. MAX ALLOWED QUERY : 500 CHARS

QUERY LENGTH LIMIT EXCEEDED. MAX ALLOWED QUERY : 500 CHARS

QUERY LENGTH LIMIT EXCEEDED. MAX ALLOWED QUERY : 500 CHARS

- менее одной трети голосов византийцев
- предложения с разделением по доменам, голосование, тайм-аут-голосование и окончательные подписи
- привязка хэша validator-set на соответствующей высоте proof
- уникальные известные подписанты в QC и доказательствах окончательности
- ответственные доказательства двусмысленности валидатора
- отказ от противоречивых решений о фиксации на одной и той же завершенной высоте

## Криптограница

QUERY LENGTH LIMIT EXCEEDED. MAX ALLOWED QUERY : 500 CHARS

- строгий аудит конфигурации для каждого дома валидатора
- доказательство release-gate
- внешняя проверка безопасности
- долгосрочные доказательства хаоса для нескольких хозяев
- доказательства политики подписанта/KMS
- обзор экономической и управленческой политики в отношении конкретной цепочки

См. [Готовность к аудиту безопасности](./security/audit-readiness.md) и [Конвейер выпуска](./release/release-pipeline.md), прежде чем рассматривать выпуск как готовый к производству.
<!-- vexo-docs:technical-parity -->
## Приложение о техническом соответствии

Это приложение сохраняет технические термины и интерфейсы, которые не должны меняться между канонической версией и переводом.

### Отслеживание разделов
- section: Model - HotStuff, финальность по тройной цепочке, QC, timeout certificate и locked-QC safety нужно читать вместе.
- section: Execution Terms - различие между qc certified, finalized, executed и state committed должно оставаться ясным.
- section: Safety Boundary - проверьте порог byzantine ниже одной трети, domain separation, привязку hash validator-set и accountable evidence.
- section: Crypto Boundary - сохраняйте идентификаторы `deterministic`, `ed25519`, `bls`, `blst-bls12381-minpk-v1` и `ecvrf-p256-sha256-tai-v1`.
- section: Operational Boundary - вместе смотрите `vexo_quorum_health_ratio`, `adaptive_round_timeout_enabled`, `recovery_finality_gate_enabled` и сигналы snapshot/replay.
- `require_network_safety` и `block_committed` должны оставаться видимыми как есть.

### Поддерживаемые интерфейсы
- `/v1/status`
- `/v1/metrics`
- `/v1/diagnostics`
- `/v1/finality/latest`
- `/v1/state/latest`
- `/v1/recovery/report`
- `execution_commit`
- `finalized`
- `qc`
- `adaptive_round_timeout_enabled`
- `recovery_finality_gate_enabled`
- `vexo_quorum_health_ratio`
- `blst-bls12381-minpk-v1`
- `ecvrf-p256-sha256-tai-v1`
- `proof-of-possession`
- `remote signer`
- `three-chain finality`

### Конфигурационные файлы
- `config.json`
- `module_config.json`
- `network_config.json`
- `consensus_config.json`
- `mempool_config.json`
- `log_config.json`
