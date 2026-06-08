# EVM and Native Accounting

> Locale: es · Español
> Este documento es una guía traducida desde la documentación canónica en inglés. Las decisiones de protocolo, seguridad y publicación siguen siendo normativas en inglés.

## Propósito

Este documento cubre conexión coherente entre native coin y EVM gas/accounting. Los comandos, campos JSON, nombres RPC, config key e identificadores de código usados en implementación y operación se mantienen en inglés por compatibilidad.

## Alcance principal

- Revise los siguientes puntos al leer este documento. Los comandos, campos JSON, métodos RPC, claves de configuración e identificadores de código se mantienen en inglés por compatibilidad.
- Para redacción normativa detallada, use el documento inglés.
- Canonical path: `docs/specs/evm-native-accounting.md`
- Locale path: `docs/locales/es/specs/evm-native-accounting.md`

## Identificadores que se conservan

- `avxo`
- `gvxo`
- `10^9 avxo`
- `vexo`
- `10^18 avxo`
- `bank`
- `0x`
- `uint64`
- `fee`
- `fee=1`
- `fee=1avxo`
- `fee=1gvxo`
- `fee=1vexo`
- `base_fee * gas`
- `value`
- `uint256`
- `contract.Invocation`
- `eth_getBalance`

## Secciones en inglés

- EVM and Native Accounting
- Core Rule
- Amount Encoding
- Fee Accounting
- EVM Execution
- Compatibility Boundary
- Failure Modes

## Notas operativas

- `MUST`, `SHOULD`, `MAY`, ejemplos de comandos, ejemplos JSON y nombres RPC mantienen la grafía inglesa.
- Después de cambiar esta traducción, ejecute `make docs-check`.
- Si esta página contradice la fuente inglesa, use la fuente inglesa y actualice este archivo locale en el mismo cambio.

## Fuente canónica

- [English canonical document](../../en/specs/evm-native-accounting.md)
