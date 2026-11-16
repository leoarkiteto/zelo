---
status: accepted
---

# A quota é uma conta a receber da feature finance

O condomínio precisa de emitir quotas a partir de um orçamento aprovado, repartidas pelas unidades segundo a sua permilagem. Ao contrário da fatura (ADR 0002), a quota tem exatamente a forma de uma conta a receber: um valor, uma unidade, um vencimento, uma liquidação única e um recibo. Decidimos que cada quota emitida **é** uma conta a receber (`finance_accounts`, categoria `condo_fee`, com a unidade), com um detalhe próprio da quota ligado a ela (emissão, permilagem congelada, parte da base e parte do fundo de reserva). O orçamento, a emissão e o detalhe da quota vivem dentro de `internal/features/finance/`, e não numa feature nova.

## Considered Options

- **Feature `quotas` separada, com tabelas próprias, como a fatura**: rejeitada porque a quota teria de reimplementar a liquidação, a inadimplência derivada e a Conta corrente, e os seus valores ficariam fora das Entradas e do Previsto do painel financeiro.
- **Feature `quotas` separada que cria contas a receber na feature `finance`**: rejeitada porque a emissão e a liquidação têm de ser atómicas com as contas a receber, e uma feature não pode importar outra (`scripts/check-feature-boundaries.sh`).

## Consequences

- As quotas entram automaticamente nas Entradas, no Previsto e na inadimplência que já existem.
- Toda conta a receber liquidada, quota ou não, passa a ter um Recibo gerado pelo sistema a partir dos dados. Não é o comprovativo anexado às contas a pagar, que continua só do síndico.
- Uma conta a receber ligada a uma quota não é editada nem cancelada pelo ecrã genérico de contas. Só a emissão a cancela, para manter a soma do período. A liquidação continua pelo fluxo genérico.
- A divisão entre base e fundo de reserva fica no detalhe da quota, não na conta a receber. O saldo do fundo de reserva (fase 2) é somado a partir desse detalhe.
