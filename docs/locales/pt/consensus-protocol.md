> Locale: pt · Português

QUERY LENGTH LIMIT EXCEEDED. MAX ALLOWED QUERY : 500 CHARS

QUERY LENGTH LIMIT EXCEEDED. MAX ALLOWED QUERY : 500 CHARS

QUERY LENGTH LIMIT EXCEEDED. MAX ALLOWED QUERY : 500 CHARS

QUERY LENGTH LIMIT EXCEEDED. MAX ALLOWED QUERY : 500 CHARS

- menos de um terço do poder de voto bizantino
- propostas separadas por domínio, votação, timeout-vote e assinaturas de finalidade
- vinculação de hash definida pelo validador na altura de prova relevante
- signatários conhecidos únicos em CQs e provas de finalidade
- evidência responsável por equívoco do validador
- rejeição de decisões conflitantes de commit na mesma altura finalizada

## Limite de Criptomoedas

QUERY LENGTH LIMIT EXCEEDED. MAX ALLOWED QUERY : 500 CHARS

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
