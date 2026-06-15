> Locale: es · Español

# Inicialización del nodo

Esta guía explica cómo inicializar los hogares de nodos de archivado y validación, iniciarlos, verificar que estén en buen estado y conectar clientes.

La conectividad entre pares debe configurarse en `network_config.json`, no pasarse repetidamente en la línea de comando `start`.

El comportamiento en tiempo de ejecución que afecta el consenso, RPC, P2P, el registro o las cuentas Web3 administradas es solo el archivo de configuración. `vexod start` rechaza indicadores como `--timeout-propose`, `--create-empty-blocks`, `--p2p-auth-token`, `--rpc-admin-token`, `--evm-account-key-env` y `--evm-account-key`; En su lugar, edite los archivos de configuración divididos para que cada operador revise el mismo comportamiento determinista del nodo.

No hay ningún cambio de modo de nodo. El inicio de un nodo se define por sus archivos de configuración, génesis, material de claves y si `validator_id` más un firmante están presentes.

## Lo que estás construyendo

El inicio de un nodo Vexo es un directorio que contiene lo necesario para que un nodo pueda iniciar:
```text
.vexo-validator-1/
  config.json             # chain ID, validator ID, data dir, split config paths
  module_config.json      # app modules, signed tx policy, fees, gas, EVM chain ID
  network_config.json     # RPC, Web3, P2P, peers, state sync, peer scoring
  consensus_config.json   # consensus timings, finality execution policy, empty blocks
  mempool_config.json     # tx queue, fee filters, replacement, WAL
  log_config.json         # structured logs, block commit logs, peer logs
  genesis.json            # initial validators and genesis app state
  validator.key.json      # validator consensus signer, validator nodes only
  node.key.json           # P2P identity signer, validators and archives
  validator.vrf.key.json  # VRF key for committee randomness when enabled
  data/                   # LevelDB chain/app/evidence/snapshot state
```
La regla importante es simple: inicialice una vez, edite los archivos de configuración y luego comience. No oculte el comportamiento de la red dentro de los indicadores del shell.

## Carrera local de cinco minutos

Utilice este flujo cuando desee demostrar que el binario funciona antes de pensar en la implementación de múltiples hosts.
```bash
make build
export VEXO_KEY_PASSPHRASE='change-me'

./bin/vexod init validator \
  --home .vexo-validator-1 \
  --chain-id vexo-chain \
  --validator validator-1 \
  --encrypt-keys \
  --overwrite

./bin/vexod validate --home .vexo-validator-1
./bin/vexod config audit --home .vexo-validator-1 --strict
./bin/vexod start --home .vexo-validator-1
```
En otra terminal:
```bash
curl -s http://127.0.0.1:26657/v1/status
curl -s http://127.0.0.1:26657/v1/diagnostics
curl -s http://127.0.0.1:26657/v1/metrics
```
Forma de estado esperada:
```json
{
  "chain_id": "vexo-chain",
  "running": true,
  "latest_height": 0,
  "peer_count": 0,
  "banned_peers": 0
}
```
La última altura puede permanecer en cero en una ejecución de un solo nodo o de un mempool vacío cuando la creación de bloques vacíos está deshabilitada. Eso no significa que el proceso esté interrumpido. Significa que el nodo no produce bloques vacíos. Agregue transacciones o ejecute una red de prueba de validadores múltiples para observar confirmaciones continuas.

## Red local de cuatro validadores

Utilice este flujo cuando desee conectividad entre pares, rotación de proponentes, registros de confirmación de bloques y crecimiento en altura.
```bash
make build

./bin/vexod network init \
  --home .vexo-network \
  --chain-id vexo-chain \
  --validators 4 \
  --overwrite

./bin/vexod network up \
  --home .vexo-network \
  --validators 4 \
  --keep-running
```
Comprobaciones útiles:
```bash
curl -s http://127.0.0.1:26657/v1/status
curl -s http://127.0.0.1:26667/v1/status
curl -s http://127.0.0.1:26677/v1/status
curl -s http://127.0.0.1:26687/v1/status
```
Si el registro de confirmación de bloque está habilitado en `log_config.json`, los registros del validador incluyen eventos como:
```json
{"event":"block_committed","height":12,"round":0,"tx_count":0}
```
Detenga la red local generada con:
```bash
./bin/vexod network stop --home .vexo-network --validators 4
```
## Web3 y Remezcla

JSON-RPC estilo Ethereum se encuentra en el punto final Web3, no en el espacio de nombres API operativo versionado de Vexo.

