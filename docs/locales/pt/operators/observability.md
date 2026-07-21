> Locale: pt · Português

# Guia de observabilidade

Este guia explica como saber se um nó Vexo está íntegro a partir de RPC, métricas, logs e evidências de liberação.

Foi escrito para operadores que precisam de sinais práticos: o que observar, o que cada número significa e quando um valor deve ser tratado como perigoso.

## Num relance

Se um nó parecer errado, verifique estes em ordem:

1. `running` e `latest_height` em `/v1/status`
2. `latest_finalized_height` e contagens de pares
3. `round_timeout`, latência de proposta/voto, tamanho do mempool e métricas de latência de commit
4. falhas de signatário, integridade do snapshot e integridade da reprodução
5. Proibições de pares e falhas de discagem entre pares

Essa ordem é importante porque separa “o processo está vivo” de “a cadeia está realmente fazendo um progresso seguro”.

## Pontos finais principais

| Ponto final | Usar |
|---|---|
| `/v1/status` | Processo rápido, altura, hash de aplicativo, finalidade e resumo de pares |
| `/v1/metrics` | Métricas JSON para dashboards e automação |
| `/metrics/text` | Métricas de texto compatíveis com Prometheus |
| `/v1/diagnostics` | Verificações combinadas de prontidão, recursos, status, peers, armazenamento e métricas |
| `/v1/finality/latest` | Última prova de finalidade para verificações de clientes leves e de segurança |
| `/v1/state/latest` | Última raiz de estado e ligação do conjunto de validadores |
| `/v1/recovery/report` | Diagnóstico de consistência de travamento/reinicialização |
| `/v1/snapshot` | Integridade do snapshot e metadados de exportação |

Os pontos finais de administração, como remoção, reprodução e controle de consenso, normalmente devem ser acessíveis apenas por meio de loopback, uma rede de operadora, mTLS ou um gateway autenticado. Os tokens de administração com escopo permanecem opcionais e são aplicados quando configurados.

## Lendo `/v1/status`

Campos importantes:

| Campo | Significado | Nota do Operador |
|---|---|---|
| `running` | O processo do nó foi iniciado e possui o estado de tempo de execução | `true` não prova a vivacidade do consenso por si só |
| `latest_height` | Altura mais recente do aplicativo comprometido localmente | Deve aumentar com o tempo em uma rede de validadores ativos |
| `latest_finalized_height` | Última altura finalizada de três cadeias HotStuff | Não deve ficar indefinidamente atrás da altura executada/confirmada |
| `latest_app_hash` | Hash de commit do aplicativo | Deve corresponder aos pares da mesma altura |
| `peer_count` | Resumo de pares conectados/pontuados compatíveis com versões anteriores | Prefira os campos de pares mais específicos abaixo |
| `active_peer_count` | Sessões de transporte ativas, quando o transporte pode reportá-las | Melhor sinal rápido para conectividade P2P ao vivo |
| `configured_peer_count` | Endereços de pares configurados ou aprendidos | A acessibilidade não é garantida |
| `scored_peer_count` | Pares conhecidos na tabela de pontuação | Útil para histórico de banimentos/limites de taxa, não para prova de sessões ao vivo |
| `banned_peers` | Pares atualmente banidos pela política de pontuação | Picos indicam ataque, configuração de peer incorreta ou limites muito rígidos |

Exemplo saudável para uma rede de host único com 4 validadores: `running=true`, `latest_height` aumentando, `latest_finalized_height` presente, `active_peer_count` perto de `3` e `banned_peers=0`.

## Métricas do Prometheus

O ponto final do texto expõe medidores como:

