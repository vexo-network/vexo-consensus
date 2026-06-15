> Locale: es · Español

# Guía de observabilidad

Esta guía explica cómo saber si un nodo Vexo está en buen estado a partir de RPC, métricas, registros y evidencia de publicación.

Está escrito para operadores que necesitan señales prácticas: qué observar, qué significa cada número y cuándo un valor debe tratarse como peligroso.

## De un vistazo

Si un nodo parece incorrecto, verifíquelo en orden:

1. `running` y `latest_height` en `/v1/status`
2. `latest_finalized_height` y recuentos de pares
3. `round_timeout`, latencia de propuesta/voto, tamaño de mempool y métricas de latencia de confirmación
4. Fallos del firmante, estado de las instantáneas y estado de la reproducción.
5. Prohibiciones de pares y fallas en el marcado de pares

Ese orden es importante porque separa “el proceso está vivo” de “la cadena está realmente avanzando de forma segura”.

## Puntos finales principales

| Punto final | Uso |
|---|---|
| `/v1/status` | Proceso rápido, altura, hash de la aplicación, finalidad y resumen de pares |
| `/v1/metrics` | Métricas JSON para paneles y automatización |
| `/metrics/text` | Métricas de texto compatibles con Prometheus |
| `/v1/diagnostics` | Verificaciones combinadas de preparación, capacidades, estado, pares, almacenamiento y métricas |
| `/v1/finality/latest` | Última prueba de finalidad para controles de seguridad y de cliente ligero |
| `/v1/state/latest` | Enlace de conjunto de validadores y raíz de estado más reciente |
| `/v1/recovery/report` | Diagnóstico de coherencia de bloqueo/reinicio |
| `/v1/snapshot` | Estado de la instantánea y metadatos de exportación |

Normalmente, los puntos finales de administración, como poda, reproducción y control de consenso, solo deberían ser accesibles a través de loopback, una red de operador, mTLS o una puerta de enlace autenticada. Los tokens de administrador con alcance siguen siendo opcionales y se aplican cuando se configuran.

## Leyendo `/v1/status`

Campos importantes:

| Campo | Significado | Nota del operador |
|---|---|---|
| `running` | El proceso del nodo se ha iniciado y posee el estado de ejecución | `true` no demuestra por sí solo la existencia del consenso |
| `latest_height` | Última altura de la aplicación comprometida localmente | Debe aumentar con el tiempo en una red de validadores en vivo |
| `latest_finalized_height` | Última altura finalizada de tres cadenas de HotStuff | No debe retrasarse indefinidamente con respecto a la altura ejecutada/comprometida |
| `latest_app_hash` | Hash de confirmación de la aplicación | Debe coincidir con sus compañeros a la misma altura |
| `peer_count` | Resumen de pares conectados/puntuados compatible con versiones anteriores | Prefiere los campos de pares más específicos a continuación |
| `active_peer_count` | Sesiones de transporte activo, cuando el transporte puede reportarlas | La mejor señal rápida para conectividad P2P en vivo |
| `configured_peer_count` | Direcciones de pares configuradas o aprendidas | La accesibilidad no está garantizada |
| `scored_peer_count` | Compañeros conocidos en la tabla de puntuación | Útil para historial de prohibiciones/límites de velocidad, no para prueba de sesiones en vivo |
| `banned_peers` | Compañeros actualmente prohibidos por la política de puntuación | Los picos indican ataque, mala configuración de pares o límites demasiado estrictos |

Ejemplo saludable para una red de un solo host con 4 validadores: `running=true`, `latest_height` en aumento, `latest_finalized_height` presente, `active_peer_count` cerca de `3` y `banned_peers=0`.

## Métricas de Prometeo

El punto final de texto expone indicadores como:

- `vexo_node_running`
- `vexo_latest_height`
- `vexo_peer_count`
- `vexo_active_peer_count`
- `vexo_configured_peer_count`
- `vexo_scored_peer_count`
- `vexo_banned_peers`
- `vexo_height_rate_per_minute`
- `vexo_round_timeouts`
- `vexo_proposal_latency_p95_nanos`
- `vexo_vote_latency_p95_nanos`
- `vexo_commit_latency_p95_nanos`
- `vexo_mempool_size`
- `vexo_snapshot_healthy`
- `vexo_replay_healthy`
- `vexo_validator_signing_failures`
- `vexo_post_commit_reconciliation_failures`

`vexo_peer_count` se conserva para paneles más antiguos. Los nuevos paneles deben representar `vexo_active_peer_count`, `vexo_configured_peer_count` y `vexo_scored_peer_count` por separado.

## Reglas de alerta sugeridas

