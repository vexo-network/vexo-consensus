# Documentação

> Locale: pt · Português
> Este documento é um documento de apoio em português para ser lido junto da fonte inglesa. Decisões de protocolo, segurança e release continuam normativas em inglês.

## Início rápido

- Use `make build` para compilar o binário.
- Crie um validator home com `vexod init validator --home .vexo-validator-1 --chain-id vexo-chain --validator validator-1 --encrypt-keys`, valide com `vexod validate --home .vexo-validator-1` e `vexod config audit --home .vexo-validator-1 --strict`, e depois inicie com `vexod start --home .vexo-validator-1`.
- Execute a rede Docker primeiro com `docker compose -f deployments/docker/compose.single-host-init.yml up` e depois com `docker compose -f deployments/docker/compose.single-host.yml up`.
- O Remix deve usar `http://127.0.0.1:28657/web3`; para verificar o chain ID use `curl -s http://127.0.0.1:26657/web3 -H 'content-type: application/json' -d '{"jsonrpc":"2.0","id":1,"method":"eth_chainId","params":[]}'`.
- Depois de alterar documentos, execute `make docs-check` para validar o locale tree e os guards de tradução.
## Visão geral

Este documento ajuda a entender o índice da documentação e a ordem de leitura recomendada e a conectar isso a decisões de implementação e operação.

- Canonical path: `docs/README.md`
- Locale path: `docs/locales/pt/README.md`

## Por que ler este documento

- o índice da documentação e a ordem de leitura recomendada
- Confira primeiro as frases MUST/SHOULD/MAY na fonte inglesa.
- Este documento localizado ajuda na compreensão; auditoria, release e segurança são decididos pela fonte inglesa.

## O que você deve conseguir fazer

- Explicar qual decisão de implementação ou operação este documento apoia.
- Relacionar os requisitos normativos da fonte inglesa com a configuração atual da rede.
- Verificar chain ID, validator ID, fee/gas e endereços peer antes de copiar exemplos.

## Checklist de uso seguro

- Confira primeiro as frases MUST/SHOULD/MAY na fonte inglesa.
- Não traduza comandos, config key, nomes RPC, campos JSON nem identificadores de código.
- Antes de copiar exemplos, ajuste chain ID, validator ID, fee/gas e endereços peer para sua rede.
- Após alterar documentos, execute `make docs-check` para verificar locale tree e guards de tradução.

## Pontos de atenção

- Este documento localizado ajuda na compreensão; auditoria, release e segurança são decididos pela fonte inglesa.
- Quando a implementação mudar, atualize a fonte inglesa e todos os documentos localizados na mesma alteração.

## Interfaces que devem ser preservadas

- `vexo-consensus`
- `/v1/*`
- `docs/locales/{en,ko,zh,ja,fr,de,es,pt,ru,ar,hi,id,vi}/`
- `make docs-check`

## Estrutura da fonte inglesa

- Documentation
- How to Read This Set
- Protocol Specs
- SDK and Extension Guides
- Operations and Release
- Security
- Localized Documentation
- Writing New Docs
- Documentation Review Checklist

## Fonte canônica

- [Documento canônico em inglês](../en/README.md)

## Lista de termos preservados

Os termos abaixo não devem ser traduzidos.

- `vexo-consensus`
- `make build`
- `vexod validate --home .vexo-validator-1`
- `vexod config audit --home .vexo-validator-1 --strict`
- `vexod start --home .vexo-validator-1`
- `curl -s http://127.0.0.1:26657/v1/status`
- `docker compose -f deployments/docker/compose.single-host-init.yml up`
- `docker compose -f deployments/docker/compose.single-host.yml up`
- `http://127.0.0.1:28657/web3`
- `/v1/*`
- `docs/locales/{en,ko,zh,ja,fr,de,es,pt,ru,ar,hi,id,vi}/`
- `make docs-check`
- `vexod status --json`
- `feature_assurance`
- `network_config.json:p2p.auth_replay_path`
- `network_config.json:p2p.node_key_path`
- `module_config.json:governance.RequireDeposit`
- `module_config.json:governance.MinDeposit`
- `consensus_config.json:consensus.execution_commit`
- `mempool_config.json:mempool.WALPath`

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
