> Locale: pt · Português

# Adicionando um validador

Este guia descreve o fluxo do operador para adicionar um validador a uma rede Vexo.

O caminho exato de admissão depende da política de participação e governança da rede. No mínimo, o validador deve ser representado em estado de cadeia, ter credenciais válidas e tornar-se parte de uma atualização do conjunto de validadores com versão de altura.

## 1. Inicializar página inicial do validador
```bash
vexod init validator \
  --home .vexo-validator-new \
  --chain-id vexo-chain \
  --validator validator-new \
  --encrypt-keys
```
Para uma chave validadora BLS:
```bash
vexod init validator \
  --home .vexo-validator-new \
  --chain-id vexo-chain \
  --validator validator-new \
  --key-type bls \
  --encrypt-keys
```
Defina `VEXO_KEY_PASSPHRASE` antes de executar esses comandos ou passe `--passphrase` para uma configuração local única.

Ao admitir um validador BLS em uma cadeia existente, inclua os metadados `bls_pop` gerados na proposta de atualização do validador.
O caminho da chave BLS padrão usa `blst-bls12381-minpk-v1`; use `vexod keys gen --type bls --bls-adapter circl-bls12381-g1sigg2-basic-v1` apenas para testes de referência/compatibilidade.

Arquive a chave pública gerada:
```bash
vexod keys show --home .vexo-validator-new --json
```
Mantenha também o `node.key.json` gerado. Assina handshakes P2P para `network_config.json:p2p.node_id`; não é uma chave de consenso do validador e não deve ser reutilizada como uma chave de conta.

## 2. Configurar endereços de rede e pares

Edite `.vexo-validator-new/network_config.json` e defina endereços de escuta locais mais pares persistentes:
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
Não confie em substituições de rede de linha de comando de longa duração para validadores de produção. Mantenha endereços de pares persistentes em `network_config.json`.

Use funções de endereço separadas:

- `p2p.listen_address` e `rpc.address` são endereços de ligação locais para esta máquina ou contêiner.
- `p2p.node_id` é a identidade de peer deste nó. Mantenha-o estável depois que os colegas aprenderem.
- `p2p.node_key_path` aponta para a chave de assinatura de handshake local para essa identidade de peer.
- `p2p.peers` contém alvos de discagem que este nó usa para alcançar outros pares; as chaves do mapa devem ser os valores `p2p.node_id` dos nós remotos.
- os metadados do validador `p2p_address` e `rpc_address` devem conter endereços públicos anunciados, e não nomes de serviços exclusivos do Docker, a menos que a rede seja intencionalmente privada.

## 3. Enviar admissão do validador

Por exemplo, fluxos de piquetagem, crie uma transação de piquetagem:
```bash
vexod staking --help
```
A transação de admissão do validador deve incluir:

- ID do validador
- endereço do validador
- chave pública de consenso
- poder de voto ou referência de participação
- pontos base de comissão do validador, se a rede permitir atualizações de comissão de autoatendimento
- Metadados P2P `node_id` se a cadeia usar metadados de gênese/validador para pré-configuração de mapas de pares
- metadados de endereços P2P públicos
- metadados de endereço RPC público, se público
- Metadados de prova de posse do BLS quando o BLS está ativado

A atualização do validador deve entrar em vigor em uma altura específica e produzir um novo hash do conjunto do validador.

Depois que o validador estiver ativo, os operadores poderão expor o estado da recompensa por meio do módulo de staking:
```bash
vexod staking query commission validator-1
vexod staking query rewards alice validator-1
```
## 4. Verifique a atualização do conjunto de validadores

Após a altura da atualização:
```bash
curl http://127.0.0.1:26657/v1/validators/<height>
```
Verifique:

- o validador aparece no conjunto específico de altura
- o poder de voto está correto
- hash do conjunto de validadores alterado conforme esperado
- as provas de finalidade fazem referência à altura correta do conjunto do validador

## 5. Planeje a rotação da chave do validador

