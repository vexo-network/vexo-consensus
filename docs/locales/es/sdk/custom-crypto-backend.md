# Guía del backend criptográfico personalizado

> Locale: es · Español
> Este documento es un documento de acompañamiento en español para leer junto con la fuente inglesa. Las decisiones de protocolo, seguridad y release siguen siendo normativas en inglés.


## Orden de lectura

Este documento explica cómo añadir un custom crypto backend. Si es tu primera lectura, sigue este orden.

1. Interfaces
2. Runtime Suite
3. Domain Separation
4. Production BLS Requirements
5. VRF Backend Requirements
6. Remote Signer Requirements
7. Test Backends

Ese orden refleja las decisiones que hay que tomar primero: elegir el backend necesario, fijar luego los sign bytes y el dominio, y por último verificar si puede usarse en producción.

## Resumen

Este documento ayuda a entender integración de custom crypto backend como BLS, VRF y signer y a conectarlo con decisiones de implementación y operación.

- Canonical path: `docs/sdk/custom-crypto-backend.md`
- Locale path: `docs/locales/es/sdk/custom-crypto-backend.md`

## Por qué leer este documento

- integración de custom crypto backend como BLS, VRF y signer
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

- `vexo-consensus`
- `vexo.consensus.proposal.v1`
- `vexo.consensus.vote.v1`
- `vexo.consensus.timeout_vote.v1`
- `vexo.finality.proof.v1`
- `BLSAdapter`
- `ValidateBLSAdapter`
- `init()`
- `crypto.adapter_name`
- `BLSAdapter.Metadata().Name`
- `BLSValidatorCredential`
- `bls_pop`
- `ValidateBLSValidatorCredentials`
- `NewBLSAggregateVerifier`
- `circl-bls12381-g1sigg2-basic-v1`
- `Metadata()`
- `NewBLSTBLSKeyDocument`
- `NewCIRCLBLSKeyDocument`
- `bls_proof_of_possession`
- `vrf.adapter_name`
- `vrf.audit_report`
- `vrf.key_source`
- `committee.backend`

- `vrf.dependency_audit`
- `vrf.audit_evidence_sha256`
- `ecvrf-p256-sha256-tai-v1`
- `remote-vrf-http-v1`
## Estructura de la fuente inglesa

- Custom Crypto Backend Guide
- Objetivo
- Interfaces
- Runtime Suite
- Domain Separation
- Production BLS Requirements
- Production VRF Requirements
- Remote Signer Requirements
- Test Backends

## VRF audit evidence SHA-256

El VRF backend debe exponer una frontera de auditoría tan clara como BLS. Completa `vrf.adapter_name`, `vrf.audit_report`, `vrf.dependency_audit`, `vrf.audit_evidence_sha256` y `vrf.key_source`; si los metadata del adapter no coinciden con la config, el runtime debe fail closed. El adapter ECVRF integrado verifica el go.mod dependency pin y el audit evidence digest; el remote VRF adapter usa una referencia externa de auditoría KMS/HSM.

## Fuente canónica

- [Documento canónico en inglés](../../en/sdk/custom-crypto-backend.md)

## Remote VRF service

`vexod keys serve-vrf` expone `POST /prove` y `POST /verify` con una clave ECVRF, y `vexod keys verify-vrf` valida el remote prover de extremo a extremo. Mantén sin traducir `VEXO_REMOTE_VRF_TOKEN`, `remote-vrf-http-v1`, `vexo.remote_vrf.prove.v1` y `vexo.remote_vrf.verify.v1`.

Mantén estos nombres de interfaz sin cambios: `vexod keys serve-vrf`, `vexod keys verify-vrf`, `POST /prove`, `POST /verify`, `VEXO_REMOTE_VRF_TOKEN`, `remote-vrf-http-v1`, `vexo.remote_vrf.prove.v1`, `vexo.remote_vrf.verify.v1`.

