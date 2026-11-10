# Vexo Lanes: disseminação de lotes certificados para um HotStuff de duas cadeias ponderado por participação

> Locale: pt · Português  
> Tipo de documento: manuscrito de pesquisa e protocolo de reprodutibilidade  
> Situação: rascunho baseado na implementação; os resultados vêm de uma única máquina com enlaces emulados e precisam ser repetidos numa rede real antes da publicação

## Resumo

Um protocolo BFT com líder da família HotStuff é limitado pelo enlace de subida do seu líder: cada transação de um bloco sai de uma máquina uma vez por validador, então a vazão cai como `1/(n−1)` enquanto os enlaces dos outros validadores ficam ociosos. O Vexo Lanes tira a disseminação do líder sem substituir o protocolo de ordenação. Cada validador sela as transações que lhe pertencem em lotes assinados na sua própria faixa; validadores com dois terços da participação confirmam o armazenamento, o que certifica a disponibilidade do lote; o líder propõe um bloco nomeando lotes certificados. O formato do bloco, o hash, a raiz de transações, os bytes assinados, a regra de voto e a regra de confirmação de duas cadeias não mudam.

Numa máquina com enlaces emulados de 100 Mbit/s e 25 ms por validador, o Lanes confirma 60.080, 45.323 e 37.099 tx/s com 4, 7 e 10 validadores, contra 21.073, 10.909 e 8.297 tx/s quando o mesmo nó difunde blocos inteiros: 2,9×, 4,2× e 4,5×, uma vantagem que cresce com `n`. Abaixo da capacidade (4.000 tx/s), o Lanes acrescenta 35 ms de latência mediana com 4 validadores e retira 25 ms e 130 ms com 7 e 10. Sem limite de banda, o Lanes perde 4 % de vazão. A avaliação revelou nove defeitos, entre eles um cronograma de líderes no qual só os validadores de índice par propunham quando o número de validadores era par.

## 1. Questão de pesquisa

É possível adicionar disseminação de lotes certificados sob um protocolo HotStuff ponderado existente, sem mudar o que os validadores votam nem como os blocos são confirmados, de modo que a vazão acompanhe a soma dos enlaces de subida dos validadores, e não o do líder? As subquestões tratam do gargalo do líder (RQ1), da escala (RQ2), da preservação da segurança, da ordem de nonces e da resistência à censura (RQ3) e do custo quando há banda de sobra (RQ4). Hipóteses: H1, a subida da linha de base por transação cresce linearmente com `n` e a do Lanes fica perto do tamanho da transação; H2, o ganho cresce com `n`; H3, sem limite de enlace o Lanes não é mais rápido e custa no máximo uma ida e volta.

## 2. Trabalhos anteriores e limite de novidade

Separar disseminação e ordenação é uma ideia estabelecida: Narwhal e Tusk, Bullshark e Shoal, Stratus, Mir-BFT e ISS, Autobahn e o Quorum Store da Aptos. Este manuscrito não reivindica como novos essa separação, os certificados de disponibilidade, as faixas por validador nem resultados numa rede ampla real. A contribuição é de engenharia: uma adaptação que preserva bit a bit o bloco, o cabeçalho, a assinatura e a regra de confirmação; a posse das faixas por remetente com um recurso contra censura; a autenticação da proposta antes de qualquer busca; e um ambiente de medição reprodutível com os defeitos que revelou.

## 3. Modelo do sistema

`n` validadores com poder de voto ponderado; validadores bizantinos detêm menos de um terço. Quóruns são `>= 2/3` do poder. Sincronia parcial, canais autenticados e assinaturas separadas por domínio. Validadores podem falhar, equivocar-se, reter lotes, inundar a própria faixa ou enviar propostas falsas.

## 4. Protocolo

**Faixas e certificados.** Cada validador sela as suas transações num `lanes.Batch` ao atingir 512 KiB ou 5 ms após a primeira, limitado por `lanes.MaxBatchTxs` e `lanes.MaxBatchBytes`. Uma faixa pouco carregada, cujo lote anterior tinha menos de 16 KiB, espera também o certificado desse lote, no máximo 50 ms. Os validadores confirmam no domínio `DomainLaneBatchAck`; confirmações de `>= 2/3` do poder formam um `BatchCert`, de modo que pelo menos um validador honesto guarda o lote. As retransmissões usam espera exponencial.

**Posse e recurso.** Uma transação pertence à faixa `lanes.Owner(members, sender)`. Todas as transações de um remetente seguem numa faixa na ordem de admissão, então as faixas nunca reordenam uma sequência de nonces. Se nenhum lote armazenado levar uma transação alheia após `lanes.FallbackDelay` (3 s), quem a guarda a coloca no próprio lote; um dono defeituoso pode atrasar, mas não censurar.

