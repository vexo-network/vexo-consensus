# Esquema de almacenamiento

> Locale: es · Español
> Este documento es una traducción directa al español de la fuente inglesa. Las decisiones de protocolo, seguridad y release siguen siendo normativas en inglés.


## Orden de lectura

Este documento explica la especificación normativa de Storage Schema. Si es tu primera lectura, sigue este orden.

1. Scope
2. Backend
3. Records
4. Indexes
5. EVM Records
6. Recovery Rules
7. Snapshot Validation
8. Schema Migration

Ese orden coincide con la forma correcta de leerlo: primero el alcance y el estado, luego las reglas de mensajes, seguridad y vivacidad, y al final las evidencias.

## Resumen

Este documento ayuda a entender namespace de durable storage, key schema y recovery marker y a conectarlo con decisiones de implementación y operación.

- Canonical path: `docs/specs/storage-schema.md`
- Locale path: `docs/locales/es/specs/storage-schema.md`

## Por qué leer este documento

- namespace de durable storage, key schema y recovery marker
- Revise primero las frases MUST/SHOULD/MAY en la fuente inglesa.
- Esta página es una traducción directa del original en inglés; la auditoría, el release y la seguridad se deciden con la fuente inglesa.

## Qué debería poder hacer después

- Explicar qué decisión de implementación u operación apoya este documento.
- Relacionar los requisitos normativos de la fuente inglesa con la configuración actual de la red.
- Verificar chain ID, validator ID, fee/gas y direcciones peer antes de copiar ejemplos.

## Checklist de uso seguro

- Revise primero las frases MUST/SHOULD/MAY en la fuente inglesa.
- No traduzca comandos, config key, nombres RPC, campos JSON ni identificadores de código.
- Antes de copiar ejemplos, adapte chain ID, validator ID, fee/gas y direcciones peer a su red.
- Después de modificar la documentación, ejecute `make docs-check` para verificar el árbol local y los controles de traducción.

## Puntos de atención

- Esta página es una traducción directa del original en inglés; la auditoría, el release y la seguridad se deciden con la fuente inglesa.
- Si cambia la implementación, actualice la fuente inglesa y todos los documentos localizados en el mismo cambio.

## Interfaces que deben conservarse

- `store.Store`
- `(height, namespace)`
- `bank`
- `events`
- `evm`
- `ibc`
- `params`
- `staking`
- `0x`
- `bank/{0x_address}`
- `auth/nonce/{0x_address}`
- `evm/code/{0x_address}`
- `evm/storage/{0x_address}/{slot}`
- `eth_getProof`
- `stateRoot`
- `EndBlock`
- `H + 1`
- `seen_ttl`
- `code/{address}`

## Estructura de la fuente inglesa

- Storage Schema
- Scope
- Backend
- Records
- Block Record
- State Record
- State Root Record
- Evidence Record
- KV Namespace
- Indexes
- EVM Records
- Recovery Rules
- Snapshot Validation
- Schema Migration

## Fuente canónica

- [Documento canónico en inglés](../../en/specs/storage-schema.md)

## Backend Pebble y registros binarios

El backend por defecto es Pebble (`github.com/cockroachdb/pebble/v2`), abierto con una caché de bloques de 256 MiB y memtables de 64 MiB. Los registros se guardan en la codificación binaria canónica del paquete `types`: uvarints mínimos, cadenas de bytes y listas con prefijo de longitud, hashes como sus 32 bytes, un valor opcional tras un byte de presencia `0` o `1`, y los campos en el orden de la estructura que los declara. Un decodificador rechaza cualquier otra cosa, incluidos bytes sobrantes, así que un registro dañado se informa en lugar de leerse como otro valor; cada registro tiene una sola codificación. Un directorio de datos escrito por un build anterior (LevelDB, registros JSON) no es legible ni se convierte: Pebble se niega a abrirlo con un error de `format major version 1`. Restaure ese nodo desde un snapshot o sincronícelo en un directorio de datos vacío.