As chaves do validador podem ser rotacionadas preparando um próximo documento-chave com metadados `active_from` e `active_until` não sobrepostos e, em seguida, iniciando o nó com a chave de rotação extra:
```bash
vexod keys gen --home .vexo-validator-new --path next-validator.key.json --id key-2 --active-from 1001
vexod keys rotation-plan --home .vexo-validator-new --key validator.key.json --key next-validator.key.json
vexod start --home .vexo-validator-new --rotation-key next-validator.key.json --dry-run
```
No momento da assinatura, o nó utiliza a chave cuja janela ativa contém a altura de consenso. Os documentos-chave do assinante remoto mantêm os mesmos requisitos de política, token de autenticação e proteção de assinatura dupla.

## 6. Iniciar validador
```bash
vexod config audit --home .vexo-validator-new --strict
vexod start --home .vexo-validator-new
```
A inicialização não possui chave de modo de rede. Use `config audit --strict` antes da inicialização quando se espera que a rede satisfaça as suposições de segurança da rede pública.

## 7. Monitore

Assista:

- latência de proposta/voto
- tempos limites redondos
- falhas de assinatura do validador
- proibições de pares
- tamanho do pool de memória
- comprometer latência
- integridade do instantâneo/repetição

Usar:
```bash
vexod ops thresholds --json
vexod ops incident --metrics-file current.json --previous-metrics-file previous.json --window 1m
```
## Notas de segurança

- Nunca reutilize chaves do validador em cadeias independentes.
- Mantenha a política de assinante remoto habilitada para validadores de produção.
- Não admita um validador BLS sem prova de posse ou defesa equivalente de chave não autorizada.
- Não corte ou prenda um validador sem evidências verificadas vinculadas ao conjunto correto do validador de altura de evidência.

<!-- vexo-docs:technical-parity -->
## Apêndice de paridade técnica

Este apêndice garante que a tradução preserve as interfaces executáveis e as seções principais do documento canônico em inglês. Comandos, chaves de configuração, métodos RPC e nomes de pacotes permanecem inalterados em todos os idiomas.

### Rastreamento de seções
- section: 1. Initialize Validator Home — Esta seção deve ser revisada junto com valores de configuração, evidências de verificação, condições de falha e ações do operador.
- section: 2. Configure Network Addresses and Peers — Esta seção deve ser revisada junto com valores de configuração, evidências de verificação, condições de falha e ações do operador.
- section: 3. Submit Validator Admission — Esta seção deve ser revisada junto com valores de configuração, evidências de verificação, condições de falha e ações do operador.
- section: 4. Verify Validator Set Update — Esta seção deve ser revisada junto com valores de configuração, evidências de verificação, condições de falha e ações do operador.
- section: 5. Plan Validator Key Rotation — Esta seção deve ser revisada junto com valores de configuração, evidências de verificação, condições de falha e ações do operador.
- section: 6. Start Validator — Esta seção deve ser revisada junto com valores de configuração, evidências de verificação, condições de falha e ações do operador.
- section: 7. Monitor — Esta seção deve ser revisada junto com valores de configuração, evidências de verificação, condições de falha e ações do operador.
- section: Safety Notes — Esta seção deve ser revisada junto com valores de configuração, evidências de verificação, condições de falha e ações do operador.

### Interfaces mantidas sem alteração
- `VEXO_KEY_PASSPHRASE` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `--passphrase` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `bls_pop` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `blst-bls12381-minpk-v1` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `vexod keys gen --type bls --bls-adapter circl-bls12381-g1sigg2-basic-v1` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `node.key.json` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `network_config.json:p2p.node_id` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `.vexo-validator-new/network_config.json` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `network_config.json` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `p2p.listen_address` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `rpc.address` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `p2p.node_id` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `p2p.node_key_path` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `p2p.peers` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `p2p_address` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `rpc_address` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `node_id` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `active_from` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `active_until` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `config audit --strict` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