Para el validador 1 de host único de Docker, la URL del proveedor personalizado de Remix es:
```text
http://127.0.0.1:28657/web3
```
Para un nodo local directo con el puerto RPC predeterminado:
```text
http://127.0.0.1:26657/web3
```
Pruebe la misma llamada que hace Remix:
```bash
curl -s http://127.0.0.1:26657/web3 \
  -H 'content-type: application/json' \
  -d '{"jsonrpc":"2.0","id":1,"method":"eth_chainId","params":[]}'
```
Si un navegador dice que falló la recuperación del ID de la cadena, verifique esto en orden:

1. La URL termina con la ruta del punto final Web3.
2. El navegador puede llegar al puerto del host. Los ejemplos de Docker exponen `28657`, `28667`, `28677` y `28687`; Dentro del contenedor, el puerto RPC sigue siendo `26657`.
3. El servidor RPC se está ejecutando; consultar el punto final de estado en el mismo host y puerto.
4. CORS está permitido por la configuración `network_config.json`/RPC. El controlador predeterminado permite la verificación previa del navegador cuando no se establece una lista CORS personalizada.
5. La cadena tiene un ID de cadena EVM distinto de cero en `module_config.json`.

## Nodo validador

Utilice `init validator` cuando el nodo proponga, vote, firme mensajes de consenso y participe en la rotación de validadores.
```bash
vexod init validator \
  --home .vexo-validator-1 \
  --chain-id vexo-chain \
  --validator validator-1 \
  --encrypt-keys
```
Configure `VEXO_KEY_PASSPHRASE` antes de ejecutar este comando, o pase `--passphrase` para una configuración local única. `--encrypt-keys` cifra `validator.key.json`, `node.key.json` y `validator.vrf.key.json`.

Regla general de custodia de claves:

- `validator.key.json` firma propuestas de consenso, votaciones, votaciones de tiempo de espera y mensajes relacionados con la finalidad.
- `node.key.json` firma únicamente apretones de manos P2P; nunca debe reutilizarse como clave de consenso del validador.
- `validator.vrf.key.json` demuestra la aleatoriedad del comité y debe tratarse como material de custodia del validador.
- Los oyentes públicos deben utilizar documentos de clave locales cifrados o documentos de clave de estilo KMS/firmante remoto. Si un nodo expone RPC público o P2P público autenticado mientras `require_network_safety=true`, el inicio rechaza las claves de validación local de texto sin formato.
- Las claves generadas se escriben con el modo de sistema de archivos `0600`; Todavía prefiero un firmante remoto/KMS para validadores de larga duración.

Para una clave de consenso BLS:
```bash
vexod init validator \
  --home .vexo-validator-1 \
  --chain-id vexo-chain \
  --validator validator-1 \
  --key-type bls \
  --encrypt-keys
```
`--key-type bls` escribe un documento clave BLS `blst-bls12381-minpk-v1` y copia la prueba de posesión en los metadatos del validador `genesis.json` como `bls_pop`.

Esto crea:

- `config.json`
- `module_config.json`
- `network_config.json`
- `consensus_config.json`
- `mempool_config.json`
- `log_config.json`
- `genesis.json`
- `validator.key.json`
- `node.key.json`
- `validator.vrf.key.json`
- `data/`

`validator.key.json` es el firmante del consenso. `node.key.json` es el firmante del protocolo de enlace P2P al que hace referencia `network_config.json:p2p.node_key_path`. Están deliberadamente separados para que los nodos de archivo y los validadores puedan usar el mismo transporte sin darle a cada par una clave de firma del validador.

Inícielo con redes basadas en configuración:
```bash
vexod start --home .vexo-validator-1
```
Después del inicio, lea los registros. Un validador en buen estado debe emitir eventos de ejecución de nodos, escucha RPC, escucha P2P y, una vez que se confirman los bloques, eventos de confirmación de bloques. Si la creación de bloques vacíos está deshabilitada, la falta de registros confirmados de bloques puede significar simplemente que no hay transacciones.

## Nodo de archivo

Utilice `init archive` cuando el nodo deba conservar datos en cadena, exponer RPC, sincronizar desde pares y evitar la firma del validador.
```bash
vexod init archive \
  --home .vexo-archive-1 \
  --chain-id vexo-chain \
  --bootstrap-peer validator-1=seed-1.example.com:26656
```
Esto crea:

- `config.json`
- `module_config.json`
- `network_config.json`
- `consensus_config.json`
- `mempool_config.json`
- `log_config.json`
- `genesis.json`
- `node.key.json`
- `data/`

