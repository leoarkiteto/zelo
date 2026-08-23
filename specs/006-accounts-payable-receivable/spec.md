# Feature Specification: Gestão de Contas a Pagar e Receber

**Feature Branch**: `006-accounts-payable-receivable`

**Created**: 2026-08-23

**Status**: Draft

**Input**: User description: "# Especificacao de Funcionalidade: Gestao de Contas a Pagar e Receber ## Contexto e Objetivo Como sindico ou gestor financeiro de um condominio, preciso de um modulo para registrar, acompanhar e gerenciar todas as movimentacoes financeiras (despesas do condominio e receitas provenientes de cotas ou taxas), para garantir a saude financeira e a transparencia na prestacao de contas aos moradores. --- ## Historias de usuarios & Personas ### Persona: Sindico/Gestor Financeiro - **Historia 1:** Como gestor, quero cadastrar uma nova despesa (conta a pagar), para que eu possa agendar pagamentos a fornecedores e manter o controle dos gastos do condominio. - **Historia 2:** Como gestor, quero lancar uma nova receita (conta a receber) associada a uma unidade/condominio ou origem externa, para ter previsibilidade de arrecadacao. - **Historia 3:** Como gestor, quero atualizar o status de uma conta (marcar como pago, recebido ou cancelado), para manter o saldo real do condominio atualizado. - **Historia 4:** Como gestor, quero filtrar lancamentos por periodo, tipo e status, para identificar rapidamente inadimplencias ou contas prestes a vencer. - **Historia 5:** Como gestor, quero visualizar o resumo financeiro (total a pagar, total a receber e saldo), para tomar decisoes orcamentarias rapidas. ### Persona: Morador - **Historia 6:** Como morador, quero visualizar as taxas/cotas pendentes vinculadas a minha unidade, para efetuar o pagamento dentro do prazo. - **Historia 7:** Como morador, quero visualizar a saude financeira do condominio, o que foi gasto, o que foi arrecadado. --- ## Requisitos funcionais ### 1. Gestao de contas a pagar (despesas) - Permitir a criacao, edicao e cancelamento de despesas. - **Campo obrigatorios:** Titulo/Descricao, Categoria, Valor, Data de Vencimento e Status Inicial. - **Campos opcionais:** Fornecedor/Credor, Data de Pagamento e Comprovante de Pagamento - **Categorias suportadas:** Manutencao, Limpeza, Utilizadas (Agua/Luz/GAs), Folha de pagamento, Servicos de Terceiros e Diversos. ## 2. Gestao de contas a receber (receitas) - Permitir a criacao, edicao e cancelamento de receitas. - **Campos obrigatorios:** Titulo/Descricao, Categoria, Valor, Data de Vencimento e Status Inicial. - **Campo opcionais:** Unidade/Condominio responsavel, Data de Recebimento e Codigo/Linha de Pagamento. - **Categorias suportadas:** Cota Condominial, Multa/Juros, Reserva de Espaco Comum e Receitas Extraordinarias. ## 3. Estados e Regras de Negocio de Status - **Status validos:** `Pendente`, `Liquidado` (Pago/Recebido), `Em Atraso` e `Cancelado`. - Transicao de estado: - Qualquer conta com data de vencimento anterior a data atual e status `Pendente` deve ser sinalizado automaticamente com `Em Atraso`. - Ao registrar o pagamento/recebimento, o sistema dever exigir a data em que a transacao ocorreu. ### 4. Visualizacao e Dashboard Financeiro - Exibicao em lista de lancamentos com destaques visuais por status. - Painel resumido exibindo: - Total a Receber no mes. - Total a Pagar no mes. - Saldo Previsto vs. Saldo Realizado. - Mecanismo de busca textual e filtros rapidos por periodo (Mes/Ano), Categoria e Status. --- ## Criterios de Aceite - [ ] O gestor consegue cadastrar uma conta a pagar ou receber em menos de 1 minuto. - [ ] O sistema impede a criacao de contas com valores zerados ou negativos. - [ ] Lancamentos em atraso ficam visualmente destacados (ex: alerta em vermelho). - [ ] Possivel filtrar a lista para ver apenas moradores inadimplentes (contas a receber `Em Atraso`) - [ ] O saldo consolidado sera recalculado sempre que o status de uma conta muda."

