> Locale: pt · Português

# Inicialização do nó

Este guia explica como inicializar o validador e arquivar os nós dos nós, iniciá-los, verificar se estão íntegros e conectar os clientes.

A conectividade peer deve ser configurada em `network_config.json`, e não passada repetidamente na linha de comando `start`.

O comportamento de tempo de execução que afeta consenso, RPC, P2P, registro ou contas Web3 gerenciadas é apenas arquivo de configuração. `vexod start` rejeita sinalizadores como `--timeout-propose`, `--create-empty-blocks`, `--p2p-auth-token`, `--rpc-admin-token`, `--evm-account-key-env` e `--evm-account-key`; edite os arquivos de configuração divididos para que cada operador revise o mesmo comportamento determinístico do nó.

Não há alternância de modo de nó. Um nó inicial é definido por seus arquivos de configuração, gênese, material de chave e se `validator_id` mais um signatário estão presentes.

## O que você está construindo

Um nó inicial do Vexo é um diretório que contém tudo que um nó precisa para iniciar:
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
A regra importante é simples: inicialize uma vez, edite os arquivos de configuração e depois inicie. Não esconda o comportamento da rede dentro dos sinalizadores do shell.

## Corrida local de cinco minutos

Use esse fluxo quando quiser provar que o binário funciona antes de pensar na implantação de vários hosts.
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
Em outro terminal:
```bash
curl -s http://127.0.0.1:26657/v1/status
curl -s http://127.0.0.1:26657/v1/diagnostics
curl -s http://127.0.0.1:26657/v1/metrics
```
Formato de status esperado:
```json
{
  "chain_id": "vexo-chain",
  "running": true,
  "latest_height": 0,
  "peer_count": 0,
  "banned_peers": 0
}
```
A altura mais recente pode permanecer em zero em uma execução de nó único ou de mempool vazio quando a criação de bloco vazio estiver desabilitada. Isso não significa que o processo esteja quebrado. Isso significa que o nó não está produzindo blocos vazios. Adicione transações ou execute uma rede de teste multivalidador para observar confirmações contínuas.

## Rede local de quatro validadores

Use esse fluxo quando desejar conectividade entre pares, rotação de proponentes, logs de confirmação de bloco e crescimento de altura.
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
Verificações úteis:
```bash
curl -s http://127.0.0.1:26657/v1/status
curl -s http://127.0.0.1:26667/v1/status
curl -s http://127.0.0.1:26677/v1/status
curl -s http://127.0.0.1:26687/v1/status
```
Se o registro de confirmação de bloco estiver habilitado em `log_config.json`, os logs do validador incluirão eventos como:
```json
{"event":"block_committed","height":12,"round":0,"tx_count":0}
```
Pare a rede local gerada com:
```bash
./bin/vexod network stop --home .vexo-network --validators 4
```
## Web3 e Remix

O JSON-RPC estilo Ethereum reside no endpoint Web3, não no namespace da API operacional Vexo versionado.

Para o validador de host único Docker 1, o URL do provedor personalizado Remix é:
```text
http://127.0.0.1:28657/web3
```
Para um nó local direto com a porta RPC padrão:
```text
http://127.0.0.1:26657/web3
```
Teste a mesma chamada que o Remix faz:
```bash
curl -s http://127.0.0.1:26657/web3 \
  -H 'content-type: application/json' \
  -d '{"jsonrpc":"2.0","id":1,"method":"eth_chainId","params":[]}'
```
Se um navegador disser que a busca do ID da cadeia falhou, verifique estes itens na ordem:

1. A URL termina com o caminho do endpoint Web3.
2. O navegador pode alcançar a porta do host. Os exemplos do Docker expõem `28657`, `28667`, `28677` e `28687`; dentro do contêiner a porta RPC ainda é `26657`.
3. O servidor RPC está em execução; consulte o endpoint de status no mesmo host e porta.
4. CORS é permitido pela configuração `network_config.json`/RPC. O manipulador padrão permite a simulação do navegador quando nenhuma lista CORS personalizada está definida.
5. A cadeia possui um ID de cadeia EVM diferente de zero em `module_config.json`.

