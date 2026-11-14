# Feature Specification: Painel Financeiro do Condomínio

**Feature Branch**: `012-financial-dashboard`

**Created**: 2026-11-14

**Status**: Draft

**Input**: User description: "Implemente o dashboard para todos os usuarios, onde ele veram graficos da receita do condominio (entradas/saidas), toda a vida financeira do condominio. Use @refs/dashboard.png como referencia de UI"

**Discovery**: decisões, vocabulário e escopo vêm da sessão de discovery (`.factory/workflows/wf-20261114-dashboard-financeiro-do-condominio-com-g/discovery-summary.md`), do [GLOSSARY.md](../../GLOSSARY.md) e do [ADR 0001](../../docs/adr/0001-home-compoe-paineis-de-features-via-fragmentos-htmx.md). Os termos **Entrada**, **Saída**, **Saldo do período**, **Movimentação financeira**, **Realizado** e **Previsto** seguem o glossário.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Ver a movimentação financeira ao entrar no portal (Priority: P1)

Como membro do condomínio (proprietário, inquilino ou síndico), ao abrir a página inicial do portal quero ver um painel com as entradas, as saídas e o saldo do condomínio nos últimos 12 meses, para acompanhar a vida financeira do condomínio com transparência.

**Why this priority**: é o objetivo do pedido. Sem os indicadores e o gráfico de entradas × saídas não há painel.

**Independent Test**: com um condomínio que tem contas liquidadas nos últimos 12 meses, entrar como inquilino e verificar na página inicial os totais de Entradas, Saídas e Saldo do período e o gráfico mensal de entradas × saídas, conferindo os valores com as liquidações cadastradas.

**Acceptance Scenarios**:

1. **Given** um condomínio com contas a receber e a pagar liquidadas nos últimos 12 meses, **When** um inquilino abre a página inicial, **Then** vê os indicadores Entradas, Saídas e Saldo do período dos últimos 12 meses, com valores iguais à soma das liquidações do período.
2. **Given** o mesmo condomínio, **When** o painel é exibido, **Then** há um gráfico com, para cada um dos 12 meses, uma barra de Entradas e uma barra de Saídas daquele mês.
3. **Given** contas canceladas ou ainda pendentes no período, **When** o painel é exibido, **Then** elas não entram em Entradas, Saídas nem no gráfico mensal.
4. **Given** uma conta com vencimento em março e liquidação em abril, **When** o painel é exibido, **Then** ela é contada em abril.
5. **Given** um proprietário e um síndico do mesmo condomínio, **When** cada um abre a página inicial, **Then** ambos veem os mesmos valores que o inquilino.

---

### User Story 2 - Entender o saldo mês a mês e para onde vai o dinheiro (Priority: P2)

Como membro, quero ver o saldo de cada mês e a divisão das entradas e saídas por categoria, para entender em quais meses o condomínio gastou mais do que arrecadou e com o que o dinheiro é gasto.

**Why this priority**: complementa a visão geral com explicação. Depende dos mesmos dados da US1.

**Independent Test**: com liquidações em várias categorias e ao menos um mês com saídas maiores que entradas, verificar o gráfico de saldo mensal (com o mês negativo abaixo do zero) e as listas por categoria com valores e percentuais corretos.

**Acceptance Scenarios**:

1. **Given** um mês em que as saídas superam as entradas, **When** o painel é exibido, **Then** o gráfico de saldo mostra esse mês abaixo da linha do zero e destacado como negativo.
2. **Given** saídas liquidadas em manutenção, limpeza e consumo, **When** o painel é exibido, **Then** a lista "Saídas por categoria" mostra cada categoria com seu valor e seu percentual do total de saídas do período, ordenada do maior para o menor valor.
3. **Given** entradas liquidadas em taxa condominial e multa e juros, **When** o painel é exibido, **Then** a lista "Entradas por categoria" mostra o mesmo formato para as entradas.
4. **Given** uma categoria sem nenhuma liquidação no período, **When** o painel é exibido, **Then** ela não aparece na lista.

---

### User Story 3 - Saber o que está previsto e em atraso agora (Priority: P2)