## Clarifications

### Session 2026-08-23

- Q: O sistema deve gerar boletos/Pix para contas a receber, ou apenas armazenar o código/linha de pagamento informado manualmente pelo gestor? → A: Apenas armazenar o código/linha de pagamento informado manualmente; sem geração de boletos/Pix (v1 sem integração bancária).
- Q: Por quanto tempo o sistema deve reter os registros financeiros (lançamentos, pagamentos, recebimentos) antes de permitir exclusão ou arquivamento? → A: Mínimo de 5 anos; após esse período, os registros podem ser arquivados ou excluídos.
- Q: Ao criar uma conta, o status inicial pode ser diferente de Pendente (ex.: Liquidado com data de transação para lançamentos retroativos)? → A: Sim; o gestor pode criar como Pendente ou Liquidado, e Liquidado exige a data da transação.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Gestor cadastra uma despesa (conta a pagar) (Priority: P1)

O gestor acessa o módulo financeiro e cadastra uma nova despesa informando título/descrição, categoria, valor e data de vencimento. A conta é criada com o status inicial `Pendente` e passa a aparecer na lista de lançamentos, permitindo agendar o pagamento a fornecedores e manter o controle dos gastos do condomínio.

**Why this priority**: É a base do módulo. Sem o cadastro de despesas não existe acompanhamento financeiro nem prestação de contas.

**Independent Test**: Autenticado como gestor, cadastrar uma despesa com os campos obrigatórios e confirmar que ela aparece na lista com status `Pendente` e valor correto.

**Acceptance Scenarios**:

1. **Given** um gestor autenticado na tela de lançamentos, **When** ele cadastra uma despesa com título, categoria, valor e data de vencimento, **Then** a conta é criada com status `Pendente` e exibida na lista.
2. **Given** o formulário de nova despesa, **When** o gestor tenta salvar sem preencher um campo obrigatório, **Then** o sistema exibe mensagem clara indicando o campo e não cria a conta.
3. **Given** o formulário de nova despesa, **When** o gestor informa valor zero ou negativo, **Then** o sistema bloqueia o cadastro e explica o motivo.
4. **Given** o cadastro de despesa, **When** o gestor preenche os campos opcionais (fornecedor/credor, data de pagamento, comprovante), **Then** essas informações ficam vinculadas à conta.

---

### User Story 2 - Gestor lança uma receita (conta a receber) (Priority: P1)

O gestor lança uma nova receita informando título/descrição, categoria, valor, data de vencimento e, quando aplicável, a unidade responsável. Receitas sem unidade específica são tratadas como receitas do condomínio ou de origem externa. O lançamento permite previsibilidade de arrecadação.

**Why this priority**: As receitas (especialmente cotas condominiais) são a principal fonte de recursos do condomínio; sem lançá-las, não há como acompanhar a arrecadação nem identificar inadimplência.

**Independent Test**: Autenticado como gestor, lançar uma receita de cota condominial vinculada a uma unidade e confirmar que ela aparece na lista com status `Pendente` e vínculo correto.

**Acceptance Scenarios**:

1. **Given** um gestor autenticado, **When** ele lança uma receita com título, categoria, valor, vencimento e unidade responsável, **Then** a conta é criada com status `Pendente` e vinculada à unidade informada.
2. **Given** uma receita sem unidade responsável, **When** o gestor a lança indicando origem "condomínio" ou "externa", **Then** a conta é criada sem vínculo com unidade específica.
3. **Given** o formulário de nova receita, **When** o gestor tenta salvar sem campos obrigatórios, **Then** o sistema bloqueia e indica os campos faltantes.
4. **Given** o formulário de nova receita, **When** o gestor informa valor zero ou negativo, **Then** o sistema bloqueia o cadastro.