- `vexo_node_running`
- `vexo_latest_height`
- `vexo_peer_count`
- `vexo_active_peer_count`
- `vexo_configured_peer_count`
- `vexo_scored_peer_count`
- `vexo_banned_peers`
- `vexo_quorum_health_ratio`
- `vexo_height_rate_per_minute`
- `vexo_adaptive_round_timeout_enabled`
- `vexo_round_timeouts`
- `vexo_adaptive_round_timeout_nanos`
- `vexo_recovery_finality_gate_enabled`
- `vexo_recovery_finality_deferrals`
- `vexo_proposal_latency_p95_nanos`
- `vexo_vote_latency_p95_nanos`
- `vexo_commit_latency_p95_nanos`
- `vexo_mempool_size`
- `vexo_snapshot_healthy`
- `vexo_replay_healthy`
- `vexo_validator_signing_failures`
- `vexo_post_commit_reconciliation_failures`

`vexo_peer_count` é mantido para painéis mais antigos. Novos painéis devem representar `vexo_active_peer_count`, `vexo_configured_peer_count`, `vexo_scored_peer_count` e `vexo_quorum_health_ratio` separadamente.

## Regras de alerta sugeridas

Ajuste os números para a contagem real do validador, intervalo de bloqueio, latência e hardware. Estes são pontos de partida, não constantes universais.

| Alerta | Condição inicial | Por que |
|---|---|---|
| Nó desativado | `vexo_node_running == 0` por 1 minuto | Processo/tempo de execução interrompido |
| Altura estagnada | `latest_height` inalterado por 2-3 intervalos de bloco esperados | Consenso ou execução paralisada |
| Finalidade estagnada | `latest_finalized_height` inalterado enquanto os blocos continuam em execução | Caminho de finalidade ou questão de quórum |
| Nenhum par ativo | `vexo_active_peer_count == 0` por 1 minuto em um nó não isolado | Interrupção de P2P, incompatibilidade de autenticação ou problema de endereço |
| Contagem de pares muito baixa | peers ativos abaixo da meta de conectividade de quorum | Problema de partição ou bootstrap |
| Pico de tempo limite da rodada | contador de tempo limite cresce mais rápido que a linha de base normal | Latência, falha do proponente ou partição de rede |
| Política adaptativa desligada | `vexo_adaptive_round_timeout_enabled == 0` em um nó que deveria executar o ritmo adaptativo | Configuração ou experimento desativou o marcapasso |
| Tempo limite adaptativo alto | `vexo_adaptive_round_timeout_nanos` cresce muito acima da linha de base de inicialização | Pico de latência da rede ou formação de quórum mais lenta |
| Pares ausentes ampliam o tempo limite | `vexo_active_peer_count` cai abaixo de `vexo_configured_peer_count` e o tempo limite adaptativo aumenta | A saúde do quórum está piorando e o marcapasso está compensando |
| Relação de quórum baixa | `vexo_quorum_health_ratio < 0.75` por várias janelas | Não há pares ativos suficientes para um caminho estável de proposta/voto |
| Backoff do proponente ativo | `vexo_quorum_health_ratio < 0.75` e o ritmo de proposta de bloco desacelera | O nó está aguardando a recuperação da saúde do quórum |
| Confirmar latência alta | p95/p99 se aproxima do orçamento de tempo limite de consenso | Sobrecarga de armazenamento/tempo de execução |
| Pressão de Mempool | o tamanho do mempool aumenta por vários minutos | Política de taxas, spam ou problema de capacidade de bloqueio |
| Instantâneo insalubre | `vexo_snapshot_healthy == 0` | Risco de sincronização/recuperação de estado |
| Repetir insalubre | `vexo_replay_healthy == 0` | Determinismo ou risco de consistência estatal |
| Falhas do signatário | `vexo_validator_signing_failures > 0` | KMS/signatário remoto/falha na política |
| Falhas de reconciliação | `vexo_post_commit_reconciliation_failures > 0` | Evidência durável ou reparo necessário |
| Gate de recuperação desligado | `vexo_recovery_finality_gate_enabled == 0` em um nó que deveria impor o gate de recuperação | Commits finalizados podem contornar o gate de segurança da recuperação |
| Adiamentos de recuperação | `vexo_recovery_finality_deferrals` aumenta | Commits finalizados estão sendo adiados por divergência de recuperação |
| Pico de pares banido | pares banidos sobe repentinamente | Ataque, pares mal configurados ou problema de limite de pontuação |

