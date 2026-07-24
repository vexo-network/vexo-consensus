> Locale: pt · Português

# Visão geral do protocolo de consenso

Esta página é a entrada de alto nível para a documentação de consenso Vexo. Os detalhes normativos estão em [Consensus Spec](./specs/consensus-spec.md), [Finality Proof Format](./specs/finality-proof-format.md), [Validator Lifecycle](./specs/validator-lifecycle.md), [Storage Schema](./specs/storage-schema.md), [Networking Spec](./specs/networking-spec.md) e [Transaction Format](./specs/tx-format.md).

## Modelo

Vexo usa um núcleo BFT no estilo HotStuff com proposal, vote, quorum certificate(QC), timeout certificate, segurança locked-QC e finalidade de três cadeias. Só é seguro votar num bloco quando ele estende o locked QC ou contém um justify QC pelo menos tão recente. Cadeias QC sintéticas ou que saltam alturas sem vincular explicitamente alturas e hashes de bloco, pai e avô são rejeitadas antes da decisão de finalidade.

## Identidade do protocolo e limite da pesquisa

Vexo não é um novo nome para HotStuff sem modificações nem o mesmo protocolo ou implementação que AptosBFT, DiemBFT, Jolteon, Ditto, Tendermint ou CometBFT. Um runtime Go separado combina conceitos de segurança da família HotStuff com tempo de rodada adaptativo, recuperação durável, ordem determinística de transações, execução modular e validator sets versionados por altura.

O caminho ativo de votação usa o validator set completo da altura e proposer determinístico. O seletor VRF committee está disponível como componente e consulta, mas ainda não controla proposal eligibility ou quorum formation. Portanto deve ser descrito como trabalho futuro. Consulte [Adaptive Recovery-Gated HotStuff for Modular Proof-of-Stake Networks](./research/adaptive-recovery-hotstuff-paper.md) para contribuições e protocolo experimental.

## Limite de execução e recuperação

Certificação QC, finalização HotStuff, execução da aplicação e commit de estado são eventos distintos. O padrão `execution_commit=finalized` executa apenas o ancestral escolhido pela regra de três cadeias. O pacemaker adaptativo e `recovery_finality_gate_enabled` controlam latência e recuperação sem alterar proposer, quorum power, safe-vote ou finalidade.

## Limite de segurança

- menos de um terço do poder de voto bizantino
- propostas separadas por domínio, votação, timeout-vote e assinaturas de finalidade
- vinculação de hash definida pelo validador na altura de prova relevante
- signatários conhecidos únicos em CQs e provas de finalidade
- evidência responsável por equívoco do validador
- rejeição de decisões conflitantes de commit na mesma altura finalizada

## Limite criptográfico

- O backend `deterministic` é somente para testes e falha na validação network safety.
- `ed25519` é suportado para testes de rede pública e preparação de lançamento.
- `bls` usa por padrão `blst-bls12381-minpk-v1` e requer proof-of-possession, verificação de subgrupo, validação de chave, auditoria de dependências e evidência release-gate.
- A validação exige metadados do adaptador VRF, mas isso não significa que VRF committee esteja no consenso ativo.

- auditoria de configuração rigorosa para cada página inicial do validador
- prova release-gate
- revisão DE segurança externa
- evidências de caos e de longo prazo de vários anfitriões
- evidência da política do signatário/KMS
- revisão da política económica e de governação específica da cadeia

Consulte [Security Audit Readiness](./security/audit-readiness.md) e [Release Pipeline](./release/release-pipeline.md) antes de tratar uma versão como pronta para produção.

<!-- vexo-docs:technical-parity -->
## Apêndice de Paridade Técnica

Este apêndice resume o que precisa permanecer igual à versão inglesa: interfaces executáveis, chaves de configuração e limites operacionais. Os nomes de comandos, rotas RPC e identificadores de código não são traduzidos. O texto abaixo explica o significado, mas preserva os valores que o software e a operação esperam ver exatamente.
`require_network_safety` e `block_committed` são termos críticos que devem permanecer intactos.
- `config.json`
- `module_config.json`
- `network_config.json`
- `consensus_config.json`
- `mempool_config.json`
- `log_config.json`

### Rastreamento de seções
- section: Model - HotStuff, three-chain finality, QC, timeout certificate e locked-QC safety devem ser lidos em conjunto.
- section: Execution Terms - QC certified, finalized, executed e state committed têm significados operacionais diferentes.
- section: Safety Boundary - menos de 1/3 de poder Byzantino, domain separation, validator-set hash binding e accountable evidence são requisitos de segurança.
- section: Crypto Boundary - `deterministic`, `ed25519`, `bls`, `blst-bls12381-minpk-v1` e `ecvrf-p256-sha256-tai-v1` precisam ser tratados com o mesmo cuidado.
- section: Operational Boundary - `vexo_quorum_health_ratio`, `adaptive_round_timeout_enabled`, `recovery_finality_gate_enabled` e snapshot/replay health são sinais de operação.

### Interfaces mantidas sem alteração
- `/v1/status`
- `/v1/metrics`
- `/v1/diagnostics`
- `/v1/finality/latest`
- `/v1/state/latest`
- `/v1/recovery/report`
- `execution_commit`
- `finalized`
- `qc`
- `adaptive_round_timeout_enabled`
- `recovery_finality_gate_enabled`
- `vexo_quorum_health_ratio`
- `blst-bls12381-minpk-v1`
- `ecvrf-p256-sha256-tai-v1`
- `proof-of-possession`
- `remote signer`
- `three-chain finality`
