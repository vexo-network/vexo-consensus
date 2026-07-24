> Locale: ru · Русский

# Обзор протокола консенсуса

Эта страница является основным входом в документацию консенсуса Vexo. Нормативные детали находятся в [Consensus Spec](./specs/consensus-spec.md), [Finality Proof Format](./specs/finality-proof-format.md), [Validator Lifecycle](./specs/validator-lifecycle.md), [Storage Schema](./specs/storage-schema.md), [Networking Spec](./specs/networking-spec.md) и [Transaction Format](./specs/tx-format.md).

## Модель

Vexo использует BFT-ядро в стиле HotStuff с proposal, vote, quorum certificate(QC), timeout certificate, правилом locked-QC и финальностью по трем цепочкам. Голосовать за блок безопасно, только если он продолжает locked QC или содержит justify QC не старше блокировки. Синтетические QC-цепочки и цепочки с пропуском высоты без явной связи высот и хэшей блока, родителя и прародителя отклоняются до решения о финальности.

## Идентичность протокола и граница исследования

Vexo не является новым названием неизмененного HotStuff и не совпадает с протоколом или реализацией AptosBFT, DiemBFT, Jolteon, Ditto, Tendermint либо CometBFT. Отдельная Go runtime сочетает проверенные понятия безопасности HotStuff с адаптивным временем раунда, надежным восстановлением, детерминированным порядком транзакций, модульным исполнением и validator sets, версионированными по высоте.

Активный путь голосования использует полный validator set текущей высоты и детерминированного proposer. Селектор VRF committee доступен как компонент и запрос, но еще не определяет proposal eligibility или quorum formation. Поэтому это будущая работа, а не включенное свойство. Вклад и экспериментальный протокол описаны в [Adaptive Recovery-Gated HotStuff for Modular Proof-of-Stake Networks](./research/adaptive-recovery-hotstuff-paper.md).

## Граница исполнения и восстановления

Сертификация QC, финализация HotStuff, исполнение приложения и commit состояния являются отдельными событиями. По умолчанию `execution_commit=finalized` исполняет только предка, выбранного правилом трех цепочек. Адаптивный pacemaker и `recovery_finality_gate_enabled` управляют задержкой и восстановлением, но не меняют proposer, quorum power, safe-vote или финальность.

## Граница безопасности

- менее одной трети голосов византийцев
- предложения с разделением по доменам, голосование, тайм-аут-голосование и окончательные подписи
- привязка хэша validator-set на соответствующей высоте proof
- уникальные известные подписанты в QC и доказательствах окончательности
- ответственные доказательства двусмысленности валидатора
- отказ от противоречивых решений о фиксации на одной и той же завершенной высоте

## Криптограница

- Backend `deterministic` предназначен только для тестов и не проходит проверку network safety.
- `ed25519` поддерживается для публичных сетевых тестов и подготовки запуска.
- `bls` по умолчанию использует `blst-bls12381-minpk-v1` и требует proof-of-possession, проверку подгруппы, валидацию ключей, аудит зависимостей и доказательства release-gate.
- Для проверки нужны метаданные VRF adapter, но это не означает, что VRF committee включен в активный консенсус.

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