Identificadores relacionados:

- `pebble: database "<dir>" was written in format major version 1, which is no longer supported`

## Historial de estado (undo log) y caché del árbol

El historial de estado es un undo log de versiones por clave. Cuando una clave cambia en una altura (las escrituras de un bloque en su altura, una escritura directa (`Set`, `Delete`, `SetBatch`) tras el primer estado confirmado en la siguiente altura confirmada), el store registra el valor que la clave tenía antes, ausencia incluida, como versión de la clave en esa altura, una vez por clave y altura; las escrituras directas anteriores al bloque que confirma su altura conservan la versión que registró la primera (`hdirect` indica esa altura). El valor de una clave en una altura confirmada es el que registra su versión más baja por encima de esa altura, o su valor actual si no tiene ninguna. Registrar un cambio solo lee el valor actual de la clave, nunca su historial, y una lectura histórica es un seek en las versiones de la clave. `PruneBelow` borra toda versión en o por debajo de la altura desde la que retiene. Los nodos del árbol se versionan igual. Dos estructuras en memoria reducen el trabajo de árbol de un bloque; ninguna se persiste ni cambia un resultado: la caché de nodos del árbol, con hasta 2^18 nodos codificados de los árboles actuales (cada batch del store la actualiza, y una generación impar mientras se confirma tal batch hace que un lector use la caché solo en el estado de su snapshot), y, por namespace, la raíz y los cambios de nodos del último cálculo de raíz, indexados por esa generación y un digest SHA-256 de las escrituras del namespace, de modo que el app hash, las state roots y el commit calculan las mismas escrituras una sola vez.

## Compactación del WAL del mempool y batch de genesis

Un commit compacta el WAL del mempool solo cuando contiene al menos 4 MiB y el doble de lo que contenía tras la última compactación o apertura. Así cada byte añadido se reescribe en promedio una vez y el log que reproduce un reinicio queda dentro del doble del contenido vivo o de 4 MiB. Una compactación explícita (`Node.Compact`, o un arranque que descarta transacciones reproducidas que la aplicación rechaza) siempre lo reescribe. `InitChain` escribe el estado de genesis de todos los módulos como un solo batch, de modo que un genesis con muchas cuentas cuesta una escritura durable y un genesis que falla no deja nada en el store.

## Finality proofs en el batch del commit y eliminación diferida del mempool

La ruta de commit del nodo escribe las finality proofs que completa el QC del bloque en el mismo batch de block/state/app-write (`CommitBlockStateWithWrites` las recibe como último argumento). Así un commit cuesta una escritura durable en lugar de una más por proof. Una proof que el store rechazaría queda fuera del batch y la guarda el paso posterior al commit, que informa del fallo. Un commit elimina sus transacciones del pool y encola el registro de eliminación sin esperar su sync. El siguiente commit espera a que ese registro sea durable antes de ejecutar su bloque, porque un reinicio repite la eliminación solo para el último bloque confirmado. Normalmente el registro comparte el sync de una admisión posterior.

## Hash diferido del state tree

El cálculo del state tree difiere el hash: un write marca las mitades que cambió, y cada node cambiado se hashea una sola vez, de abajo arriba, cuando se pide la root, una proof o los cambios. Así los nodes cercanos a la root, que reescribe cada write del bloque, se hashean una vez por bloque y no una por write. Los hashes, los nodes y las roots son los mismos que hasheando en cada write.

## Sync periódico del WAL del mempool

El WAL del mempool escribe cada cambio en el log y lo sincroniza cada 100 ms (`walSyncInterval`). Una admisión espera la escritura, no un sync. Una caída del proceso conserva cada registro; una caída de la máquina pierde como mucho las admisiones del último intervalo, que sus clientes vuelven a enviar. El registro de eliminación de un bloque confirmado se sincroniza antes del siguiente commit (`WaitDurable`).