Ajuste los números para el recuento real del validador, el intervalo de bloqueo, la latencia y el hardware. Estos son puntos de partida, no constantes universales.

| Alerta | Condición inicial | Por qué |
|---|---|---|
| Nodo caído | `vexo_node_running == 0` durante 1 minuto | Proceso/tiempo de ejecución detenido |
| Altura estancada | `latest_height` sin cambios durante 2-3 intervalos de bloque esperados | Consenso o ejecución estancada |
| Finalidad estancada | `latest_finalized_height` sin cambios mientras los bloques siguen ejecutándose | Vía de finalidad o cuestión de quórum |
| Sin compañeros activos | `vexo_active_peer_count == 0` durante 1 minuto en un nodo no aislado | Interrupción de P2P, falta de coincidencia de autenticación o problema de dirección |
| Recuento de pares demasiado bajo | pares activos por debajo del objetivo de conectividad del quórum | Problema de partición o arranque |
| Pico de tiempo de espera de ronda | contador de tiempo de espera crece más rápido que la línea de base normal | Latencia, error del proponente o partición de red |
| Confirmar latencia alta | p95/p99 se acerca al presupuesto de tiempo de espera de consenso | Sobrecarga de almacenamiento/tiempo de ejecución |
| Presión de mempool | el tamaño de mempool crece durante varios minutos | Política de tarifas, spam o problema de capacidad de bloqueo |
| Instantánea no saludable | `vexo_snapshot_healthy == 0` | Riesgo de sincronización/recuperación de estado |
| Repetir no saludable | `vexo_replay_healthy == 0` | Riesgo de determinismo o coherencia estatal |
| Fallos del firmante | `vexo_validator_signing_failures > 0` | KMS/firmante remoto/fallo de política |
| Fracasos de reconciliación | `vexo_post_commit_reconciliation_failures > 0` | Se necesita evidencia duradera o comprometerse a reparar |
| Aumento de pares prohibidos | pares prohibidos aumenta repentinamente | Ataque, pares mal configurados o problema de umbral de puntuación |

## Umbrales iniciales sugeridos

Utilícelos como valores de alerta iniciales y luego ajústelos después de una línea de base real a largo plazo:

| Señal | Advertencia | Crítico | Primera acción |
|---|---:|---:|---|
| Tasa de altura | por debajo del 50% de lo esperado para 2 ventanas | crecimiento cero durante intervalos de 2-3 bloques | compare todos los validadores, verifique los registros del proponente/firma/pares |
| Desfase de altura finalizado | crece durante 5 minutos | crece mientras la altura ejecutada sigue aumentando durante 10 minutos | inspeccionar registros de control de calidad/prueba de finalidad y hash del conjunto de validadores |
| Compañeros activos | por debajo del objetivo de conectividad del quórum | cero pares activos | comprobar la dirección anunciada, TLS/auth, génesis/ID de cadena no coinciden |
| Tiempos de espera de ronda | 3 veces el valor inicial normal | bucle de tiempo de espera continuo | aumentar el presupuesto de tiempo de espera o investigar la latencia/partición |
| Latencia de propuesta p95 | por encima del 50 % de `timeout_propose` | por encima del 80 % de `timeout_propose` | proponente de perfil, mempool, compromiso DA, disco |
| Latencia de voto p95 | por encima del 50% del presupuesto previo a la votación/compromiso | por encima del 80% del presupuesto | inspeccionar CPU, firmante, transporte, contrapresión de chismes |
| Confirmar latencia p95 | por encima del 50 % del intervalo de bloqueo | por encima del 80 % del intervalo de bloqueo | inspeccionar LevelDB, estado de raíces, ejecución de EVM, instantáneas |
| Tamaño del grupo de memoria | aumentando durante 5 minutos | cerca de `max_txs` o rotación de reemplazo sostenida | inspeccionar tarifa base, tarifa mínima, validez de transmisión, spam |
| Fallos del firmante | cualquier valor distinto de cero | fallas repetidas en una ventana de altura | detener el validador si aparece una protección de doble signo o una discrepancia de clave |
| Estado de la instantánea | un cheque fallido | exportación/verificación/restauración fallidas repetidas | pausar el servicio de sincronización de estado y ejecutar el informe de recuperación |
| Repetir salud | un error de repetición estricto | repetición de discrepancia en la última altura segura | preservar el directorio de datos y detener la actualización/lanzamiento inseguro |
| Compañeros prohibidos | pico repentino | muchos compañeros baneados después del lanzamiento de la configuración | comprobar límites de puntuación, TLS CA, identidad de pares, prueba de autenticación opcional y desviación horaria |