<!-- vexo-docs:technical-parity -->
## Apéndice de paridad técnica

Este apéndice asegura que la traducción conserve las interfaces ejecutables y las secciones clave del documento canónico en inglés. Los comandos, claves de configuración, métodos RPC y nombres de paquetes se mantienen sin cambios en todos los idiomas.

### Seguimiento de secciones
- section: Goal — Esta sección debe revisarse junto con valores de configuración, evidencia de verificación, condiciones de fallo y acciones del operador.
- section: Interfaces — Esta sección debe revisarse junto con valores de configuración, evidencia de verificación, condiciones de fallo y acciones del operador.
- section: Runtime Suite — Esta sección debe revisarse junto con valores de configuración, evidencia de verificación, condiciones de fallo y acciones del operador.
- section: Domain Separation — Esta sección debe revisarse junto con valores de configuración, evidencia de verificación, condiciones de fallo y acciones del operador.
- section: Production BLS Requirements — Esta sección debe revisarse junto con valores de configuración, evidencia de verificación, condiciones de fallo y acciones del operador.
- section: Production VRF Requirements — Esta sección debe revisarse junto con valores de configuración, evidencia de verificación, condiciones de fallo y acciones del operador.
- section: Remote Signer Requirements — Esta sección debe revisarse junto con valores de configuración, evidencia de verificación, condiciones de fallo y acciones del operador.
- section: Test Backends — Esta sección debe revisarse junto con valores de configuración, evidencia de verificación, condiciones de fallo y acciones del operador.

### Interfaces conservadas sin cambios
- `vexo-consensus` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `supranational/blst` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `vexo.consensus.proposal.v1` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `vexo.consensus.vote.v1` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `vexo.consensus.timeout_vote.v1` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `vexo.finality.proof.v1` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `crypto.adapter_name` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `BLSAdapter.Metadata().Name` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `crypto.audit_evidence_sha256` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `bls_pop` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `blst-bls12381-minpk-v1` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `github.com/supranational/blst` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `RELEASE_CGO_ENABLED=1` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `RELEASE_REQUIRE_BLS=1` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `make release-portable RELEASE_REQUIRE_BLS=0` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `circl-bls12381-g1sigg2-basic-v1` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `bls_proof_of_possession` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `vrf.adapter_name` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `vrf.audit_report` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `vrf.dependency_audit` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `vrf.audit_evidence_sha256` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `vrf.key_source` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `committee.backend` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `crypto.NewProductionVRF` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `production_adapter: true` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `ecvrf-p256-sha256-tai-v1` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `vrf_public_key` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `remote-vrf-http-v1` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `remote-http:<base-url>` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `POST /prove` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `public_key` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `issued_at_unix_nano` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `deadline_unix_nano` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `vexo.remote_vrf.prove.v1` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `POST /verify` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `vexo.remote_vrf.verify.v1` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `{ "valid": true, "nonce": "<same nonce>" }` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `VEXO_REMOTE_VRF_TOKEN` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `Authorization: Bearer <token>` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `vrf.tls_cert_path` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `vrf.tls_key_path` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `vrf.tls_ca_path` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `vrf.tls_server_name` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `keys serve-vrf` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `--auth-token` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `--auth-token-env` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `vexod keys serve-vrf` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `crypto.NewRemoteVRFService` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `--home` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `remote-vrf-nonces.jsonl` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `remote-vrf-audit.jsonl` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `--nonce-path` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `--audit-log` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `crypto.RemoteVRFServiceConfig.ReplayStore` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `RequireDurableReplayStore: true` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `crypto.NewFileRemoteVRFReplayStore` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `consensus_config.json` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `vexod` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `vrf_key_paths` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `VEXO_KEY_PASSPHRASE` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `vrf.keys` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `vexod keys serve-remote` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `--guard-path` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `consensus_proposal` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `consensus_vote` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `consensus_timeout_vote` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `finality_proof` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
