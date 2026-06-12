# Node Initialization

> Locale: es · Español
> Este documento es un documento de acompañamiento en español para leer junto con la fuente inglesa. Las decisiones de protocolo, seguridad y release siguen siendo normativas en inglés.

## Resumen

Este documento ayuda a entender inicialización de nodos archive/validator y uso de archivos de configuración separados y a conectarlo con decisiones de implementación y operación.

- Canonical path: `docs/operators/node-initialization.md`
- Locale path: `docs/locales/es/operators/node-initialization.md`

## Por qué leer este documento

- inicialización de nodos archive/validator y uso de archivos de configuración separados
- Revise primero las frases MUST/SHOULD/MAY en la fuente inglesa.
- Este documento localizado ayuda a comprender; auditoría, release y seguridad se deciden con la fuente inglesa.

## Qué debería poder hacer después

- Explicar qué decisión de implementación u operación apoya este documento.
- Relacionar los requisitos normativos de la fuente inglesa con la configuración actual de la red.
- Verificar chain ID, validator ID, fee/gas y direcciones peer antes de copiar ejemplos.

## Checklist de uso seguro

- Revise primero las frases MUST/SHOULD/MAY en la fuente inglesa.
- No traduzca comandos, config key, nombres RPC, campos JSON ni identificadores de código.
- Antes de copiar ejemplos, adapte chain ID, validator ID, fee/gas y direcciones peer a su red.
- Después de modificar documentación, ejecute `make docs-check` para verificar locale tree y guards de traducción.

## Puntos de atención

- Este documento localizado ayuda a comprender; auditoría, release y seguridad se deciden con la fuente inglesa.
- Si cambia la implementación, actualice la fuente inglesa y todos los documentos localizados en el mismo cambio.

## Interfaces que deben conservarse

- `network_config.json`
- `start`
- `vexod start`
- `--timeout-propose`
- `--create-empty-blocks`
- `--p2p-auth-token`
- `--rpc-admin-token`
- `--evm-account-key-env`
- `--evm-account-key`
- `validator_id`
- `init validator`
- `VEXO_KEY_PASSPHRASE`
- `--passphrase`
- `--encrypt-keys`
- `validator.key.json`
- `validator.vrf.key.json`
- `--key-type bls`
- `genesis.json`
- `bls_pop`
- `config.json`
- `module_config.json`
- `consensus_config.json`
- `mempool_config.json`

- `node.key.json`
- `p2p.node_id`
- `p2p.node_key_path`
- `node_id`
- `node_key_path`
## Estructura de la fuente inglesa

- Node Initialization
- Validator Node
- Archive Node
- Split Configuration Files
- Key Types
- Config-Based Peers
- Consensus Timing
- Multi-Validator Network

## Fuente canónica

- [Documento canónico en inglés](../../en/operators/node-initialization.md)
<!-- vexo-docs-ops-update-2026-06 -->

## Nota operativa reciente

En un nuevo home de nodo revise juntos `p2p.dial_timeout`, `p2p.auth_replay_path` y `p2p.require_auth_replay_store` dentro de `network_config.json`. El valor por defecto `10s` cubre TCP dial, TLS, signed handshake y replay-store. En redes públicas estos valores deben estar en la configuración revisada, no escondidos en flags de shell.

## State Sync al iniciar

El bloque `state_sync` de `network_config.json` sirve para nuevos nodos archive, validator de reemplazo o nodos restaurados en una máquina limpia. Cuando `state_sync.enabled` es true, `vexod start` prueba `state_sync.snapshot_urls` en orden, verifica chain ID, checksum, state root y KV namespace, restaura en LevelDB, reconstruye índices y solo entonces arranca el nodo. Si el estado local ya alcanza `state_sync.min_height` y `state_sync.trust_local_higher` es true, conserva el store local y registra `state_sync_skipped`.