## Nó validador

Use `init validator` quando o nó irá propor, votar, assinar mensagens de consenso e participar da rotação do validador.
```bash
vexod init validator \
  --home .vexo-validator-1 \
  --chain-id vexo-chain \
  --validator validator-1 \
  --encrypt-keys
```
Defina `VEXO_KEY_PASSPHRASE` antes de executar este comando ou passe `--passphrase` para uma configuração local única. `--encrypt-keys` criptografa `validator.key.json`, `node.key.json` e `validator.vrf.key.json`.

Regra prática de custódia de chaves:

- `validator.key.json` assina propostas de consenso, votações, votos de tempo limite e mensagens relacionadas a finalidades.
- `node.key.json` assina apenas handshakes P2P; ela nunca deve ser reutilizada como chave de consenso do validador.
- `validator.vrf.key.json` comprova a aleatoriedade do comitê e deve ser tratado como material de custódia do validador.
- Os ouvintes públicos devem usar documentos de chave local criptografados ou documentos de chave de assinante remoto/estilo KMS. Se um nó expõe RPC público ou P2P público autenticado enquanto `require_network_safety=true`, a inicialização rejeita chaves do validador local em texto simples.
- As chaves geradas são escritas no modo de sistema de arquivos `0600`; ainda prefiro um assinante/KMS remoto para validadores de longa duração.

Para uma chave de consenso BLS:
```bash
vexod init validator \
  --home .vexo-validator-1 \
  --chain-id vexo-chain \
  --validator validator-1 \
  --key-type bls \
  --encrypt-keys
```
`--key-type bls` grava um documento chave `blst-bls12381-minpk-v1` BLS e copia o comprovante de posse nos metadados do validador `genesis.json` como `bls_pop`.

Isso cria:

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

`validator.key.json` é o signatário do consenso. `node.key.json` é o signatário do handshake P2P referenciado por `network_config.json:p2p.node_key_path`. Eles são deliberadamente separados para que os nós de arquivo e os validadores possam usar o mesmo transporte sem fornecer a cada par uma chave de assinatura do validador.

Comece com rede orientada por configuração:
```bash
vexod start --home .vexo-validator-1
```
Após a inicialização, leia os logs. Um validador íntegro deve emitir eventos de execução de nó, escuta RPC, escuta P2P e, uma vez confirmados os blocos, eventos de confirmação de bloco. Se a criação de bloco vazio estiver desabilitada, a falta de logs confirmados por bloco pode simplesmente significar que não há transações.

## Nó de arquivo

Use `init archive` quando o nó deve manter dados em cadeia, expor RPC, sincronizar com pares e evitar assinatura do validador.
```bash
vexod init archive \
  --home .vexo-archive-1 \
  --chain-id vexo-chain \
  --bootstrap-peer validator-1=seed-1.example.com:26656
```
Isso cria:

- `config.json`
- `module_config.json`
- `network_config.json`
- `consensus_config.json`
- `mempool_config.json`
- `log_config.json`
- `genesis.json`
- `node.key.json`
- `data/`

**não** cria `validator.key.json`.

Comece com:
```bash
vexod start --home .vexo-archive-1
```
Os nós de arquivo não assinam votos de consenso. Eles são úteis para RPC, indexação, sincronização de estado, exibição de provas históricas e manutenção de um histórico de consultas mais amplo do que a remoção de validadores.

## Dividir arquivos de configuração

As casas dos nós usam arquivos de configuração separados para que os operadores possam editar um subsistema sem misturar configurações não relacionadas:

- `config.json` contém identidade do nó, ID da cadeia, caminho de dados e ponteiros para os arquivos de configuração divididos.
- `module_config.json` contém seleção de módulo de aplicativo, política de execução/ante e política de governança em nível de módulo.
- `network_config.json` contém RPC, identidade de nó P2P, configurações de escuta/peer/seed, configurações de TLS/auth e política de pontuação de peer.
- `consensus_config.json` contém tempo de loop de consenso, política de bloco vazio, backend criptográfico, VRF, admissão de validador e política de comitê.
- `mempool_config.json` contém tamanho do mempool, taxa, prioridade, WAL, duplicado e política TTL.
- `log_config.json` contém formato de log, nível, registro de eventos de confirmação de bloco e registro de eventos de pares.
- `genesis.json` contém validadores genesis imutáveis, metadados validadores e estado do módulo genesis.

As configurações de `network_config.json` RPC também incluem `shutdown_timeout`, `web3_max_subscriptions_per_connection` e `web3_idle_timeout`. `shutdown_timeout` limita o desligamento normal do loop de consenso, do servidor RPC e do transporte de nó para que os operadores não esperem para sempre em um caminho de parada travado. O padrão gerado é `10s`; O padrão das assinaturas Web3 é 256 por conexão com um tempo limite de inatividade `2m` para que os pontos de extremidade RPC públicos não possam acumular assinaturas inativas ilimitadas.

`network_config.json` As configurações P2P incluem `auth_replay_path`, `require_auth_replay_store` e `dial_timeout`. O padrão gerado grava evidência de repetição nonce em `data/p2p_auth_replay.jsonl` e usa um tempo limite de discagem de saída `10s`. Para testes de loopback privados, o replay store é, em sua maioria, uma contabilidade inofensiva; para P2P autenticado publicamente, é um requisito de segurança porque evita que um nonce de handshake assinado capturado seja reproduzido após a reinicialização. `dial_timeout` deve ser longo o suficiente para TLS, verificação de handshake assinado e latência entre regiões; defini-lo muito baixo faz com que pares saudáveis ​​pareçam esquisitos e pode diminuir a vitalidade após reinicializações.

`network_config.json` também possui sincronização de estado de inicialização. Isso é útil para nós de arquivo, validadores de substituição ou nós restaurados em uma máquina limpa. Quando `state_sync.enabled` é verdadeiro, `vexod start` baixa o primeiro instantâneo válido de `state_sync.snapshot_urls`, verifica o ID da cadeia, soma de verificação, raízes de estado e namespaces KV, restaura-o no LevelDB, reconstrói os índices e só então inicia o nó. Se o estado local já satisfaz `state_sync.min_height` e `state_sync.trust_local_higher` for verdadeiro, a inicialização registra `state_sync_skipped` e mantém o armazenamento local.

Exemplo de bloco `state_sync`:
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
A inicialização registra `state_sync_candidate_failed` para um erro de busca, `state_sync_candidate_rejected` para um snapshot inválido ou obsoleto e `state_sync_applied` após uma restauração verificada. Mantenha `max_snapshot_bytes` abaixo do maior instantâneo que sua infraestrutura atende intencionalmente, mas alto o suficiente para o crescimento normal do estado. Não aponte nós públicos para uma fonte de snapshot de terceiros não autenticada, a menos que o operador tenha uma política de confiança fora de banda e evidência de finalidade/cliente leve para essa fonte.

Se um campo alterar o comportamento da rede, edite o arquivo de configuração dividido e confirme ou distribua o arquivo revisado. Não confie em sinalizadores `vexod start` longos para comportamento em tempo de execução. O comando start rejeita intencionalmente o tempo de consenso, bloco vazio, autenticação P2P, administração RPC e sinalizadores de chave Web3 gerenciados para que os operadores não executem acidentalmente um comportamento diferente da configuração revisada.

## Qual arquivo devo editar?

