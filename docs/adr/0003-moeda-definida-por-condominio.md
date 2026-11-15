---
status: accepted
---

# A moeda é definida por condomínio

O zelo nasceu para condomínios no Brasil e formata todo o dinheiro em reais (R$). O módulo de faturas atende condomínios em Portugal, em euros. Decidimos guardar a moeda no condomínio (`condominiums.currency`, `EUR` ou `BRL`, com `BRL` por defeito) e formatar todos os valores da app, incluindo os da feature `finance`, na moeda do condomínio ativo. O síndico escolhe a moeda num ecrã de definições do condomínio. Os valores continuam em cêntimos inteiros, e a fatura não tem moeda própria: usa sempre a moeda do condomínio.

A moeda é independente do idioma, que continua a ser uma preferência de cada utilizador (`en`, `pt-br` e agora `pt-PT`). Um utilizador em `pt-br` num condomínio em EUR vê valores em euros.

## Considered Options

- **EUR só no módulo de faturas, BRL no resto**: rejeitada porque o mesmo condomínio mostraria duas moedas.
- **Moeda por fatura**: rejeitada porque os totais e os indicadores teriam de somar moedas diferentes, o que exigiria câmbio.
- **Derivar a moeda do idioma**: rejeitada porque o idioma é do utilizador e a moeda é do condomínio.

## Consequences

- Os condomínios existentes ficam em BRL e não mudam de comportamento.
- Trocar a moeda de um condomínio que já tem valores registrados não converte nada. O ecrã de definições deve avisar disso.
- A formatação de dinheiro sai de `internal/features/finance/templates/helpers.go` e passa para `internal/shared/`, para as duas features a usarem.
