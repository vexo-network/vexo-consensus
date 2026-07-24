# Руководство по обновлению EVM

> Locale: ru · Русский
> Этот документ является русским переводом английского источника. Решения по протоколу, безопасности и релизу принимаются по английскому источнику.

Это руководство объясняет, как обновлять встроенный стек EVM, не ломая обработку chain ID, совместимость Web3 и релизные доказательства. Оно предназначено для операторов и сопровождающих, которым нужно обновить go-ethereum, скорректировать fork presets или изменить поведение EVM в контролируемом релизе.

## Что считается обновлением EVM

Считайте релизно-значимым обновлением любое изменение, которое может повлиять на исполнение в стиле Ethereum или на поведение, видимое для Web3:

- обновление версии `go-ethereum` в `modules/evm/backend/geth`
- изменения в `modules/evm/ethcompat`
- изменения в `modules/evm`
- изменения `execution.evm_fork_preset`
- изменения `execution.evm_chain_config_json`
- изменения в приёме raw transactions, gas accounting, receipts, traces, proofs или полей ответа блока
- изменения в работе управляемых Web3-аккаунтов, таких как `eth_accounts`, `eth_coinbase`, `eth_sign`, `eth_signTransaction` или `eth_sendTransaction`

## Безопасный порядок обновления

Соблюдайте этот порядок, чтобы код, конфигурация и документация не расходились:

1. Сначала обновите изолированный geth-backed adapter.
2. Затем обновите corpus fixtures и conformance tests.
3. Если меняется семантика, обновите `docs/specs/evm-native-accounting.md`, `docs/specs/tx-format.md` и `docs/sdk/rpc-api-versioning.md`.
4. Если меняется формат release evidence, обновите `docs/release/release-pipeline.md`.
5. Если меняются операционные настройки, обновите документацию по конфигурации узла.
6. Перед merge снова запустите validation matrix.

Не повышайте версию EVM runtime и не выкатывайте её одновременно, если только conformance suites, RPC smoke checks и Docker deployment checks уже не пройдены.

## Процесс обновления

### 1. Зафиксировать объём изменения

Точно опишите, что именно меняется:

- только fork behavior
- только transaction admission
- только execution semantics
- только RPC compatibility
- только обработка blob / receipt / trace
- только поведение managed account или wallet

Такой разрез удерживает review в фокусе и не даёт несвязанному коду двигаться вместе.

### 2. Менять в самой узкой прослойке

Предпочтительные границы такие:

- `modules/evm/backend/geth` для изменений интеграции с upstream go-ethereum
- `modules/evm/ethcompat` для raw transaction decoding, сохранения hash и работы с fixtures
- `modules/evm` для state transition, receipts, logs, storage и snapshots
- `rpc` для изменений поверхности Web3 request/response
- `cmd/vexod` только если CLI или release workflow должны показать новое поведение

Если изменение доходит до application modules, держите границу module явной и сохраняйте детерминированные записи состояния.

### 3. Обновить конфигурацию по умолчанию

Когда меняется семантика, в том же патче обновляйте default config:

- `execution.evm_fork_preset`
- `execution.evm_chain_config_json`
- `execution.allow_unprotected_legacy_tx`
- при необходимости RPC-поля managed account в `network_config.json`
- EVM chain ID в `module_config.json`

Никогда не пытайтесь объяснять runtime behavior скрытым CLI flag. По одним файлам конфигурации должно быть понятно, как ведёт себя узел.

### 4. Запустить conformance stack

Минимально выполните:

```bash
make evm-conformance
go test ./modules/evm -count=1
go test ./rpc -count=1
```

Затем проверьте пользовательские пути, которые обычно ломаются первыми:

```bash
curl -s http://127.0.0.1:26657/web3 \
  -H 'content-type: application/json' \
  -d '{"jsonrpc":"2.0","id":1,"method":"eth_chainId","params":[]}'
```

Для Docker single-host deployment проверьте также:

```text
http://127.0.0.1:28657/web3
```

Обязательно проверьте такие поведения:

- `eth_chainId`
- `eth_blockNumber`
- `eth_gasPrice`
- `eth_call`
- `eth_estimateGas`
- `eth_sendRawTransaction`
- `eth_getTransactionReceipt`
- `eth_getBalance`
- `eth_getCode`
- `eth_getStorageAt`
- `eth_getProof`

Затем разверните простой контракт, proxy contract и UUPS upgrade path через тот же RPC endpoint, который будет использоваться в production.

### 5. Подтвердить proxy и upgrade

Обновление EVM не завершено, пока все пункты ниже не выполняются:

- обычный deploy контракта проходит
- deploy proxy проходит
- вызов UUPS upgrade проходит
- после upgrade чтение storage и code даёт ожидаемый результат
- nonce tracking остаётся монотонным
- block producer принимает получившиеся транзакции без unsafe proposal errors

