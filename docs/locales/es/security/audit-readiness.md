# Security Audit Readiness

> Locale: es · Español
> Este documento es un documento de acompañamiento en español para leer junto con la fuente inglesa. Las decisiones de protocolo, seguridad y release siguen siendo normativas en inglés.

## Resumen

Este documento ayuda a entender threat model, supuestos de seguridad y evidencias de auditoría y a conectarlo con decisiones de implementación y operación.

- Canonical path: `docs/security/audit-readiness.md`
- Locale path: `docs/locales/es/security/audit-readiness.md`

## Por qué leer este documento

- threat model, supuestos de seguridad y evidencias de auditoría
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

- `MaxScore`
- `release gate`
- `/v1/*`
- `chain_id`
- `(height, round)`

- `crypto.audit_evidence_sha256`
- `vrf.dependency_audit`
- `vrf.audit_evidence_sha256`
- `docs/security/ecvrf-audit-evidence.json`
## Estructura de la fuente inglesa

- Security Audit Readiness
- Scope
- Threat Model
- Assets
- Adversaries
- Objetivos de seguridad
- Security Assumptions
- Known Limitations
- Formal-ish Safety Argument
- Required Evidence for Audit
- Auditor Focus Areas

## VRF audit evidence SHA-256

La entrega de auditoría debe incluir VRF adapter audit evidence además de BLS. Fija el SHA-256 de un archivo como `docs/security/ecvrf-audit-evidence.json` en `vrf.audit_evidence_sha256` o `--vrf-audit-sha256`, y revisa dependency audit, key custody, TLS/mTLS o pinned CA, auth, replay defense y service availability como una sola frontera.

## Fuente canónica

- [Documento canónico en inglés](../../en/security/audit-readiness.md)

<!-- vexo-docs:technical-parity -->
## Apéndice de paridad técnica

Este apéndice asegura que la traducción conserve las interfaces ejecutables y las secciones clave del documento canónico en inglés. Los comandos, claves de configuración, métodos RPC y nombres de paquetes se mantienen sin cambios en todos los idiomas.

### Seguimiento de secciones
- section: Scope — Esta sección debe revisarse junto con valores de configuración, evidencia de verificación, condiciones de fallo y acciones del operador.
- section: Threat Model — Esta sección debe revisarse junto con valores de configuración, evidencia de verificación, condiciones de fallo y acciones del operador.
- section: Security Assumptions — Esta sección debe revisarse junto con valores de configuración, evidencia de verificación, condiciones de fallo y acciones del operador.
- section: Known Limitations — Esta sección debe revisarse junto con valores de configuración, evidencia de verificación, condiciones de fallo y acciones del operador.
- section: Formal-ish Safety Argument — Esta sección debe revisarse junto con valores de configuración, evidencia de verificación, condiciones de fallo y acciones del operador.
- section: Required Evidence for Audit — Esta sección debe revisarse junto con valores de configuración, evidencia de verificación, condiciones de fallo y acciones del operador.
- section: Auditor Focus Areas — Esta sección debe revisarse junto con valores de configuración, evidencia de verificación, condiciones de fallo y acciones del operador.
- section: Practical Audit Walkthrough — Esta sección debe revisarse junto con valores de configuración, evidencia de verificación, condiciones de fallo y acciones del operador.
- section: Remote Signer Audit Notes — Esta sección debe revisarse junto con valores de configuración, evidencia de verificación, condiciones de fallo y acciones del operador.
- section: EVM/Web3 Audit Notes — Esta sección debe revisarse junto con valores de configuración, evidencia de verificación, condiciones de fallo y acciones del operador.
- section: Snapshot and WAL Audit Notes — Esta sección debe revisarse junto con valores de configuración, evidencia de verificación, condiciones de fallo y acciones del operador.

### Interfaces conservadas sin cambios
- `docs/security/blst-audit-evidence.json` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `remote-vrf-http-v1` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `vexod keys serve-vrf` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `release collect-evidence` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `/v1/*` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `chain_id` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `go.mod` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `/v1/recovery/report` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
