# zelo

Portal do condomínio: moradores, síndico, serviços, chamados e a vida financeira do condomínio.

## Finanças

**Conta a receber**:
Valor que o condomínio tem a receber (taxa condominial, multa e juros, reserva de área comum, receita extraordinária).
_Avoid_: Cobrança (quando usado para o conjunto), receivable no texto de produto

**Conta a pagar**:
Valor que o condomínio deve pagar (manutenção, limpeza, consumo, folha, serviços de terceiros, outros).
_Avoid_: Despesa (para a conta ainda não paga), payable no texto de produto

**Liquidação**:
O momento em que uma conta é efetivamente paga ou recebida, registrado com sua data de liquidação.
_Avoid_: Baixa, quitação

**Entrada**:
Dinheiro efetivamente recebido pelo condomínio, isto é, uma conta a receber liquidada.
_Avoid_: Receita (quando usada para entradas e saídas juntas), crédito

**Saída**:
Dinheiro efetivamente pago pelo condomínio, isto é, uma conta a pagar liquidada.
_Avoid_: Despesa (quando ainda não paga), débito, gasto

**Saldo do período**:
Entradas menos saídas dentro de um período.
_Avoid_: Lucro, resultado, balanço

**Movimentação financeira**:
O conjunto das entradas e saídas do condomínio ao longo do tempo.
_Avoid_: Receita, fluxo (sozinho), vida financeira

**Realizado**:
Visão das finanças baseada apenas em liquidações (o que de fato entrou e saiu).
_Avoid_: Caixa (como adjetivo), efetivado

**Previsto**:
Visão das finanças baseada nos vencimentos de contas não canceladas, incluindo as ainda pendentes ou vencidas.
_Avoid_: Projetado, orçado, estimado

**Moeda do condomínio**:
A moeda (EUR ou BRL) em que todos os valores de um condomínio são registrados e exibidos. Uma fatura usa sempre a moeda do condomínio.
_Avoid_: Moeda da fatura, divisa

## Quotas

**Quota**:
O valor periódico que cada unidade deve ao condomínio para cobrir as despesas comuns, calculado pela sua permilagem. Em pt-br aparece como "taxa condominial".
_Avoid_: Mensalidade, cota, condomínio (para o valor), taxa (sozinha)

**Permilagem**:
A parte de uma unidade no condomínio, que define quanto da quota lhe cabe. A soma das permilagens de todas as unidades é 100%. Em pt-br aparece como "fração ideal".
_Avoid_: Cota-parte, coeficiente, percentagem da unidade

**Fundo de reserva**:
A parte de cada quota destinada a despesas futuras ou imprevistas, separada das despesas correntes. Em Portugal é o Fundo Comum de Reserva, com mínimo legal.
_Avoid_: Poupança, caixa de reserva, fundo (sozinho)

**Orçamento**:
As despesas previstas do condomínio para um ano, aprovadas em assembleia, de onde saem as quotas. Não é o Previsto, que vem dos vencimentos das contas.
_Avoid_: Orçado, previsão anual, Previsto (para o orçamento)

**Período**:
O mês ou trimestre a que uma quota se refere.
_Avoid_: Competência, mês de referência, ciclo

**Emissão**:
O lançamento das quotas de um período para todas as unidades de uma só vez.
_Avoid_: Lançamento, geração, faturação, boleto

**Recibo**:
O comprovativo, emitido pelo condomínio, de que uma conta a receber foi liquidada. Não é um documento fiscal, nem o comprovativo anexado a uma conta a pagar.
_Avoid_: Comprovante (para o recibo), quitação, nota, fatura-recibo

**Conta corrente**:
O extrato de uma unidade: as contas a receber pendentes e as liquidadas, com os seus recibos.
_Avoid_: Minhas cobranças, extrato (sozinho), histórico

## Faturas

**Fatura**:
Documento fiscal recebido de um fornecedor e registrado pelo síndico (fatura, fatura-recibo, fatura simplificada, nota de débito ou outro). É uma entidade própria, separada da Conta a pagar (ver `docs/adr/0002-fatura-e-entidade-separada-da-conta-a-pagar.md`).
_Avoid_: Conta a pagar (para a fatura), nota fiscal, invoice no texto de produto

**Fornecedor**:
Pessoa ou empresa que emite faturas para o condomínio, identificada pelo NIF. Cada condomínio tem os seus fornecedores.
_Avoid_: Prestador (quando for o do diretório de serviços), favorecido, payee

**Linha da fatura**:
Cada produto ou serviço da fatura, com base tributável e taxa de IVA. O total da fatura é sempre a soma das linhas.
_Avoid_: Item (sozinho), rubrica

**Nota de crédito**:
Documento do fornecedor que reduz o valor de uma fatura original, à qual fica ligada. Não é uma fatura normal.
_Avoid_: Estorno, devolução, fatura negativa

**Retenção na fonte**:
Valor que o condomínio retém do total da fatura para entregar ao Estado. Não é pago ao fornecedor.
_Avoid_: Desconto, imposto retido (sozinho)

**Valor a pagar**:
Total da fatura menos a retenção na fonte: o que é devido ao fornecedor.
_Avoid_: Total (quando houver retenção), valor líquido

**Valor em dívida**:
Valor a pagar menos a soma dos pagamentos registrados. Quando negativo, é exibido como crédito a favor.
_Avoid_: Saldo (sozinho), pendente

**Pagamento da fatura**:
Um pagamento, total ou parcial, feito ao fornecedor por conta de uma fatura. Uma fatura pode ter vários.
_Avoid_: Liquidação (reservado à Conta a pagar), baixa

**Rascunho**:
Fatura guardada sem validação completa. Não aceita pagamentos, não entra nos totais e pode ser eliminada.
_Avoid_: Pré-registro, fatura pendente

**Anulação**:
Marca irreversível que tira uma fatura registrada de uso, preservando-a e aos seus pagamentos. Faturas registradas não se eliminam: anulam-se.
_Avoid_: Cancelamento (reservado à Conta a pagar), exclusão