**Propostas por faixas.** O líder toma apenas lotes certificados, por faixa em ordem até o primeiro lote não certificado, alternando faixas até o limite do bloco, assina o bloco como antes e difunde um `lane_proposal` com os identificadores dos lotes, as posições recusadas (`Excluded`) e as transações fora de lote (`Extra`).

**Autenticação antes da busca.** O receptor verifica o líder e a assinatura antes de buscar qualquer coisa. Se o conjunto de validadores depende de um bloco ainda não executado, a proposta é guardada (no máximo 16) e julgada de novo após a próxima execução. O bloco é reconstruído a partir dos lotes armazenados e só é julgado se o hash e a raiz coincidirem com o cabeçalho assinado; caso contrário, o validador pede o `proposal` inteiro com `proposal_request`.

**Cronograma proporcional à participação.** O líder da visão `v` é `validator.SelectProposer(validators, v)`: os validadores possuem `VotingPower` posições consecutivas de `[0, W)`, a visão `v` cai na posição `v·S mod W`, com um passo coprimo com `W` a partir de `ceil(W·(φ−1))`. Cada janela de `W` visões dá a cada validador exatamente a sua parte. O cronograma depende só da visão.

**Limites de recursos.** 64 MiB por autor num armazenamento de 256 MiB; expiração após 30 s; no máximo dois envios por lote e par; no máximo `consensus.MaxLaneRefs` lotes por proposta; uma codificação binária versionada.

## 5. Argumento de correção

Segurança: as regras de voto e de confirmação não mudam, e um validador só vota num bloco cujo hash reconstruído coincide com o cabeçalho assinado; as faixas podem atrasar um voto, mas não mudar o seu objeto. Disponibilidade: cada lote nomeado é guardado por um validador honesto e, em último caso, busca-se a proposta inteira. Vivacidade: após a estabilização, lotes honestos são certificados e propostos; um validador atrasado julga as propostas logo depois de executar. É um argumento, não uma prova verificada por máquina.

## 6. Método experimental

Tratamentos: Lanes (nó padrão) contra a linha de base com `DisseminateByProposal`, na qual o líder difunde blocos inteiros; todo o resto é compartilhado. Todos os validadores rodam num processo num Apple M4; cada subida passa por um enlace virtual de taxa e latência fixas, com mensagens de consenso antes do tráfego em massa, sem perda nem variação. Os clientes enviam transações de 89 bytes; as rodadas de vazão enviam 150.000 de uma vez, as de latência 20.000 em malha aberta a 4.000 tx/s (`VEXO_RATE`). As células de vazão com 10 validadores usam tempo limite de 5 s nos dois tratamentos, pois a linha de base trava após a altura 6 com o padrão. Cada célula foi medida três vezes.

## 7. Resultados

| Validadores | Lanes tx/s | Linha de base tx/s | Fator | Lanes B/tx | Linha de base B/tx |
|---|---|---|---|---|---|
| 4 | 60,080 (58,287–61,676) | 21,073 (20,955–21,235) | 2.85× | 81.8–96.1 | 334.6–335.6 |
| 7 | 45,323 (44,308–46,715) | 10,909 (9,394–11,685) | 4.15× | 123.4–217.6 | 660.0–725.5 |
| 10 | 37,099 (34,913–39,378) | 8,297 (8,241–8,330) | 4.47× | 135.3–146.0 | 291.3–325.9 |

Todas as rodadas confirmaram as 150.000 transações. Só pelas propostas, a linha de base paga `s·(n−1)` bytes por transação; o Lanes, 0,2–0,4 B/tx. Isso apoia H1 e H2.

| Validadores | Lanes p50 / p99 | Linha de base p50 / p99 | Proposta Lanes B/tx | Proposta linha de base B/tx |
|---|---|---|---|---|
| 4 | 323–327 / 395–483 ms | 280–298 / 358–373 ms | 6.0–6.1 | 96.9–100.1 |
| 7 | 379–386 / 471–486 ms | 405–421 / 747–801 ms | 8.3–8.4 | 117.0–124.1 |
| 10 | 439–462 / 556–659 ms | 558–610 / 1,546–1,745 ms | 11.3–11.7 | 141.1–376.4 |

Com 4 validadores, a certificação custa cerca de uma ida e volta; a partir de 7 validadores o enlace do líder da linha de base já atrasa as propostas com esta carga, e com 10 só o Lanes acompanhou a carga oferecida. H3 vale apenas para conjuntos pequenos.

| Tratamento | tx/s | Subida B/tx |
|---|---|---|
| Lanes | 110,033 (108,496–111,814) | 61.4 |
| Linha de base | 114,522 (113,095–116,408) | 147.4–148.3 |

Sem limite de enlace, a linha de base é 4 % mais rápida; ainda assim o Lanes sobe menos da metade dos bytes. A linha de base é limitada pelo enlace do líder e o Lanes, nesta matriz, pela máquina compartilhada.

