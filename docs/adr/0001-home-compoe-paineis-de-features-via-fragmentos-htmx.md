---
status: accepted
---

# A home compõe painéis de outras features por fragmentos HTMX

A página `/` é da feature `home`, mas precisa mostrar o painel financeiro, cujos dados e regras pertencem à feature `finance`. Features não podem importar umas às outras (`scripts/check-feature-boundaries.sh`). Decidimos que a `finance` serve o painel como um fragmento HTML numa rota própria (`GET /finance/overview`) e que a home o carrega com `hx-get` + `hx-trigger="load"`. Assim cada feature continua dona dos seus dados, das suas consultas, do seu template e do controle de acesso. Se o fragmento falhar, a home continua funcionando e mostra um aviso com o link "Tentar novamente".

## Considered Options

- **Transferir `/` para a `finance`**: rejeitada porque a saudação, as ações rápidas e os papéis não são responsabilidade financeira.
- **Página separada (`/finance/dashboard`) com link na home**: rejeitada porque o morador não veria a movimentação financeira ao entrar no portal, que era o objetivo.
- **Interface compartilhada em `internal/shared` que a home chamaria**: rejeitada porque moveria regras e consultas financeiras para o código compartilhado só para atravessar a fronteira.

## Consequences

- O painel custa uma segunda requisição depois do carregamento da home.
- Novos painéis de outras features na home devem seguir o mesmo padrão: rota de fragmento na feature dona, carregada pela home.
- O fragmento aplica seu próprio controle de acesso; a home não decide quem vê dados financeiros.
