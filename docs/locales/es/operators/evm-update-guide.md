# Guía de actualización de EVM

> Locale: es · Español
> Este documento es la traducción al español de la fuente inglesa. Las decisiones de protocolo, seguridad y release se toman según la fuente inglesa.

Esta guía explica cómo actualizar la pila EVM integrada sin romper el manejo de chain ID, la compatibilidad Web3 ni las pruebas de release. Está pensada para operadores y mantenedores que necesitan subir go-ethereum, ajustar fork presets o cambiar el comportamiento de EVM en una release controlada.

## Qué cuenta como actualización de EVM

Trata como una actualización sensible para release cualquier cambio que pueda afectar la ejecución estilo Ethereum o el comportamiento visible para Web3:

- subida de versión de `go-ethereum` en `modules/evm/backend/geth`
- cambios en `modules/evm/ethcompat`
- cambios en `modules/evm`
- cambios en `execution.evm_fork_preset`
- cambios en `execution.evm_chain_config_json`
- cambios en admisión de raw transactions, gas accounting, receipts, traces, proofs o campos de respuesta de bloques
- cambios en el manejo de cuentas Web3 gestionadas como `eth_accounts`, `eth_coinbase`, `eth_sign`, `eth_signTransaction` o `eth_sendTransaction`

## Orden seguro de actualización

Siga este orden para que código, configuración y documentación sigan alineados:

1. Actualice primero el adapter geth aislado.
2. Actualice después el corpus de fixtures y las pruebas de conformance.
3. Si cambia la semántica, actualice `docs/specs/evm-native-accounting.md`, `docs/specs/tx-format.md` y `docs/sdk/rpc-api-versioning.md`.
4. Si cambia el formato de release evidence, actualice `docs/release/release-pipeline.md`.
5. Si cambian los controles visibles para operadores, actualice la documentación de configuración del nodo.
6. Vuelva a ejecutar la matriz de validación antes de mergear.

No suba la versión runtime de EVM y la publique al mismo tiempo, salvo que las suites de conformance, los smoke checks RPC y las comprobaciones Docker ya hayan pasado.

## Flujo de actualización

### 1. Fijar el alcance

Documente con precisión la intención del cambio:

- solo fork behavior
- solo transaction admission
- solo execution semantics
- solo RPC compatibility
- solo tratamiento de blob / receipt / trace
- solo comportamiento de cuentas gestionadas o wallets

Esa separación mantiene la revisión enfocada y evita mover código que no tiene relación.

### 2. Cambiar en la capa más estrecha

Use estas fronteras como preferencia:

- `modules/evm/backend/geth` para cambios de integración con upstream go-ethereum
- `modules/evm/ethcompat` para raw transaction decoding, preservación de hash y manejo de fixtures
- `modules/evm` para state transition, receipts, logs, storage y snapshots
- `rpc` para cambios en la superficie Web3 request/response
- `cmd/vexod` solo cuando el CLI o el release workflow deban exponer el nuevo comportamiento

Si el cambio alcanza los application modules, mantenga explícita la frontera del módulo y conserve escrituras de estado deterministas.

### 3. Refrescar la configuración por defecto

Cuando cambie la semántica, actualice la configuración por defecto en el mismo parche:

- `execution.evm_fork_preset`
- `execution.evm_chain_config_json`
- `execution.allow_unprotected_legacy_tx`
- si hace falta, los campos RPC de cuentas gestionadas en `network_config.json`
- el EVM chain ID en `module_config.json`

No dependa de un flag CLI oculto para explicar el comportamiento runtime. La configuración debe dejar claro el comportamiento del nodo solo con los archivos.

### 4. Ejecutar la pila de conformance

Como mínimo, ejecute:

```bash
make evm-conformance
go test ./modules/evm -count=1
go test ./rpc -count=1
```

Luego verifique los flujos visibles para el usuario que suelen romperse primero:

```bash
curl -s http://127.0.0.1:26657/web3 \
  -H 'content-type: application/json' \
  -d '{"jsonrpc":"2.0","id":1,"method":"eth_chainId","params":[]}'
```