## Limites iniciais sugeridos

Use-os como valores de alerta iniciais e, em seguida, ajuste após uma linha de base real de longo prazo:

| Sinal | Aviso | Crítico | Primeira Ação |
|---|---:|---:|---|
| Taxa de altura | abaixo de 50% do esperado para 2 janelas | crescimento zero para intervalos de 2-3 blocos | compare todos os validadores, verifique os registros do proponente/assinatura/par |
| Atraso de altura finalizado | cresce por 5 minutos | cresce enquanto a altura executada continua aumentando por 10 minutos | inspecionar logs de prova de QC/finalidade e hash definido pelo validador |
| Pares ativos | abaixo da meta de conectividade de quórum | zero pares ativos | verifique o endereço anunciado, TLS/auth, incompatibilidade de genesis/ID de cadeia |
| Tempos limite de rodada | 3x linha de base normal | loop de tempo limite contínuo | aumentar o orçamento de tempo limite ou investigar latência/partição |
| Latência da proposta p95 | acima de 50% de `timeout_propose` | acima de 80% de `timeout_propose` | proponente de perfil, mempool, compromisso DA, disco |
| Latência de voto p95 | acima de 50% do orçamento pré-votado/pré-comprometido | acima de 80% do orçamento | inspecionar CPU, signatário, transporte, contrapressão de fofoca |
| Confirmar latência p95 | acima de 50% do intervalo de bloqueio | acima de 80% do intervalo de bloqueio | inspecionar LevelDB, raízes de estado, execução de EVM, instantâneos |
| Tamanho do pool de memória | aumentando por 5 minutos | perto de `max_txs` ou rotatividade de substituição sustentada | inspecionar taxa base, taxa mínima, validade tx, spam |
| Falhas do signatário | qualquer valor diferente de zero | falhas repetidas em uma janela de altura | pare o validador se aparecer guarda de sinal duplo ou incompatibilidade de chave |
| Saúde instantânea | uma verificação falhou | falha repetida na exportação/verificação/restauração | pausar o serviço de sincronização de estado e executar o relatório de recuperação |
| Saúde de repetição | uma falha estrita de repetição | repetir incompatibilidade na última altura segura | preservar o diretório de dados e interromper a atualização/lançamento inseguro |
| Pares banidos | pico repentino | muitos peers foram banidos após o lançamento da configuração | verificar limites de pontuação, CA TLS, identidade de pares, prova de autenticação opcional e distorção de relógio |

A regra mais importante: alerta sobre **mudanças ao longo do tempo**. Um único número pode ser enganoso; taxa de altura, atraso de finalidade, rotatividade de pares, crescimento de mempool e falhas de signatários juntos contam a história real.

## Matriz de Triagem de Incidentes

| Situação | Camada Provável | O que preservar | Próximo passo seguro |
|---|---|---|---|
| Altura parada, pares saudáveis ​​| consenso/signatário/tempo de execução | registros de consenso, registros de signatários, amostra de mempool | verificar a chave do proponente e arredondar os registros de tempo limite |
| Os peers foram eliminados após a implantação | rede/configuração | configuração de rede, certificados TLS, addrbook, registros de pares | reverter alteração de endereço/TLS/autenticação anunciada |
| Hashes de aplicativos diferem na mesma altura | execução/armazenamento | diretórios de dados, registros de bloco, logs de aplicativos, saída de reprodução | interromper os nós afetados e executar a reprodução estrita |
| Prova de caráter definitivo rejeitada | conjunto de finalidade/validador | prova JSON, validador definido na altura da prova | verificar o hash do conjunto de validadores e o domínio de bytes de assinatura |
| A restauração do instantâneo falha | sincronização/armazenamento de estado | arquivo de instantâneo, soma de verificação, raízes de estado, logs de restauração | não tente novamente com dados ativos; restaurar em diretório limpo |
| Signatário remoto rejeita solicitações | custódia de chaves | log de auditoria do signatário, arquivo de proteção, arquivo nonce, logs de nó | distinguir rejeição política de interrupção de transporte |
| Aumento de pares banidos | P2P/segurança | instantâneos de pontuação de pares e motivos de banimento | inspecionar fofocas malformadas ou configuração errada compartilhada |