**No** crea `validator.key.json`.

Empiece con:
```bash
vexod start --home .vexo-archive-1
```
Los nodos de archivo no firman votos de consenso. Son útiles para RPC, indexación, sincronización de estado, entrega de pruebas históricas y para mantener un historial de consultas más amplio que los validadores de poda.

## Dividir archivos de configuración

Los hogares de nodos usan archivos de configuración separados para que los operadores puedan editar un subsistema sin mezclar configuraciones no relacionadas:

- `config.json` contiene la identidad del nodo, el ID de la cadena, la ruta de los datos y punteros a los archivos de configuración divididos.
- `module_config.json` contiene la selección del módulo de aplicación, la política de ejecución/ante y la política de gobernanza a nivel de módulo.
- `network_config.json` contiene RPC, identidad del nodo P2P, configuración de escucha/par/semilla, configuración de TLS/autenticación y política de puntuación de pares.
- `consensus_config.json` contiene sincronización de bucle de consenso, política de bloque vacío, backend criptográfico, VRF, admisión de validador y política de comité.
- `mempool_config.json` contiene tamaño de mempool, tarifa, prioridad, WAL, duplicado y política TTL.
- `log_config.json` contiene formato de registro, nivel, registro de eventos de confirmación de bloque y registro de eventos de pares.
- `genesis.json` contiene validadores de génesis inmutables, metadatos del validador y estado del módulo de génesis.

`network_config.json` La configuración de RPC también incluye `shutdown_timeout`, `web3_max_subscriptions_per_connection` y `web3_idle_timeout`. `shutdown_timeout` limita el cierre elegante para el bucle de consenso, el servidor RPC y el transporte de nodos para que los operadores no esperen eternamente en una ruta de parada atascada. El valor predeterminado generado es `10s`; Las suscripciones Web3 tienen un valor predeterminado de 256 por conexión con un tiempo de espera de inactividad `2m` para que los puntos finales RPC públicos no puedan acumular suscripciones inactivas ilimitadas.

`network_config.json` Las configuraciones P2P incluyen `auth_replay_path`, `require_auth_replay_store` y `dial_timeout`. El valor predeterminado generado escribe evidencia de reproducción nonce en `data/p2p_auth_replay.jsonl` y utiliza un tiempo de espera de marcado saliente `10s`. Para las pruebas de loopback privado, la tienda de reproducción es en su mayor parte una contabilidad inofensiva; para P2P autenticado públicamente, es un requisito de seguridad porque evita que un protocolo de enlace firmado y capturado se reproduzca después del reinicio. `dial_timeout` debería ser lo suficientemente largo para TLS, verificación de protocolo de enlace firmado y latencia entre regiones; establecerlo demasiado bajo hace que los compañeros sanos parezcan inestables y puede ralentizar la vitalidad después de los reinicios.

`network_config.json` también posee la sincronización del estado de inicio. Esto es útil para nodos de archivo, validadores de reemplazo o nodos restaurados en una máquina limpia. Cuando `state_sync.enabled` es verdadero, `vexod start` descarga la primera instantánea válida de `state_sync.snapshot_urls`, verifica el ID de la cadena, la suma de comprobación, las raíces del estado y los espacios de nombres KV, la restaura en LevelDB, reconstruye los índices y solo entonces inicia el nodo. Si el estado local ya satisface `state_sync.min_height` y `state_sync.trust_local_higher` es verdadero, el inicio registra `state_sync_skipped` y mantiene el almacén local.

Ejemplo de bloque `state_sync`:
```json
{
  "state_sync": {
    "enabled": true,
    "snapshot_urls": ["https://snapshots.example.com/vexo-chain/latest.json"],
    "timeout": "30s",
    "min_height": 1000000,
    "require_fresh": true,
    "trust_local_higher": true,
    "max_snapshot_bytes": 268435456,
    "retry_all_snapshots": true
  }
}
```
El inicio registra `state_sync_candidate_failed` para un error de recuperación, `state_sync_candidate_rejected` para una instantánea no válida o obsoleta y `state_sync_applied` después de una restauración verificada. Mantenga `max_snapshot_bytes` por debajo de la instantánea más grande que su infraestructura brinda intencionalmente, pero lo suficientemente alta para el crecimiento normal del estado. No apunte nodos públicos a una fuente de instantáneas de terceros no autenticada a menos que el operador tenga una política de confianza fuera de banda y evidencia de finalidad/cliente ligero para esa fuente.