---

### User Story 3 - Gestor atualiza o status de uma conta (Priority: P1)

O gestor registra o pagamento de uma despesa ou o recebimento de uma receita, informando a data em que a transação ocorreu, e a conta passa para `Liquidado`. O gestor também pode cancelar uma conta pendente. Contas `Pendente` com vencimento anterior à data atual são sinalizadas automaticamente como `Em Atraso`.

**Why this priority**: A atualização de status é o que mantém o saldo real do condomínio. Sem ela, o módulo mostra apenas previsões, não a realidade financeira.

**Independent Test**: Marcar uma despesa pendente como paga informando a data de pagamento e confirmar que o status muda para `Liquidado` e que o resumo financeiro reflete a mudança.

**Acceptance Scenarios**:

1. **Given** uma conta com status `Pendente`, **When** o gestor registra o pagamento/recebimento informando a data da transação, **Then** a conta passa para `Liquidado` com a data registrada.
2. **Given** o registro de pagamento/recebimento, **When** o gestor tenta liquidar sem informar a data da transação, **Then** o sistema exige a data e não conclui a operação.
3. **Given** uma conta `Pendente` com data de vencimento anterior à data atual, **When** ela é exibida na lista, **Then** é sinalizada automaticamente como `Em Atraso`, sem ação manual.
4. **Given** uma conta `Pendente` ou `Em Atraso`, **When** o gestor a cancela, **Then** a conta passa para `Cancelado` e sai dos totais ativos.
5. **Given** uma conta `Liquidado` ou `Cancelado`, **When** o gestor tenta editá-la, **Then** a edição não é permitida; a conta fica disponível apenas para consulta.
6. **Given** uma mudança de status de conta, **When** o resumo financeiro é consultado, **Then** o saldo consolidado é recalculado automaticamente.

---

### User Story 4 - Gestor filtra e busca lançamentos (Priority: P2)

O gestor usa busca textual e filtros rápidos por período (mês/ano), categoria e status para localizar lançamentos. Um filtro específico permite ver apenas contas a receber `Em Atraso`, identificando rapidamente moradores inadimplentes e contas prestes a vencer.

**Why this priority**: Com volume crescente de lançamentos, a capacidade de localizar rapidamente contas específicas é essencial para a operação diária, mas o módulo já entrega valor com cadastro e status.

**Independent Test**: Com lançamentos de períodos, categorias e status variados, aplicar filtros e confirmar que a lista mostra apenas os lançamentos correspondentes.

**Acceptance Scenarios**:

1. **Given** lançamentos em meses diferentes, **When** o gestor filtra por um mês/ano, **Then** a lista mostra apenas os lançamentos do período selecionado.
2. **Given** lançamentos de categorias e status variados, **When** o gestor filtra por categoria e/ou status, **Then** a lista mostra apenas os lançamentos correspondentes.
3. **Given** a busca textual, **When** o gestor digita parte do título ou do fornecedor, **Then** a lista mostra os lançamentos correspondentes ao termo.
4. **Given** contas a receber `Em Atraso` e contas em outros status, **When** o gestor aplica o filtro de inadimplentes, **Then** a lista mostra apenas contas a receber `Em Atraso`.
5. **Given** lançamentos `Em Atraso` na lista, **When** são exibidos, **Then** recebem destaque visual claro (por exemplo, alerta em vermelho).

---

### User Story 5 - Gestor visualiza o resumo financeiro (dashboard) (Priority: P2)

O gestor acessa um painel resumido com Total a Receber no mês, Total a Pagar no mês e Saldo Previsto vs. Saldo Realizado. O painel é atualizado automaticamente a cada mudança de status e apoia decisões orçamentárias rápidas.