Durante incidentes, prefira preservar os dados em vez de “limpar”. A exclusão de WALs, addrbooks, signer guards ou diretórios LevelDB pode destruir a evidência necessária para distinguir um bug de um erro do operador.

## Registrar eventos para manter

Os logs estruturados devem ser retidos com ID do nó, ID do validador, ID da cadeia, altura, arredondamento, hash de bloco e ID do par, quando relevante.

Eventos importantes:

- `node_running`
- `rpc_listening`
- `p2p_listening`
- `peer_configured`
- `peer_connected`
- `peer_disconnected`
- `peer_dial_failed`
- `peer_banned`
- `consensus_loop_running`
- `block_committed`
- `round_timeout`
- `validator_signing_failure`
- `evidence_received`
- `evidence_applied`
- `snapshot_exported`
- `replay_checked`
- `upgrade_halt`
- `upgrade_applied`

Para candidatos a lançamento, arquive logs junto com amostras de métricas, amostras pprof, arquivos de configuração, genesis, somas de verificação binárias e manifestos de evidências.

## Manual de primeira resposta

Quando um operador vê um problema:

1. Verifique `/v1/status` em pelo menos dois validadores.
2. Compare `latest_height`, `latest_finalized_height`, `latest_app_hash` e contagens de pares.
3. Verifique `/v1/diagnostics` quanto a recursos ausentes ou verificações de armazenamento/reprodução/instantâneo não íntegros.
4. Inspecione os logs de eventos de pares em busca de erros de autenticação, TLS, gênese, ID de cadeia ou espera.
5. Inspecione as métricas do mempool e da taxa básica se os txs não estiverem incluídos.
6. Verifique os logs do signatário e do signatário remoto se as assinaturas do validador falharem.
7. Exporte o relatório de recuperação antes de excluir ou modificar dados.
8. Se houver suspeita de conflito de finalidade, interrompa a automação, preserve os registros/evidências e execute a detecção de conflitos de finalidade.

## Layout do painel

Um painel útil geralmente possui cinco linhas:

1. **Atividade**: nó em execução, altura mais recente, altura finalizada, taxa de altura.
2. **Latência de consenso**: tempos limites de rodada, proposta/votação/confirmação p95 e p99.
3. **Rede**: pares ativos/configurados/pontuados, pares banidos, mensagens de janela de pares.
4. **Execução**: tamanho do mempool, taxa de gás/base, contagem de tx, latência de commit.
5. **Recuperação e segurança**: integridade do snapshot, integridade da reprodução, falhas de signatário, falhas de reconciliação.

Mantenha os painéis chatos. O objetivo não é mostrar todos os contadores internos; é tornar óbvios os estados perigosos antes que os validadores diverjam ou os usuários percebam transações paralisadas.

## Liberar evidências de observabilidade

Para um release candidate, observabilidade não é apenas monitoramento ao vivo. Torna-se uma evidência:

1. Colete a linha de base `/v1/status`, `/v1/metrics`, `/v1/diagnostics`, `/v1/finality/latest` e `/v1/recovery/report` de cada validador.
2. Execute a carga pela duração e taxa escolhidas.
3. Injete pelo menos uma reinicialização, uma interrupção de peer e um exercício de exportação/verificação/restauração de snapshot.
4. Colete as métricas finais de cada validador.
5. Armazene as amostras antes/depois, logs, amostras pprof, logs de auditoria do signatário e manifesto de evidências em `dist/`.

Um bom pacote de evidências permite que um revisor responda: a altura aumentou, a finalidade progrediu, os pares se recuperaram, os txs foram confirmados, os instantâneos foram verificados, a reprodução permaneceu saudável, os signatários evitaram a assinatura dupla e o binário de liberação exato produziu os resultados?

