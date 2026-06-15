# Validator Lifecycle

> Locale: es · Español
> Este documento es un documento de acompañamiento en español para leer junto con la fuente inglesa. Las decisiones de protocolo, seguridad y release siguen siendo normativas en inglés.


## Orden de lectura

Este documento explica la especificación normativa de Validator Lifecycle. Si es tu primera lectura, sigue este orden.

1. Scope
2. Admission
3. Validator Set
4. Rotation
5. Evidence Lifecycle
6. Slashing
7. Jail and Unbonding

Ese orden coincide con la forma correcta de leerlo: primero el alcance y el estado, luego las reglas de mensajes, seguridad y vivacidad, y al final las evidencias.

## Resumen

Este documento ayuda a entender ciclo validator join, rotation, jail, slashing y leave y a conectarlo con decisiones de implementación y operación.

- Canonical path: `docs/specs/validator-lifecycle.md`
- Locale path: `docs/locales/es/specs/validator-lifecycle.md`

## Por qué leer este documento

- ciclo validator join, rotation, jail, slashing y leave
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

- `vexovaloper...`
- `address`
- `vexovalcons...`
- `vexo...`
- `H`
- `H + 1`

## Estructura de la fuente inglesa

- Validator Lifecycle
- Scope
- Admission
- Validator Set
- Rotation
- Evidence Lifecycle
- Slashing
- Jail and Unbonding

## Fuente canónica

- [Documento canónico en inglés](../../en/specs/validator-lifecycle.md)

<!-- vexo-docs:technical-parity -->
## Apéndice de paridad técnica

Este apéndice asegura que la traducción conserve las interfaces ejecutables y las secciones clave del documento canónico en inglés. Los comandos, claves de configuración, métodos RPC y nombres de paquetes se mantienen sin cambios en todos los idiomas.

### Seguimiento de secciones
- section: Scope — Esta sección debe revisarse junto con valores de configuración, evidencia de verificación, condiciones de fallo y acciones del operador.
- section: Admission — Esta sección debe revisarse junto con valores de configuración, evidencia de verificación, condiciones de fallo y acciones del operador.
- section: Validator Set — Esta sección debe revisarse junto con valores de configuración, evidencia de verificación, condiciones de fallo y acciones del operador.
- section: Rotation — Esta sección debe revisarse junto con valores de configuración, evidencia de verificación, condiciones de fallo y acciones del operador.
- section: Evidence Lifecycle — Esta sección debe revisarse junto con valores de configuración, evidencia de verificación, condiciones de fallo y acciones del operador.
- section: Slashing — Esta sección debe revisarse junto con valores de configuración, evidencia de verificación, condiciones de fallo y acciones del operador.
- section: Jail and Unbonding — Esta sección debe revisarse junto con valores de configuración, evidencia de verificación, condiciones de fallo y acciones del operador.

### Interfaces conservadas sin cambios
- `vexovaloper...` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `vexovalcons...` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `vexo...` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `staking tx withdraw-unbonded` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
