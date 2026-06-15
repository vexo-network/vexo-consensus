# Documentation

> Locale: es · Español
> Este documento es un documento de acompañamiento en español para leer junto con la fuente inglesa. Las decisiones de protocolo, seguridad y release siguen siendo normativas en inglés.

## Inicio rápido

- Usa `make build` para compilar el binario.
- Crea un validator home con `vexod init validator --home .vexo-validator-1 --chain-id vexo-chain --validator validator-1 --encrypt-keys`, compruébalo con `vexod validate --home .vexo-validator-1` y `vexod config audit --home .vexo-validator-1 --strict`, y luego arráncalo con `vexod start --home .vexo-validator-1`.
- Ejecuta la red Docker primero con `docker compose -f deployments/docker/compose.single-host-init.yml up` y después con `docker compose -f deployments/docker/compose.single-host.yml up`.
- Remix debe apuntar a `http://127.0.0.1:28657/web3`; para comprobar el chain ID usa `curl -s http://127.0.0.1:26657/web3 -H 'content-type: application/json' -d '{"jsonrpc":"2.0","id":1,"method":"eth_chainId","params":[]}'`.
- Tras modificar documentos, ejecuta `make docs-check` para verificar el árbol local y los controles de traducción.
## Resumen

Este documento ayuda a entender el índice de documentación y el orden de lectura recomendado y a conectarlo con decisiones de implementación y operación.

- Canonical path: `docs/README.md`
- Locale path: `docs/locales/es/README.md`

## Por qué leer este documento

- el índice de documentación y el orden de lectura recomendado
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
- `/v1/*`
- `docs/locales/{en,ko,zh,ja,fr,de,es,pt,ru,ar,hi,id,vi}/`
- `make docs-check`

## Estructura de la fuente inglesa

- Documentation
- How to Read This Set
- Protocol Specs
- SDK and Extension Guides
- Operations and Release
- Security
- Localized Documentation
- Writing New Docs
- Documentation Review Checklist

## Fuente canónica

- [Documento canónico en inglés](../en/README.md)

## Lista de términos que se conservan

Los términos siguientes no se traducen.

- `vexo-consensus`
- `make build`
- `vexod validate --home .vexo-validator-1`
- `vexod config audit --home .vexo-validator-1 --strict`
- `vexod start --home .vexo-validator-1`
- `curl -s http://127.0.0.1:26657/v1/status`
- `docker compose -f deployments/docker/compose.single-host-init.yml up`
- `docker compose -f deployments/docker/compose.single-host.yml up`
- `http://127.0.0.1:28657/web3`
- `/v1/*`
- `docs/locales/{en,ko,zh,ja,fr,de,es,pt,ru,ar,hi,id,vi}/`
- `make docs-check`
- `vexod status --json`
- `feature_assurance`
- `network_config.json:p2p.auth_replay_path`
- `network_config.json:p2p.node_key_path`
- `module_config.json:governance.RequireDeposit`
- `module_config.json:governance.MinDeposit`
- `consensus_config.json:consensus.execution_commit`
- `mempool_config.json:mempool.WALPath`

<!-- vexo-docs:technical-parity -->
## Apéndice de paridad técnica

Este apéndice asegura que la traducción conserve las interfaces ejecutables y las secciones clave del documento canónico en inglés. Los comandos, claves de configuración, métodos RPC y nombres de paquetes se mantienen sin cambios en todos los idiomas.

### Seguimiento de secciones
- section: How to Read This Set — Esta sección debe revisarse junto con valores de configuración, evidencia de verificación, condiciones de fallo y acciones del operador.
- section: Start Here — Esta sección debe revisarse junto con valores de configuración, evidencia de verificación, condiciones de fallo y acciones del operador.
- section: Protocol Specs — Esta sección debe revisarse junto con valores de configuración, evidencia de verificación, condiciones de fallo y acciones del operador.
- section: SDK and Extension Guides — Esta sección debe revisarse junto con valores de configuración, evidencia de verificación, condiciones de fallo y acciones del operador.
- section: Operations and Release — Esta sección debe revisarse junto con valores de configuración, evidencia de verificación, condiciones de fallo y acciones del operador.
- section: Security — Esta sección debe revisarse junto con valores de configuración, evidencia de verificación, condiciones de fallo y acciones del operador.
- section: Localized Documentation — Esta sección debe revisarse junto con valores de configuración, evidencia de verificación, condiciones de fallo y acciones del operador.
- section: Writing New Docs — Esta sección debe revisarse junto con valores de configuración, evidencia de verificación, condiciones de fallo y acciones del operador.
- section: Production Claim Rule — Esta sección debe revisarse junto con valores de configuración, evidencia de verificación, condiciones de fallo y acciones del operador.
- section: Documentation Review Checklist — Esta sección debe revisarse junto con valores de configuración, evidencia de verificación, condiciones de fallo y acciones del operador.

### Interfaces conservadas sin cambios
- `vexo-consensus` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `/v1/*` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `docs/locales/{en,ko,zh,ja,fr,de,es,pt,ru,ar,hi,id,vi}/` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `make docs-check` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `vexod status --json` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `feature_assurance` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `network_config.json:p2p.auth_replay_path` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `network_config.json:p2p.node_key_path` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `module_config.json:governance.RequireDeposit` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `module_config.json:governance.MinDeposit` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `consensus_config.json:consensus.execution_commit` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `mempool_config.json:mempool.WALPath` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
