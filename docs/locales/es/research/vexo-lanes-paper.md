# Vexo Lanes: difusión de lotes certificados para un HotStuff de dos cadenas ponderado por participación

> Locale: es · Español  
> Tipo de documento: manuscrito de investigación y protocolo de reproducibilidad  
> Estado: borrador basado en la implementación; los resultados provienen de una sola máquina con enlaces emulados y deben repetirse en una red real antes de publicarse

## Resumen

Un protocolo BFT con líder de la familia HotStuff está limitado por el enlace de subida de su líder: cada transacción de un bloque sale de una máquina una vez por validador, de modo que el rendimiento cae como `1/(n−1)` mientras los enlaces de los demás validadores quedan ociosos. Vexo Lanes saca la difusión del líder sin sustituir el protocolo de ordenación. Cada validador sella las transacciones que le pertenecen en lotes firmados de su propio carril; los validadores con dos tercios de la participación confirman haberlo almacenado, lo que certifica su disponibilidad; el líder propone un bloque nombrando lotes certificados. El formato del bloque, su hash, su raíz de transacciones, los bytes firmados, la regla de voto y la regla de confirmación de dos cadenas no cambian.

En una máquina con enlaces emulados de 100 Mbit/s y 25 ms por validador, Lanes confirma 60.080, 45.323 y 37.099 tx/s con 4, 7 y 10 validadores, frente a 21.073, 10.909 y 8.297 tx/s cuando el mismo nodo difunde bloques completos: 2,9×, 4,2× y 4,5×, una ventaja que crece con `n`. Por debajo de la capacidad (4.000 tx/s), Lanes añade 35 ms de latencia mediana con 4 validadores y quita 25 ms y 130 ms con 7 y 10. Sin límite de ancho de banda, Lanes pierde un 4 % de rendimiento. La evaluación reveló nueve defectos, entre ellos un calendario de líderes en el que solo proponían los validadores de índice par cuando su número era par.

## 1. Pregunta de investigación

¿Puede añadirse difusión de lotes certificados bajo un protocolo HotStuff ponderado existente, sin cambiar sobre qué votan los validadores ni cómo se confirman los bloques, de modo que el rendimiento escale con la suma de los enlaces de subida de los validadores y no con el del líder? Las subpreguntas tratan el cuello de botella del líder (RQ1), el escalado (RQ2), la preservación de la seguridad, el orden de nonces y la resistencia a la censura (RQ3) y el coste cuando sobra ancho de banda (RQ4). Hipótesis: H1, la subida de la línea base por transacción crece linealmente con `n` y la de Lanes queda cerca del tamaño de la transacción; H2, la aceleración crece con `n`; H3, sin límite de enlace Lanes no es más rápido y cuesta como mucho un viaje de ida y vuelta.

## 2. Trabajo previo y límite de novedad

Separar difusión y ordenación es una idea establecida: Narwhal y Tusk, Bullshark y Shoal, Stratus, Mir-BFT e ISS, Autobahn y el Quorum Store de Aptos. Este manuscrito no reclama como nuevos esa separación, los certificados de disponibilidad, los carriles por validador ni resultados en una red amplia real. La contribución es de ingeniería: una adaptación que conserva bit a bit el bloque, la cabecera, la firma y la regla de confirmación; la propiedad de carriles por remitente con un respaldo contra la censura; la autenticación de la propuesta antes de cualquier descarga; y un banco de medición reproducible con los defectos que reveló.

## 3. Modelo del sistema

`n` validadores con poder de voto ponderado; los validadores bizantinos tienen menos de un tercio. Los quórums son `>= 2/3` del poder. Sincronía parcial, canales autenticados y firmas separadas por dominio. Los validadores pueden caerse, equivocar, retener lotes, inundar su carril o enviar propuestas falsas.

## 4. Protocolo

**Carriles y certificados.** Cada validador sella sus transacciones en un `lanes.Batch` al llegar a 512 KiB o 5 ms después de la primera, limitado por `lanes.MaxBatchTxs` y `lanes.MaxBatchBytes`. Un carril con poca carga, cuyo lote anterior tenía menos de 16 KiB, espera además el certificado de ese lote, como mucho 50 ms. Los validadores confirman en el dominio `DomainLaneBatchAck`; las confirmaciones de `>= 2/3` del poder forman un `BatchCert`, así que al menos un validador honesto guarda el lote. Las retransmisiones usan espera exponencial.

**Propiedad y respaldo.** Una transacción pertenece al carril `lanes.Owner(members, sender)`. Todas las transacciones de un remitente viajan por un carril en orden de admisión, por lo que los carriles nunca reordenan una secuencia de nonces. Si ningún lote almacenado lleva una transacción ajena tras `lanes.FallbackDelay` (3 s), quien la guarda la pone en su propio lote; un propietario defectuoso puede retrasar pero no censurar.