| Meta | Arquivo | Campo |
|---|---|---|
| Alterar porta de ligação RPC | `network_config.json` | `rpc.address` |
| Alterar porta de ligação P2P | `network_config.json` | `p2p.listen_address` |
| Adicionar pares persistentes | `network_config.json` | `p2p.peers` |
| Adicionar pares iniciais | `network_config.json` | `p2p.seeds` |
| Habilitar/desabilitar blocos vazios | `consensus_config.json` | campo de bloco vazio de consenso |
| Ajustar tempos limite de consenso | `consensus_config.json` | campos de proposta, pré-votação, pré-confirmação e tempo limite de confirmação |
| Exigir execução finalizada | `consensus_config.json` | campo de confirmação de execução de consenso |
| Habilitar/desabilitar módulos | `module_config.json` | lista de módulos de aplicação |
| Alterar ID da cadeia EVM | `module_config.json` | campo de ID da cadeia EVM de execução |
| Ajustar taxa básica/gás | `module_config.json` | campos taxa base de execução, taxa dinâmica, gás alvo e gás máximo |
| Configurar o mempool WAL | `mempool_config.json` | caminho WAL do mempool |
| Logs de commit do bloco de controle | `log_config.json` | registrar campo de eventos de commit |
| Controlar registros de pares | `log_config.json` | registrar campo de eventos de pares |

Na dúvida, execute:
```bash
vexod config paths --home .vexo-validator-1
vexod config show --home .vexo-validator-1
vexod doctor --home .vexo-validator-1
```
## Tipos de chave

O padrão de inicialização do validador é `--key-type bls` porque a validação de segurança de rede requer finalidade agregada BLS auditada. `--key-type ed25519` permanece disponível para experimentos privados e implantações personalizadas fora do portão de segurança da rede. `--encrypt-keys` deve ser usado para qualquer nó home não descartável. A geração de chaves autônomas também oferece suporte a chaves VRF:
```bash
vexod keys gen --home .vexo-ed25519 --type ed25519
vexod keys gen --home .vexo-bls --type bls
vexod keys gen --home .vexo-bls-circl --type bls --bls-adapter circl-bls12381-g1sigg2-basic-v1
VEXO_KEY_PASSPHRASE='change-me' vexod keys gen --home .vexo-vrf --type vrf --encrypt
```
As chaves VRF não são signatários de consenso. Eles são usados ​​para seleção de comitê apoiado por VRF e devem ser referenciados de `consensus_config.json` a `vrf_key_paths` mais a chave de metadados do validador `vrf_public_key` quando esse back-end estiver habilitado.

`config.json` aponta para os arquivos de configuração divididos:
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
Cada caminho pode ser absoluto ou relativo ao nó inicial. Se omitido, `vexod` usa o arquivo `<home>/<name>_config.json` padrão.

Exemplo `module_config.json`:
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
A política de governança também reside em `module_config.json`. As configurações seguras de rede geradas exigem um depósito de proposta:
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
O depósito é o saldo nativo garantido pelo apresentador da proposta. As propostas aprovadas reembolsam o depósito; propostas rejeitadas movem-no para `RejectedDeposits`. Use um endereço controlado pelo seu módulo de tesouraria/conjunto comunitário se os depósitos rejeitados devem financiar um tesouro em vez da conta do módulo padrão.

Exemplo `network_config.json`:
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
`rpc.evm_account_key_envs` e `rpc.evm_account_private_keys` são métodos de conta gerenciada Web3 opcionais e posteriores, como `eth_accounts`, `eth_sign`, `eth_signTransaction` e `eth_sendTransaction`. Prefira `evm_account_key_envs` para que a chave privada seja injetada pelo ambiente de processo ou gerenciador de segredos em vez de armazenada em JSON. Mantenha ambas as listas vazias para operação normal do validador, a menos que este nó esteja agindo intencionalmente como um ponto final de carteira quente Web3 local. A segurança de inicialização rejeita teclas de atalho EVM gerenciadas em ouvintes RPC públicos.

Exemplo `consensus_config.json`:
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
`vrf_key_paths` são resolvidos em relação ao diretório que contém `consensus_config.json`. Use documentos de chave criptografados e forneça `VEXO_KEY_PASSPHRASE` ao processo do nó quando a custódia da chave VRF local for inevitável. Não coloque escalares privados VRF brutos diretamente em `consensus_config.json` para redes executadas por operadora.

Use `vexod config paths --home <home>` para inspecionar todos os caminhos resolvidos.

