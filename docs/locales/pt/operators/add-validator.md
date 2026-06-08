# Adding a Validator

> Locale: pt · Português
> Este documento é um guia traduzido a partir da documentação canônica em inglês. Decisões de protocolo, segurança e release continuam normativas em inglês.

## Objetivo

Este documento cobre adição de validator, validação de configuração e verificações de staking. Comandos, campos JSON, nomes RPC, config key e identificadores de código usados na implementação e operação permanecem em inglês por compatibilidade.

## Escopo principal

- Verifique os itens abaixo ao ler este documento. Comandos, campos JSON, métodos RPC, chaves de configuração e identificadores de código permanecem em inglês por compatibilidade.
- Para texto normativo detalhado, use o documento em inglês.
- Canonical path: `docs/operators/add-validator.md`
- Locale path: `docs/locales/pt/operators/add-validator.md`

## Identificadores preservados

- `VEXO_KEY_PASSPHRASE`
- `--passphrase`
- `bls_pop`
- `.vexo-validator-new/network_config.json`
- `network_config.json`
- `p2p.listen_address`
- `rpc.address`
- `p2p.peers`
- `p2p_address`
- `rpc_address`
- `active_from`
- `active_until`
- `config audit --strict`

## Seções em inglês

- Adding a Validator
- 1. Initialize Validator Home
- 2. Configure Network Addresses and Peers
- 3. Submit Validator Admission
- 4. Verify Validator Set Update
- 5. Plan Validator Key Rotation
- 6. Start Validator
- 7. Monitor
- Safety Notes

## Notas operacionais

- `MUST`, `SHOULD`, `MAY`, exemplos de comando, exemplos JSON e nomes RPC preservam a grafia em inglês.
- Após alterar esta tradução, execute `make docs-check`.
- Se esta página divergir da fonte inglesa, use a fonte inglesa e atualize este arquivo locale na mesma mudança.

## Fonte canônica

- [English canonical document](../../en/operators/add-validator.md)