**Propuestas por carriles.** El líder toma solo lotes certificados, por carril en orden hasta el primer lote no certificado, alternando carriles hasta el límite del bloque, firma el bloque como antes y difunde un `lane_proposal` con los identificadores de lotes, las posiciones rechazadas (`Excluded`) y las transacciones fuera de lote (`Extra`).

**Autenticación antes de descargar.** El receptor comprueba el líder y la firma antes de descargar nada. Si el conjunto de validadores depende de un bloque aún no ejecutado, la propuesta se guarda (como mucho 16) y se juzga de nuevo tras la siguiente ejecución. El bloque se reconstruye con los lotes almacenados y solo se juzga si su hash y su raíz coinciden con la cabecera firmada; si no, el validador pide el `proposal` completo con `proposal_request`.

**Calendario proporcional a la participación.** El líder de la vista `v` es `validator.SelectProposer(validators, v)`: los validadores poseen `VotingPower` posiciones consecutivas de `[0, W)`, la vista `v` cae en la posición `v·S mod W`, con un paso coprimo con `W` a partir de `ceil(W·(φ−1))`. Cada ventana de `W` vistas da a cada validador exactamente su parte. El calendario depende solo de la vista.

**Límites de recursos.** 64 MiB por autor en un almacén de 256 MiB; caducidad a los 30 s; como mucho dos envíos por lote y par; como mucho `consensus.MaxLaneRefs` lotes por propuesta; una codificación binaria versionada.

## 5. Argumento de corrección

Seguridad: las reglas de voto y confirmación no cambian, y un validador solo vota un bloque cuyo hash reconstruido coincide con la cabecera firmada; los carriles pueden retrasar un voto pero no cambiar su objeto. Disponibilidad: cada lote nombrado lo guarda un validador honesto, y en último caso se descarga la propuesta completa. Vivacidad: tras la estabilización los lotes honestos se certifican y se proponen; un validador rezagado juzga las propuestas justo después de ejecutar. Es un argumento, no una prueba verificada por máquina.

## 6. Método experimental

Tratamientos: Lanes (nodo por defecto) frente a la línea base con `DisseminateByProposal`, en la que el líder difunde bloques completos; todo lo demás se comparte. Todos los validadores corren en un proceso en un Apple M4; cada subida pasa por un enlace virtual de tasa y latencia fijas, con los mensajes de consenso antes del tráfico masivo y sin pérdida ni fluctuación. Los clientes envían transacciones de 89 bytes; las pruebas de rendimiento envían 150.000 de golpe y las de latencia 20.000 en lazo abierto a 4.000 tx/s (`VEXO_RATE`). Las celdas de rendimiento con 10 validadores usan un tiempo límite de 5 s en ambos tratamientos, porque la línea base se detiene tras la altura 6 con el valor por defecto. Cada celda se midió tres veces.

## 7. Resultados

| Validadores | Lanes tx/s | Línea base tx/s | Factor | Lanes B/tx | Línea base B/tx |
|---|---|---|---|---|---|
| 4 | 60,080 (58,287–61,676) | 21,073 (20,955–21,235) | 2.85× | 81.8–96.1 | 334.6–335.6 |
| 7 | 45,323 (44,308–46,715) | 10,909 (9,394–11,685) | 4.15× | 123.4–217.6 | 660.0–725.5 |
| 10 | 37,099 (34,913–39,378) | 8,297 (8,241–8,330) | 4.47× | 135.3–146.0 | 291.3–325.9 |

Todas las pruebas confirmaron las 150.000 transacciones. Solo por las propuestas, la línea base paga `s·(n−1)` bytes por transacción; Lanes, 0,2–0,4 B/tx. Esto respalda H1 y H2.

| Validadores | Lanes p50 / p99 | Línea base p50 / p99 | Propuesta Lanes B/tx | Propuesta línea base B/tx |
|---|---|---|---|---|
| 4 | 323–327 / 395–483 ms | 280–298 / 358–373 ms | 6.0–6.1 | 96.9–100.1 |
| 7 | 379–386 / 471–486 ms | 405–421 / 747–801 ms | 8.3–8.4 | 117.0–124.1 |
| 10 | 439–462 / 556–659 ms | 558–610 / 1,546–1,745 ms | 11.3–11.7 | 141.1–376.4 |

Con 4 validadores la certificación cuesta cerca de un viaje de ida y vuelta; desde 7 validadores el enlace del líder de la línea base ya retrasa las propuestas con esta carga, y con 10 solo Lanes siguió la carga ofrecida. H3 se cumple solo para conjuntos pequeños.

| Tratamiento | tx/s | Subida B/tx |
|---|---|---|
| Lanes | 110,033 (108,496–111,814) | 61.4 |
| Línea base | 114,522 (113,095–116,408) | 147.4–148.3 |

Sin límite de enlace, la línea base es un 4 % más rápida; aun así Lanes sube menos de la mitad de los bytes. La línea base está limitada por el enlace del líder y Lanes, en esta matriz, por la máquina compartida.