**Why this priority**: O resumo transforma os lançamentos em informação de decisão, mas depende dos dados das histórias anteriores para existir.

**Independent Test**: Com lançamentos liquidados e pendentes no mês, abrir o painel e conferir se os totais e os saldos previsto/realizado correspondem aos lançamentos.

**Acceptance Scenarios**:

1. **Given** lançamentos a receber e a pagar no mês corrente, **When** o gestor abre o painel, **Then** o Total a Receber no mês e o Total a Pagar no mês são exibidos corretamente.
2. **Given** lançamentos `Pendente` e `Liquidado` no mês, **When** o gestor abre o painel, **Then** o Saldo Previsto e o Saldo Realizado são exibidos e coerentes com os lançamentos.
3. **Given** uma mudança de status de conta, **When** o gestor consulta o painel novamente, **Then** os totais e saldos refletem a mudança sem ação manual.

---

### User Story 6 - Morador visualiza taxas/cotas pendentes da sua unidade (Priority: P2)

O morador acessa sua área e vê as taxas/cotas pendentes vinculadas à sua unidade, com valor, vencimento e status, para efetuar o pagamento dentro do prazo. Contas de outras unidades não são visíveis.

**Why this priority**: É o canal que permite ao morador quitar suas obrigações em dia; sem ele, a cobrança depende de meios externos ao sistema.

**Independent Test**: Autenticado como morador de uma unidade com pendências, acessar a área de pendências e confirmar que apenas as contas da própria unidade são exibidas.

**Acceptance Scenarios**:

1. **Given** um morador autenticado, **When** ele acessa a área de pendências, **Then** vê as taxas/cotas vinculadas à sua unidade com valor, vencimento e status.
2. **Given** o morador visualizando pendências, **When** a lista é exibida, **Then** não aparecem contas de outras unidades nem informações de fornecedores.
3. **Given** uma pendência da unidade com status `Em Atraso`, **When** o morador a visualiza, **Then** ela recebe destaque visual de atraso.

---

### User Story 7 - Morador visualiza a saúde financeira do condomínio (Priority: P3)

O morador acessa uma visão agregada da saúde financeira do condomínio: quanto foi gasto e quanto foi arrecadado no período, sem detalhes confidenciais (fornecedores, comprovantes ou contas de outras unidades).

**Why this priority**: Atende à transparência na prestação de contas, mas é um valor complementar ao fluxo principal de gestão e pagamento.

**Independent Test**: Autenticado como morador, acessar a visão de saúde financeira e confirmar os totais agregados de gastos e arrecadação do período.

**Acceptance Scenarios**:

1. **Given** um morador autenticado, **When** ele acessa a saúde financeira, **Then** vê os totais de gastos e arrecadação do período selecionado (mês/ano).
2. **Given** a visão de saúde financeira, **When** o morador a consulta, **Then** não vê comprovantes, fornecedores ou detalhes de contas de outras unidades.
3. **Given** um período sem lançamentos, **When** o morador consulta a saúde financeira, **Then** os valores aparecem zerados com mensagem amigável.

---

### Edge Cases