A configuração do arquivo tem:
```json
{
  "schema_version": "v1",
  "validator_id": "",
  "chain_id": "vexo-chain",
  "consensus_config_path": "consensus_config.json"
}
```
O arquivo `consensus_config.json` desativa o loop de consenso local:
```json
{
  "schema_version": "v1",
  "consensus": {
    "loop_enabled": false
  }
}
```
As casas do validador geradas definem `"require_network_safety": true` em `config.json` por padrão. Este não é um modo; é uma porta de segurança de inicialização que rejeita criptografia determinística, transações não assinadas/não concedidas, falta de taxas/pisos de gás, falta de mempool WAL durável, falta de política de substituição para transações do mesmo signatário/nonce, aleatoriedade do comitê inseguro e valores `execution_commit` diferentes de `finalized`.

Quando `require_network_safety` estiver habilitado, execute:
```bash
vexod config audit --home <home> --strict
```
antes de iniciar o nó. A auditoria deve passar para todos os validadores e arquivos que participam da mesma rede.

## Pares baseados em configuração

Endereços de peering e escuta em `network_config.json`:
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
`vexod start` carrega esses peers automaticamente:
```bash
vexod start --home .vexo-archive-1
```
Peers e sementes persistentes são configurados em `network_config.json`; `vexod start` não aceita substituições de host de peer ou de seed.

Não coloque configurações de host de longa duração ou `host:port` na linha de comando `vexod start`. Edite `rpc.address`, `p2p.listen_address`, `p2p.peers` e `p2p.seeds` em `network_config.json`.

Mantenha `p2p.node_id` estável durante a vida útil do nó inicial. `p2p.node_key_path` deve apontar para `node.key.json` ou outro documento de chave local/gerenciado usado apenas para assinatura de handshake de pares. Os mapas de pares devem usar IDs de nós de pares, e não endereços de contas ou nomes de operadores validadores, a menos que sejam intencionalmente iguais.

Para transporte de peer gRPC criptografado e autenticado, defina também `p2p.tls_cert_path`, `p2p.tls_key_path`, `p2p.tls_ca_path` e, opcionalmente, `p2p.tls_server_name` em `network_config.json`. Os caminhos TLS relativos são resolvidos no diretório inicial do nó. Mantenha `p2p.dial_timeout` no mesmo arquivo para que cada operador use o mesmo comportamento de reconexão; não esconda o tempo dos pares em scripts de shell.

## Momento de consenso

O tempo do loop de consenso reside em `consensus_config.json`:
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
- `timeout_propose` controla quanto tempo uma rodada espera por uma proposta.
- `timeout_prevote` controla a janela de coleta de votos.
- `timeout_precommit` controla a janela de coleta de certificados de commit.
- `timeout_commit` controla o atraso mínimo após um bloco confirmado.
- `create_empty_blocks: false` significa que o nó só propõe quando as transações estão disponíveis.
- `execution_commit: "finalized"` aguarda a decisão de finalidade de três cadeias do HotStuff antes de executar o ancestral finalizado e é o padrão do validador gerado. `execution_commit: "qc"` executa e persiste os blocos certificados pelo QC imediatamente, mas a porta de segurança os rejeita.

`round_timeout` é mantido apenas como agregado de compatibilidade. Prefira os campos de tempo limite no estilo Tendermint acima.

Quando `create_empty_blocks` é falso, a altura pode permanecer inalterada enquanto o mempool estiver vazio. Isso é esperado: a cadeia está aguardando um trabalho útil em vez de comprometer blocos vazios. Quando uma transação aparece e o estado da rodada de consenso local passa por outro proponente, o nó avança para a próxima rodada onde seu validador é o proponente e constrói a partir do mempool. Esse caminho de recuperação mantém a atividade acionada pela transação sem reativar o spam de bloco vazio.

## Rede Multivalidadora

Para uma rede gerada:
```bash
vexod network init \
  --home .vexo-network \
  --chain-id vexo-chain \
  --validators 4
```
Cada home validador gerado recebe:

