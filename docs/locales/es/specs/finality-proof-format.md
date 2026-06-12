# Finality Proof Format

> Locale: es · Español
> Este documento es un documento de acompañamiento en español para leer junto con la fuente inglesa. Las decisiones de protocolo, seguridad y release siguen siendo normativas en inglés.

## Resumen

Este documento ayuda a entender campos de finality proof, orden de verificación y validator set binding y a conectarlo con decisiones de implementación y operación.

- Canonical path: `docs/specs/finality-proof-format.md`
- Locale path: `docs/locales/es/specs/finality-proof-format.md`

## Por qué leer este documento

- campos de finality proof, orden de verificación y validator set binding
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

- `finality.Proof`
- `Header`
- `QuorumCert`
- `ValidatorSetHeight`
- `ValidatorSetHash`
- `/v1/finality/latest`
- `/v1/finality/{height}`
- `/v1/status.latest_height`
- `Proof.ValidatorSetHeight == Header.Height`
- `Proof.ValidatorSetHash == loaded_set.Hash()`
- `Header.ValidatorSetHash == loaded_set.Hash()`
- `QuorumCert.Height == Header.Height`
- `QuorumCert.BlockHash == Proof.BlockHash == HeaderHash(Header)`
- `finality.AttackDetector`
- `--validator-set`
- `BLSAdapter`
- `vexo.finality.proof.v1`

## Estructura de la fuente inglesa

- Finality Proof Format
- Scope
- Proof Fields
- Header Fields
- Quorum Certificate Fields
- Verification Algorithm
- Accountable Safety Detection
- Ed25519 Model
- BLS Model

## Fuente canónica

- [Documento canónico en inglés](../../en/specs/finality-proof-format.md)

<!-- vexo-docs:technical-parity -->
## Apéndice de paridad técnica

Este apéndice asegura que la traducción conserve las interfaces ejecutables y las secciones clave del documento canónico en inglés. Los comandos, claves de configuración, métodos RPC y nombres de paquetes se mantienen sin cambios en todos los idiomas.

### Seguimiento de secciones
- section: Scope — Esta sección debe revisarse junto con valores de configuración, evidencia de verificación, condiciones de fallo y acciones del operador.
- section: Proof Fields — Esta sección debe revisarse junto con valores de configuración, evidencia de verificación, condiciones de fallo y acciones del operador.
- section: Header Fields — Esta sección debe revisarse junto con valores de configuración, evidencia de verificación, condiciones de fallo y acciones del operador.
- section: Quorum Certificate Fields — Esta sección debe revisarse junto con valores de configuración, evidencia de verificación, condiciones de fallo y acciones del operador.
- section: Commit Chain Fields — Esta sección debe revisarse junto con valores de configuración, evidencia de verificación, condiciones de fallo y acciones del operador.
- section: Verification Algorithm — Esta sección debe revisarse junto con valores de configuración, evidencia de verificación, condiciones de fallo y acciones del operador.
- section: Accountable Safety Detection — Esta sección debe revisarse junto con valores de configuración, evidencia de verificación, condiciones de fallo y acciones del operador.
- section: Ed25519 Model — Esta sección debe revisarse junto con valores de configuración, evidencia de verificación, condiciones de fallo y acciones del operador.
- section: BLS Model — Esta sección debe revisarse junto con valores de configuración, evidencia de verificación, condiciones de fallo y acciones del operador.

### Interfaces conservadas sin cambios
- `finality.Proof` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `/v1/finality/latest` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `/v1/finality/{height}` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `strict: true` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `/v1/status.latest_height` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `/v1/finality/*` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `Proof.ValidatorSetHeight <= Header.Height` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `Proof.ValidatorSetHash == loaded_set.Hash()` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `Header.ValidatorSetHash == loaded_set.Hash()` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `QuorumCert.Height == Header.Height` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `QuorumCert.BlockHash == Proof.BlockHash == HeaderHash(Header)` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `Header.TxRoot` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `HeaderHash(link.Header)` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `finality.AttackDetector` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `--validator-set` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `blst-bls12381-minpk-v1` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `supranational/blst` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
