> Locale: pt · Português

# Documentação

Este diretório é o manual prático do `vexo-consensus`. Destina-se a developers, operadores, responsáveis por release e revisores que precisam entender a rede sem deduzir seu comportamento apenas do código-fonte.

Cada página deve explicar responsabilidade, arquivos, comandos, chaves de configuração e APIs da implementação, condições de segurança e evidências exigidas antes de uma rede real. O inglês continua sendo a fonte normativa para protocolo, segurança, release, SDK, comandos, configuração e RPC; esta tradução auxilia a leitura, mas não substitui a fonte inglesa em decisões de auditoria.

Para começar, use os comandos abaixo e depois leia `Node Initialization`, `Docker Deployment`, `Observability Guide` e `RPC API Versioning`.

| Tarefa | Caminho do Comando |
|---|---|
| Crie um binário local | `make build` |
| Criar uma página inicial do validador | __ VEXO_CODE_1__ |
| Valide uma acomodação | `vexod validate --home .vexo-validator-1` e `vexod config audit --home .vexo-validator-1 --strict` |
| Execute um nó | `vexod start --home .vexo-validator-1` |
| Consulta um nó | `curl -s http://127.0.0.1:26657/v1/status` |
| Execute a rede de quatro validadores do Docker | __ VEXO_CODE_5__ seguido de __ VEXO_CODE_6__ |
| Connect Remix | Use o validador Docker 1 URL Web3 `http://127.0.0.1:28657/web3` |
| Verifique o ID da cadeia Web3 | __ VEXO_CODE_7__ |

## Início rápido

- `make build`
- `vexod init validator --home .vexo-validator-1 --chain-id vexo-chain --validator validator-1 --encrypt-keys`
- `vexod validate --home .vexo-validator-1`
- `vexod config audit --home .vexo-validator-1 --strict`
- `vexod start --home .vexo-validator-1`
- `curl -s http://127.0.0.1:26657/v1/status`
- `docker compose -f deployments/docker/compose.single-host-init.yml up`
- `docker compose -f deployments/docker/compose.single-host.yml up`
- `http://127.0.0.1:28657/web3`
## Comece aqui

| Documento | Finalidade |
|---|---|
| [Production Readiness Guide](./production-readiness.md) | Mapa único de protocolo, tempo de execução, operações, evidências e prontidão de lançamento |

## Especificações do protocolo

- [Consensus Spec](./specs/consensus-spec.md), [Finality Proof Format](./specs/finality-proof-format.md) e [Validator Lifecycle](./specs/validator-lifecycle.md) descrevem segurança, finalidade e mudanças no validator set.
- [Networking Spec](./specs/networking-spec.md), [Storage Schema](./specs/storage-schema.md) e [Transaction Format](./specs/tx-format.md) cobrem transporte, recuperação durável e admissão de transações.
- [EVM and Native Accounting](./specs/evm-native-accounting.md) define a fronteira entre contabilidade nativa e EVM.

## SDK e extensões

[App Module Guide](./sdk/app-module-guide.md), [Custom Crypto Backend](./sdk/custom-crypto-backend.md), [Custom Storage and Transport](./sdk/custom-storage-transport.md) e `RPC API Versioning` mostram como estender o runtime sem quebrar contratos de consenso ou RPC.

## Operação, release e segurança

`Node Initialization`, [Adding a Validator](./operators/add-validator.md), `Observability Guide`, [Runbook de lançamento](./release/launch-runbook.md), `Release Pipeline` e [Version Compatibility Matrix](./release/version-compatibility.md) formam o percurso operacional. [Security Audit Readiness](./security/audit-readiness.md) registra o modelo de ameaça e evidências obrigatórias.

## Regra de maturidade

Código existente não comprova prontidão para produção. São necessários testes unitários, adversariais e E2E, artefatos operacionais, suposições e modos de falha e resultados do release gate. Comandos, métodos RPC e chaves de configuração permanecem iguais em todas as traduções.

## Pesquisa e publicação

Para preparar um artigo, comece por [`Adaptive Recovery-Gated HotStuff Research Draft`](./research/adaptive-recovery-hotstuff-paper.md). O documento separa os mecanismos realmente implementados, como timeout adaptativo de rodada, barreira de finalidade durante recuperação e ordenação determinística de transações, dos trabalhos anteriores. Ele reúne perguntas de pesquisa, hipóteses, protocolo experimental, artefatos reproduzíveis e ética de pesquisa. Desempenho ainda não medido não é apresentado como resultado, e PoS, BFT ou HotStuff não são reivindicados como novas invenções.

Os nomes normativos mantidos para navegação entre idiomas são `Node Initialization`, `Docker Deployment`, `Observability Guide`, `RPC API Versioning`, `Production Readiness`, `Release Pipeline` e `Adaptive Recovery-Gated HotStuff Research Draft`.

<!-- vexo-docs:technical-parity -->
## Apêndice de paridade técnica

Este apêndice garante que a tradução preserve as interfaces executáveis e as seções principais do documento canônico em inglês. Comandos, chaves de configuração, métodos RPC e nomes de pacotes permanecem inalterados em todos os idiomas.

### Rastreamento de seções
- section: How to Read This Set — Esta seção deve ser revisada junto com valores de configuração, evidências de verificação, condições de falha e ações do operador.
- section: Start Here — Esta seção deve ser revisada junto com valores de configuração, evidências de verificação, condições de falha e ações do operador.
- section: Protocol Specs — Esta seção deve ser revisada junto com valores de configuração, evidências de verificação, condições de falha e ações do operador.
- section: SDK and Extension Guides — Esta seção deve ser revisada junto com valores de configuração, evidências de verificação, condições de falha e ações do operador.
- section: Operations and Release — Esta seção deve ser revisada junto com valores de configuração, evidências de verificação, condições de falha e ações do operador.
- section: Security — Esta seção deve ser revisada junto com valores de configuração, evidências de verificação, condições de falha e ações do operador.
- section: Localized Documentation — Esta seção deve ser revisada junto com valores de configuração, evidências de verificação, condições de falha e ações do operador.
- section: Writing New Docs — Esta seção deve ser revisada junto com valores de configuração, evidências de verificação, condições de falha e ações do operador.
- section: Production Claim Rule — Esta seção deve ser revisada junto com valores de configuração, evidências de verificação, condições de falha e ações do operador.
- section: Documentation Review Checklist — Esta seção deve ser revisada junto com valores de configuração, evidências de verificação, condições de falha e ações do operador.

### Interfaces mantidas sem alteração
- `vexo-consensus` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `/v1/*` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `docs/locales/{en,ko,zh,ja,fr,de,es,pt,ru,ar,hi,id,vi}/` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `make docs-check` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `vexod status --json` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `feature_assurance` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `network_config.json:p2p.auth_replay_path` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `network_config.json:p2p.node_key_path` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `module_config.json:governance.RequireDeposit` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `module_config.json:governance.MinDeposit` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `consensus_config.json:consensus.execution_commit` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `mempool_config.json:mempool.WALPath` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
