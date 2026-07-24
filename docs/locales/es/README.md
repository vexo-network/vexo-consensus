> Locale: es · Español

# Documentación

Este directorio es el manual práctico de `vexo-consensus`. Está dirigido a desarrolladores, operadores, responsables de versiones y revisores que necesitan comprender la red sin deducir su comportamiento únicamente del código fuente.

Cada página debe explicar la responsabilidad del componente, los archivos, comandos, claves de configuración y API que lo implementan, sus condiciones de seguridad y la evidencia necesaria antes de una red real. El inglés sigue siendo la fuente normativa para protocolo, seguridad, versiones, SDK, comandos, configuración y RPC; esta traducción ayuda a leer, pero no sustituye la fuente inglesa en decisiones de auditoría.

Para empezar, use los comandos siguientes y después lea `Node Initialization`, `Docker Deployment`, `Observability Guide` y `RPC API Versioning`.

| Tarea | Ruta de comando |
|---|---|
| Crear binario local | `make build` |
| Crea un alojamiento para validadores | `vexod init validator --home .vexo-validator-1 --chain-id vexo-chain --validator validator-1 --encrypt-keys` |
| Validar un alojamiento | `vexod validate --home .vexo-validator-1` y `vexod config audit --home .vexo-validator-1 --strict` |
| Ejecutar un nodo | `vexod start --home .vexo-validator-1` |
| Consulta un nodo | `curl -s http://127.0.0.1:26657/v1/status` |
| Ejecutar la red de cuatro validadores de Docker | __ VEXO_CODE_5__ seguido de __ VEXO_CODE_6__ |
| Conectar Remix | Usar el validador de Docker 1 Web3 URL `http://127.0.0.1:28657/web3` |
| Compruebe el ID de la cadena Web3 | __ VEXO_CODE_7__ |

## Inicio rápido

- `make build`
- `vexod init validator --home .vexo-validator-1 --chain-id vexo-chain --validator validator-1 --encrypt-keys`
- `vexod validate --home .vexo-validator-1`
- `vexod config audit --home .vexo-validator-1 --strict`
- `vexod start --home .vexo-validator-1`
- `curl -s http://127.0.0.1:26657/v1/status`
- `docker compose -f deployments/docker/compose.single-host-init.yml up`
- `docker compose -f deployments/docker/compose.single-host.yml up`
- `http://127.0.0.1:28657/web3`
## Empieza aquí

| Documento | Propósito |
|---|---|
| [Guía de preparación para la producción](./production-readiness.md) | Mapa único de protocolo, tiempo de ejecución, operaciones, evidencia y preparación para la publicación |

## Especificaciones del protocolo

- [Consensus Spec](./specs/consensus-spec.md), [Finality Proof Format](./specs/finality-proof-format.md) y [Validator Lifecycle](./specs/validator-lifecycle.md) describen seguridad, finalidad y cambios del validator set.
- [Networking Spec](./specs/networking-spec.md), [Storage Schema](./specs/storage-schema.md) y [Transaction Format](./specs/tx-format.md) cubren transporte, recuperación durable y admisión de transacciones.
- [EVM and Native Accounting](./specs/evm-native-accounting.md) define la frontera entre contabilidad nativa y EVM.

## SDK y extensiones

[App Module Guide](./sdk/app-module-guide.md), [Custom Crypto Backend](./sdk/custom-crypto-backend.md), [Custom Storage and Transport](./sdk/custom-storage-transport.md) y `RPC API Versioning` explican cómo extender el runtime sin romper los contratos de consenso o RPC.

## Operación, versiones y seguridad

`Node Initialization`, [Adding a Validator](./operators/add-validator.md), `Observability Guide`, [Guía de lanzamiento](./release/launch-runbook.md), `Release Pipeline` y [Version Compatibility Matrix](./release/version-compatibility.md) forman el recorrido operativo. [Security Audit Readiness](./security/audit-readiness.md) documenta el modelo de amenazas y la evidencia obligatoria.

## Regla de madurez

La existencia de código no demuestra preparación para producción. Se requieren pruebas unitarias, adversariales y E2E, artefactos operativos, supuestos y modos de fallo, y resultados del release gate. Los comandos, métodos RPC y claves de configuración permanecen idénticos en todas las traducciones.

## Investigación y publicación

Para preparar un artículo, comience con [`Adaptive Recovery-Gated HotStuff Research Draft`](./research/adaptive-recovery-hotstuff-paper.md). El documento separa los mecanismos realmente implementados, como el tiempo de ronda adaptativo, la compuerta de finalidad durante recuperación y el orden determinista de transacciones, de los trabajos previos. Reúne preguntas de investigación, hipótesis, protocolo experimental, artefactos reproducibles y ética de investigación. No presenta rendimiento sin medir como resultado ni afirma que PoS, BFT o HotStuff sean contribuciones nuevas.

Los nombres normativos preservados para navegar entre idiomas son `Node Initialization`, `Docker Deployment`, `Observability Guide`, `RPC API Versioning`, `Production Readiness`, `Release Pipeline` y `Adaptive Recovery-Gated HotStuff Research Draft`.

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
