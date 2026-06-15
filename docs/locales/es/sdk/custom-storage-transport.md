# Custom Storage and Transport Guide

> Locale: es · Español
> Este documento es un documento de acompañamiento en español para leer junto con la fuente inglesa. Las decisiones de protocolo, seguridad y release siguen siendo normativas en inglés.


## Orden de lectura

Este documento explica cómo implementar y registrar un custom storage y un transport adapter. Si es tu primera vez, léelo en este orden.

1. Custom Storage
2. Storage Requirements
3. Custom Transport
4. Transport Requirements
5. Compatibility

Ese orden coincide con los riesgos que conviene revisar primero: comprobar si el almacenamiento soporta crash, pruning, snapshot y replay, y después verificar que el transporte maneja autenticación, negociación de versión, reconexión y bloqueo correctamente.

## Resumen

Este documento ayuda a entender implementación y registro de custom storage y transport adapter y a conectarlo con decisiones de implementación y operación.

- Canonical path: `docs/sdk/custom-storage-transport.md`
- Locale path: `docs/locales/es/sdk/custom-storage-transport.md`

## Por qué leer este documento

- implementación y registro de custom storage y transport adapter
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

- `store.Store`
- `store.HistoricalSnapshotKVStore`
- `store.SnapshotKVStore`
- `transport.Transport`

## Estructura de la fuente inglesa

- Custom Storage and Transport Guide
- Custom Storage
- Storage Requirements
- Custom Transport
- Transport Requirements
- Compatibility

## Fuente canónica

- [Documento canónico en inglés](../../en/sdk/custom-storage-transport.md)

<!-- vexo-docs:technical-parity -->
## Apéndice de paridad técnica

Este apéndice asegura que la traducción conserve las interfaces ejecutables y las secciones clave del documento canónico en inglés. Los comandos, claves de configuración, métodos RPC y nombres de paquetes se mantienen sin cambios en todos los idiomas.

### Seguimiento de secciones
- section: Custom Storage — Esta sección debe revisarse junto con valores de configuración, evidencia de verificación, condiciones de fallo y acciones del operador.
- section: Storage Requirements — Esta sección debe revisarse junto con valores de configuración, evidencia de verificación, condiciones de fallo y acciones del operador.
- section: Custom Transport — Esta sección debe revisarse junto con valores de configuración, evidencia de verificación, condiciones de fallo y acciones del operador.
- section: Transport Requirements — Esta sección debe revisarse junto con valores de configuración, evidencia de verificación, condiciones de fallo y acciones del operador.
- section: Compatibility — Esta sección debe revisarse junto con valores de configuración, evidencia de verificación, condiciones de fallo y acciones del operador.

### Interfaces conservadas sin cambios
- `store.Store` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `store.HistoricalSnapshotKVStore` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `store.SnapshotKVStore` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `store.AppBlockCommitStore` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `vexod start` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `runtime.NewNetworkSafeWithStore` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `runtime.NewNetworkSafeWithStoreContext` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `runtime.NewNetworkSafeWithStoreAndCryptoRegistryContext` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `config.ValidateNetworkSafety` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `app.AtomicBlockApplication` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `transport.Transport` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `transport.GRPCConfig.RequireTLS` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