## 8. Defectos revelados por la evaluación

1. **Calendario por paridad.** `(height−1+round) mod n` hacía proponer solo a índices pares con `n` par; sustituido por el calendario proporcional (`node/leader_schedule_test.go`).
2. **Repetición de transacciones confirmadas.** El rumor tardío se readmitía (152.629 confirmaciones para 150.000 transacciones).
3. **Caducidad cuadrática de la caché de vistas.** Cada admisión recorría la caché entera; ahora la más antigua primero y con límite.
4. **Tormenta de retransmisiones.** 200 ms fijos en un enlace saturado; ahora espera exponencial.
5. **Evidencias duplicadas sin límite.** Ahora una vez por infracción en una ventana de 1.024.
6. **Peticiones de cuerpos de confirmación.** Ahora una petición, repetida al cabo de un segundo.
7. **Propuestas juzgadas solo al retransmitirse.** Una de cada cuatro pruebas con 10 validadores cayó a 2.280 tx/s; ahora se guardan y se juzgan tras la ejecución (`consensus/transport_reactor_test.go`).
8. **Coste por lote con poca carga.** Los lotes de dos transacciones saturaban la máquina; corregido con el ritmo de carriles.
9. **Métrica de latencia mal etiquetada.** Ahora latencia del envío a la confirmación por transacción.

## 9. Amenazas a la validez

- Red emulada en una máquina, sin pérdida ni fluctuación.
- Una CPU compartida limita a Lanes con `n` grande.
- La línea base necesita 5 s de tiempo límite con 10 validadores; una codificación más compacta reduciría la diferencia.
- La espera de 300 ms antes de pedir una propuesta completa es corta en enlaces saturados.
- Solo transferencias pequeñas; solo tres repeticiones (34,9k–39,4k tx/s con 10 validadores).
- Los certificados Ed25519 concatenan firmas; BLS los reduciría.

## 10. Reproducción

```bash
go test ./lanes ./consensus ./node ./validator ./mempool -count=1
VEXO_THROUGHPUT=150000 VEXO_VALIDATORS=4 VEXO_UPLINK_MBIT=100 VEXO_LATENCY_MS=25 \
  go test ./node -run TestConsensusThroughput -v -count=1 -timeout 20m
VEXO_DISSEMINATION=proposals VEXO_THROUGHPUT=150000 VEXO_VALIDATORS=4 VEXO_UPLINK_MBIT=100 VEXO_LATENCY_MS=25 \
  go test ./node -run TestConsensusThroughput -v -count=1 -timeout 20m
```

`VEXO_ROUND_TIMEOUT_MS` cambia el tiempo límite; sin `VEXO_UPLINK_MBIT` ni `VEXO_LATENCY_MS` el enlace no tiene límite. Toda cifra publicada debe registrar la revisión, la máquina, la versión de Go y las líneas de registro completas.

<!-- vexo-docs:technical-parity -->

## Anexo de paridad técnica

Estos nombres de implementación y experimento no se traducen:

- `lanes/batch.go`, `lanes/lanes.go`, `lanes/wire.go`, `VerifyBatch`, `VerifyCert`
- `consensus/lane_proposal.go`, `consensus/transport_reactor.go`, `consensus/transport_reactor_test.go`
- `validator/proposer.go`, `node/lanes.go`, `node/throughput_test.go`, `node/shaped_transport_test.go`, `node/leader_schedule_test.go`
- `lanes.Batch`, `lanes.MaxBatchTxs`, `lanes.MaxBatchBytes`, `lanes.Owner(members, sender)`, `lanes.FallbackDelay`
- `DomainLaneBatchAck`, `BatchCert`, `consensus.LaneProposal`, `consensus.MaxLaneRefs`, `Excluded`, `Extra`
- `lane_proposal`, `proposal_request`, `proposal`, `DisseminateByProposal`
- `validator.SelectProposer(validators, v)`, `VotingPower`, `[0, W)`, `v·S mod W`, `ceil(W·(φ−1))`, `0..W−1`, `W`, `S`, `v`
- `n`, `s`, `>= 2/3`, `1/(n−1)`, `s·(n−1)`, `(height−1+round) mod n`
- `VEXO_THROUGHPUT`, `VEXO_VALIDATORS`, `VEXO_UPLINK_MBIT`, `VEXO_LATENCY_MS`, `VEXO_DISSEMINATION`, `VEXO_ROUND_TIMEOUT_MS`, `VEXO_RATE`
- `VEXO_RATE=4000 VEXO_THROUGHPUT=20000`, `VEXO_VALIDATORS`, `VEXO_UPLINK_MBIT`, `VEXO_LATENCY_MS`, `VEXO_DISSEMINATION`, `VEXO_ROUND_TIMEOUT_MS`, `VEXO_RATE`
