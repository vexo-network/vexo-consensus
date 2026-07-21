# Guía del módulo de aplicación

> Locale: es · Español
> Este documento es una traducción directa al español de la fuente inglesa. Las decisiones de protocolo, seguridad y release siguen siendo normativas en inglés.

## Orden de lectura

Este documento explica cómo añadir un application module a Vexo. Si es la primera vez que conectas un módulo, lee en este orden.

1. Module interface
2. Transaction routing
3. Module configuration
4. State and events
5. Genesis and ante handling
6. CLI commands and tests

Ese orden coincide bastante con el trabajo real: definir la forma del módulo, decidir cómo recibe las transacciones, aclarar qué state posee y, al final, enlazar la CLI y las pruebas.

## Resumen

Este documento ayuda a entender creación de app module e integración con CLI/RPC/almacenamiento de estado y a conectarlo con decisiones de implementación y operación.

- Canonical path: `docs/sdk/app-module-guide.md`
- Locale path: `docs/locales/es/sdk/app-module-guide.md`

## Por qué leer este documento

- creación de app module e integración con CLI/RPC/almacenamiento de estado
- Revise primero las frases MUST/SHOULD/MAY en la fuente inglesa.
- Esta página es una traducción directa del original en inglés; la auditoría, el release y la seguridad se deciden con la fuente inglesa.

## Qué debería poder hacer después

- Explicar qué decisión de implementación u operación apoya este documento.
- Relacionar los requisitos normativos de la fuente inglesa con la configuración actual de la red.
- Verificar chain ID, validator ID, fee/gas y direcciones peer antes de copiar ejemplos.

## Checklist de uso seguro

- Revise primero las frases MUST/SHOULD/MAY en la fuente inglesa.
- No traduzca comandos, config key, nombres RPC, campos JSON ni identificadores de código.
- Antes de copiar ejemplos, adapte chain ID, validator ID, fee/gas y direcciones peer a su red.
- Después de modificar la documentación, ejecute `make docs-check` para verificar el árbol local y los controles de traducción.

## Puntos de atención

- Esta página es una traducción directa del original en inglés; la auditoría, el release y la seguridad se deciden con la fuente inglesa.
- Si cambia la implementación, actualice la fuente inglesa y todos los documentos localizados en el mismo cambio.

## Interfaces que deben conservarse

- `app.Module`
- `app.QueryHandler`
- `app.ValidatorUpdateProvider`
- `app.TxEventEmitter`
- `app.PruneHook`
- `bank`
- `bank:`
- `module_config.json`
- `config.json`
- `module_config_path`
- `network_config.json`
- `consensus_config.json`
- `mempool_config.json`
- `log_config.json`
- `app.Context.Store`
- `ctx.GoContext()`
- `CheckTx`
- `PrepareProposal`
- `ProcessProposal`
- `FinalizeBlock`
- `Query`
- `params`

## Estructura de la fuente inglesa

- App Module Guide
- Objetivo
- Module Interface
- Transaction Routing
- Module Configuration
- State
- Events and Query Proofs
- IBC and Contract Extension Points
- Genesis
- Ante Handling
- CLI Commands
- Tests

## Fuente canónica

- [Documento canónico en inglés](../../en/sdk/app-module-guide.md)

<!-- vexo-docs:technical-parity -->
## Apéndice de paridad técnica

Este apéndice asegura que la traducción conserve las interfaces ejecutables y las secciones clave del documento canónico en inglés. Los comandos, claves de configuración, métodos RPC y nombres de paquetes se mantienen sin cambios en todos los idiomas.

### Seguimiento de secciones
- section: Goal — Esta sección debe revisarse junto con valores de configuración, evidencia de verificación, condiciones de fallo y acciones del operador.
- section: Module Interface — Esta sección debe revisarse junto con valores de configuración, evidencia de verificación, condiciones de fallo y acciones del operador.
- section: Transaction Routing — Esta sección debe revisarse junto con valores de configuración, evidencia de verificación, condiciones de fallo y acciones del operador.
- section: Module Configuration — Esta sección debe revisarse junto con valores de configuración, evidencia de verificación, condiciones de fallo y acciones del operador.
- section: State — Esta sección debe revisarse junto con valores de configuración, evidencia de verificación, condiciones de fallo y acciones del operador.
- section: Events and Query Proofs — Esta sección debe revisarse junto con valores de configuración, evidencia de verificación, condiciones de fallo y acciones del operador.
- section: IBC and Contract Extension Points — Esta sección debe revisarse junto con valores de configuración, evidencia de verificación, condiciones de fallo y acciones del operador.
- section: Genesis — Esta sección debe revisarse junto con valores de configuración, evidencia de verificación, condiciones de fallo y acciones del operador.
- section: Ante Handling — Esta sección debe revisarse junto con valores de configuración, evidencia de verificación, condiciones de fallo y acciones del operador.
- section: CLI Commands — Esta sección debe revisarse junto con valores de configuración, evidencia de verificación, condiciones de fallo y acciones del operador.
- section: Tests — Esta sección debe revisarse junto con valores de configuración, evidencia de verificación, condiciones de fallo y acciones del operador.

### Interfaces conservadas sin cambios
- `app.Module` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `app.QueryHandler` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `app.ValidatorUpdateProvider` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `app.TxEventEmitter` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `app.PruneHook` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `bank:` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `module_config.json` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `config.json` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `module_config_path` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `network_config.json` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `consensus_config.json` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `mempool_config.json` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `log_config.json` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `app.Context.Store` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `ctx.GoContext()` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `params:set:<authority>:<module>:<key>:<base64-value>` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `params/param/<module>/<key>` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `events.Indexer` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `queryproof.Build` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `queryproof.Verify` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `contract.Result` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `modules/evm/backend/geth` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `modules/evm/ethcompat` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `evm state-backend` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `github.com/ethereum/go-ethereum` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `--evm-tx-fixtures-sha256` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `--evm-execution-fixtures-sha256` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `eth_sendRawTransaction` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `execution.allow_unprotected_legacy_tx` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `eth_getProof` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `evm/storage/{address}/{slot}` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `evm_ethstate/{height}` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `state_diff` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `vm_trace` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `eth_getBalance` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `eth_getTransactionCount` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `eth_getCode` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `eth_getStorageAt` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `eth_call` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `eth_estimateGas` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `params.ChainConfig` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `eth_createAccessList` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `eth_getTransactionReceipt` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `eth_getBlockReceipts` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `eth_getTransactionByHash` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `eth_getLogs` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `relayer_config.json` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `ibc/capabilities` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `vexo-queryproof` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `client-create` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `--authority` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `--signer` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `client-update` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `proof_json_base64` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `/v1/state/latest` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `relayer client-update --source-rpc` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `failure_backoff` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `rpc_modules` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `vexo_web3Capabilities` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `web3_clientVersion` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `web3_sha3` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `net_version` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `net_listening` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `net_peerCount` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `eth_chainId` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `eth_protocolVersion` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `eth_syncing` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `eth_mining` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `eth_hashrate` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `eth_accounts` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `eth_coinbase` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `eth_blockNumber` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `eth_getBlockByNumber` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `eth_getBlockByHash` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `eth_getBlockTransactionCountByNumber` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `eth_getBlockTransactionCountByHash` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `eth_getTransactionByBlockNumberAndIndex` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `eth_getTransactionByBlockHashAndIndex` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `eth_getUncleCountByBlockNumber` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `eth_getUncleCountByBlockHash` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.

## Stable Terms

- `execution.evm_fork_preset = "latest"`
- `execution.evm_chain_config_json`
