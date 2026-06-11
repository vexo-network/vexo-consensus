# Custom Storage and Transport Guide

> Locale: es · Español
> Este documento es un documento de acompañamiento en español para leer junto con la fuente inglesa. Las decisiones de protocolo, seguridad y release siguen siendo normativas en inglés.

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