Como membro, quero ver quanto o condomínio ainda tem a receber e a pagar no mês corrente e quanto está em atraso, para entender a situação imediata do condomínio.

**Why this priority**: dá a visão do "agora", que o histórico realizado não mostra.

**Independent Test**: com contas pendentes que vencem no mês corrente e contas a receber vencidas antes de hoje, verificar os indicadores A receber, A pagar e Em atraso.

**Acceptance Scenarios**:

1. **Given** contas a receber pendentes com vencimento no mês corrente, **When** o painel é exibido, **Then** o indicador "A receber" mostra a soma delas.
2. **Given** contas a pagar pendentes com vencimento no mês corrente, **When** o painel é exibido, **Then** o indicador "A pagar" mostra a soma delas.
3. **Given** contas a receber pendentes com vencimento anterior a hoje, **When** o painel é exibido, **Then** o indicador "Em atraso" mostra apenas o valor total delas, sem quantidade, unidades ou moradores.
4. **Given** uma conta a receber que vence hoje e está pendente, **When** o painel é exibido, **Then** ela não entra em "Em atraso".
5. **Given** que o membro escolheu outro ano no seletor, **When** o painel é exibido, **Then** A receber, A pagar e Em atraso continuam referentes ao mês corrente e a hoje.

---

### User Story 4 - Consultar outro ano (Priority: P3)

Como membro, quero escolher um ano específico, para consultar a movimentação financeira de anos anteriores.

**Why this priority**: amplia o histórico além dos 12 meses, mas o painel padrão já entrega o valor principal.

**Independent Test**: com liquidações em dois anos diferentes, trocar o período no seletor e verificar que os indicadores do período, os gráficos e as listas passam a refletir o ano escolhido.

**Acceptance Scenarios**:

1. **Given** um condomínio com liquidações em 2025 e 2026, **When** o membro abre o seletor de período, **Then** as opções são "Últimos 12 meses", 2026 e 2025, e nenhum ano sem liquidações aparece.
2. **Given** o membro escolhe 2025, **When** o painel é atualizado, **Then** os gráficos mostram janeiro a dezembro de 2025 e os indicadores e listas do período se referem a 2025.
3. **Given** o membro escolhe o ano corrente, **When** o painel é atualizado, **Then** os gráficos vão de janeiro até o mês atual, sem meses futuros.
4. **Given** o membro troca o período, **When** o painel é atualizado, **Then** o restante da página inicial (saudação, ações rápidas, papéis) não é recarregado nem alterado.

---

### User Story 5 - Síndico chega rápido à gestão (Priority: P3)

Como síndico, quero, a partir do painel, ir para a gestão de contas, e, num condomínio ainda sem movimentação, ser convidado a registrar a primeira conta.

**Why this priority**: conveniência. A gestão já existe.

**Independent Test**: entrar como síndico e como inquilino e comparar a presença do link "Gerenciar finanças" e do botão "Registrar conta" no estado vazio.

**Acceptance Scenarios**:

1. **Given** um síndico, **When** o painel é exibido, **Then** há um link "Gerenciar finanças" para a gestão de contas.
2. **Given** um proprietário ou inquilino, **When** o painel é exibido, **Then** esse link não aparece.
3. **Given** um período sem nenhuma liquidação, **When** um síndico vê o painel, **Then** aparece a mensagem de período sem movimentação e um botão "Registrar conta". Para proprietários e inquilinos aparece só a mensagem.

---

### Edge Cases