## 8. Defeitos revelados pela avaliação

1. **Cronograma por paridade.** `(height−1+round) mod n` fazia apenas índices pares proporem com `n` par; substituído pelo cronograma proporcional (`node/leader_schedule_test.go`).
2. **Repetição de transações confirmadas.** Fofoca tardia era readmitida (152.629 confirmações para 150.000 transações).
3. **Expiração quadrática do cache de vistas.** Cada admissão percorria o cache inteiro; agora a mais antiga primeiro, com limite.
4. **Tempestade de retransmissões.** 200 ms fixos num enlace saturado; agora espera exponencial.
5. **Evidências duplicadas sem limite.** Agora uma vez por infração numa janela de 1.024.
6. **Pedidos de corpos de confirmação.** Agora um pedido, repetido após um segundo.
7. **Propostas julgadas só na retransmissão.** Uma em cada quatro rodadas com 10 validadores caiu para 2.280 tx/s; agora são guardadas e julgadas após a execução (`consensus/transport_reactor_test.go`).
8. **Custo por lote com pouca carga.** Lotes de duas transações saturavam a máquina; corrigido com o ritmo das faixas.
9. **Métrica de latência mal rotulada.** Agora latência do envio à confirmação por transação.

## 9. Ameaças à validade

- Rede emulada numa máquina, sem perda nem variação.
- Uma CPU compartilhada limita o Lanes com `n` grande.
- A linha de base precisa de 5 s de tempo limite com 10 validadores; uma codificação mais compacta reduziria a diferença.
- A espera de 300 ms antes de pedir uma proposta inteira é curta em enlaces saturados.
- Apenas transferências pequenas; apenas três repetições (34,9k–39,4k tx/s com 10 validadores).
- Certificados Ed25519 concatenam assinaturas; BLS os reduziria.

## 10. Reprodução

```bash
go test ./lanes ./consensus ./node ./validator ./mempool -count=1
VEXO_THROUGHPUT=150000 VEXO_VALIDATORS=4 VEXO_UPLINK_MBIT=100 VEXO_LATENCY_MS=25 \
  go test ./node -run TestConsensusThroughput -v -count=1 -timeout 20m
VEXO_DISSEMINATION=proposals VEXO_THROUGHPUT=150000 VEXO_VALIDATORS=4 VEXO_UPLINK_MBIT=100 VEXO_LATENCY_MS=25 \
  go test ./node -run TestConsensusThroughput -v -count=1 -timeout 20m
```

`VEXO_ROUND_TIMEOUT_MS` altera o tempo limite; sem `VEXO_UPLINK_MBIT` e `VEXO_LATENCY_MS` o enlace não tem limite. Todo número publicado precisa registrar a revisão, a máquina, a versão do Go e as linhas de log completas.

<!-- vexo-docs:technical-parity -->

## Apêndice de paridade técnica

Estes nomes de implementação e de experimento não são traduzidos:

- `lanes/batch.go`, `lanes/lanes.go`, `lanes/wire.go`, `VerifyBatch`, `VerifyCert`
- `consensus/lane_proposal.go`, `consensus/transport_reactor.go`, `consensus/transport_reactor_test.go`
- `validator/proposer.go`, `node/lanes.go`, `node/throughput_test.go`, `node/shaped_transport_test.go`, `node/leader_schedule_test.go`
- `lanes.Batch`, `lanes.MaxBatchTxs`, `lanes.MaxBatchBytes`, `lanes.Owner(members, sender)`, `lanes.FallbackDelay`
- `DomainLaneBatchAck`, `BatchCert`, `consensus.LaneProposal`, `consensus.MaxLaneRefs`, `Excluded`, `Extra`
- `lane_proposal`, `proposal_request`, `proposal`, `DisseminateByProposal`
- `validator.SelectProposer(validators, v)`, `VotingPower`, `[0, W)`, `v·S mod W`, `ceil(W·(φ−1))`, `0..W−1`, `W`, `S`, `v`
- `n`, `s`, `>= 2/3`, `1/(n−1)`, `s·(n−1)`, `(height−1+round) mod n`
- `VEXO_THROUGHPUT`, `VEXO_VALIDATORS`, `VEXO_UPLINK_MBIT`, `VEXO_LATENCY_MS`, `VEXO_DISSEMINATION`, `VEXO_ROUND_TIMEOUT_MS`, `VEXO_RATE`
- `VEXO_RATE=4000 VEXO_THROUGHPUT=20000`, `VEXO_VALIDATORS`, `VEXO_UPLINK_MBIT`, `VEXO_LATENCY_MS`, `VEXO_DISSEMINATION`, `VEXO_ROUND_TIMEOUT_MS`, `VEXO_RATE`