```json
{
  "state_sync": {
    "enabled": true,
    "snapshot_urls": ["https://snapshots.example.com/vexo-chain/latest.json"],
    "timeout": "30s",
    "min_height": 1000000,
    "require_fresh": true,
    "trust_local_higher": true,
    "max_snapshot_bytes": 268435456,
    "retry_all_snapshots": true
  }
}
```

Revise `state_sync_candidate_failed`, `state_sync_candidate_rejected` y `state_sync_applied`. En redes públicas no use snapshots de terceros sin política de confianza y finality/light-client evidence.

<!-- vexo-docs:technical-parity -->
## Apéndice de paridad técnica

Este apéndice asegura que la traducción conserve las interfaces ejecutables y las secciones clave del documento canónico en inglés. Los comandos, claves de configuración, métodos RPC y nombres de paquetes se mantienen sin cambios en todos los idiomas.

### Seguimiento de secciones
- section: Validator Node — Esta sección debe revisarse junto con valores de configuración, evidencia de verificación, condiciones de fallo y acciones del operador.
- section: Archive Node — Esta sección debe revisarse junto con valores de configuración, evidencia de verificación, condiciones de fallo y acciones del operador.
- section: Split Configuration Files — Esta sección debe revisarse junto con valores de configuración, evidencia de verificación, condiciones de fallo y acciones del operador.
- section: Key Types — Esta sección debe revisarse junto con valores de configuración, evidencia de verificación, condiciones de fallo y acciones del operador.
- section: Config-Based Peers — Esta sección debe revisarse junto con valores de configuración, evidencia de verificación, condiciones de fallo y acciones del operador.
- section: Consensus Timing — Esta sección debe revisarse junto con valores de configuración, evidencia de verificación, condiciones de fallo y acciones del operador.
- section: Multi-Validator Network — Esta sección debe revisarse junto con valores de configuración, evidencia de verificación, condiciones de fallo y acciones del operador.

### Interfaces conservadas sin cambios
- `network_config.json` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `vexod start` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `--timeout-propose` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `--create-empty-blocks` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `--p2p-auth-token` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `--rpc-admin-token` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `--evm-account-key-env` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `--evm-account-key` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `validator_id` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `VEXO_KEY_PASSPHRASE` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `--passphrase` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `--encrypt-keys` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `validator.key.json` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `node.key.json` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `validator.vrf.key.json` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `require_network_safety=true` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `--key-type bls` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `blst-bls12381-minpk-v1` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `genesis.json` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `bls_pop` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `config.json` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `module_config.json` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `consensus_config.json` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `mempool_config.json` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `log_config.json` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `data/` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `network_config.json:p2p.node_key_path` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `shutdown_timeout` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `web3_max_subscriptions_per_connection` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `web3_idle_timeout` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `auth_replay_path` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `require_auth_replay_store` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `dial_timeout` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `data/p2p_auth_replay.jsonl` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `--key-type ed25519` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `vrf_key_paths` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `vrf_public_key` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `vexod` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `<home>/<name>_config.json` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `rpc.evm_account_key_envs` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `rpc.evm_account_private_keys` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `eth_accounts` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `eth_sign` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `eth_signTransaction` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `eth_sendTransaction` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `evm_account_key_envs` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `vexod config paths --home <home>` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `"require_network_safety": true` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `execution_commit` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `require_network_safety` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `host:port` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `rpc.address` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `p2p.listen_address` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `p2p.peers` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `p2p.seeds` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `p2p.node_id` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `p2p.node_key_path` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `p2p.tls_cert_path` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `p2p.tls_key_path` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `p2p.tls_ca_path` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `p2p.tls_server_name` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `p2p.dial_timeout` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `timeout_propose` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `timeout_prevote` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `timeout_precommit` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `timeout_commit` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `create_empty_blocks: false` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `execution_commit: "finalized"` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `execution_commit: "qc"` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `round_timeout` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `create_empty_blocks` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `vexod network up` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `make network-e2e` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `p2p_host_template` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `rpc_host_template` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `validator-%d` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `p2p_advertise_host_template` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `rpc_advertise_host_template` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `p2p_listen_host` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `rpc_listen_host` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
