# HotStuff adaptativo com gate de recuperação para redes Proof-of-Stake modulares

> Locale: pt · Português  
> Tipo de documento: manuscrito de pesquisa e protocolo de reprodutibilidade  
> Estado: rascunho fundamentado na implementação; alegações de desempenho exigem artefatos medidos.

## Resumo

Este manuscrito estuda replicação de máquina de estados BFT no estilo HotStuff para redes Proof-of-Stake modulares. A implementação combina finality de três cadeias e validator sets versionados por altura com três mecanismos operacionais. Um controlador limitado adapta o round timeout usando latências p95 de proposal, vote e commit e a saúde dos peers ativos. Um recovery finality gate adia commits de aplicação finalizados quando o histórico durável de blocos e o histórico de estado divergem acima de uma altura segura comum. Por fim, uma ordenação determinística remove a ordem de chegada local do mempool para um conjunto idêntico de transações, preservando as dependências de nonce de cada signer.

A contribuição não afirma que PoS, BFT, HotStuff, view synchronization adaptativa ou order fairness sejam novos. A pergunta é se essa composição limitada de controle, recuperação e ordenação reduz timeouts evitáveis e inconsistências de recuperação sem modificar a regra de segurança HotStuff de base. Fatos implementados, hipóteses refutáveis e conclusões que ainda precisam de experimentos ficam separados. Nenhum ganho de throughput ou latency deve ser publicado antes de repetições com binary, configuração, topologia e workload fixados.

## Questões de pesquisa

RQ1 compara a política adaptativa com o mesmo sistema em fixed timeout sob latência variável, medindo quantidade de timeout e p95 commit latency. RQ2 injeta falhas de storage e restart para verificar se o gate impede o estado da aplicação de avançar além da altura durável comum. RQ3 permuta o mesmo conjunto de transações e exige ordem de proposta idêntica e nonces crescentes por signer. RQ4 mede o custo adicional de CPU, memória, rede e latência em condições estáveis sem falhas.

H1 a H4 são hipóteses direcionais e falseáveis, não resultados. A existência do código não demonstra melhoria. Se o benefício não for significativo, o resultado negativo ou a fronteira de aplicação deve ser relatado sem exagero.

## Trabalhos anteriores e limite de novidade

HotStuff já apresentou BFT liderado sob partial synchrony, quorum certificates, chained commit, comunicação linear no caminho favorável e responsividade. LibraBFT/DiemBFT e AptosBFT já combinaram descendentes de HotStuff com governança de validadores ponderada por stake. Jolteon e Ditto estudam menor latência, adaptação de rede e fallback assíncrono; Fever trata view synchronization responsiva. Tendermint é outra linhagem PoS BFT por rounds. Narwhal/Tusk separa disseminação confiável e ordenação. Aequitas, Wendy e Themis definem propriedades de fairness mais fortes do que a determinação por hash usada aqui.

Logo, não se pode usar “primeira blockchain PoS+BFT”, “primeira rede PoS com HotStuff”, “idêntica ao AptosBFT”, “liveness assíncrona” ou “complexidade ótima” sem prova, “proteção MEV completa”, nem “production-ready” com base em teste single-host. A contribuição candidata é mais estreita: integrar um bounded feedback controller, um local durable-history commit gate e ordenação determinística consciente de nonce em um nó PoS modular em Go e avaliá-los de forma reproduzível contra baselines fixed e gate-disabled.

## Modelo e mecanismos

Na altura h, Vh é o conjunto ativo e Ph seu voting power total. Um QC é válido quando signers conhecidos e únicos fornecem pelo menos dois terços de Ph. O conjunto e seu hash são versionados por altura. A admissão pode ser permissionless com stake mínimo, limitada por quantidade ou restrita por configuração. Essa camada cuida de Sybil resistance e governance; ela não muda o fault threshold BFT.

A rede é parcialmente síncrona. Safety depende de menos de um terço de poder bizantino, signatures válidas, binding correto do validator set e storage durável. Liveness também requer que o atraso eventualmente fique limitado, um quorum honesto permaneça alcançável, signers estejam disponíveis e haja conectividade suficiente. Não há promessa de progresso em uma rede permanentemente assíncrona.

EVM é um workload de aplicação sob Vexo consensus. Executar bytecode Ethereum e oferecer compatibilidade `/web3` não significa implementar fork choice ou devp2p consensus do Ethereum.

A regra de segurança acompanha `locked_qc` e `high_qc`. Uma proposta só é segura se estender o lock ou carregar um justify QC pelo menos tão recente. Um validador não pode votar em blocos diferentes na mesma altura e round. Três links certificados consecutivos, ligados por altura e hash, finalizam o bloco avô. O controlador não altera esse predicado, quorum threshold, QC verification ou three-chain rule.

O timeout adaptativo usa base budget T0, current budget Tt, soma das latências p95 e um floor associado ao déficit de peers. Após timeout cresce na direção de 1,5×Tt; após progress decai na direção de 0,8×Tt. Três vezes a latência observada forma um floor candidato e o resultado fica entre T0 e 8×T0. Sem peer ativo, o floor é 2×T0. Idle sem trabalho e erro local de execution/storage não consomem round. Trata-se de controlador operacional limitado, não de pacemaker teoricamente ótimo.