La regla más importante: alerta sobre **cambios a lo largo del tiempo**. Un solo número puede resultar engañoso; La tasa de altura, el retraso en la finalidad, la rotación de pares, el crecimiento de mempool y las fallas de firmantes cuentan la historia real.

## Matriz de clasificación de incidentes

| Situación | Capa probable | Qué conservar | Siguiente paso seguro |
|---|---|---|---|
| Altura detenida, compañeros sanos | consenso/firmante/tiempo de ejecución | registros de consenso, registros de firmantes, muestra de mempool | verificar la clave del proponente y redondear los registros de tiempo de espera |
| Los pares cayeron después del despliegue | redes/configuración | configuración de red, certificados TLS, libro de direcciones, registros de pares | revertir el cambio de dirección/TLS/autenticación anunciados |
| Los hash de las aplicaciones difieren a la misma altura | ejecución/almacenamiento | directorios de datos, registros de bloques, registros de aplicaciones, salida de reproducción | detener los nodos afectados y ejecutar una reproducción estricta |
| Prueba de finalidad rechazada | conjunto de finalidad/validador | JSON de prueba, validador colocado a la altura de prueba | verificar el hash establecido por el validador y firmar el dominio de bytes |
| La restauración de instantáneas falla | sincronización/almacenamiento de estado | archivo de instantánea, suma de comprobación, raíces de estado, registros de restauración | no vuelva a intentarlo con datos en vivo; restaurar en un directorio limpio |
| El firmante remoto rechaza solicitudes | custodia de claves | registro de auditoría del firmante, archivo de protección, archivo nonce, registros de nodo | distinguir el rechazo de la política de la interrupción del transporte |
| Los pares prohibidos aumentan | P2P/seguridad | instantáneas de puntuación de pares y motivos de prohibición | inspeccionar chismes con formato incorrecto o configuración incorrecta compartida |

Durante los incidentes, prefiera preservar los datos a “limpiarlos”. La eliminación de WAL, libretas de direcciones, protecciones de firmantes o directorios LevelDB puede destruir la evidencia necesaria para distinguir un error de un error del operador.

## Registrar eventos para conservar

Los registros estructurados deben conservarse con el ID del nodo, el ID del validador, el ID de la cadena, la altura, la ronda, el hash del bloque y el ID del par, cuando sea relevante.

Eventos importantes:

- `node_running`
- `rpc_listening`
- `p2p_listening`
- `peer_configured`
- `peer_connected`
- `peer_disconnected`
- `peer_dial_failed`
- `peer_banned`
- `consensus_loop_running`
- `block_committed`
- `round_timeout`
- `validator_signing_failure`
- `evidence_received`
- `evidence_applied`
- `snapshot_exported`
- `replay_checked`
- `upgrade_halt`
- `upgrade_applied`

Para los candidatos a lanzamiento, archive registros junto con muestras de métricas, muestras de pprof, archivos de configuración, génesis, sumas de verificación binarias y manifiestos de evidencia.

## Guía de primera respuesta

Cuando un operador ve un problema:

1. Marque `/v1/status` en al menos dos validadores.
2. Compare `latest_height`, `latest_finalized_height`, `latest_app_hash` y los recuentos de pares.
3. Verifique `/v1/diagnostics` para detectar capacidades faltantes o comprobaciones de almacenamiento/reproducción/instantáneas en mal estado.
4. Inspeccione los registros de eventos de pares en busca de errores de autenticación, TLS, génesis, ID de cadena o retroceso.
5. Inspeccione las métricas de mempool y tarifa base si los txs no están incluidos.
6. Verifique los registros del firmante y del firmante remoto si fallan las firmas del validador.
7. Exporte el informe de recuperación antes de eliminar o modificar datos.
8. Si se sospecha un conflicto de finalidad, detenga la automatización, conserve los registros/evidencias y ejecute la detección de conflictos de finalidad.

## Diseño del panel

Un panel útil suele tener cinco filas:

1. **Vivencia**: nodo en ejecución, altura más reciente, altura finalizada, tasa de altura.
2. **Latencia de consenso**: tiempos de espera de ronda, propuesta/voto/compromiso p95 y p99.
3. **Red**: pares activos/configurados/puntuados, pares prohibidos, mensajes de ventana de pares.
4. **Ejecución**: tamaño de mempool, tarifa base/gas, recuento de tx, latencia de confirmación.
5. **Recuperación y seguridad**: estado de la instantánea, estado de la reproducción, fallas del firmante, fallas de conciliación.

Mantenga los paneles aburridos. El objetivo no es mostrar todos los contadores internos; es hacer obvios los estados peligrosos antes de que los validadores diverjan o los usuarios noten transacciones estancadas.

## Liberar evidencia de la observabilidad

