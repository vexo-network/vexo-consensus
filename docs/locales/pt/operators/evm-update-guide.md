# Guia de atualização de EVM

> Locale: pt · Português
> Este documento é a tradução em português da fonte inglesa. As decisões de protocolo, segurança e release seguem a fonte inglesa.

Este guia explica como atualizar a pilha EVM integrada sem quebrar o tratamento de chain ID, a compatibilidade Web3 ou as evidências de release. Ele é voltado para operadores e mantenedores que precisam atualizar go-ethereum, ajustar fork presets ou alterar o comportamento de EVM em uma release controlada.

## O que conta como atualização de EVM

Trate como atualização sensível para release qualquer mudança que possa afetar execução estilo Ethereum ou comportamento visível para Web3:

- atualização de versão de `go-ethereum` em `modules/evm/backend/geth`
- mudanças em `modules/evm/ethcompat`
- mudanças em `modules/evm`
- mudanças em `execution.evm_fork_preset`
- mudanças em `execution.evm_chain_config_json`
- mudanças em admissão de raw transactions, gas accounting, receipts, traces, proofs ou campos de resposta de blocos
- mudanças no tratamento de contas Web3 gerenciadas como `eth_accounts`, `eth_coinbase`, `eth_sign`, `eth_signTransaction` ou `eth_sendTransaction`

## Ordem segura de atualização

Siga esta ordem para manter código, configuração e documentação alinhados:

1. Atualize primeiro o adapter geth isolado.
2. Atualize depois o corpus de fixtures e os testes de conformance.
3. Se a semântica mudar, atualize `docs/specs/evm-native-accounting.md`, `docs/specs/tx-format.md` e `docs/sdk/rpc-api-versioning.md`.
4. Se o formato de release evidence mudar, atualize `docs/release/release-pipeline.md`.
5. Se os controles visíveis para o operador mudarem, atualize a documentação de configuração do nó.
6. Execute novamente a matriz de validação antes do merge.

Não aumente a versão runtime do EVM e a publique ao mesmo tempo, a menos que as suites de conformance, os smoke checks RPC e as verificações Docker já tenham passado.

## Fluxo de atualização

### 1. Fixar o escopo

Registre com precisão a intenção da mudança:

- apenas fork behavior
- apenas transaction admission
- apenas execution semantics
- apenas RPC compatibility
- apenas tratamento de blob / receipt / trace
- apenas comportamento de contas gerenciadas ou wallets

Essa divisão mantém a revisão focada e evita que código sem relação se mova junto.

### 2. Alterar na camada mais estreita

Prefira estes limites:

- `modules/evm/backend/geth` para mudanças de integração com o upstream go-ethereum
- `modules/evm/ethcompat` para raw transaction decoding, preservação de hash e tratamento de fixtures
- `modules/evm` para state transition, receipts, logs, storage e snapshots
- `rpc` para mudanças na superfície Web3 request/response
- `cmd/vexod` apenas quando o CLI ou o release workflow precisarem expor o novo comportamento

Se a mudança alcançar application modules, mantenha explícita a fronteira do módulo e preserve escritas de estado determinísticas.

### 3. Atualizar a configuração padrão

Quando a semântica mudar, atualize a configuração padrão no mesmo patch:

- `execution.evm_fork_preset`
- `execution.evm_chain_config_json`
- `execution.allow_unprotected_legacy_tx`
- se necessário, os campos RPC de contas gerenciadas em `network_config.json`
- o EVM chain ID em `module_config.json`

Nunca dependa de um flag CLI oculto para explicar o comportamento em runtime. A configuração deve deixar claro como o nó se comporta apenas pelos arquivos.

### 4. Executar a pilha de conformance

No mínimo, rode:

```bash
make evm-conformance
go test ./modules/evm -count=1
go test ./rpc -count=1
```

Depois verifique os fluxos visíveis para o usuário que costumam quebrar primeiro:

```bash
curl -s http://127.0.0.1:26657/web3 \
  -H 'content-type: application/json' \
  -d '{"jsonrpc":"2.0","id":1,"method":"eth_chainId","params":[]}'
```

Em implantações Docker single-host, confirme também:

```text
http://127.0.0.1:28657/web3
```

Confira pelo menos estes comportamentos:

- `eth_chainId`
- `eth_blockNumber`
- `eth_gasPrice`
- `eth_call`
- `eth_estimateGas`
- `eth_sendRawTransaction`
- `eth_getTransactionReceipt`
- `eth_getBalance`
- `eth_getCode`
- `eth_getStorageAt`
- `eth_getProof`

Depois faça deploy de um contrato simples, de um proxy contract e do fluxo UUPS upgrade usando o mesmo endpoint RPC que a wallet ou a ferramenta usará em produção.

### 5. Confirmar proxy e upgrade

A atualização de EVM só termina quando tudo isso for verdadeiro:

- um deploy normal de contrato funciona
- um deploy de proxy funciona
- uma chamada UUPS upgrade funciona
- após o upgrade, leituras de storage e code retornam o esperado
- o nonce tracking continua monotônico
- o block producer aceita as transações resultantes sem erros unsafe proposal

Se o deploy do proxy funciona, mas o upgrade falha, ainda não está pronto para publicar. Trate isso como release blocker, não como aviso.

### 6. Atualizar evidências

Quando a superfície de EVM mudar, atualize também o bundle de release evidence:

- `--evm-tx-fixtures`
- `--evm-execution-fixtures`
- `--evm-default-fixtures`
- `--evm-web3-conformance-evidence`
- qualquer referência SHA-256 fixada

A release evidence deve dizer o que mudou, o que foi testado e qual commit ou versão foi verificada. Não descreva uma atualização de EVM como concluída se a evidência não bater com o código que realmente foi executado.

## Matriz de validação

Use esta tabela como merge gate.

| Check | Por que importa |
| --- | --- |
| `make evm-conformance` | detecta regressões de fork rule e execução |
| `go test ./modules/evm -count=1` | verifica receipts, logs, storage, balances e snapshots |
| `go test ./rpc -count=1` | verifica compatibilidade Web3 request/response |
| `make network-e2e` | confirma que o nó ainda sobe, encontra peers e faz commit |
| Docker single-host smoke | confirma o caminho usado pelo Remix e pelas ferramentas de browser |
| Contract deploy | confirma admissão de transações e geração de receipts |
| Proxy deploy | confirma suposições de ABI e storage layout |
| UUPS upgrade | confirma semântica de upgrade e leituras após upgrade |

Se qualquer check estiver vermelho, não diga que a atualização terminou.

## Critérios de rollback

Faça rollback da atualização de EVM quando qualquer uma destas coisas acontecer:

- `eth_chainId` muda inesperadamente
- `eth_sendRawTransaction` começa a rejeitar transações válidas
- `eth_call` ou `eth_estimateGas` divergem das fork rules esperadas
- receipts, logs ou proofs deixam de bater com o committed state
- transações de proxy ou upgrade começam a falhar
- a release evidence não combina mais com o caminho atual do código

O rollback deve restaurar juntos a última versão boa do adapter, os defaults de config e o conjunto de fixtures.

## Apêndice de paridade técnica

Este apêndice mantém o guia alinhado com o restante da árvore documental.

- Mantenha `modules/evm/backend/geth`, `modules/evm/ethcompat`, `modules/evm`, `rpc` e `cmd/vexod` como fronteiras estáveis de implementação.
- Mantenha sem alteração de escrita `execution.evm_fork_preset`, `execution.evm_chain_config_json`, `execution.allow_unprotected_legacy_tx`, `eth_chainId`, `eth_call`, `eth_estimateGas`, `eth_sendRawTransaction`, `eth_getTransactionReceipt`, `eth_getProof`, `eth_getStorageAt`, `eth_accounts`, `eth_coinbase`, `eth_signTransaction` e `eth_sendTransaction`.
- Mantenha também sem alteração `make evm-conformance`, `make network-e2e`, `--evm-default-fixtures`, `--evm-tx-fixtures`, `--evm-execution-fixtures` e `--evm-web3-conformance-evidence`.
- A pergunta operacional continua simples: esta atualização preserva execução estilo Ethereum e ainda se encaixa na segurança de Vexo consensus e release?

- Keep `go test -race ./rpc -count=1` in the verification matrix to catch managed nonce allocation and pending-state races.

<!-- vexo-docs:technical-parity -->