Si un campo cambia el comportamiento de la red, edite el archivo de configuración dividido y confirme o distribuya ese archivo revisado. No confíe en indicadores `vexod start` largos para el comportamiento en tiempo de ejecución. El comando de inicio rechaza intencionalmente el tiempo de consenso, el bloque vacío, la autenticación P2P, el administrador de RPC y los indicadores de clave Web3 administrada para que los operadores no ejecuten accidentalmente un comportamiento diferente al de la configuración revisada.

## ¿Qué archivo edito?

| Gol | Archivo | Campo |
|---|---|---|
| Cambiar el puerto de enlace RPC | `network_config.json` | `rpc.address` |
| Cambiar el puerto de enlace P2P | `network_config.json` | `p2p.listen_address` |
| Agregar pares persistentes | `network_config.json` | `p2p.peers` |
| Agregar pares semilla | `network_config.json` | `p2p.seeds` |
| Activar/desactivar bloques vacíos | `consensus_config.json` | campo de bloque vacío de consenso |
| Ajustar los tiempos de espera de consenso | `consensus_config.json` | campos de propuesta, prevoto, precompromiso y tiempo de espera de confirmación |
| Requerir ejecución finalizada | `consensus_config.json` | campo de compromiso de ejecución de consenso |
| Activar/desactivar módulos | `module_config.json` | lista de módulos de aplicación |
| Cambiar ID de cadena EVM | `module_config.json` | campo de ID de cadena EVM de ejecución |
| Sintonizar tarifa base/gas | `module_config.json` | ejecución de campos con tarifa base, tarifa dinámica, gas objetivo y gas máximo |
| Configurar mempool WAL | `mempool_config.json` | ruta WAL de mempool |
| Registros de confirmación del bloque de control | `log_config.json` | registrar el campo de eventos de confirmación |
| Controlar registros de pares | `log_config.json` | registrar campo de eventos de pares |

En caso de duda, ejecute:
```bash
vexod config paths --home .vexo-validator-1
vexod config show --home .vexo-validator-1
vexod doctor --home .vexo-validator-1
```
## Tipos de claves

El inicio del validador tiene como valor predeterminado `--key-type bls` porque la validación de seguridad de la red requiere la finalidad agregada de BLS auditada. `--key-type ed25519` permanece disponible para experimentos privados e implementaciones personalizadas fuera de la puerta de seguridad de la red. `--encrypt-keys` debe usarse para cualquier nodo principal no desechable. La generación de claves independiente también admite claves VRF:
```bash
vexod keys gen --home .vexo-ed25519 --type ed25519
vexod keys gen --home .vexo-bls --type bls
vexod keys gen --home .vexo-bls-circl --type bls --bls-adapter circl-bls12381-g1sigg2-basic-v1
VEXO_KEY_PASSPHRASE='change-me' vexod keys gen --home .vexo-vrf --type vrf --encrypt
```
Las claves VRF no son firmantes de consenso. Se utilizan para la selección de comités respaldados por VRF y se debe hacer referencia a ellos desde `consensus_config.json` hasta `vrf_key_paths` más la clave de metadatos del validador `vrf_public_key` cuando ese backend está habilitado.

`config.json` apunta a los archivos de configuración divididos:
```json
{
  "schema_version": "v1",
  "chain_id": "vexo-chain",
  "module_config_path": "module_config.json",
  "network_config_path": "network_config.json",
  "consensus_config_path": "consensus_config.json",
  "mempool_config_path": "mempool_config.json",
  "log_config_path": "log_config.json"
}
```
Cada ruta puede ser absoluta o relativa al inicio del nodo. Si se omite, `vexod` usa el archivo `<home>/<name>_config.json` predeterminado.