Если proxy deploy проходит, а upgrade падает, публиковать ещё нельзя. Считайте это release blocker, а не предупреждением.

### 6. Обновить evidence

Когда меняется поверхность EVM, обновите и release evidence bundle:

- `--evm-tx-fixtures`
- `--evm-execution-fixtures`
- `--evm-default-fixtures`
- `--evm-web3-conformance-evidence`
- любые закреплённые SHA-256 fixture reference

В release evidence должно быть написано, что именно изменилось, что тестировали и какой commit или version был подтверждён. Нельзя говорить, что обновление EVM завершено, если evidence не совпадает с реально выполненным кодом.

## Матрица проверки

Используйте эту таблицу как merge gate.

| Check | Почему важно |
| --- | --- |
| `make evm-conformance` | ловит regressions fork rule и execution |
| `go test ./modules/evm -count=1` | проверяет receipts, logs, storage, balances и snapshots |
| `go test ./rpc -count=1` | проверяет совместимость Web3 request/response |
| `make network-e2e` | подтверждает, что узел по-прежнему стартует, имеет peers и делает commit |
| Docker single-host smoke | подтверждает путь, который используют Remix и browser tools |
| Contract deploy | подтверждает admission транзакций и генерацию receipts |
| Proxy deploy | подтверждает предположения по ABI и storage layout |
| UUPS upgrade | подтверждает семантику upgrade и чтение после upgrade |

Если хотя бы один пункт красный, не говорите, что обновление завершено.

## Критерии отката

Откатывайте обновление EVM, если происходит что-то из этого:

- `eth_chainId` неожиданно меняется
- `eth_sendRawTransaction` начинает отклонять валидные транзакции
- `eth_call` или `eth_estimateGas` расходятся с ожидаемыми fork rules
- receipts, logs или proofs перестают совпадать с committed state
- начинают падать proxy или upgrade транзакции
- release evidence больше не соответствует текущему коду

Rollback должен одновременно вернуть последнюю хорошую версию adapter, значения config по умолчанию и набор fixtures.

## Техническое приложение по паритету

Это приложение держит руководство в одном стиле с остальным деревом документации.

- Сохраняйте `modules/evm/backend/geth`, `modules/evm/ethcompat`, `modules/evm`, `rpc` и `cmd/vexod` как стабильные границы реализации.
- Сохраняйте написание `execution.evm_fork_preset`, `execution.evm_chain_config_json`, `execution.allow_unprotected_legacy_tx`, `eth_chainId`, `eth_call`, `eth_estimateGas`, `eth_sendRawTransaction`, `eth_getTransactionReceipt`, `eth_getProof`, `eth_getStorageAt`, `eth_accounts`, `eth_coinbase`, `eth_signTransaction` и `eth_sendTransaction` без изменений.
- Сохраняйте без изменений и `make evm-conformance`, `make network-e2e`, `--evm-default-fixtures`, `--evm-tx-fixtures`, `--evm-execution-fixtures`, `--evm-web3-conformance-evidence`.
- Операционный вопрос остаётся простым: сохраняет ли это обновление execution в стиле Ethereum и одновременно соответствует ли оно безопасности Vexo consensus и release?

- Keep `go test -race ./rpc -count=1` in the verification matrix to catch managed nonce allocation and pending-state races.

## Совместимость добытых объектов

Remix и ethers повторно разбирают транзакцию и блок после получения receipt. Поле `gas` добытой транзакции должно сохранять отправленный gas limit, а фактический расход находится в `gasUsed` receipt. Нужны применимые поля `v`, `r`, `s`, `yParity` и ненулевые `blockHash`, `blockNumber`, `transactionIndex`.

Receipt содержит `transactionHash`, `transactionIndex`, `blockHash`, `blockNumber`, `from`, `to`, `contractAddress`, `status`, `gasUsed`, `cumulativeGasUsed`, `type`, `logs`; каждый log также содержит `logIndex`, `removed` и местоположение. Для genesis `0x0` метод `eth_getBlockByNumber` возвращает zero parent hash, а не null. `eth_getTransactionByHash`, `eth_getTransactionReceipt` и `eth_getBlockByNumber` должны указывать одинаковое местоположение. Одного `status = 0x1` недостаточно.

## Повторная проверка proxy и upgrade

С одной учетной записью и endpoint последовательно разверните implementation V1, proxy с initialization calldata, прочитайте состояние, разверните implementation V2, выполните разрешенный UUPS upgrade и проверьте сохранность storage. Запишите transaction hash, block hash, contract address, nonce, status, gas limit и gas used. Отличайте отмену wallet от ошибки после отправки; при `invalid transaction nonce` совместно исследуйте pending nonce allocation и mempool.

<!-- vexo-docs:technical-parity -->
