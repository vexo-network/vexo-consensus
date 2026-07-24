# HotStuff adaptativo con compuerta de recuperación para redes Proof-of-Stake modulares

> Locale: es · Español  
> Tipo de documento: manuscrito de investigación y protocolo de reproducibilidad  
> Estado: borrador basado en la implementación; las afirmaciones de rendimiento requieren mediciones archivadas.

## Resumen

Este trabajo estudia una replicación de máquina de estados BFT de estilo HotStuff para redes Proof-of-Stake modulares. La implementación combina finalización de tres cadenas y conjuntos de validadores versionados por altura con tres mecanismos operativos. Un controlador acotado adapta el timeout de ronda usando las latencias p95 de propuesta, voto y commit, junto con la salud de los peers activos. Una compuerta de finalización consciente de la recuperación aplaza commits de aplicación finalizados cuando el historial durable de bloques y el historial de estado divergen por encima de una altura segura común. Un orden determinista elimina el orden de llegada local del mempool para un conjunto idéntico de transacciones, pero conserva las dependencias de nonce de cada firmante.

La contribución no es afirmar que PoS, BFT, HotStuff, la sincronización adaptativa de vistas o la equidad de orden sean nuevos. La pregunta es si esta composición concreta y acotada reduce timeouts evitables e inconsistencias durante la recuperación sin cambiar la regla de seguridad HotStuff subyacente. Se separan hechos implementados, hipótesis refutables y conclusiones que aún necesitan experimentos. No se publicará una mejora de throughput o latency hasta completar repeticiones con binary, configuración, topología y workload fijados.

## Preguntas de investigación

RQ1 compara la política adaptativa con el mismo sistema usando timeout fijo bajo cambios de latencia de red, observando frecuencia de timeout y p95 commit latency. RQ2 inyecta fallos de almacenamiento y reinicio para comprobar que la compuerta impide que el estado de aplicación avance más allá de la altura durable común. RQ3 permuta un conjunto idéntico de transacciones y exige el mismo orden de propuesta y nonces crecientes por firmante. RQ4 mide CPU, memoria, red y latencia adicional en condiciones estables sin fallos.

H1 a H4 son hipótesis direccionales y falsables, no resultados. La existencia del código no demuestra una mejora. Si el beneficio no es significativo, debe informarse como resultado negativo o límite de aplicabilidad.

## Trabajo previo y límite de novedad

HotStuff ya introdujo BFT liderado bajo sincronía parcial, certificados de quorum, commit encadenado, comunicación lineal en el camino favorable y capacidad de respuesta. LibraBFT/DiemBFT y AptosBFT ya combinan descendientes de HotStuff con gobernanza de validadores ponderada por stake. Jolteon y Ditto estudian menor latencia, adaptación de red y fallback asíncrono; Fever aborda sincronización responsiva de vistas. Tendermint constituye otra familia PoS BFT por rondas. Narwhal/Tusk separa difusión fiable y orden. Aequitas, Wendy y Themis formalizan equidad de orden más fuerte que la determinación por hash usada aquí.

Por ello no son válidas expresiones como “primera blockchain PoS+BFT”, “primera red PoS con HotStuff”, “idéntica a AptosBFT”, “liveness asíncrona” o “complejidad óptima” sin prueba, “protección MEV completa”, ni “production-ready” por un test single-host. La contribución candidata es más estrecha: integrar un controlador acotado, una compuerta local de historia durable y orden determinista sensible al nonce en un nodo PoS modular escrito en Go, y evaluarlos de forma reproducible contra baselines fixed y gate-disabled.

## Modelo y mecanismos

En la altura h, Vh es el conjunto activo y Ph su voting power total. Un QC es válido cuando firmantes conocidos y únicos aportan al menos dos tercios de Ph. El conjunto y su hash se versionan por altura. La admisión puede ser permissionless con stake mínimo, limitada por cantidad o restringida por configuración. Esta capa trata la resistencia Sybil y governance; no cambia el umbral BFT.

La red se modela como parcialmente síncrona. La safety requiere menos de un tercio de poder bizantino, firmas válidas, vinculación al conjunto correcto y almacenamiento durable. La liveness requiere además que el retraso llegue a estar acotado, exista un quorum honesto alcanzable, los signers estén disponibles y haya conectividad suficiente. No se promete progreso en asincronía permanente.

EVM es un workload de aplicación bajo Vexo consensus. Ejecutar bytecode Ethereum y ofrecer compatibilidad `/web3` no equivale a implementar fork choice ni devp2p consensus de Ethereum.

La regla de seguridad sigue `locked_qc` y `high_qc`. Una propuesta solo es segura si extiende el lock o incluye un justify QC al menos igual de reciente. Un validador no puede votar bloques distintos en la misma altura y ronda. Tres enlaces certificados consecutivos, ligados por altura y hash, finalizan el bloque abuelo. El controlador no modifica ese predicado, el umbral, la verificación QC ni la regla three-chain.

El timeout adaptativo usa el presupuesto base T0, el actual Tt, la suma de latencias p95 y un suelo según déficit de peers. Tras timeout crece hacia 1,5×Tt; tras progreso decrece hacia 0,8×Tt. Tres veces la latencia observada forma un suelo candidato y el resultado se limita entre T0 y 8×T0. Sin peers activos, el suelo es 2×T0. El idle sin trabajo y los errores locales de ejecución o almacenamiento no consumen rondas. Es un controlador operativo acotado, no un pacemaker demostrado como óptimo.