Ejemplo `module_config.json`:
```json
{
  "schema_version": "v1",
  "application": {
    "Modules": ["bank", "staking", "governance", "params", "ibc"]
  },
  "execution": {
    "RequireSigned": true,
    "RequireNonce": true,
    "MinFee": 1,
    "BaseFee": 1,
    "EVMChainID": 83960,
    "DynamicBaseFee": true,
    "TargetGas": 5000000,
    "BaseFeeChangeDenominator": 8,
    "MinBaseFee": 1,
    "MaxBaseFee": 0,
    "MinGas": 1,
    "MaxGas": 10000000,
    "FeeCollector": "fee_collector",
    "FeeDenom": "avxo",
    "DisplayDenom": "vexo",
    "DisplayExponent": 18,
    "GasDenom": "gas"
  },
  "bank": {
    "MintAuthority": "governance"
  },
  "staking": {
    "UnbondingDelay": 1209600,
    "MaxCommissionBPS": 10000
  },
  "governance": {
    "QuorumPower": 1,
    "YesThresholdPower": 1,
    "VetoPower": 1,
    "VotingPeriod": 10,
    "Timelock": 10
  }
}
```
La política de gobernanza también reside en `module_config.json`. Las configuraciones seguras para la red generadas requieren un depósito de propuesta:
```json
{
  "governance": {
    "QuorumPower": 1,
    "YesThresholdPower": 1,
    "VotingPeriod": 100,
    "Timelock": 10,
    "RequireDeposit": true,
    "MinDeposit": "1avxo",
    "DepositDenom": "avxo",
    "DepositEscrow": "module:governance:deposit_escrow",
    "RejectedDeposits": "module:governance:rejected_deposits"
  }
}
```
El depósito es un saldo nativo depositado en custodia del remitente de la propuesta. Las propuestas aprobadas reembolsan el depósito; las propuestas rechazadas lo mueven a `RejectedDeposits`. Utilice una dirección controlada por su módulo de tesorería/grupo comunitario si los depósitos rechazados deben financiar una tesorería en lugar de la cuenta del módulo predeterminado.

Ejemplo `network_config.json`:
```json
{
  "schema_version": "v1",
  "rpc": {
    "enabled": true,
    "address": "0.0.0.0:26657",
    "evm_account_key_envs": [],
    "evm_account_private_keys": []
  },
  "p2p": {
    "enabled": true,
    "node_id": "validator-1",
    "node_key_path": "node.key.json",
    "listen_address": "0.0.0.0:26656",
    "dial_timeout": "10s",
    "auth_replay_path": "data/p2p_auth_replay.jsonl",
    "require_auth_replay_store": true,
    "tls_cert_path": "tls/node.crt",
    "tls_key_path": "tls/node.key",
    "tls_ca_path": "tls/ca.crt",
    "tls_server_name": "validator.example.com",
    "peers": {
      "validator-1": "seed-1.example.com:26656"
    }
  },
  "peer_scoring": {
    "InitialScore": 100,
    "MaxScore": 1000,
    "BanThreshold": 0
  }
}
```
`rpc.evm_account_key_envs` y `rpc.evm_account_private_keys` son métodos de cuenta administrada Web3 opcionales y posteriores, como `eth_accounts`, `eth_sign`, `eth_signTransaction` y `eth_sendTransaction`. Prefiera `evm_account_key_envs` para que el entorno de proceso o el administrador secreto inyecte la clave privada en lugar de almacenarla en JSON. Mantenga ambas listas vacías para el funcionamiento normal del validador, a menos que este nodo actúe intencionalmente como un punto final local de billetera activa Web3. La seguridad del inicio rechaza las teclas de acceso rápido de EVM administradas en los oyentes RPC públicos.

Ejemplo `consensus_config.json`:
```json
{
  "schema_version": "v1",
  "consensus": {
    "loop_enabled": true,
    "interval": "50ms",
    "timeout_propose": "3s",
    "timeout_prevote": "1s",
    "timeout_precommit": "1s",
    "timeout_commit": "1s",
    "max_block_bytes": 1048576,
    "create_empty_blocks": false,
    "execution_commit": "finalized"
  },
  "vrf_key_paths": ["validator.vrf.key.json"]
}
```
`vrf_key_paths` se resuelven en relación con el directorio que contiene `consensus_config.json`. Utilice documentos de clave cifrados y proporcione `VEXO_KEY_PASSPHRASE` al proceso del nodo cuando la custodia de la clave VRF local sea inevitable. No coloque escalares privados VRF sin procesar directamente en `consensus_config.json` para redes administradas por operadores.

Utilice `vexod config paths --home <home>` para inspeccionar todas las rutas resueltas.

La configuración del archivo tiene:
```json
{
  "schema_version": "v1",
  "validator_id": "",
  "chain_id": "vexo-chain",
  "consensus_config_path": "consensus_config.json"
}
```
Archivar `consensus_config.json` desactiva el bucle de consenso local:
```json
{
  "schema_version": "v1",
  "consensus": {
    "loop_enabled": false
  }
}
```
Las casas de validación generadas se establecen en `"require_network_safety": true` en `config.json` de forma predeterminada. Este no es un modo; es una puerta de seguridad para startups que rechaza criptomonedas deterministas, transacciones no firmadas/no canceladas, falta de tarifas/gas mínimos, falta de WAL de mempool duradero, falta de política de reemplazo para transacciones del mismo firmante/nonce, aleatoriedad de comité inseguro y valores `execution_commit` distintos de `finalized`.

