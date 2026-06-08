# RPC API Versioning

> Locale: es · Español
> Este documento es una guía traducida desde la documentación canónica en inglés. Las decisiones de protocolo, seguridad y publicación siguen siendo normativas en inglés.

## Propósito

Este documento cubre versionado de RPC API, alias de compatibilidad y política de estabilidad. Los comandos, campos JSON, nombres RPC, config key e identificadores de código usados en implementación y operación se mantienen en inglés por compatibilidad.

## Alcance principal

- Revise los siguientes puntos al leer este documento. Los comandos, campos JSON, métodos RPC, claves de configuración e identificadores de código se mantienen en inglés por compatibilidad.
- Para redacción normativa detallada, use el documento inglés.
- Canonical path: `docs/sdk/rpc-api-versioning.md`
- Locale path: `docs/locales/es/sdk/rpc-api-versioning.md`

## Identificadores que se conservan

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

## Secciones en inglés

- RPC API Versioning
- Stability Goal
- Current Stable API
- Versioning Rules
- Compatibility Aliases
- Error Format
- Query Proofs
- Event Queries
- IBC Queries
- Web3 EVM Configuration
- Operational Compatibility

## Notas operativas

- `MUST`, `SHOULD`, `MAY`, ejemplos de comandos, ejemplos JSON y nombres RPC mantienen la grafía inglesa.
- Después de cambiar esta traducción, ejecute `make docs-check`.
- Si esta página contradice la fuente inglesa, use la fuente inglesa y actualice este archivo locale en el mismo cambio.

## Fuente canónica

- [English canonical document](../../en/sdk/rpc-api-versioning.md)