- **Período sem liquidações**: no lugar dos gráficos e listas aparece "Nenhuma movimentação financeira em <período>". Os indicadores do período mostram zero. Os indicadores do mês corrente continuam sendo calculados normalmente.
- **Mês sem liquidações dentro de um período com dados**: o mês aparece no eixo com barras zeradas e saldo zero. Meses não são omitidos.
- **Saldo do período negativo**: o indicador mostra o valor negativo, destacado como negativo.
- **Usuário logado sem papel ativo no condomínio**: o painel não aparece na página inicial, e a consulta direta aos dados do painel é negada.
- **Membro de outro condomínio**: nunca vê valores de um condomínio diferente do da sua sessão.
- **Falha ao carregar o painel**: a página inicial continua funcionando. No lugar do painel aparece um aviso curto com "Tentar novamente".
- **Período inválido na consulta** (ex.: ano inexistente ou texto qualquer): o painel usa o padrão "Últimos 12 meses".
- **Ano sem liquidações pedido explicitamente**: é exibido como período vazio.
- **Tela estreita**: os gráficos rolam na horizontal dentro do próprio cartão, sem comprimir os 12 meses.
- **Página antiga de saúde financeira**: quem acessa o endereço antigo é redirecionado para a página inicial, e o item sai do menu.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: A página inicial DEVE exibir o painel financeiro do condomínio da sessão para todo usuário com papel ativo de proprietário, inquilino ou síndico nesse condomínio.
- **FR-002**: O painel NÃO DEVE ser exibido, e seus dados DEVEM ser negados, para usuários sem papel ativo no condomínio da sessão. Os valores DEVEM sempre se restringir ao condomínio da sessão.
- **FR-003**: O painel DEVE mostrar apenas valores agregados. NÃO DEVE exibir lançamentos individuais, títulos de contas, fornecedores, unidades ou moradores.
- **FR-004**: Todos os membros (proprietário, inquilino, síndico) DEVEM ver os mesmos valores para o mesmo período.
- **FR-005**: Entradas, Saídas, Saldo do período, o gráfico mensal, o gráfico de saldo e as listas por categoria DEVEM considerar apenas contas liquidadas (**Realizado**), agrupadas pelo mês da data de liquidação. Contas canceladas e pendentes NÃO DEVEM ser consideradas.
- **FR-006**: O painel DEVE exibir os indicadores do período **Entradas** (soma das contas a receber liquidadas), **Saídas** (soma das contas a pagar liquidadas) e **Saldo do período** (Entradas − Saídas).
- **FR-007**: O painel DEVE exibir os indicadores do mês corrente (**Previsto**): **A receber** (soma das contas a receber pendentes com vencimento no mês corrente) e **A pagar** (soma das contas a pagar pendentes com vencimento no mês corrente).
- **FR-008**: O painel DEVE exibir o indicador **Em atraso**: o valor total das contas a receber pendentes com vencimento anterior a hoje. NÃO DEVE exibir quantidade de contas, unidades ou moradores.
- **FR-009**: Os indicadores A receber, A pagar e Em atraso NÃO DEVEM variar com o período selecionado.
- **FR-010**: O painel DEVE exibir um gráfico de barras agrupadas com Entradas e Saídas de cada mês do período, incluindo meses sem movimentação (barras zeradas).
- **FR-011**: O painel DEVE exibir um gráfico de linha com o Saldo do período de cada mês, com uma linha de referência no zero e os meses negativos destacados. NÃO DEVE exibir saldo acumulado nem saldo inicial.
- **FR-012**: O painel DEVE exibir as listas **Saídas por categoria** e **Entradas por categoria**, cada uma com o valor por categoria, o percentual sobre o total do tipo no período e uma barra proporcional, ordenadas do maior para o menor valor, omitindo categorias sem valor.
- **FR-013**: O período padrão DEVE ser os últimos 12 meses, terminando no mês corrente, inclusive.
- **FR-014**: O membro DEVE poder escolher como período "Últimos 12 meses" ou um ano específico. Só DEVEM ser oferecidos os anos que têm ao menos uma liquidação.
- **FR-015**: Num ano específico, o eixo dos gráficos DEVE cobrir de janeiro a dezembro. No ano corrente, DEVE ir apenas até o mês atual.
- **FR-016**: Trocar o período DEVE atualizar apenas o painel, sem recarregar o restante da página inicial, e DEVE funcionar também sem scripts no navegador (envio comum de formulário).
- **FR-017**: Um período inválido DEVE ser tratado como "Últimos 12 meses".
- **FR-018**: Cada valor dos gráficos DEVE poder ser consultado de forma exata: ao passar o cursor sobre a barra ou o ponto, e numa tabela de dados recolhível associada a cada gráfico.
- **FR-019**: Num período sem liquidações, o painel DEVE substituir gráficos e listas pela mensagem "Nenhuma movimentação financeira em <período>" e mostrar os indicadores do período zerados.
- **FR-020**: Para o síndico, o painel DEVE exibir o link "Gerenciar finanças" para a gestão de contas e, no estado vazio, um botão "Registrar conta" para o cadastro de conta. Proprietários e inquilinos NÃO DEVEM ver esses elementos.
- **FR-021**: Se o painel falhar ao carregar, o restante da página inicial DEVE continuar funcionando, e o espaço do painel DEVE exibir um aviso curto com a ação "Tentar novamente".
- **FR-022**: Os valores DEVEM ser exibidos como moeda formatada segundo o idioma do usuário, e todos os textos do painel DEVEM existir em português e inglês.
- **FR-023**: O endereço antigo da saúde financeira DEVE redirecionar para a página inicial, e o item correspondente DEVE sair do menu de navegação.
- **FR-024**: O painel DEVE seguir o sistema de design do portal: Entradas em verde, Saídas em vermelho, Saldo na cor principal com meses negativos em vermelho, e Em atraso em âmbar. Não entram cores ou marcas da imagem de referência.
- **FR-025**: Em telas estreitas, o painel DEVE se organizar em uma coluna, e os gráficos DEVEM rolar horizontalmente dentro do próprio cartão.
- **FR-026**: Os dados de demonstração do ambiente de desenvolvimento DEVEM incluir 12 meses de entradas e saídas liquidadas, de modo que todos os gráficos tenham dados. A carga de demonstração DEVE continuar repetível sem duplicar dados e DEVE continuar recusando o ambiente de produção.

