# Adding a Validator

> Locale: es · Español
> Este documento es una guía traducida desde la documentación canónica en inglés. Las decisiones de protocolo, seguridad y publicación siguen siendo normativas en inglés.

## Propósito

Este documento cubre el alta de un validator, validación de configuración y controles de staking. Los comandos, campos JSON, nombres RPC, config key e identificadores de código usados en implementación y operación se mantienen en inglés por compatibilidad.

## Alcance principal

- Revise los siguientes puntos al leer este documento. Los comandos, campos JSON, métodos RPC, claves de configuración e identificadores de código se mantienen en inglés por compatibilidad.
- Para redacción normativa detallada, use el documento inglés.
- Canonical path: `docs/operators/add-validator.md`
- Locale path: `docs/locales/es/operators/add-validator.md`

## Identificadores que se conservan

- `VEXO_KEY_PASSPHRASE`
- `--passphrase`
- `bls_pop`
- `.vexo-validator-new/network_config.json`
- `network_config.json`
- `p2p.listen_address`
- `rpc.address`
- `p2p.peers`
- `p2p_address`
- `rpc_address`
- `active_from`
- `active_until`
- `config audit --strict`

## Secciones en inglés

- Adding a Validator
- 1. Initialize Validator Home
- 2. Configure Network Addresses and Peers
- 3. Submit Validator Admission
- 4. Verify Validator Set Update
- 5. Plan Validator Key Rotation
- 6. Start Validator
- 7. Monitor
- Safety Notes

## Notas operativas

- `MUST`, `SHOULD`, `MAY`, ejemplos de comandos, ejemplos JSON y nombres RPC mantienen la grafía inglesa.
- Después de cambiar esta traducción, ejecute `make docs-check`.
- Si esta página contradice la fuente inglesa, use la fuente inglesa y actualice este archivo locale en el mismo cambio.

## Fuente canónica

- [English canonical document](../../en/operators/add-validator.md)
