# Esquema de armazenamento

> Locale: pt · Português
> Este documento é uma tradução direta para o português da fonte inglesa. Decisões de protocolo, segurança e release continuam normativas em inglês.


## Ordem de leitura

Este documento explica a especificação normativa de Storage Schema. Se esta for a sua primeira leitura, siga esta ordem.

1. Scope
2. Backend
3. Records
4. Indexes
5. EVM Records
6. Recovery Rules
7. Snapshot Validation
8. Schema Migration

Essa ordem corresponde à forma correta de ler o documento: primeiro o escopo e o estado, depois as regras de mensagens, segurança e vivacidade, e por fim as evidências.

## Visão geral

Este documento ajuda a entender namespace de durable storage, key schema e recovery marker e a conectar isso a decisões de implementação e operação.

- Canonical path: `docs/specs/storage-schema.md`
- Locale path: `docs/locales/pt/specs/storage-schema.md`

## Por que ler este documento

- namespace de durable storage, key schema e recovery marker
- Confira primeiro as frases MUST/SHOULD/MAY na fonte inglesa.
- Esta página é uma tradução direta do original em inglês; auditoria, release e segurança continuam sendo decididos pela fonte inglesa.

## O que você deve conseguir fazer

- Explicar qual decisão de implementação ou operação este documento apoia.
- Relacionar os requisitos normativos da fonte inglesa com a configuração atual da rede.
- Verificar chain ID, validator ID, fee/gas e endereços peer antes de copiar exemplos.

## Checklist de uso seguro

- Confira primeiro as frases MUST/SHOULD/MAY na fonte inglesa.
- Não traduza comandos, config key, nomes RPC, campos JSON nem identificadores de código.
- Antes de copiar exemplos, ajuste chain ID, validator ID, fee/gas e endereços peer para sua rede.
- Após alterar os documentos, execute `make docs-check` para verificar a árvore local e os controles de tradução.

## Pontos de atenção

- Esta página é uma tradução direta do original em inglês; auditoria, release e segurança continuam sendo decididos pela fonte inglesa.
- Quando a implementação mudar, atualize a fonte inglesa e todos os documentos localizados na mesma alteração.

## Interfaces que devem ser preservadas

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

## Estrutura da fonte inglesa

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

## Fonte canônica

- [Documento canônico em inglês](../../en/specs/storage-schema.md)

## Backend Pebble e registros binários

O backend padrão é o Pebble (`github.com/cockroachdb/pebble/v2`), aberto com cache de blocos de 256 MiB e memtables de 64 MiB. Os registros são armazenados na codificação binária canônica do pacote `types`: uvarints mínimos, cadeias de bytes e listas prefixadas pelo comprimento, hashes como seus 32 bytes, um valor opcional após um byte de presença `0` ou `1`, e os campos na ordem da estrutura que os declara. Um decodificador rejeita qualquer outra coisa, inclusive bytes sobrando, então um registro danificado é reportado em vez de lido como outro valor; cada registro tem uma única codificação. Um diretório de dados escrito por um build anterior (LevelDB, registros JSON) não é legível nem convertido: o Pebble se recusa a abri-lo com um erro de `format major version 1`. Restaure esse nó a partir de um snapshot ou sincronize-o em um diretório de dados vazio.

Identificadores relacionados:

- `pebble: database "<dir>" was written in format major version 1, which is no longer supported`

## Histórico de estado (undo log) e cache da árvore

O histórico de estado é um undo log de versões por chave. Quando uma chave muda numa altura (as escritas de um bloco na sua altura, uma escrita direta (`Set`, `Delete`, `SetBatch`) após o primeiro estado confirmado na próxima altura confirmada), o store registra o valor que a chave tinha antes, ausência incluída, como versão da chave nessa altura, uma vez por chave e altura; escritas diretas anteriores ao bloco que confirma sua altura mantêm a versão que a primeira registrou (`hdirect` indica essa altura). O valor de uma chave numa altura confirmada é o que registra sua versão mais baixa acima dessa altura, ou seu valor atual se não houver nenhuma. Registrar uma mudança só lê o valor atual da chave, nunca seu histórico, e uma leitura histórica é um seek nas versões da chave. `PruneBelow` apaga toda versão na altura a partir da qual retém ou abaixo dela. Os nós da árvore são versionados do mesmo modo. Duas estruturas em memória reduzem o trabalho de árvore de um bloco; nenhuma é persistida nem muda um resultado: o cache de nós da árvore, com até 2^18 nós codificados das árvores atuais (todo batch do store o atualiza, e uma geração ímpar enquanto tal batch é confirmado faz um leitor usar o cache só no estado do seu snapshot), e, por namespace, a raiz e as mudanças de nós do último cálculo de raiz, indexadas por essa geração e um digest SHA-256 das escritas do namespace, de modo que o app hash, as state roots e o commit calculam as mesmas escritas uma única vez.

