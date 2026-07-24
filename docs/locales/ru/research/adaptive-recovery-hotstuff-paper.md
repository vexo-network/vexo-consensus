# Адаптивный HotStuff с барьером восстановления для модульных Proof-of-Stake сетей

> Locale: ru · Русский  
> Тип документа: исследовательская рукопись и протокол воспроизводимости  
> Статус: черновик, основанный на реализации; заявления о производительности требуют измеренных артефактов.

## Аннотация

В работе исследуется BFT-репликация конечного автомата в стиле HotStuff для модульных Proof-of-Stake сетей. Реализация объединяет three-chain finality и наборы валидаторов, версионируемые по высоте, с тремя эксплуатационными механизмами. Ограниченный адаптивный контроллер меняет round timeout на основе p95 времени обработки proposal, vote и commit, а также состояния активных peers. Recovery finality gate откладывает финализированный application commit, если долговечные истории блоков и состояния расходятся выше общей безопасной высоты. Детерминированное упорядочивание устраняет зависимость порядка одинакового набора транзакций от локального времени поступления в mempool, сохраняя nonce-зависимости каждого signer.

Работа не утверждает, что PoS, BFT, HotStuff, адаптивная синхронизация views или order fairness являются новыми. Более узкий вопрос состоит в том, уменьшает ли именно эта ограниченная композиция управления, восстановления и упорядочивания лишние timeouts и несогласованность при recovery, не меняя базовое правило безопасности HotStuff. Реализованные факты, опровержимые гипотезы и выводы, требующие измерений, разделены. Числа throughput и latency нельзя публиковать как результат до повторных экспериментов с закрепленными binary, config, topology и workload.

## Исследовательские вопросы

RQ1 сравнивает адаптивную политику с тем же узлом при fixed timeout в условиях меняющейся сетевой задержки по числу timeouts и p95 commit latency. RQ2 внедряет ошибки storage и restart и проверяет, не позволяет ли recovery gate состоянию приложения перейти общую долговечную высоту block/state. RQ3 переставляет одинаковый набор транзакций и требует идентичного proposal order и возрастающего nonce для каждого signer. RQ4 измеряет дополнительные CPU, memory, network и latency затраты в устойчивой сети без отказов.

H1–H4 являются направленными, фальсифицируемыми гипотезами, а не результатами. Наличие кода не доказывает улучшение. Если выигрыш незначим, отрицательный результат или граница применимости должны быть опубликованы без усиления формулировки.

## Предыдущие работы и граница новизны

HotStuff уже описал leader-based BFT при partial synchrony, quorum certificates, chained commit, линейную коммуникацию на благоприятном пути и responsiveness. LibraBFT/DiemBFT и AptosBFT уже объединили производные HotStuff со stake-weighted управлением валидаторами. Jolteon и Ditto изучают снижение задержки, адаптацию к сети и asynchronous fallback; Fever рассматривает responsive view synchronization. Tendermint относится к другой round-based PoS BFT линии. Narwhal/Tusk отделяет надежное распространение транзакций от ordering. Aequitas, Wendy и Themis формулируют более сильную order fairness, чем применяемая здесь hash-детерминированность.

Поэтому недопустимы заявления «первая PoS+BFT blockchain», «первая PoS сеть с HotStuff», «идентична AptosBFT», «асинхронная liveness» или «оптимальная коммуникация» без доказательства, «полная защита от MEV», а также «production-ready» на основании single-host теста. Возможный systems contribution уже: интеграция bounded feedback controller, локального durable-history commit gate и nonce-aware deterministic ordering в модульный PoS-узел на Go с воспроизводимым сравнением против fixed и gate-disabled baselines.

## Системная модель и механизмы

На высоте h обозначим активный набор через Vh, а полную voting power через Ph. QC действителен, когда уникальные известные signers дают не менее двух третей Ph. Набор и его hash версионируются по высоте. Admission может быть permissionless при minimum stake, ограниченным по числу или закрытым конфигурацией. Этот слой обеспечивает Sybil resistance и governance, но не меняет BFT fault threshold.

Сеть считается частично синхронной. Safety предполагает менее одной трети Byzantine voting power, корректные signatures, привязку к нужному validator set и надежный durable store. Для liveness дополнительно требуется, чтобы задержка со временем стала ограниченной, честный quorum был достижим, signers доступны и peer connectivity достаточна. Прогресс в постоянно асинхронной сети не обещается.

EVM является application workload под Vexo consensus. Выполнение Ethereum bytecode и совместимость `/web3` не означают реализацию Ethereum fork choice или devp2p consensus.

Базовое правило отслеживает `locked_qc` и `high_qc`. Proposal безопасен, только если продолжает lock или содержит justify QC не старее lock. Validator не может голосовать за разные blocks на одной height/round. Три последовательных certified links, связанные по высоте и hash, финализируют grandparent. Адаптивный controller не меняет этот predicate, quorum threshold, QC verification или three-chain rule.

Адаптивный timeout использует базовый бюджет T0, текущий Tt, сумму p95 latency и floor, зависящий от дефицита peers. После timeout значение растет к 1,5×Tt; после progress уменьшается к 0,8×Tt. Троекратная наблюдаемая сумма latency образует candidate floor. Итог ограничен диапазоном T0…8×T0. При отсутствии active peers floor равен 2×T0. Idle без работы и локальные execution/storage errors не расходуют rounds. Это ограниченный эксплуатационный controller, а не доказанно оптимальный pacemaker.

