> Locale: es · Español

# Descripción del protocolo de consenso

Esta página es la entrada de alto nivel a la documentación de consenso de Vexo. Los detalles normativos están en [Consensus Spec](./specs/consensus-spec.md), [Finality Proof Format](./specs/finality-proof-format.md), [Validator Lifecycle](./specs/validator-lifecycle.md), [Storage Schema](./specs/storage-schema.md), [Networking Spec](./specs/networking-spec.md) y [Transaction Format](./specs/tx-format.md).

## Modelo

Vexo usa un núcleo BFT de estilo HotStuff con proposal, vote, quorum certificate(QC), timeout certificate, seguridad locked-QC y finalidad de tres cadenas. Solo es seguro votar un bloque si extiende el locked QC o incluye un justify QC igual o más reciente. Las cadenas QC sintéticas o con saltos de altura, sin enlazar explícitamente alturas y hashes de bloque, padre y abuelo, se rechazan antes de registrar finalidad.

## Identidad del protocolo y límite de investigación

Vexo no es un nombre nuevo para HotStuff sin modificar ni el mismo protocolo o implementación que AptosBFT, DiemBFT, Jolteon, Ditto, Tendermint o CometBFT. Un runtime Go independiente combina conceptos de seguridad de la familia HotStuff con tiempos de ronda adaptativos, recuperación durable, orden determinista de transacciones, ejecución modular y validator sets versionados por altura.

La ruta activa de votos usa el validator set completo de cada altura y proposer determinista. El selector VRF committee está disponible como componente y consulta, pero todavía no controla proposal eligibility ni quorum formation. Por ello debe documentarse como trabajo futuro. Consulte [Adaptive Recovery-Gated HotStuff for Modular Proof-of-Stake Networks](./research/adaptive-recovery-hotstuff-paper.md) para la contribución y el protocolo experimental.

## Límite de ejecución y recuperación

Certificación QC, finalidad HotStuff, ejecución de aplicación y commit de estado son eventos distintos. El valor predeterminado `execution_commit=finalized` ejecuta solo el ancestro elegido por la regla de tres cadenas. El pacemaker adaptativo y `recovery_finality_gate_enabled` controlan latencia y recuperación, pero no cambian proposer, quorum power, safe-vote ni finalidad.

## Límite de seguridad

- menos de un tercio del poder de voto bizantino
- propuestas separadas por dominio, votación, votación de tiempo de espera y firmas de firmeza
- enlace hash de conjunto de validador a la altura de prueba relevante
- firmantes únicos conocidos en controles de calidad y pruebas de firmeza
- evidencia responsable del equívoco del validador
- rechazo de decisiones de compromiso conflictivas a la misma altura finalizada

## Límite criptográfico

- El backend `deterministic` es exclusivo de pruebas y no supera la validación network safety.
- `ed25519` se admite para pruebas de red pública y preparación del lanzamiento.
- `bls` usa por defecto `blst-bls12381-minpk-v1` y exige proof-of-possession, comprobación de subgrupo, validación de claves, auditoría de dependencias y evidencia release-gate.
- La validación requiere metadatos del adaptador VRF, pero eso no significa que VRF committee esté en la ruta activa de consenso.

- auditoría de configuración estricta para cada casa de validador
- evidencia de release-gate
- revisión DE seguridad externa
- evidencia de caos y a largo plazo para varios anfitriones
- evidencia DE LA política del firmante/KMS
- revisión DE LA política económica Y DE gobernanza específica DE LA cadena

Consulte [Security Audit Readiness](./security/audit-readiness.md) y [Release Pipeline](./release/release-pipeline.md) antes de tratar una versión como lista para producción.
<!-- vexo-docs:technical-parity -->
## Apéndice de paridad técnica

Este apéndice conserva los términos técnicos y las interfaces que no deben cambiar entre la versión canónica y la traducción.

### Seguimiento de secciones
- section: Model - HotStuff, finalización de tres cadenas, QC, timeout certificate y locked-QC safety deben leerse juntos.
- section: Execution Terms - la diferencia entre qc certified, finalized, executed y state committed debe mantenerse clara.
- section: Safety Boundary - verificar el umbral byzantino inferior a un tercio, la separación por dominio, el hash del validator set y la accountable evidence.
- section: Crypto Boundary - conservar los identificadores `deterministic`, `ed25519`, `bls`, `blst-bls12381-minpk-v1` y `ecvrf-p256-sha256-tai-v1`.
- section: Operational Boundary - leer juntos `vexo_quorum_health_ratio`, `adaptive_round_timeout_enabled`, `recovery_finality_gate_enabled` y las señales de snapshot/replay.
- `require_network_safety` y `block_committed` deben permanecer visibles tal cual en la traducción.
- `config.json`
- `module_config.json`
- `network_config.json`
- `consensus_config.json`
- `mempool_config.json`
- `log_config.json`

### Interfaces que se conservan
- `/v1/status`
- `/v1/metrics`
- `/v1/diagnostics`
- `/v1/finality/latest`
- `/v1/state/latest`
- `/v1/recovery/report`
- `execution_commit`
- `finalized`
- `qc`
- `adaptive_round_timeout_enabled`
- `recovery_finality_gate_enabled`
- `vexo_quorum_health_ratio`
- `blst-bls12381-minpk-v1`
- `ecvrf-p256-sha256-tai-v1`
- `proof-of-possession`
- `remote signer`
- `three-chain finality`

## Notas operativas

Al crear un nuevo home de validador, revise `config.json` junto con `module_config.json`, `network_config.json`, `consensus_config.json`, `mempool_config.json` y `log_config.json`.
En producción, `vexo_quorum_health_ratio` y `adaptive_round_timeout_enabled` deben observarse juntos.

- `execution_commit=finalized` tiene prioridad.
- `qc` solo debe activarse en redes de prueba controladas.
- `recovery_finality_gate_enabled` debe verificarse con pruebas de snapshot y replay.