- O que acontece quando a data de vencimento é igual à data atual? A conta permanece `Pendente`; ela se torna `Em Atraso` somente quando a data de vencimento for anterior à data atual.
- O que acontece ao criar uma conta já `Liquidado`? É permitido, desde que a data da transação seja informada no momento da criação.
- O que acontece ao tentar editar uma conta `Liquidado` ou `Cancelado`? A edição é bloqueada; a conta fica disponível somente para consulta.
- O que acontece ao tentar cancelar uma conta `Liquidado`? O cancelamento é bloqueado; apenas contas `Pendente` ou `Em Atraso` podem ser canceladas.
- O que acontece ao liquidar uma conta `Em Atraso`? É permitido, desde que a data da transação seja informada; a conta passa para `Liquidado` e sai do destaque de atraso.
- O que acontece quando a data de liquidação é anterior ao vencimento? É permitido (pagamento/recebimento antecipado).
- O que acontece com uma receita sem unidade responsável? Ela é tratada como receita do condomínio/origem externa, sem vínculo com unidade.
- O que acontece com valores monetários fora do formato esperado? O sistema valida o formato monetário e impede o cadastro com valor inválido.
- O que acontece quando a busca/filtro não encontra resultados? O sistema exibe um estado vazio com mensagem amigável.
- O que acontece quando o período do resumo não tem lançamentos? O painel exibe zeros.
- O que acontece quando um morador tenta acessar funções do gestor? O acesso é negado; moradores veem somente informações da própria unidade e dados agregados.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: O sistema DEVE permitir ao gestor cadastrar, editar e cancelar contas a pagar (despesas).
- **FR-002**: Toda despesa DEVE ter os campos obrigatórios título/descrição, categoria, valor, data de vencimento e status inicial; os campos fornecedor/credor, data de pagamento e comprovante de pagamento são opcionais.
- **FR-003**: Despesas DEVEM suportar as categorias: Manutenção, Limpeza, Utilidades (Água/Luz/Gás), Folha de Pagamento, Serviços de Terceiros e Diversos.
- **FR-004**: O sistema DEVE permitir ao gestor cadastrar, editar e cancelar contas a receber (receitas).
- **FR-005**: Toda receita DEVE ter os campos obrigatórios título/descrição, categoria, valor, data de vencimento e status inicial; os campos unidade/condomínio responsável, data de recebimento e código/linha de pagamento são opcionais.
- **FR-006**: Receitas DEVEM suportar as categorias: Cota Condominial, Multa/Juros, Reserva de Espaço Comum e Receitas Extraordinárias.
- **FR-007**: Os status válidos DEVEM ser: `Pendente`, `Liquidado` (Pago/Recebido), `Em Atraso` e `Cancelado`.
- **FR-008**: Todo lançamento DEVE ter um status inicial; o padrão para novos lançamentos é `Pendente`, mas o gestor PODE criar um lançamento já como `Liquidado`, desde que informe a data da transação. O status `Em Atraso` não pode ser selecionado manualmente.
- **FR-009**: Toda conta com status `Pendente` e data de vencimento anterior à data atual DEVE ser sinalizada automaticamente como `Em Atraso`, sem ação manual.
- **FR-010**: Ao registrar o pagamento/recebimento, o sistema DEVE exigir a data em que a transação ocorreu.
- **FR-011**: O sistema DEVE impedir a criação de contas com valor zero ou negativo.
- **FR-012**: A lista de lançamentos DEVE exibir destaques visuais por status, com alerta claro para `Em Atraso` (por exemplo, destaque em vermelho).
- **FR-013**: O painel resumido DEVE exibir Total a Receber no mês, Total a Pagar no mês e Saldo Previsto vs. Saldo Realizado.
- **FR-014**: O sistema DEVE oferecer busca textual e filtros rápidos por período (mês/ano), categoria e status.
- **FR-015**: O sistema DEVE permitir filtrar a lista para exibir apenas contas a receber `Em Atraso` (moradores inadimplentes).
- **FR-016**: O morador DEVE poder visualizar as taxas/cotas pendentes vinculadas à sua unidade, sem ver contas de outras unidades.
- **FR-017**: O morador DEVE poder visualizar a saúde financeira do condomínio (total gasto e total arrecadado) de forma agregada, sem detalhes confidenciais.
- **FR-018**: O saldo consolidado DEVE ser recalculado automaticamente sempre que o status de uma conta mudar.
- **FR-019**: Edição e cancelamento DEVEM ser permitidos apenas para contas `Pendente` ou `Em Atraso`; contas `Liquidado` ou `Cancelado` são somente leitura.
- **FR-020**: Receitas DEVEM poder ser vinculadas a uma unidade específica ou ao condomínio/origem externa quando não houver unidade responsável.
- **FR-021**: O sistema NÃO DEVE gerar boletos/Pix; o código/linha de pagamento é registrado manualmente pelo gestor, sem integração com provedor bancário/pagamento.
- **FR-022**: O sistema DEVE reter os registros financeiros por no mínimo 5 anos; antes desse prazo, a exclusão definitiva não é permitida, e o arquivamento pode ser usado após o período.