Recovery gate вычисляет Hsafe=min(Hs,Hb), когда существуют durable state height Hs и block-index height Hb. Пока они различаются, finalized application commits выше Hsafe откладываются. Это локальное ограничение persistence, а не дополнительная vote phase и не network certificate.

Deterministic ordering получает salt из chain ID и height. Transactions с signer/nonce metadata группируются в signer chains, внутри сортируются по возрастающему nonce, а головы chains объединяются по salted transaction hash. Для одинакового candidate set исчезает зависимость от arrival order. Но first-seen fairness, censorship resistance, confidentiality и strong order-fairness не гарантируются, поскольку proposer влияет на inclusion.

Текущий consensus vote path использует полный height-versioned validator set и deterministic proposer. ECVRF committee selector доступен как component и query, но не связан с quorum formation или proposal eligibility. VRF committee consensus остается будущей работой.

## Экспериментальная методика

Все treatments используют один binary и одну application config. Сравниваются fixed вариант с выключенной адаптацией и включенным gate, adaptive вариант с обеими функциями и gate-disabled ablation только в изолированной исследовательской сети. При наличии ресурсов используются 4, 7, 16 и 31 validators; single-host применяется только как smoke test.

Условия включают latency 10, 50, 100 и 250 ms, ступенчатые изменения, jitter, loss 0/1/5/10%, restart обычного validator и текущего proposer, недоступность чуть меньше одной трети power, minority partition с healing, signer delay и внедренное durable-history mismatch. Workloads включают native transfer, EVM transfer, contract creation, event logs, proxy deployment и UUPS upgrade.

Собираются committed/finalized height, proposal/vote/commit p50/p95/p99, end-to-end finality latency, timeout count, round distribution, current adaptive timeout, peer count, recovery deferrals, throughput, gas, CPU, RSS, disk/network bytes, rejection, double-sign и invalid nonce. Run попадает в performance analysis только если все validators имеют одинаковые app hash и finalized block hash, transaction/receipt/block locations согласованы, deployed code существует, а proxy state сохранен после upgrade.

После warm-up для каждого условия выполняется не менее тридцати независимых повторений, если меньшее число заранее не обосновано power analysis. Порядок treatments рандомизируется, seeds сохраняются. Публикуются median, IQR, p95, confidence intervals и effect size. Нельзя выбирать только лучший run; правила исключения задаются до просмотра результатов.

## Корректность, воспроизводимость и этика

Адаптация меняет только время попытки timeout vote, но не определение безопасного vote или QC. Gate лишь ограничивает commits и не способен разрешить commit, отклоненный базовым правилом. Детерминированный порядок помогает получить одинаковый execution input, но не заменяет proof против conflicting finality.

Публикуемое доказательство должно формализовать stake-weighted quorum intersection, lock monotonicity, уникальность finalized block на height, validator-set transition, vote WAL crash recovery и safety-neutral характер controller/gate. Unit tests и adversarial simulations являются evidence, но не заменяют formal proof и independent audit.

Для каждого эксперимента архивируются commit, dirty-tree status, Go/OS/CPU/memory/container, topology, genesis, split configs, binary SHA-256, workload seed, raw JSON/JSONL/CSV, logs, final app hashes, analysis scripts и журнал failed runs. Известный механизм нельзя просто переименовать и объявить изобретением. Throughput, latency и validator count не фабрикуются; hypothesis, observation и interpretation разделяются.

Использование ИИ раскрывается согласно правилам venue, а авторы отвечают за каждый claim, citation, experiment и proof. Fault injection проводится только в собственных или разрешенных isolated systems. Private keys, operator tokens, participant data и production endpoints не публикуются. Найденные уязвимости проходят coordinated disclosure.

До подачи рукопись должна совпадать с pinned source revision, поиск prior art должен быть сохранен, baselines воспроизводимы, multi-host fault measurements завершены, а каждая таблица и figure повторно строиться из raw data и scripts. Отрицательные результаты, ограничения, корректная proof wording и external methodology review остаются в финальной версии. До этого корректное название — «основанный на реализации исследовательский черновик», а не «новый доказанный consensus».

<!-- vexo-docs:technical-parity -->

## Приложение технического соответствия

Следующие имена сохраняются без перевода:

- `/web3`, `V_h`, `P_h`, `locked_qc`, `high_qc`
- `consensus/state_machine.go`, `consensus/state_machine_test.go`
- `consensus/commit_rule.go`, `consensus/commit_rule_test.go`
- `consensus/timeout.go`, `consensus/pacemaker.go`
- `node/adaptive_timeout.go`, `node/loop.go`, `node/adaptive_timeout_test.go`
- `node/recovery.go`, `node/consensus_loop.go`
- `fairordering/fairordering.go`, `modules/staking`, `consensus/wal.go`
- `modules/evm`, `modules/evm/backend/geth`
- `consensus_config.json`, `adaptive_round_timeout_enabled`
- `recovery_finality_gate_enabled`, `execution_commit = "finalized"`
- `/v1/status`, `/v1/metrics`, `/v1/finality/latest`, `/metrics/text`
- `deployments/docker/README.md`, `http://127.0.0.1:28657/web3`
- `make check`, `make fuzz-smoke`, `make ops-verify`
- `make network-e2e`, `make evm-conformance`
- `go run ./cmd/vexod consensus adversarial --json`
- `Fpeer = 2 * T0`, `Hs != Hb`, `h > Hsafe`