Cuando `require_network_safety` esté habilitado, ejecute:
```bash
vexod config audit --home <home> --strict
```
antes de iniciar el nodo. La auditoría debe pasar por cada validador y hogar de archivo que participe en la misma red.

## Pares basados en configuración

Direcciones de peer y escucha en vivo en `network_config.json`:
```json
{
  "p2p": {
    "enabled": true,
    "node_id": "validator-1",
    "node_key_path": "node.key.json",
    "listen_address": "0.0.0.0:26656",
    "dial_timeout": "10s",
    "auth_replay_path": "data/p2p_auth_replay.jsonl",
    "require_auth_replay_store": true,
    "tls_cert_path": "tls/node.crt",
    "tls_key_path": "tls/node.key",
    "tls_ca_path": "tls/ca.crt",
    "tls_server_name": "validator.example.com",
    "peers": {
      "validator-1": "seed-1.example.com:26656",
      "validator-2": "seed-2.example.com:26656"
    },
    "seeds": {
      "seed-1": "seed-1.example.com:26656"
    }
  }
}
```
`vexod start` carga estos pares automáticamente:
```bash
vexod start --home .vexo-archive-1
```
Los pares y semillas persistentes se configuran en `network_config.json`; `vexod start` no acepta anulaciones de host inicial o de pares.

No coloque la configuración de host de larga duración o `host:port` en la línea de comando `vexod start`. Edite `rpc.address`, `p2p.listen_address`, `p2p.peers` y `p2p.seeds` en `network_config.json` en su lugar.

Mantenga `p2p.node_id` estable durante la vida útil del nodo principal. `p2p.node_key_path` debe apuntar a `node.key.json` u otro documento de clave local/administrado utilizado solo para la firma del protocolo de enlace entre pares. Los mapas de pares deben usar ID de nodos de pares, no direcciones de cuentas ni nombres de operadores de validación, a menos que sean intencionalmente iguales.

Para el transporte entre pares gRPC cifrado y autenticado, configure también `p2p.tls_cert_path`, `p2p.tls_key_path`, `p2p.tls_ca_path` y, opcionalmente, `p2p.tls_server_name` en `network_config.json`. Las rutas TLS relativas se resuelven desde el directorio de inicio del nodo. Mantenga `p2p.dial_timeout` en el mismo archivo para que todos los operadores utilicen el mismo comportamiento de reconexión; no oculte la sincronización entre pares en los scripts de shell.

## Momento de consenso

La sincronización del bucle de consenso se encuentra en `consensus_config.json`:
```json
{
  "consensus": {
    "loop_enabled": true,
    "interval": "50ms",
    "timeout_propose": "3s",
    "timeout_prevote": "1s",
    "timeout_precommit": "1s",
    "timeout_commit": "1s",
    "max_block_bytes": 1048576,
    "create_empty_blocks": false,
    "execution_commit": "finalized"
  }
}
```
- `timeout_propose` controla cuánto tiempo espera una ronda para recibir una propuesta.
- `timeout_prevote` controla la ventana de recolección de votos.
- `timeout_precommit` controla la ventana de recopilación de certificados de confirmación.
- `timeout_commit` controla el retraso mínimo después de un bloque comprometido.
- `create_empty_blocks: false` significa que el nodo solo propone cuando las transacciones están disponibles.
- `execution_commit: "finalized"` espera la decisión de finalidad de tres cadenas de HotStuff antes de ejecutar el ancestro finalizado y es el validador generado por defecto. `execution_commit: "qc"` ejecuta y persiste los bloques certificados por control de calidad inmediatamente, pero la puerta de seguridad lo rechaza.

`round_timeout` se conserva únicamente como un agregado de compatibilidad. Prefiere los campos de tiempo de espera estilo Tendermint anteriores.

Cuando `create_empty_blocks` es falso, la altura puede permanecer sin cambios mientras el mempool esté vacío. Eso es lo que se espera: la cadena está esperando trabajo útil en lugar de comprometer bloques vacíos. Cuando aparece una transacción y el estado de la ronda de consenso local ha superado a otro proponente, el nodo avanza a la siguiente ronda donde su validador es el proponente y construye desde el mempool. Esta ruta de recuperación mantiene la actividad activada por transacciones sin volver a habilitar el spam de bloques vacíos.