## Compactação do WAL do mempool e batch de genesis

Um commit compacta o WAL do mempool só quando ele contém pelo menos 4 MiB e o dobro do que continha após a última compactação ou abertura. Assim cada byte acrescentado é reescrito em média cerca de uma vez, e o log que um reinício reproduz fica dentro do dobro do conteúdo vivo ou de 4 MiB. Uma compactação explícita (`Node.Compact`, ou um início que descarta transações reproduzidas que a aplicação recusa) sempre o reescreve. `InitChain` grava o estado de genesis de todos os módulos como um único batch, de modo que um genesis com muitas contas custa uma escrita durável e um genesis que falha não deixa nada no store.

## Finality proofs no batch do commit e remoção adiada do mempool

O caminho de commit do nó grava as finality proofs que o QC do bloco completa no mesmo batch de block/state/app-write (`CommitBlockStateWithWrites` as recebe como último argumento). Assim um commit custa uma escrita durável em vez de uma a mais por proof. Uma proof que o store recusaria fica fora do batch e é gravada pela etapa posterior ao commit, que relata a falha. Um commit remove suas transações do pool e enfileira o registro de remoção sem esperar seu sync. O commit seguinte espera esse registro ficar durável antes de executar seu bloco, porque um reinício refaz a remoção apenas para o último bloco confirmado. Em geral o registro compartilha o sync de uma admissão posterior.

## Hash adiado do state tree

O cálculo do state tree adia o hash: um write marca as metades que mudou, e cada node alterado é hasheado uma única vez, de baixo para cima, quando a root, uma proof ou as mudanças são pedidas. Assim os nodes perto da root, que todo write do bloco reescreve, são hasheados uma vez por bloco, e não uma vez por write. Os hashes, os nodes e as roots são os mesmos de hashear a cada write.

<!-- vexo-docs:technical-parity -->
## Apêndice de paridade técnica

Este apêndice garante que a tradução preserve as interfaces executáveis e as seções principais do documento canônico em inglês. Comandos, chaves de configuração, métodos RPC e nomes de pacotes permanecem inalterados em todos os idiomas.

### Rastreamento de seções
- section: Scope — Esta seção deve ser revisada junto com valores de configuração, evidências de verificação, condições de falha e ações do operador.
- section: Backend — Esta seção deve ser revisada junto com valores de configuração, evidências de verificação, condições de falha e ações do operador.
- section: Records — Esta seção deve ser revisada junto com valores de configuração, evidências de verificação, condições de falha e ações do operador.
- section: Indexes — Esta seção deve ser revisada junto com valores de configuração, evidências de verificação, condições de falha e ações do operador.
- section: EVM Records — Esta seção deve ser revisada junto com valores de configuração, evidências de verificação, condições de falha e ações do operador.
- section: Recovery Rules — Esta seção deve ser revisada junto com valores de configuração, evidências de verificação, condições de falha e ações do operador.
- section: Snapshot Validation — Esta seção deve ser revisada junto com valores de configuração, evidências de verificação, condições de falha e ações do operador.
- section: Schema Migration — Esta seção deve ser revisada junto com valores de configuração, evidências de verificação, condições de falha e ações do operador.

### Interfaces mantidas sem alteração
- `store.Store` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `eth_getBalance` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `eth_getProof` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `bank/{0x_address}` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `auth/nonce/{0x_address}` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `evm/code/{0x_address}` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `evm/storage/{0x_address}/{slot}` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `seen_ttl` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `code/{address}` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `storage/{address}/{slot}` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `receipts/{tx_hash}` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `logs/by_height/{height}/{tx_hash}/{log_index}` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `logs/by_address/{address}/{height}/{tx_hash}/{log_index}` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `logs/{address}` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `evm_receipts` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `receipt_index/{tx_hash}` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `blob_sidecars/by_tx/{tx_hash}` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `blob_sidecars/by_blob/{blob_hash}` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `tree:` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `receipt_height/{height}/{tx_hash}` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `receipt_pruned_below` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
