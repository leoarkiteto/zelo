---
status: accepted
---

# A fatura é uma entidade separada da conta a pagar

O condomínio precisa registrar faturas recebidas de fornecedores em Portugal. Essas faturas têm fornecedor com NIF, várias linhas com taxas de IVA diferentes, retenção na fonte, vários pagamentos parciais, vários anexos e notas de crédito. A conta a pagar atual (`finance_accounts` com `account_type='payable'`) tem fornecedor em texto livre, um único valor, uma única liquidação e um único comprovante. Decidimos que a fatura é uma entidade própria, numa feature nova (`internal/features/invoices/`), com as suas tabelas (fornecedores, faturas, linhas, discriminação do IVA, pagamentos, anexos). Ela não cria nem altera contas a pagar, e a feature `finance` continua como está.

## Considered Options

- **Enriquecer a conta a pagar até virar fatura**: rejeitada porque mudaria o comportamento e o esquema de uma funcionalidade em uso (liquidação única, painel financeiro) só para acomodar um caso novo.
- **Registrar a fatura e gerar uma conta a pagar ligada**: rejeitada porque duplicaria valor e estado em dois lugares que teriam de ficar sincronizados a cada pagamento parcial.

## Consequences

- Os pagamentos de faturas **não** entram nas Saídas nem no "A pagar" do painel financeiro atual. Os indicadores de faturas virão num painel próprio da feature `invoices` (fase 2), carregado como fragmento HTMX conforme a ADR 0001.
- "Liquidação" e "Cancelamento" continuam a ser termos da conta a pagar. A fatura usa "Pagamento da fatura" e "Anulação" (ver `GLOSSARY.md`).
- Código partilhado entre as duas features (formatação de dinheiro, armazenamento de anexos) passa para `internal/shared/`, nunca para uma importação entre features.