## Reutilización de lecturas en el historial

Un commit registra el valor previo de una key cambiada como su versión, normalmente sin volver a leerlo: el store recuerda los valores que `Get` leyó desde el último commit de batch, hasta 2^16, y la ejecución de un bloque lee la mayoría de las keys que cambia. Un valor se usa solo si ningún batch se confirmó desde que se leyó, así que es el valor actual; una key ausente no se recuerda y se lee.

## Nodes del state tree versionados

Los nodes del tree se guardan por versión y ninguna altura posterior los reescribe. Un batch escribe cada node que cambia bajo `tree: | namespace | height | node key`, y un node interno guarda las versiones de sus dos mitades, de modo que cada node se encuentra desde su padre. Un root record (`troot:`) marca cada altura en que cambió el tree; el tree de cualquier altura se lee desde su último root record igual o menor. Un node reemplazado queda para las alturas menores y figura en la orphan list de la altura que lo reemplazó (`torph:`, una por batch), que `PruneBelow` elimina. Como las versiones solo crecen, las escrituras de un bloque no se dispersan entre las keys existentes, lo que reduce los flush y compactions de Pebble. El version index de las keys (`hidx:`) también tiene una entrada por batch.

Identificadores relacionados:

- `troot: | namespace | height`
- `torph: | height | first entry`
- `hidx: | height | first group`

## Nodes decodificados en la caché de nodes

La caché de nodes del tree guarda hasta 2^18 nodes de los trees actuales ya decodificados, con las versiones de sus mitades, así que una lectura no decodifica ni asigna memoria.

<!-- vexo-docs:technical-parity -->
## Apéndice de paridad técnica

Este apéndice asegura que la traducción conserve las interfaces ejecutables y las secciones clave del documento canónico en inglés. Los comandos, claves de configuración, métodos RPC y nombres de paquetes se mantienen sin cambios en todos los idiomas.

### Seguimiento de secciones
- section: Scope — Esta sección debe revisarse junto con valores de configuración, evidencia de verificación, condiciones de fallo y acciones del operador.
- section: Backend — Esta sección debe revisarse junto con valores de configuración, evidencia de verificación, condiciones de fallo y acciones del operador.
- section: Records — Esta sección debe revisarse junto con valores de configuración, evidencia de verificación, condiciones de fallo y acciones del operador.
- section: Indexes — Esta sección debe revisarse junto con valores de configuración, evidencia de verificación, condiciones de fallo y acciones del operador.
- section: EVM Records — Esta sección debe revisarse junto con valores de configuración, evidencia de verificación, condiciones de fallo y acciones del operador.
- section: Recovery Rules — Esta sección debe revisarse junto con valores de configuración, evidencia de verificación, condiciones de fallo y acciones del operador.
- section: Snapshot Validation — Esta sección debe revisarse junto con valores de configuración, evidencia de verificación, condiciones de fallo y acciones del operador.
- section: Schema Migration — Esta sección debe revisarse junto con valores de configuración, evidencia de verificación, condiciones de fallo y acciones del operador.

### Interfaces conservadas sin cambios
- `store.Store` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `eth_getBalance` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `eth_getProof` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `bank/{0x_address}` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `auth/nonce/{0x_address}` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `evm/code/{0x_address}` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `evm/storage/{0x_address}/{slot}` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `seen_ttl` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `code/{address}` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `storage/{address}/{slot}` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `receipts/{tx_hash}` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `logs/by_height/{height}/{tx_hash}/{log_index}` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `logs/by_address/{address}/{height}/{tx_hash}/{log_index}` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `logs/{address}` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `evm_receipts` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `receipt_index/{tx_hash}` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `blob_sidecars/by_tx/{tx_hash}` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `blob_sidecars/by_blob/{blob_hash}` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `tree:` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `receipt_height/{height}/{tx_hash}` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `receipt_pruned_below` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