En despliegues Docker de un solo host, confirme también:

```text
http://127.0.0.1:28657/web3
```

Compruebe al menos estos comportamientos:

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

Después despliegue un contrato simple, un proxy contract y el camino UUPS upgrade usando el mismo endpoint RPC que usará la wallet o la herramienta en producción.

### 5. Confirmar proxy y upgrade

La actualización de EVM no está terminada hasta que se cumplan estas condiciones:

- un deploy normal de contrato funciona
- un deploy de proxy funciona
- una llamada UUPS upgrade funciona
- tras el upgrade, las lecturas de storage y code devuelven lo esperado
- el nonce tracking sigue siendo monótono
- el block producer acepta las transacciones resultantes sin errores unsafe proposal

Si el deploy del proxy funciona pero el upgrade falla, todavía no es publicable. Trátelo como un release blocker, no como una advertencia.

### 6. Refrescar la evidencia

Cuando cambie la superficie de EVM, actualice también el bundle de release evidence:

- `--evm-tx-fixtures`
- `--evm-execution-fixtures`
- `--evm-default-fixtures`
- `--evm-web3-conformance-evidence`
- cualquier referencia SHA-256 fijada

La release evidence debe decir qué cambió, qué se probó y qué commit o versión se verificó. No describa una actualización de EVM como completada si la evidencia no coincide con el código realmente ejecutado.

## Matriz de validación

Use esta tabla como merge gate.

| Check | Por qué importa |
| --- | --- |
| `make evm-conformance` | detecta regresiones de fork rule y de ejecución |
| `go test ./modules/evm -count=1` | verifica receipts, logs, storage, balances y snapshots |
| `go test ./rpc -count=1` | verifica compatibilidad Web3 request/response |
| `make network-e2e` | confirma que el nodo sigue arrancando, conectando peers y haciendo commit |
| Docker single-host smoke | confirma la ruta que usan Remix y las herramientas del navegador |
| Contract deploy | confirma admisión de transacciones y generación de receipts |
| Proxy deploy | confirma supuestos de ABI y storage layout |
| UUPS upgrade | confirma semántica de upgrade y lecturas después del upgrade |

Si alguna comprobación está en rojo, no diga que la actualización está terminada.

## Criterios de rollback

Haga rollback de la actualización de EVM cuando ocurra cualquiera de estas cosas:

- `eth_chainId` cambia de forma inesperada
- `eth_sendRawTransaction` empieza a rechazar transacciones válidas
- `eth_call` o `eth_estimateGas` se desvían de las fork rules esperadas
- receipts, logs o proofs dejan de coincidir con el committed state
- las transacciones de proxy o upgrade empiezan a fallar
- la release evidence ya no coincide con la ruta de código actual

El rollback debe restaurar juntos la última versión buena del adapter, los valores por defecto de config y el conjunto de fixtures.

## Apéndice de paridad técnica

Este apéndice mantiene la guía alineada con el resto del árbol documental.

- Mantenga `modules/evm/backend/geth`, `modules/evm/ethcompat`, `modules/evm`, `rpc` y `cmd/vexod` como fronteras estables de implementación.
- Mantenga sin cambios de ortografía `execution.evm_fork_preset`, `execution.evm_chain_config_json`, `execution.allow_unprotected_legacy_tx`, `ethChainId`, `eth_call`, `eth_estimateGas`, `eth_sendRawTransaction`, `eth_getTransactionReceipt`, `eth_getProof`, `eth_getStorageAt`, `eth_accounts`, `eth_coinbase`, `eth_signTransaction` y `eth_sendTransaction`.
- Mantenga también sin cambios `make evm-conformance`, `make network-e2e`, `--evm-default-fixtures`, `--evm-tx-fixtures`, `--evm-execution-fixtures` y `--evm-web3-conformance-evidence`.
- La pregunta operativa sigue siendo simple: ¿esta actualización preserva la ejecución estilo Ethereum y al mismo tiempo encaja con la seguridad de Vexo consensus y release?

<!-- vexo-docs:technical-parity -->