- seu próprio `validator.key.json`
- seus próprios arquivos de configuração divididos: `module_config.json`, `network_config.json`, `consensus_config.json`, `mempool_config.json` e `log_config.json`
- um `genesis.json` compartilhado
- `network_config.json` entradas de pares para os outros validadores

`vexod network up` e `make network-e2e` usam um tempo limite de nível de processo enquanto aguardam que todos os validadores iniciem, enviem a transação de fumaça e observem o crescimento da altura. O tempo limite do comando padrão é intencionalmente maior que o intervalo de consenso porque abrange inicialização do processo, abertura do LevelDB, handshakes assinados P2P, verificações de TLS/autenticação, admissão de transação e finalidade. Se você reduzir agressivamente os tempos limite de consenso, mantenha o tempo limite de rede grande o suficiente para diagnosticar erros de inicialização, em vez de eliminar o chicote muito cedo.

Para redes conteinerizadas ou multi-host, coloque os valores de topologia em um arquivo JSON:
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
- `p2p_host_template` e `rpc_host_template` são alvos de discagem gravados na lista de pares `network_config.json` de cada nó. No Docker, podem ser nomes de serviços como `validator-%d`.
- `p2p_advertise_host_template` e `rpc_advertise_host_template` são endereços públicos gravados nos metadados do validador em `genesis.json`. Use nomes DNS ou IPs públicos aqui para redes públicas.
- `p2p_listen_host` e `rpc_listen_host` são hosts de ligação local. Use `0.0.0.0` para contêineres ou servidores que devem escutar em todas as interfaces.
- Não reutilize nomes de serviços exclusivos do Docker como endereços públicos anunciados, a menos que a rede seja intencionalmente privada.

Em seguida, gere casas de nós a partir desse arquivo:
```bash
vexod network init \
  --home .vexo-network \
  --chain-id vexo-chain \
  --validators 4 \
  --network-config ./topology.json
```
## Solução de problemas

| Sintoma | Causa mais provável | O que verificar |
|---|---|---|
| `latest_height` não aumenta | Blocos vazios desativados e sem txs, validadores on-line insuficientes ou assinante indisponível | `consensus_config.json`, logs do validador, `/v1/diagnostics` |
| `peer_count` é `0` | Endereços de pares não podem ser acessados ​​ou `network_config.json` foi gerado para nomes de host errados | `p2p.peers`, portas de host de contêiner, DNS, firewall |
| Erro `p2p auth replay store` | P2P público/autenticado requer armazenamento de reprodução durável | `p2p.auth_replay_path` e permissão de gravação na página inicial |
| `eth_chainId` falha no Remix | URL errado, porta de host errada ou CORS/preflight do navegador bloqueado pela configuração personalizada | Use o URL do endpoint Web3 e, em seguida, enrole o mesmo endpoint diretamente |
| `config audit --strict` falha | O portão de segurança encontrou uma propriedade de configuração insegura | Leia a verificação com falha e edite o arquivo de configuração dividido que ele nomeia |
| `no block_committed logs` | Registro desativado ou nenhum bloco está sendo criado | `log_config.json`, `create_empty_blocks`, conteúdo do mempool |
| `managed EVM key rejected` | Chaves privadas ativas são configuradas em um ouvinte RPC público | Remova `evm_account_private_keys` ou mantenha o RPC privado |

## Lista de verificação mínima do operador

Antes de entregar um nó para outra máquina ou operador:

- `vexod validate --home <home>` passa.
- `vexod config audit --home <home> --strict` passa para aquela casa exata.
- `config.json`, arquivos de configuração divididos, `genesis.json` e metadados do validador público são revisados.
- `validator.key.json`, `node.key.json` e `validator.vrf.key.json` são criptografados ou substituídos por documentos de chave KMS/signatário remoto.
- `network_config.json:p2p.peers` contém endereços que podem ser discados na máquina de destino, e não nomes somente do Docker, a menos que o nó realmente seja executado dentro dessa rede Docker.
- `network_config.json` ouvintes RPC/P2P públicos têm material TLS quando `require_network_safety` está habilitado.
- `module_config.json:execution.EVMChainID` é definido antes das carteiras Web3 ou Remix se conectarem.
- `mempool_config.json` possui um caminho WAL se o nó precisar recuperar txs pendentes após a reinicialização.
- `log_config.json` permite confirmação de bloco e logs de pares enquanto a rede está sendo ativada.

