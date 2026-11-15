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
