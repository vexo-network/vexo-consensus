> Locale: pt · Português

# Visão geral do protocolo de consenso

Esta página é o ponto de entrada de alto nível para a documentação de consenso da Vexo. Para um mapa de documentação mais amplo, consulte [Documentação](./README.md).

Para detalhes normativos, use os arquivos de especificações:

- [Especificações de consenso](./specs/consensus-spec.md)
- [Formato de Prova de Finalidade](./specs/finality-proof-format.md)
- [Ciclo de vida do validador](./specs/validator-lifecycle.md)
- [Esquema de armazenamento](./specs/storage-schema.md)
- [Especificações de rede](./specs/networking-spec.md)
- [Formato de transação](./specs/tx-format.md)

## Modelo

Vexo usa um núcleo BFT estilo HotStuff com propostas, votos, certificados de quórum, certificados de tempo limite, segurança de controle de qualidade bloqueado e finalidade de três cadeias.

Um bloco é seguro para votação apenas quando estende o CQ bloqueado ou carrega um CQ justificado pelo menos tão novo quanto o bloqueio. Um bloco é finalizado quando a regra das três cadeias prova uma extensão segura da cadeia pai/avó.

A implementação vincula a decisão de três cadeias às alturas explícitas do bloco, dos pais e dos avós. O QC do bloco deve certificar a altura/hash do pai, e o QC do pai deve certificar a altura/hash do avô; cadeias de CQ sintéticas ou com altura ignorada são rejeitadas antes que uma decisão definitiva seja registrada.

## Termos de Execução

Vexo usa estes termos de forma consistente:

- **Certificado QC**: um bloco tem votos suficientes para formar um certificado de quórum.
- **Finalizado**: a regra de três cadeias do HotStuff finaliza um bloco ancestral.
- **Executado**: a aplicação executou `FinalizeBlock` para um bloco.
- **Estado confirmado**: gravações KV do aplicativo, registro de bloco, registro de estado e raízes de estado do módulo foram confirmadas de forma durável.

O caminho de execução do nó usa dois limites separados:

- **Limite de confirmação de execução**: um bloco certificado pelo QC pode ser executado e persistido atomicamente conforme gravações do aplicativo + registro de bloco + registro de estado + raízes de estado.
- **Limite de finalidade de consenso**: a regra das três cadeias finaliza um ancestral e é a única fonte para provas de finalidade de cliente leve.

`consensus_config.json` expõe esta escolha através de `execution_commit`. As casas do validador geradas têm como padrão `finalized`, que executa apenas o ancestral selecionado pela regra de finalidade de três cadeias para que os commits de estado se alinhem com o limite de finalidade mais estrito. O limite `qc` de latência mais baixa permanece disponível para implantações personalizadas, mas `require_network_safety` o rejeita. Operadores e usuários do SDK devem tratar os logs `block_committed` como eventos de confirmação de estado para o limite de execução configurado. As provas de finalidade descrevem a finalidade do consenso no nível do conjunto de validadores.

## Limite de segurança

A segurança depende de:

- menos de um terço do poder de voto bizantino
- proposta separada por domínio, votação, votação de tempo limite e assinaturas de finalidade
- vinculação de hash definida pelo validador na altura de prova relevante
- signatários únicos conhecidos em CQs e provas de finalidade
- evidência responsável para equívoco do validador
- rejeição de decisões de commit conflitantes na mesma altura finalizada

## Limite criptográfico

- `deterministic` é apenas para teste e falha na validação de segurança da rede.
- `ed25519` é compatível com testes de rede pública e preparação de lançamento.
- `bls` tem como padrão `blst-bls12381-minpk-v1` e requer prova de posse ou defesa equivalente de chave não autorizada, verificações de subgrupo, validação de chave pública, evidência de auditoria de dependência e evidência de liberação. O adaptador CIRCL integrado continua sendo uma integração de referência para a interface de tempo de execução e não é uma isenção de segurança de produção.
- A validação de segurança de rede requer metadados do adaptador VRF para seleção do comitê VRF. O adaptador ECVRF integrado pode satisfazer a interface de tempo de execução; O VRF determinístico permanece apenas para teste e não deve ser usado para redes com valor.

## Limite Operacional

O código inclui verificações orientadas à produção, mas as implantações públicas ainda exigem:

- auditoria de configuração rigorosa para cada página inicial do validador
- evidência de liberação
- revisão de segurança externa
- evidência de caos e longo prazo multi-host
- evidência da política do signatário/KMS
- revisão da política económica e de governação específica da cadeia

Consulte [Preparação para auditoria de segurança](./security/audit-readiness.md) e [Pipeline de lançamento](./release/release-pipeline.md) antes de tratar uma versão como pronta para produção.

<!-- vexo-docs:technical-parity -->
## Apêndice de paridade técnica

Este apêndice garante que a tradução preserve as interfaces executáveis e as seções principais do documento canônico em inglês. Comandos, chaves de configuração, métodos RPC e nomes de pacotes permanecem inalterados em todos os idiomas.

### Rastreamento de seções
- section: Model — Esta seção deve ser revisada junto com valores de configuração, evidências de verificação, condições de falha e ações do operador.
- section: Execution Terms — Esta seção deve ser revisada junto com valores de configuração, evidências de verificação, condições de falha e ações do operador.
- section: Safety Boundary — Esta seção deve ser revisada junto com valores de configuração, evidências de verificação, condições de falha e ações do operador.
- section: Crypto Boundary — Esta seção deve ser revisada junto com valores de configuração, evidências de verificação, condições de falha e ações do operador.
- section: Operational Boundary — Esta seção deve ser revisada junto com valores de configuração, evidências de verificação, condições de falha e ações do operador.

### Interfaces mantidas sem alteração
- `consensus_config.json` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `execution_commit` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `require_network_safety` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `block_committed` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `blst-bls12381-minpk-v1` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
