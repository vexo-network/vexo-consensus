# Node Initialization

> Locale: es · Español
> Este documento es una guía traducida desde la documentación canónica en inglés. Las decisiones de protocolo, seguridad y publicación siguen siendo normativas en inglés.

## Propósito

Este documento cubre inicialización de nodos archive/validator y uso de archivos de configuración separados. Los comandos, campos JSON, nombres RPC, config key e identificadores de código usados en implementación y operación se mantienen en inglés por compatibilidad.

## Alcance principal

- Revise los siguientes puntos al leer este documento. Los comandos, campos JSON, métodos RPC, claves de configuración e identificadores de código se mantienen en inglés por compatibilidad.
- Para redacción normativa detallada, use el documento inglés.
- Canonical path: `docs/operators/node-initialization.md`
- Locale path: `docs/locales/es/operators/node-initialization.md`

## Identificadores que se conservan

- `network_config.json`
- `start`
- `vexod start`
- `--timeout-propose`
- `--create-empty-blocks`
- `--p2p-auth-token`
- `--rpc-admin-token`
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

## Secciones en inglés

- Node Initialization
- Validator Node
- Archive Node
- Split Configuration Files
- Key Types
- Config-Based Peers
- Consensus Timing
- Multi-Validator Network

## Notas operativas

- `MUST`, `SHOULD`, `MAY`, ejemplos de comandos, ejemplos JSON y nombres RPC mantienen la grafía inglesa.
- Después de cambiar esta traducción, ejecute `make docs-check`.
- Si esta página contradice la fuente inglesa, use la fuente inglesa y actualice este archivo locale en el mismo cambio.

## Fuente canónica

- [English canonical document](../../en/operators/node-initialization.md)