## Red de validadores múltiples

Para una red generada:
```bash
vexod network init \
  --home .vexo-network \
  --chain-id vexo-chain \
  --validators 4
```
Cada hogar validador generado recibe:

- su propio `validator.key.json`
- sus propios archivos de configuración divididos: `module_config.json`, `network_config.json`, `consensus_config.json`, `mempool_config.json` y `log_config.json`
- un `genesis.json` compartido
- `network_config.json` entradas de pares para los otros validadores

`vexod network up` y `make network-e2e` utilizan un tiempo de espera a nivel de proceso mientras esperan que todos los validadores comiencen, envíen la transacción de humo y observen el crecimiento en altura. El tiempo de espera predeterminado del comando es intencionalmente más largo que el intervalo de consenso porque cubre el inicio del proceso, la apertura de LevelDB, los protocolos de enlace firmados P2P, las comprobaciones de autenticación/TLS, la admisión de transacciones y la finalidad. Si reduce los tiempos de espera de consenso de manera agresiva, mantenga el tiempo de espera de la red lo suficientemente grande como para diagnosticar errores de inicio en lugar de cerrar el arnés demasiado pronto.

Para redes en contenedores o de múltiples hosts, coloque los valores de topología en un archivo JSON:
```json
{
  "p2p_base_port": 26656,
  "rpc_base_port": 26657,
  "p2p_port_step": 0,
  "rpc_port_step": 0,
  "p2p_host_template": "validator-%d",
  "rpc_host_template": "validator-%d",
  "p2p_advertise_host_template": "validator-%d.public.example.com",
  "rpc_advertise_host_template": "rpc-%d.public.example.com",
  "p2p_listen_host": "0.0.0.0",
  "rpc_listen_host": "0.0.0.0"
}
```
- `p2p_host_template` y `rpc_host_template` son objetivos de marcado escritos en la lista de pares `network_config.json` de cada nodo. En Docker, pueden ser nombres de servicios como `validator-%d`.
- `p2p_advertise_host_template` y `rpc_advertise_host_template` son direcciones públicas escritas en metadatos del validador en `genesis.json`. Utilice aquí nombres DNS o IP públicas para redes públicas.
- `p2p_listen_host` y `rpc_listen_host` son hosts de enlace local. Utilice `0.0.0.0` para contenedores o servidores que deberían escuchar en todas las interfaces.
- No reutilice nombres de servicios exclusivos de Docker como direcciones públicas anunciadas a menos que la red sea intencionalmente privada.

Luego genere casas de nodos a partir de ese archivo:
```bash
vexod network init \
  --home .vexo-network \
  --chain-id vexo-chain \
  --validators 4 \
  --network-config ./topology.json
```
## Solución de problemas

| Síntoma | Causa más probable | Qué comprobar |
|---|---|---|
| `latest_height` no aumenta | Bloques vacíos deshabilitados y sin transmisiones, no hay suficientes validadores en línea o firmante no disponible | `consensus_config.json`, registros del validador, `/v1/diagnostics` |
| `peer_count` es `0` | No se puede acceder a las direcciones de pares o se generó `network_config.json` para nombres de host incorrectos | `p2p.peers`, puertos de host de contenedores, DNS, firewall |
| `p2p auth replay store` error | El P2P público/autenticado requiere un almacenamiento de reproducción duradero | `p2p.auth_replay_path` y escriba permiso debajo de la casa |
| `eth_chainId` falla en Remix | URL incorrecta, puerto de host incorrecto o CORS/verificación previa del navegador bloqueado por una configuración personalizada | Utilice la URL del punto final Web3 y luego enrolle el mismo punto final directamente |
| `config audit --strict` falla | La puerta de seguridad encontró una propiedad de configuración insegura | Lea la verificación fallida, luego edite el archivo de configuración dividido que denomina |
| `no block_committed logs` | Registro deshabilitado o no se están creando bloques | `log_config.json`, `create_empty_blocks`, contenidos de mempool |
| `managed EVM key rejected` | Las claves privadas activas se configuran en un oyente RPC público | Elimine `evm_account_private_keys` o mantenga RPC privado |

## Lista de verificación mínima del operador

Antes de entregar un nodo a otra máquina u operador:

