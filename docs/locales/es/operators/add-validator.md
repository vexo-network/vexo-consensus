# Adding a Validator

> Locale: es · Español
> Este documento es un documento de acompañamiento en español para leer junto con la fuente inglesa. Las decisiones de protocolo, seguridad y release siguen siendo normativas en inglés.


## Orden de lectura

Este documento explica cómo añadir un validator a una red Vexo. Si es tu primera lectura, sigue este orden.

1. Initialize Validator Home
2. Configure Network Addresses and Peers
3. Submit Validator Admission
4. Verify Validator Set Update
5. Plan Validator Key Rotation
6. Start Validator
7. Monitor
8. Safety Notes

Ese orden coincide con el flujo operativo real: crear primero el nuevo validator home y las claves, configurar después las direcciones de red y los peers, verificar la admisión y el cambio de validator set, y por último revisar rotación, arranque, monitoreo y notas de seguridad.

## Resumen

Este documento ayuda a entender el alta de un validator, validación de configuración y controles de staking y a conectarlo con decisiones de implementación y operación.

- Canonical path: `docs/operators/add-validator.md`
- Locale path: `docs/locales/es/operators/add-validator.md`

## Por qué leer este documento

- el alta de un validator, validación de configuración y controles de staking
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

- `node.key.json`
- `p2p.node_id`
- `p2p.node_key_path`
- `node_id`
- `node_key_path`
## Estructura de la fuente inglesa

- Adding a Validator
- 1. Initialize Validator Home
- 2. Configure Network Addresses and Peers
- 3. Submit Validator Admission
- 4. Verify Validator Set Update
- 5. Plan Validator Key Rotation
- 6. Start Validator
- 7. Monitor
- Safety Notes

## Fuente canónica

- [Documento canónico en inglés](../../en/operators/add-validator.md)

<!-- vexo-docs:technical-parity -->
## Apéndice de paridad técnica

Este apéndice asegura que la traducción conserve las interfaces ejecutables y las secciones clave del documento canónico en inglés. Los comandos, claves de configuración, métodos RPC y nombres de paquetes se mantienen sin cambios en todos los idiomas.

### Seguimiento de secciones
- section: 1. Initialize Validator Home — Esta sección debe revisarse junto con valores de configuración, evidencia de verificación, condiciones de fallo y acciones del operador.
- section: 2. Configure Network Addresses and Peers — Esta sección debe revisarse junto con valores de configuración, evidencia de verificación, condiciones de fallo y acciones del operador.
- section: 3. Submit Validator Admission — Esta sección debe revisarse junto con valores de configuración, evidencia de verificación, condiciones de fallo y acciones del operador.
- section: 4. Verify Validator Set Update — Esta sección debe revisarse junto con valores de configuración, evidencia de verificación, condiciones de fallo y acciones del operador.
- section: 5. Plan Validator Key Rotation — Esta sección debe revisarse junto con valores de configuración, evidencia de verificación, condiciones de fallo y acciones del operador.
- section: 6. Start Validator — Esta sección debe revisarse junto con valores de configuración, evidencia de verificación, condiciones de fallo y acciones del operador.
- section: 7. Monitor — Esta sección debe revisarse junto con valores de configuración, evidencia de verificación, condiciones de fallo y acciones del operador.
- section: Safety Notes — Esta sección debe revisarse junto con valores de configuración, evidencia de verificación, condiciones de fallo y acciones del operador.

### Interfaces conservadas sin cambios
- `VEXO_KEY_PASSPHRASE` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `--passphrase` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `bls_pop` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `blst-bls12381-minpk-v1` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `vexod keys gen --type bls --bls-adapter circl-bls12381-g1sigg2-basic-v1` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `node.key.json` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `network_config.json:p2p.node_id` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `.vexo-validator-new/network_config.json` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `network_config.json` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `p2p.listen_address` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `rpc.address` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `p2p.node_id` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `p2p.node_key_path` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `p2p.peers` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `p2p_address` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `rpc_address` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `node_id` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `active_from` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `active_until` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `config audit --strict` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
