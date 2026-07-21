# Versionado de la API RPC

> Locale: es · Español
> Este documento es una traducción directa al español de la fuente inglesa. Las decisiones de protocolo, seguridad y release siguen siendo normativas en inglés.

## Resumen

Este documento ayuda a entender versionado de RPC API, alias de compatibilidad y política de estabilidad y a conectarlo con decisiones de implementación y operación.

- Canonical path: `docs/sdk/rpc-api-versioning.md`
- Locale path: `docs/locales/es/sdk/rpc-api-versioning.md`

## Por qué leer este documento

- versionado de RPC API, alias de compatibilidad y política de estabilidad
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

- `/v1`
- `/v1/healthz`
- `/v1/readyz`
- `/v1/status`
- `/v1/diagnostics`
- `/v1/metrics`
- `/v1/metrics/text`
- `/v1/peers`
- `/v1/tx`
- `/v1/evidence`
- `/v1/recovery`
- `/v1/snapshot/latest`
- `/v1/snapshot/export`
- `/v1/snapshot/chunk?index=0&size=10000`
- `/v1/blocks`
- `/v1/blocks/latest`
- `/v1/blocks/{height}`
- `/v1/state/latest`
- `/v1/state/{height}/{namespace}`
- `/v1/events?key={attribute_key}&value={attribute_value}`
- `/v1/proof?namespace={namespace}&key={key}`
- `/v1/proof?namespace={namespace}&key={key}&height=latest`

## Estructura de la fuente inglesa

- RPC API Versioning
- Objetivo de estabilidad
- Current Stable API
- Versioning Rules
- Compatibility Aliases
- Error Format
- Query Proofs
- Event Queries
- IBC Queries
- Web3 EVM Configuration
- Operational Compatibility

## Fuente canónica

- [Documento canónico en inglés](../../en/sdk/rpc-api-versioning.md)

## RPC capability discovery

La nueva interfaz RPC capability discovery muestra qué funciones provider están conectadas de verdad. Los operadores llaman a `/v1/capabilities`; las integraciones SDK usan `rpc.Config.RequiredCapabilities` o `rpc.Config.RequireAllCapabilities` para fallar al inicio si falta una capacidad.

Mantén estos nombres de interfaz sin cambios: `/v1/capabilities`, `CapabilityResponse`, `CapabilitySnapshot`, `RequiredCapabilities`, `RequireAllCapabilities`, `metrics`, `blocks`, `finality`, `strict_replay`, `consensus_control`.

<!-- vexo-docs:technical-parity -->
- `admin_token` and `admin_tokens` are stable configuration keys and must remain unchanged when describing optional bearer-token enforcement.

## Apéndice de paridad técnica

Este apéndice asegura que la traducción conserve las interfaces ejecutables y las secciones clave del documento canónico en inglés. Los comandos, claves de configuración, métodos RPC y nombres de paquetes se mantienen sin cambios en todos los idiomas.

### Seguimiento de secciones
- section: Stability Goal — Esta sección debe revisarse junto con valores de configuración, evidencia de verificación, condiciones de fallo y acciones del operador.
- section: Current Stable API — Esta sección debe revisarse junto con valores de configuración, evidencia de verificación, condiciones de fallo y acciones del operador.
- section: Versioning Rules — Esta sección debe revisarse junto con valores de configuración, evidencia de verificación, condiciones de fallo y acciones del operador.
- section: Capability Discovery — Esta sección debe revisarse junto con valores de configuración, evidencia de verificación, condiciones de fallo y acciones del operador.
- section: Compatibility Aliases — Esta sección debe revisarse junto con valores de configuración, evidencia de verificación, condiciones de fallo y acciones del operador.
- section: Error Format — Esta sección debe revisarse junto con valores de configuración, evidencia de verificación, condiciones de fallo y acciones del operador.
- section: Query Proofs — Esta sección debe revisarse junto con valores de configuración, evidencia de verificación, condiciones de fallo y acciones del operador.
- section: Event Queries — Esta sección debe revisarse junto con valores de configuración, evidencia de verificación, condiciones de fallo y acciones del operador.
- section: IBC Queries — Esta sección debe revisarse junto con valores de configuración, evidencia de verificación, condiciones de fallo y acciones del operador.
- section: Web3 JSON-RPC Bridge — Esta sección debe revisarse junto con valores de configuración, evidencia de verificación, condiciones de fallo y acciones del operador.
- section: Web3 EVM Configuration — Esta sección debe revisarse junto con valores de configuración, evidencia de verificación, condiciones de fallo y acciones del operador.
- section: Operational Compatibility — Esta sección debe revisarse junto con valores de configuración, evidencia de verificación, condiciones de fallo y acciones del operador.

### Interfaces conservadas sin cambios
- `/v1` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `/v1/healthz` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `/v1/readyz` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `/v1/status` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `/v1/diagnostics` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `/v1/capabilities` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `/v1/metrics` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `/v1/metrics/text` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `/v1/peers` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `/v1/tx` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `/v1/evidence` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `/v1/recovery` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `/v1/snapshot/latest` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `/v1/snapshot/export` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `/v1/snapshot/chunk?index=0&size=10000` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `/v1/blocks` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `/v1/blocks/latest` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `/v1/blocks/{height}` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `/v1/state/latest` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `/v1/state/{height}/{namespace}` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `/v1/events?key={attribute_key}&value={attribute_value}` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `/v1/proof?namespace={namespace}&key={key}` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `/v1/proof?namespace={namespace}&key={key}&height=latest` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `/v1/finality/latest` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `/v1/finality/{height}` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `/v1/ibc/client/{client_id}` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `/v1/ibc/connection/{connection_id}` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `/v1/ibc/channel/{port_id}/{channel_id}` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `/v1/validators/{height}` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `/v1/committee/{height}/{round}` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `/v1/prune` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `/v1/replay` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `/v1/consensus/start` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `/v1/consensus/stop` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `network_config.json` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `tls_cert_path` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `tls_key_path` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `tls_ca_path` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `tls_server_name` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `vexod start` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `strict: true` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `eth_gasPrice` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `vexo_web3Capabilities` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `require_network_safety` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `rpc.NewNetworkSafeServer` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `rpc.NewNetworkSafeHandlerWithConfig` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `rpc.Config.RequiredCapabilities` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `rpc.Config.RequireAllCapabilities` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `pending_txs` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `state_by_height` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `app_query` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `strict_replay` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `consensus_control` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `/status` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `/tx` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `/blocks/latest` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `/v1/*` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `/v2/*` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `/v1/proof` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `commit_chain` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `/v1/status.latest_height` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `/v1/events` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `Index: true` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `{ "path": [...], "value": ... }` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `packets/{source_port}/{source_channel}/{sequence}` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `rpc_modules` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `web3_clientVersion` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `web3_sha3` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `eth_accounts` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `eth_coinbase` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `net_version` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `net_listening` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `net_peerCount` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `eth_chainId` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `eth_protocolVersion` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `eth_syncing` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `eth_mining` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `eth_hashrate` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `eth_blockNumber` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `eth_blobBaseFee` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `eth_maxPriorityFeePerGas` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `eth_feeHistory` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