### Key Entities *(include if feature involves data)*

- **Lançamento Financeiro (Conta)**: representa uma movimentação a pagar ou a receber; atributos principais: tipo (pagar/receber), título/descrição, categoria, valor, data de vencimento, status, data de liquidação, comprovante, fornecedor/credor e unidade responsável/código de pagamento.
- **Unidade**: representa um imóvel do condomínio; possui identificação, morador(es) vinculado(s) e os lançamentos a receber a ela associados.
- **Resumo Financeiro**: visão derivada dos lançamentos; expõe totais do mês (a receber, a pagar), saldo previsto e saldo realizado, sem armazenamento próprio.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: O gestor consegue cadastrar uma conta a pagar ou a receber em menos de 1 minuto.
- **SC-002**: 100% das tentativas de criar contas com valor zero ou negativo são bloqueadas com mensagem clara.
- **SC-003**: 100% das contas `Pendente` com vencimento anterior à data atual são exibidas automaticamente como `Em Atraso`.
- **SC-004**: O saldo consolidado é recalculado imediatamente após cada mudança de status, sem ação manual.
- **SC-005**: O filtro de inadimplentes exibe apenas contas a receber `Em Atraso`, com precisão de 100%.
- **SC-006**: 90% dos gestores localizam um lançamento específico em até 30 segundos usando busca e filtros.
- **SC-007**: Nenhuma conta é liquidada sem o registro da data da transação (100% das liquidações exigem a data).
- **SC-008**: O morador identifica suas pendências em até 30 segundos após acessar a área.

## Assumptions

- A moeda utilizada é o Real (BRL), com valores expressos em duas casas decimais; as datas seguem o fuso local.
- O sistema atende um único condomínio com múltiplas unidades; receitas sem unidade são tratadas como receitas do condomínio/origem externa.
- A liquidação é sempre pelo valor integral; pagamentos e recebimentos parciais estão fora do escopo desta versão.
- Multas e juros automáticos por atraso estão fora do escopo; valores de multa/juros são lançados manualmente pelo gestor na categoria apropriada.
- As categorias são listas fixas conforme especificado; personalização de categorias não faz parte desta versão.
- O comprovante de pagamento é um anexo (imagem ou PDF) vinculado à conta.
- A emissão de boletos/Pix está fora do escopo; o código/linha de pagamento é registrado manualmente pelo gestor.
- A data de vencimento igual à data atual não caracteriza atraso; a conta torna-se `Em Atraso` somente quando o vencimento for anterior à data atual.
- O acesso é autenticado: o gestor tem permissões administrativas; o morador vê somente informações da própria unidade e dados agregados.
- Dados financeiros detalhados (fornecedores, comprovantes) são restritos ao gestor.
- Notificações automáticas (e-mail/push) de vencimento ou atraso estão fora do escopo desta versão; a identificação de atrasos ocorre pelos destaques visuais e filtros.
- Importação em massa de dados financeiros existentes (planilhas/CSV) está fora do escopo; os lançamentos são cadastrados manualmente.
- Em v1, um usuário morador está vinculado a uma única unidade; o vínculo de um morador a múltiplas unidades pode ser tratado como melhoria futura.
- Em caso de edição concorrente da mesma conta por dois gestores, a última alteração salva prevalece; não há bloqueio de edição simultânea nesta versão.