<!-- vexo-docs:technical-parity -->
## Apêndice de paridade técnica

Este apêndice garante que a tradução preserve as interfaces executáveis e as seções principais do documento canônico em inglês. Comandos, chaves de configuração, métodos RPC e nomes de pacotes permanecem inalterados em todos os idiomas.

### Rastreamento de seções
- section: Validator Node — Esta seção deve ser revisada junto com valores de configuração, evidências de verificação, condições de falha e ações do operador.
- section: Archive Node — Esta seção deve ser revisada junto com valores de configuração, evidências de verificação, condições de falha e ações do operador.
- section: Split Configuration Files — Esta seção deve ser revisada junto com valores de configuração, evidências de verificação, condições de falha e ações do operador.
- section: Key Types — Esta seção deve ser revisada junto com valores de configuração, evidências de verificação, condições de falha e ações do operador.
- section: Config-Based Peers — Esta seção deve ser revisada junto com valores de configuração, evidências de verificação, condições de falha e ações do operador.
- section: Consensus Timing — Esta seção deve ser revisada junto com valores de configuração, evidências de verificação, condições de falha e ações do operador.
- section: Multi-Validator Network — Esta seção deve ser revisada junto com valores de configuração, evidências de verificação, condições de falha e ações do operador.

### Interfaces mantidas sem alteração
- `network_config.json` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `vexod start` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `--timeout-propose` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `--create-empty-blocks` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `--p2p-auth-token` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `--rpc-admin-token` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `--evm-account-key-env` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `--evm-account-key` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `validator_id` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `VEXO_KEY_PASSPHRASE` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `--passphrase` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `--encrypt-keys` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `validator.key.json` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `node.key.json` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `validator.vrf.key.json` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `require_network_safety=true` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `--key-type bls` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `blst-bls12381-minpk-v1` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `genesis.json` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `bls_pop` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `config.json` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `module_config.json` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `consensus_config.json` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `mempool_config.json` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `log_config.json` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `data/` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `network_config.json:p2p.node_key_path` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `shutdown_timeout` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `web3_max_subscriptions_per_connection` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `web3_idle_timeout` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `auth_replay_path` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `require_auth_replay_store` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `dial_timeout` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `data/p2p_auth_replay.jsonl` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `--key-type ed25519` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `vrf_key_paths` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `vrf_public_key` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `vexod` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `<home>/<name>_config.json` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `rpc.evm_account_key_envs` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `rpc.evm_account_private_keys` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `eth_accounts` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `eth_sign` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `eth_signTransaction` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `eth_sendTransaction` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `evm_account_key_envs` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `vexod config paths --home <home>` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `"require_network_safety": true` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `execution_commit` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `require_network_safety` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `host:port` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `rpc.address` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `p2p.listen_address` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `p2p.peers` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `p2p.seeds` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `p2p.node_id` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `p2p.node_key_path` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `p2p.tls_cert_path` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `p2p.tls_key_path` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `p2p.tls_ca_path` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `p2p.tls_server_name` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `p2p.dial_timeout` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `timeout_propose` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `timeout_prevote` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `timeout_precommit` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `timeout_commit` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `create_empty_blocks: false` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `execution_commit: "finalized"` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `execution_commit: "qc"` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `round_timeout` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `create_empty_blocks` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `vexod network up` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `make network-e2e` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `p2p_host_template` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `rpc_host_template` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `validator-%d` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `p2p_advertise_host_template` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `rpc_advertise_host_template` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `p2p_listen_host` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `rpc_listen_host` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.

## Stable Terms

- `EVMForkPreset: "latest"`
- `params.ChainConfig`