<!-- vexo-docs:technical-parity -->
## Apêndice de paridade técnica

Este apêndice garante que a tradução preserve as interfaces executáveis e as seções principais do documento canônico em inglês. Comandos, chaves de configuração, métodos RPC e nomes de pacotes permanecem inalterados em todos os idiomas.

### Rastreamento de seções
- section: Core Endpoints — Esta seção deve ser revisada junto com valores de configuração, evidências de verificação, condições de falha e ações do operador.
- section: Reading `/v1/status` — Esta seção deve ser revisada junto com valores de configuração, evidências de verificação, condições de falha e ações do operador.
- section: Prometheus Metrics — Esta seção deve ser revisada junto com valores de configuração, evidências de verificação, condições de falha e ações do operador.
- section: Suggested Alert Rules — Esta seção deve ser revisada junto com valores de configuração, evidências de verificação, condições de falha e ações do operador.
- section: Suggested Starting Thresholds — Esta seção deve ser revisada junto com valores de configuração, evidências de verificação, condições de falha e ações do operador.
- section: Incident Triage Matrix — Esta seção deve ser revisada junto com valores de configuração, evidências de verificação, condições de falha e ações do operador.
- section: Log Events to Keep — Esta seção deve ser revisada junto com valores de configuração, evidências de verificação, condições de falha e ações do operador.
- section: First Response Playbook — Esta seção deve ser revisada junto com valores de configuração, evidências de verificação, condições de falha e ações do operador.
- section: Dashboard Layout — Esta seção deve ser revisada junto com valores de configuração, evidências de verificação, condições de falha e ações do operador.
- section: Release Evidence From Observability — Esta seção deve ser revisada junto com valores de configuração, evidências de verificação, condições de falha e ações do operador.

### Interfaces mantidas sem alteração
- `/v1/status` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `/v1/metrics` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `/metrics/text` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `/v1/diagnostics` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `/v1/finality/latest` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `/v1/state/latest` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `/v1/recovery/report` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `/v1/snapshot` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `latest_height` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `latest_finalized_height` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `latest_app_hash` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `peer_count` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `active_peer_count` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `configured_peer_count` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `scored_peer_count` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `banned_peers` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `banned_peers=0` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `vexo_node_running` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `vexo_latest_height` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `vexo_peer_count` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `vexo_active_peer_count` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `vexo_configured_peer_count` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `vexo_scored_peer_count` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `vexo_banned_peers` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `vexo_quorum_health_ratio` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `vexo_height_rate_per_minute` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `vexo_adaptive_round_timeout_enabled` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `vexo_round_timeouts` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `vexo_adaptive_round_timeout_nanos` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `vexo_recovery_finality_gate_enabled` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `vexo_recovery_finality_deferrals` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `vexo_proposal_latency_p95_nanos` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `vexo_vote_latency_p95_nanos` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `vexo_commit_latency_p95_nanos` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `vexo_mempool_size` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `vexo_snapshot_healthy` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `vexo_replay_healthy` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `vexo_validator_signing_failures` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `vexo_post_commit_reconciliation_failures` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `vexo_node_running == 0` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `vexo_active_peer_count == 0` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `vexo_snapshot_healthy == 0` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `vexo_replay_healthy == 0` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `vexo_validator_signing_failures > 0` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `vexo_post_commit_reconciliation_failures > 0` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `timeout_propose` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `max_txs` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `node_running` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `rpc_listening` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `p2p_listening` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `peer_configured` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `peer_connected` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `peer_disconnected` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `peer_dial_failed` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `peer_banned` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `consensus_loop_running` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `block_committed` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `round_timeout` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `validator_signing_failure` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `evidence_received` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `evidence_applied` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `snapshot_exported` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `replay_checked` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `upgrade_halt` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `upgrade_applied` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
- `dist/` — Este nome é usado sem alteração em exemplos executáveis e validação de configuração; não deve ser traduzido.