Para un candidato a lanzamiento, la observabilidad no es solo monitoreo en vivo. Se convierte en evidencia:

1. Recopile la línea base `/v1/status`, `/v1/metrics`, `/v1/diagnostics`, `/v1/finality/latest` y `/v1/recovery/report` de cada validador.
2. Ejecute la carga durante la duración y velocidad elegidas.
3. Inyecte al menos un reinicio, una interrupción de pares y un simulacro de exportación/verificación/restauración de instantáneas.
4. Recopile métricas finales de cada validador.
5. Almacene las muestras de antes/después, los registros, las muestras de pprof, los registros de auditoría de firmantes y el manifiesto de evidencia en `dist/`.

Un buen paquete de evidencia permite al revisor responder: ¿aumentó la altura, progresó la finalidad, se recuperaron los pares, se confirmaron los txs, se verificaron las instantáneas, se mantuvo la reproducción en buen estado, evitaron los firmantes la doble firma y el binario de publicación exacto produjo los resultados?

<!-- vexo-docs:technical-parity -->
## Apéndice de paridad técnica

Este apéndice asegura que la traducción conserve las interfaces ejecutables y las secciones clave del documento canónico en inglés. Los comandos, claves de configuración, métodos RPC y nombres de paquetes se mantienen sin cambios en todos los idiomas.

### Seguimiento de secciones
- section: Core Endpoints — Esta sección debe revisarse junto con valores de configuración, evidencia de verificación, condiciones de fallo y acciones del operador.
- section: Reading `/v1/status` — Esta sección debe revisarse junto con valores de configuración, evidencia de verificación, condiciones de fallo y acciones del operador.
- section: Prometheus Metrics — Esta sección debe revisarse junto con valores de configuración, evidencia de verificación, condiciones de fallo y acciones del operador.
- section: Suggested Alert Rules — Esta sección debe revisarse junto con valores de configuración, evidencia de verificación, condiciones de fallo y acciones del operador.
- section: Suggested Starting Thresholds — Esta sección debe revisarse junto con valores de configuración, evidencia de verificación, condiciones de fallo y acciones del operador.
- section: Incident Triage Matrix — Esta sección debe revisarse junto con valores de configuración, evidencia de verificación, condiciones de fallo y acciones del operador.
- section: Log Events to Keep — Esta sección debe revisarse junto con valores de configuración, evidencia de verificación, condiciones de fallo y acciones del operador.
- section: First Response Playbook — Esta sección debe revisarse junto con valores de configuración, evidencia de verificación, condiciones de fallo y acciones del operador.
- section: Dashboard Layout — Esta sección debe revisarse junto con valores de configuración, evidencia de verificación, condiciones de fallo y acciones del operador.
- section: Release Evidence From Observability — Esta sección debe revisarse junto con valores de configuración, evidencia de verificación, condiciones de fallo y acciones del operador.

### Interfaces conservadas sin cambios
- `/v1/status` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `/v1/metrics` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `/metrics/text` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `/v1/diagnostics` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `/v1/finality/latest` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `/v1/state/latest` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `/v1/recovery/report` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `/v1/snapshot` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `latest_height` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `latest_finalized_height` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `latest_app_hash` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `peer_count` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `active_peer_count` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `configured_peer_count` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `scored_peer_count` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `banned_peers` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `banned_peers=0` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `vexo_node_running` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `vexo_latest_height` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `vexo_peer_count` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `vexo_active_peer_count` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `vexo_configured_peer_count` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `vexo_scored_peer_count` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `vexo_banned_peers` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `vexo_height_rate_per_minute` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `vexo_round_timeouts` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `vexo_proposal_latency_p95_nanos` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `vexo_vote_latency_p95_nanos` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `vexo_commit_latency_p95_nanos` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `vexo_mempool_size` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `vexo_snapshot_healthy` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `vexo_replay_healthy` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `vexo_validator_signing_failures` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `vexo_post_commit_reconciliation_failures` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `vexo_node_running == 0` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `vexo_active_peer_count == 0` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `vexo_snapshot_healthy == 0` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `vexo_replay_healthy == 0` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `vexo_validator_signing_failures > 0` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `vexo_post_commit_reconciliation_failures > 0` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `timeout_propose` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `max_txs` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `node_running` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `rpc_listening` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `p2p_listening` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `peer_configured` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `peer_connected` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `peer_disconnected` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `peer_dial_failed` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `peer_banned` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `consensus_loop_running` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `block_committed` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `round_timeout` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `validator_signing_failure` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `evidence_received` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `evidence_applied` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `snapshot_exported` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `replay_checked` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `upgrade_halt` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `upgrade_applied` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `dist/` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