La compuerta calcula Hsafe=min(Hs,Hb) cuando existen la altura durable de estado Hs y la altura del índice de bloques Hb. Mientras difieran, aplaza commits finalizados por encima de Hsafe. Es una restricción local de persistencia, no otra fase de votación ni un certificado de red.

El orden determinista deriva un salt de chain ID y altura. Las transacciones con firmante y nonce se agrupan en cadenas por firmante, se ordenan por nonce creciente y sus cabezas se fusionan por hash salado. Así se elimina la dependencia del orden de llegada para un candidato idéntico. No se garantiza first-seen fairness, resistencia a censura, confidencialidad ni strong order-fairness, pues el proposer todavía influye en la inclusión.

El camino de voto actual usa el conjunto completo versionado por altura y proposer determinista. El selector ECVRF existe como componente y consulta, pero no participa en quorum formation ni proposal eligibility. El consensus por comité VRF queda como trabajo futuro.

## Diseño experimental

Todos los tratamientos usan el mismo binary y configuración de aplicación. Se comparan fixed con adaptación apagada y compuerta activa, adaptive con ambas activas, y una ablación con compuerta apagada solamente en una red aislada desechable. Cuando haya recursos se usan 4, 7, 16 y 31 validadores; single-host sirve solo como smoke test.

Las condiciones incluyen latencias de 10, 50, 100 y 250 ms, cambios escalonados, jitter, pérdida de 0/1/5/10 %, reinicio de un validador, reinicio del proposer actual, indisponibilidad justo por debajo de un tercio del poder, partición minoritaria y recuperación, retraso del signer e inconsistencia durable inyectada. Los workloads incluyen transferencias nativas y EVM, creación de contrato, event logs, proxy deployment y UUPS upgrade.

Se recogen alturas committed/finalized, p50/p95/p99 de proposal/vote/commit, latencia final end-to-end, cantidad de timeouts, distribución de rondas, timeout adaptativo, peers, recovery deferrals, throughput, gas, CPU, RSS, disco/red, rechazos, double-sign e invalid nonce. Un run solo entra al análisis si todos los validadores acuerdan app hash y finalized block hash, las ubicaciones de transaction/receipt/block coinciden, existe código desplegado y el estado del proxy se conserva tras el upgrade.

Después de warm-up se hacen al menos treinta repeticiones independientes por condición, salvo una justificación previa mediante análisis de potencia. Se aleatoriza el orden de treatments y se guardan seeds. Se reportan mediana, IQR, p95, intervalos de confianza y tamaño del efecto. No se elige únicamente el mejor run y las reglas de exclusión se fijan antes de examinar resultados.

## Corrección, reproducción y ética

La adaptación cambia cuándo intentar timeout vote, no qué vote o QC es seguro. La compuerta solo restringe commits y nunca puede autorizar uno rechazado por la regla base. El orden determinista apoya una entrada de ejecución común, pero no reemplaza la prueba contra finalidades conflictivas.

Una prueba publicable debe formalizar intersección de quorums ponderados, monotonía del lock, unicidad del bloque finalizado en cada altura, transición de validator set, recuperación del vote WAL y neutralidad de seguridad del controlador y la compuerta. Tests y simulaciones adversariales son evidencia, no sustituyen proof formal ni auditoría independiente.

Cada experimento archiva commit, dirty-tree status, Go/OS/CPU/memoria/contenedor, topología, genesis, split configs, SHA-256 del binary, seed, datos JSON/JSONL/CSV, logs, app hashes finales, scripts de análisis y registro de runs fallidos. No se renombra trabajo conocido como invención, no se fabrican cifras y se separan hypothesis, observation e interpretation.

La asistencia de IA se declara según la política del venue y los autores responden por cada claim, citation, experiment y proof. La inyección de fallos se realiza únicamente en sistemas aislados propios o autorizados. Private keys, operator tokens, datos de participantes y production endpoints no se publican. Los defectos de seguridad siguen divulgación coordinada.

Antes de enviar, el manuscrito debe coincidir con una revisión fijada, la búsqueda de prior art debe estar archivada, las bases deben ser reproducibles, las mediciones multi-host completas y cada tabla/figura regenerable desde raw data. Se mantienen resultados negativos, limitaciones, redacción de proof apropiada y revisión metodológica externa. Hasta entonces la descripción correcta es “borrador de investigación basado en la implementación”, no “consensus nuevo y probado”.

<!-- vexo-docs:technical-parity -->

## Anexo de paridad técnica

Se mantienen sin traducir:

- `/web3`, `V_h`, `P_h`, `locked_qc`, `high_qc`
- `consensus/state_machine.go`, `consensus/state_machine_test.go`
- `consensus/commit_rule.go`, `consensus/commit_rule_test.go`
- `consensus/timeout.go`, `consensus/pacemaker.go`
- `node/adaptive_timeout.go`, `node/loop.go`, `node/adaptive_timeout_test.go`
- `node/recovery.go`, `node/consensus_loop.go`
- `fairordering/fairordering.go`, `modules/staking`, `consensus/wal.go`
- `modules/evm`, `modules/evm/backend/geth`
- `consensus_config.json`, `adaptive_round_timeout_enabled`
- `recovery_finality_gate_enabled`, `execution_commit = "finalized"`
- `/v1/status`, `/v1/metrics`, `/v1/finality/latest`, `/metrics/text`
- `deployments/docker/README.md`, `http://127.0.0.1:28657/web3`
- `make check`, `make fuzz-smoke`, `make ops-verify`
- `make network-e2e`, `make evm-conformance`
- `go run ./cmd/vexod consensus adversarial --json`
- `Fpeer = 2 * T0`, `Hs != Hb`, `h > Hsafe`
