> Locale: es · Español

# Agregar un validador

Esta guía describe el flujo del operador para agregar un validador a una red Vexo.

La ruta de admisión exacta depende de la política de gobernanza y participación de la cadena. Como mínimo, el validador debe estar representado en estado de cadena, tener credenciales válidas y formar parte de una actualización del conjunto de validadores con versión de altura.

## 1. Inicializar el inicio del validador
```bash
vexod init validator \
  --home .vexo-validator-new \
  --chain-id vexo-chain \
  --validator validator-new \
  --encrypt-keys
```
Para una clave de validador BLS:
```bash
vexod init validator \
  --home .vexo-validator-new \
  --chain-id vexo-chain \
  --validator validator-new \
  --key-type bls \
  --encrypt-keys
```
Configure `VEXO_KEY_PASSPHRASE` antes de ejecutar estos comandos, o pase `--passphrase` para una configuración local única.

Al admitir un validador BLS en una cadena existente, incluya los metadatos `bls_pop` generados en la propuesta de actualización del validador.
La ruta de clave BLS predeterminada utiliza `blst-bls12381-minpk-v1`; use `vexod keys gen --type bls --bls-adapter circl-bls12381-g1sigg2-basic-v1` solo para pruebas de referencia/compatibilidad.

Archive la clave pública generada:
```bash
vexod keys show --home .vexo-validator-new --json
```
También conserve el `node.key.json` generado. Firma apretones de manos P2P para `network_config.json:p2p.node_id`; no es una clave de consenso del validador y no debe reutilizarse como clave de cuenta.

## 2. Configurar direcciones de red y pares

Edite `.vexo-validator-new/network_config.json` y configure direcciones de escucha locales más pares persistentes:
```json
{
  "schema_version": "v1",
  "rpc": {
    "enabled": true,
    "address": "0.0.0.0:26657"
  },
  "p2p": {
    "enabled": true,
    "node_id": "validator-new",
    "node_key_path": "node.key.json",
    "listen_address": "0.0.0.0:26656",
    "peers": {
      "validator-1": "validator-1.example.com:26656",
      "validator-2": "validator-2.example.com:26656",
      "validator-3": "validator-3.example.com:26656"
    }
  },
  "peer_scoring": {
    "InitialScore": 100,
    "MaxScore": 1000,
    "BanThreshold": 0
  }
}
```
No confíe en anulaciones de red de línea de comandos de larga duración para los validadores de producción. Mantenga las direcciones de pares persistentes en `network_config.json`.

Utilice roles de dirección separados:

- `p2p.listen_address` y `rpc.address` son direcciones de enlace locales para esta máquina o contenedor.
- `p2p.node_id` es la identidad del par de este nodo. Manténgalo estable después de que sus compañeros lo hayan aprendido.
- `p2p.node_key_path` apunta a la clave de firma del protocolo de enlace local para esa identidad de igual.
- `p2p.peers` contiene objetivos de marcado que este nodo utiliza para comunicarse con otros pares; Las claves del mapa deben ser los valores `p2p.node_id` de los nodos remotos.
- Los metadatos del validador `p2p_address` y `rpc_address` deben contener direcciones públicas anunciadas, no nombres de servicios exclusivos de Docker, a menos que la red sea intencionalmente privada.

## 3. Enviar admisión del validador

Por ejemplo, flujos de participación, cree una transacción de participación:
```bash
vexod staking --help
```
La transacción de admisión del validador debe incluir:

- ID del validador
- dirección del validador
- clave pública de consenso
- poder de voto o referencia de participación
- puntos básicos de comisión del validador, si la cadena permite actualizaciones de comisiones de autoservicio
- Metadatos P2P `node_id` si la cadena usa metadatos de génesis/validador para preconfigurar mapas de pares
- metadatos de direcciones P2P públicas
- metadatos de direcciones RPC públicas, si son públicas
- Metadatos de prueba de posesión de BLS cuando BLS está habilitado

La actualización del validador debe entrar en vigor a una altura específica y producir un nuevo hash del conjunto de validadores.

Una vez que el validador está activo, los operadores pueden exponer el estado de la recompensa a través del módulo de apuesta:
```bash
vexod staking query commission validator-1
vexod staking query rewards alice validator-1
```
## 4. Verificar la actualización del conjunto de validadores

Después de la altura de actualización:
```bash
curl http://127.0.0.1:26657/v1/validators/<height>
```
Comprobar:

- el validador aparece en el conjunto específico de altura
- el poder de voto es correcto
- el hash del conjunto del validador cambió como se esperaba
- las pruebas de finalidad hacen referencia a la altura correcta del conjunto del validador

## 5. Rotación de claves del validador de planes

Las claves de validación se pueden rotar preparando un siguiente documento de clave con metadatos `active_from` y `active_until` que no se superpongan y luego iniciando el nodo con la clave de rotación adicional:
```bash
vexod keys gen --home .vexo-validator-new --path next-validator.key.json --id key-2 --active-from 1001
vexod keys rotation-plan --home .vexo-validator-new --key validator.key.json --key next-validator.key.json
vexod start --home .vexo-validator-new --rotation-key next-validator.key.json --dry-run
```
En el momento de la firma, el nodo utiliza la clave cuya ventana activa contiene la altura del consenso. Los documentos clave del firmante remoto mantienen los mismos requisitos de política, token de autenticación y protección de doble firma.

## 6. Iniciar validador
```bash
vexod config audit --home .vexo-validator-new --strict
vexod start --home .vexo-validator-new
```
El inicio no tiene interruptor de modo de red. Utilice `config audit --strict` antes del inicio cuando se espera que la red satisfaga los supuestos de seguridad de la red pública.

## 7. Monitorear

Mirar:

- latencia de propuesta/voto
- tiempos de espera de ronda
- fallos en la firma del validador
- prohibiciones de pares
- tamaño del grupo de memoria
- cometer latencia
- salud de instantánea/repetición

Uso:
```bash
vexod ops thresholds --json
vexod ops incident --metrics-file current.json --previous-metrics-file previous.json --window 1m
```
## Notas de seguridad

- Nunca reutilice las claves del validador en cadenas independientes.
- Mantener la política de firmante remoto habilitada para los validadores de producción.
- No admita un validador BLS sin prueba de posesión o defensa de clave fraudulenta equivalente.
- No corte ni encarcele a un validador sin evidencia verificada vinculada al conjunto correcto de validador de altura de evidencia.

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