O recovery gate calcula Hsafe=min(Hs,Hb) quando existem a altura durável de estado Hs e a altura do índice de blocos Hb. Enquanto diferirem, commits finalizados acima de Hsafe são adiados. É uma restrição local de persistência, não outra fase de vote nem um certificado de rede.

A ordenação determinística deriva um salt do chain ID e da altura. Transações com signer e nonce são agrupadas por signer, ordenadas por nonce crescente e suas cabeças são mescladas por hash com salt. Isso elimina dependência da chegada para um candidato idêntico. Não garante first-seen fairness, censorship resistance, confidentiality nem strong order-fairness, pois o proposer ainda influencia inclusion.

O caminho de vote atual usa o full height-versioned validator set e proposer determinístico. O selector ECVRF existe como componente e query, mas não participa de quorum formation nem proposal eligibility. Consensus por committee VRF permanece trabalho futuro.

## Desenho experimental

Todos os treatments usam o mesmo binary e application config. Comparam-se fixed com adaptação off e gate on, adaptive com ambos on, e uma ablação gate-off apenas em rede de pesquisa isolada e descartável. Quando houver recursos usam-se 4, 7, 16 e 31 validadores; single-host serve apenas como smoke.

As condições incluem 10, 50, 100 e 250 ms de latência, mudanças em degraus, jitter, 0/1/5/10% loss, restart de validador e do proposer atual, indisponibilidade logo abaixo de um terço do poder, minority partition/heal, signer delay e durable-history mismatch injetado. Workloads incluem transfer nativo, EVM transfer, contract creation, event log, proxy deployment e UUPS upgrade.

São coletados committed/finalized height, proposal/vote/commit p50/p95/p99, end-to-end finality latency, timeouts, round distribution, current adaptive timeout, peer count, recovery deferral, throughput, gas, CPU, RSS, disk/network bytes, rejection, double-sign e invalid nonce. Um run só entra na análise se todos os validadores concordarem em app hash e finalized block hash, transaction/receipt/block locations forem consistentes, o código do contrato existir e o state do proxy sobreviver ao upgrade.

Após warm-up, cada condição deve ter pelo menos trinta repetições independentes, salvo justificativa prévia por power analysis. A ordem dos treatments é randomizada e os seeds são guardados. Relatam-se mediana, IQR, p95, confidence interval e effect size. Não se seleciona somente o melhor run; regras de exclusão são definidas antes de olhar os resultados.

## Correção, reprodução e ética

A política adaptativa muda quando tentar timeout vote, não o que torna vote ou QC seguro. O gate apenas restringe commits e não pode autorizar um commit rejeitado pela regra base. A ordenação determinística ajuda a produzir execution input comum, mas não substitui a proof contra finalidades conflitantes.

Uma proof publicável deve formalizar stake-weighted quorum intersection, lock monotonicity, unicidade do bloco finalizado por altura, validator-set transition, vote WAL crash recovery e neutralidade de safety do controller/gate. Tests e simulações adversariais são evidence, não substituem formal proof ou auditoria independente.

Cada experimento arquiva commit, dirty-tree status, Go/OS/CPU/memória/container, topologia, genesis, split configs, SHA-256 do binary, workload seed, raw JSON/JSONL/CSV, logs, app hashes finais, scripts de análise e registro de runs falhos. Mecanismo conhecido não é renomeado como invenção, números não são fabricados e hypothesis, observation e interpretation ficam separados.

O uso de IA é declarado conforme a política do venue e os autores continuam responsáveis por cada claim, citation, experiment e proof. Fault injection ocorre somente em sistemas isolados próprios ou autorizados. Private keys, operator tokens, dados de participantes e production endpoints não entram nos artefatos. Vulnerabilidades seguem coordinated disclosure.

Antes da submissão, o manuscrito deve coincidir com uma source revision fixada, a busca de prior art deve estar arquivada, baselines reproduzíveis, medições multi-host completas e toda tabela/figura regenerável a partir de raw data. Resultados negativos, limitações, redação de proof adequada e revisão metodológica externa permanecem na versão final. Até lá, a descrição correta é “rascunho de pesquisa fundamentado na implementação”, não “consensus novo e provado”.

<!-- vexo-docs:technical-parity -->

## Apêndice de paridade técnica

Estes nomes permanecem sem tradução:

- `/web3`, `V_h`, `P_h`, `locked_qc`, `high_qc`
- `consensus/state_machine.go`, `consensus/state_machine_test.go`
- `consensus/commit_rule.go`, `consensus/commit_rule_test.go`
- `consensus/timeout.go`, `consensus/pacemaker.go`
- `node/adaptive_timeout.go`, `node/loop.go`, `node/adaptive_timeout_test.go`
- `node/recovery.go`, `node/consensus_loop.go`
- `fairordering/fairordering.go`, `modules/staking`, `consensus/wal.go`
- `modules/evm`, `modules/evm/backend/geth`
- `consensus_config.json`, `adaptive_round_timeout_enabled`
- `recovery_finality_gate_enabled`, `execution_commit = "finalized"`
- `/v1/status`, `/v1/metrics`, `/v1/finality/latest`, `/metrics/text`
- `deployments/docker/README.md`, `http://127.0.0.1:28657/web3`
- `make check`, `make fuzz-smoke`, `make ops-verify`
- `make network-e2e`, `make evm-conformance`
- `go run ./cmd/vexod consensus adversarial --json`
- `Fpeer = 2 * T0`, `Hs != Hb`, `h > Hsafe`
