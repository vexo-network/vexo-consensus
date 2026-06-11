# Runbook de lanzamiento

> Locale: es · Español
> Este documento es un documento de acompañamiento en español para leer junto con la fuente inglesa. Las decisiones de protocolo, seguridad y release siguen siendo normativas en inglés.

## Resumen

Este documento ayuda a entender checklist operativa y procedimiento antes del lanzamiento de red y a conectarlo con decisiones de implementación y operación.

- Canonical path: `docs/release/launch-runbook.md`
- Locale path: `docs/locales/es/release/launch-runbook.md`

## Por qué leer este documento

- checklist operativa y procedimiento antes del lanzamiento de red
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
- `checksums.txt`
- `sbom-go-modules.json`
- `sbom-go-version.txt`
- `release-manifest.json`
- `release-audit-pack.json`
- `evidence-manifest.json`
- `--evidence-manifest`
- `--evm-default-fixtures`
- `chain_id`

- `--bls-audit`
- `--bls-audit-sha256`
- `--vrf-audit`
- `--vrf-audit-sha256`
## Estructura de la fuente inglesa

- Runbook de lanzamiento
- Prelaunch Gate
- Release Candidate Gate
- Genesis Gate
- Launch Window
- Postlaunch Archive

## VRF audit evidence SHA-256

Al validar un release candidate, pasa a `release gate` los digests de auditoría BLS y VRF. Usa al menos `--bls-audit`, `--bls-audit-sha256`, `--vrf-audit`, `--vrf-audit-sha256` y `--evidence-manifest`, y comprueba que cada archivo evidence coincida con el SHA-256 del manifest.

## Fuente canónica

- [Documento canónico en inglés](../../en/release/launch-runbook.md)