### Key Entities *(include if feature involves data)*

- **Conta a pagar / Conta a receber** (existentes): tipo, categoria, valor, vencimento, status (pendente, liquidada, cancelada; "em atraso" é derivado) e data de liquidação. São a única fonte de dados do painel.
- **Período do painel**: "Últimos 12 meses" ou um ano específico. Define os meses do eixo e o intervalo das liquidações consideradas.
- **Resumo do período** (derivado): Entradas, Saídas e Saldo do período; série mensal de entradas, saídas e saldo; totais por categoria de cada tipo.
- **Situação do mês corrente** (derivado): A receber, A pagar e Em atraso.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: 100% dos membros com papel ativo veem o painel na página inicial, e 0% dos usuários sem papel ativo conseguem obter dados do painel.
- **SC-002**: Para um conjunto de dados de teste conhecido, todos os indicadores, barras, pontos de saldo e totais por categoria coincidem exatamente, ao centavo, com a soma das liquidações correspondentes.
- **SC-003**: Nenhuma informação individual (título de conta, fornecedor, unidade, morador ou quantidade de contas em atraso) aparece no painel, verificado em todos os papéis.
- **SC-004**: O painel fica visível em até 2 segundos após a abertura da página inicial, com 5 anos de movimentação cadastrada.
- **SC-005**: Um membro consegue responder "quanto o condomínio arrecadou e gastou em <mês>" e "qual categoria mais consumiu dinheiro no período" em menos de 30 segundos, sem sair da página inicial.
- **SC-006**: A página inicial continua utilizável (saudação, ações rápidas, navegação) em 100% dos casos em que o painel falha ao carregar.

## Assumptions

- O painel é somente leitura. Não há novos dados persistidos nem novas tabelas: tudo é derivado das contas a pagar e a receber existentes.
- "Hoje" e os limites de mês seguem a convenção já usada na gestão financeira do portal.
- A exibição do painel na página inicial é composta conforme o ADR 0001: a página inicial carrega um painel servido pela área financeira, que aplica o próprio controle de acesso.
- O seletor oferece apenas "Últimos 12 meses" e anos com liquidações. Não há a opção "desde sempre".
- Saldo inicial, saldo em caixa e conta bancária não existem no sistema e estão fora do escopo.
- Fora do escopo: exportar, busca, notificações, orçamento ("meta × realizado"), lançamentos individuais no painel, edição a partir do painel e os widgets da referência sem correspondência no domínio (mapa, satisfação, visitantes).
- A página da área financeira para moradores "Minhas cobranças" continua existindo sem mudanças.