- `vexod validate --home <home>` pasa.
- `vexod config audit --home <home> --strict` pasa para esa casa exacta.
- Se revisan `config.json`, archivos de configuración divididos, `genesis.json` y metadatos del validador público.
- `validator.key.json`, `node.key.json` y `validator.vrf.key.json` están cifrados o reemplazados por documentos de clave KMS/firmante remoto.
- `network_config.json:p2p.peers` contiene direcciones que se pueden marcar desde la máquina de destino, no nombres exclusivos de Docker, a menos que el nodo realmente se ejecute dentro de esa red Docker.
- `network_config.json` los oyentes públicos RPC/P2P tienen material TLS cuando `require_network_safety` está habilitado.
- `module_config.json:execution.EVMChainID` se configura antes de que se conecten las billeteras Web3 o Remix.
- `mempool_config.json` tiene una ruta WAL si el nodo debe recuperar los txs pendientes después del reinicio.
- `log_config.json` habilita la confirmación de bloques y los registros de pares mientras se activa la red.

<!-- vexo-docs:technical-parity -->
## Apéndice de paridad técnica

Este apéndice asegura que la traducción conserve las interfaces ejecutables y las secciones clave del documento canónico en inglés. Los comandos, claves de configuración, métodos RPC y nombres de paquetes se mantienen sin cambios en todos los idiomas.

### Seguimiento de secciones
- section: Validator Node — Esta sección debe revisarse junto con valores de configuración, evidencia de verificación, condiciones de fallo y acciones del operador.
- section: Archive Node — Esta sección debe revisarse junto con valores de configuración, evidencia de verificación, condiciones de fallo y acciones del operador.
- section: Split Configuration Files — Esta sección debe revisarse junto con valores de configuración, evidencia de verificación, condiciones de fallo y acciones del operador.
- section: Key Types — Esta sección debe revisarse junto con valores de configuración, evidencia de verificación, condiciones de fallo y acciones del operador.
- section: Config-Based Peers — Esta sección debe revisarse junto con valores de configuración, evidencia de verificación, condiciones de fallo y acciones del operador.
- section: Consensus Timing — Esta sección debe revisarse junto con valores de configuración, evidencia de verificación, condiciones de fallo y acciones del operador.
- section: Multi-Validator Network — Esta sección debe revisarse junto con valores de configuración, evidencia de verificación, condiciones de fallo y acciones del operador.

### Interfaces conservadas sin cambios
- `network_config.json` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `vexod start` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `--timeout-propose` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `--create-empty-blocks` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `--p2p-auth-token` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `--rpc-admin-token` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `--evm-account-key-env` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `--evm-account-key` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `validator_id` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `VEXO_KEY_PASSPHRASE` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `--passphrase` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `--encrypt-keys` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `validator.key.json` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `node.key.json` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `validator.vrf.key.json` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `require_network_safety=true` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `--key-type bls` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `blst-bls12381-minpk-v1` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `genesis.json` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `bls_pop` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `config.json` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `module_config.json` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `consensus_config.json` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `mempool_config.json` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `log_config.json` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `data/` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `network_config.json:p2p.node_key_path` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `shutdown_timeout` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `web3_max_subscriptions_per_connection` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `web3_idle_timeout` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `auth_replay_path` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `require_auth_replay_store` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `dial_timeout` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `data/p2p_auth_replay.jsonl` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `--key-type ed25519` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `vrf_key_paths` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `vrf_public_key` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `vexod` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `<home>/<name>_config.json` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `rpc.evm_account_key_envs` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `rpc.evm_account_private_keys` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `eth_accounts` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `eth_sign` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `eth_signTransaction` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `eth_sendTransaction` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `evm_account_key_envs` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `vexod config paths --home <home>` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `"require_network_safety": true` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `execution_commit` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `require_network_safety` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `host:port` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `rpc.address` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `p2p.listen_address` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `p2p.peers` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `p2p.seeds` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `p2p.node_id` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `p2p.node_key_path` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `p2p.tls_cert_path` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `p2p.tls_key_path` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `p2p.tls_ca_path` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `p2p.tls_server_name` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `p2p.dial_timeout` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `timeout_propose` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `timeout_prevote` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `timeout_precommit` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `timeout_commit` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `create_empty_blocks: false` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `execution_commit: "finalized"` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `execution_commit: "qc"` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `round_timeout` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `create_empty_blocks` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `vexod network up` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `make network-e2e` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `p2p_host_template` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `rpc_host_template` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `validator-%d` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `p2p_advertise_host_template` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `rpc_advertise_host_template` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `p2p_listen_host` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
- `rpc_listen_host` — Este nombre se usa tal cual en ejemplos ejecutables y validación de configuración; no debe traducirse.
