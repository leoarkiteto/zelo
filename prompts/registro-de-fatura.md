# Implementação de Módulo de Registo de Faturas – Portugal

Quero que desenvolvas um módulo completo de **Registo e Gestão de Faturas** na minha aplicação web.

O objetivo é permitir que os utilizadores registem, consultem, editem e acompanhem faturas recebidas de fornecedores em Portugal, independentemente da natureza da despesa.

Antes de começar a desenvolver, analisa a arquitetura existente do projeto, identifica as tecnologias utilizadas, os padrões de desenvolvimento, os componentes reutilizáveis, o sistema de autenticação, a estrutura da base de dados e o padrão visual da aplicação.

**IMPORTANTE:** Não alteres tecnologias, bibliotecas, arquitetura ou funcionalidades existentes sem necessidade. Segue os padrões já utilizados no projeto.

## 1. Objetivo funcional

O módulo deverá permitir o registo de diferentes tipos de faturas:

- Faturas de compras de produtos e mercadorias.
- Faturas de prestação de serviços.
- Faturas de despesas mensais (água, eletricidade, gás, internet, telefone, rendas, etc.).
- Faturas de despesas administrativas.
- Faturas de manutenção e serviços recorrentes.
- Outras despesas.

A aplicação deverá utilizar um formulário dinâmico, apresentando campos específicos conforme o tipo de despesa selecionado, evitando formulários excessivamente extensos.

## 2. Estrutura da página

Criar uma página denominada "Registo de Faturas", organizada nas seguintes secções:

### A. Identificação da fatura

Campos:

- Tipo de documento:
  - Fatura.
  - Fatura-recibo.
  - Fatura simplificada.
  - Nota de crédito.
  - Nota de débito.
  - Outro.
- Tipo de despesa:
  - Compra de produtos.
  - Prestação de serviços.
  - Despesa mensal.
  - Despesa administrativa.
  - Manutenção.
  - Outra despesa.
- Número da fatura.
- Data de emissão.
- Data de vencimento.
- Data da operação, quando diferente da data de emissão.
- Descrição geral da fatura.
- Moeda (EUR por defeito).

### B. Dados do fornecedor

Permitir selecionar um fornecedor já existente ou cadastrar um novo.

Campos:

- Nome / Denominação social.
- NIF.
- País.
- Morada.
- Código postal.
- Localidade.
- Email.
- Telefone.

Implementar pesquisa de fornecedores por nome ou NIF.

Quando um fornecedor existente for selecionado, preencher automaticamente os seus dados.

### C. Detalhes da despesa

Criar campos comuns:

- Categoria da despesa.
- Descrição.
- Projeto associado (opcional).
- Centro de custo (opcional).
- Período de referência (opcional).
- Observações.

Categorias iniciais:

- Compras e mercadorias.
- Equipamentos e informática.
- Material de escritório.
- Água.
- Eletricidade.
- Gás.
- Internet e telecomunicações.
- Rendas.
- Seguros.
- Contabilidade.
- Consultoria.
- Manutenção.
- Limpeza.
- Transportes e deslocações.
- Combustível.
- Despesas bancárias.
- Outros serviços.
- Outras despesas.

### D. Campos dinâmicos por tipo de despesa

Implementar apresentação condicional de campos.

**Quando for uma compra de produtos:**

- Produto.
- Referência do produto.
- Quantidade.
- Unidade de medida.
- Preço unitário.
- Desconto.
- Taxa de IVA.
- Total da linha.

Permitir adicionar várias linhas de produtos.

**Quando for uma prestação de serviços:**

- Descrição do serviço.
- Data de início da prestação.
- Data de fim da prestação.
- Contrato associado.
- Projeto associado.

**Quando for uma despesa mensal:**

- Tipo de serviço.
- Período de faturação.
- Data de início.
- Data de fim.
- Número de contrato ou cliente.
- Identificador do contador ou instalação, quando aplicável.
- CPE para eletricidade, quando aplicável.
- CUI para gás, quando aplicável.
- Local de consumo.
- Indicação de despesa recorrente.

Estes campos devem ser opcionais quando não forem aplicáveis.

### E. Valores e impostos

Implementar uma secção para os valores financeiros.

Campos:

- Valor bruto dos produtos/serviços.
- Descontos.
- Base tributável.
- Taxa de IVA.
- Valor do IVA.
- Retenção na fonte, quando aplicável.
- Motivo de isenção de IVA, quando aplicável.
- Total da fatura.
- Valor já pago.
- Valor em dívida.

Considerar as taxas de IVA aplicáveis em Portugal continental, Madeira e Açores, incluindo taxas reduzidas, intermédias, normais e situações de isenção.

Não assumir que todas as faturas possuem IVA de 23%.

Permitir diferentes taxas de IVA na mesma fatura.

Os valores devem ser calculados automaticamente, mas o utilizador deve conseguir consultar a discriminação dos impostos.

Utilizar precisão decimal adequada para cálculos financeiros, evitando erros de arredondamento.

### F. Pagamentos

Criar uma secção para controlo dos pagamentos.

Campos:

- Estado do pagamento:
  - Pendente.
  - Parcialmente pago.
  - Pago.
  - Anulado.
- Data do pagamento.
- Valor pago.
- Método de pagamento:
  - Transferência bancária.
  - Cartão.
  - Débito direto.
  - Dinheiro.
  - Multibanco.
  - MB WAY.
  - Outro.
- Referência de pagamento.
- Conta bancária utilizada.

**IMPORTANTE:** Uma fatura pode ter vários pagamentos. Portanto, não limitar a relação a um único pagamento por fatura.

O estado deverá ser atualizado automaticamente:

- Pendente: nenhum pagamento registado.
- Parcialmente pago: valor pago inferior ao total.
- Pago: valor pago igual ao total.
- Anulado: documento marcado como anulado, preservando o histórico.

### G. Documentos anexados

Permitir carregar documentos relacionados com a fatura.

Formatos aceites:

- PDF.
- JPG.
- JPEG.
- PNG.

Permitir anexar mais do que um documento.

Guardar os ficheiros de forma segura, respeitando a infraestrutura já existente no projeto.

## 3. Regras de negócio

Implementar as seguintes regras:

1. O número da fatura é obrigatório.
2. A data de emissão é obrigatória.
3. O fornecedor é obrigatório.
4. A descrição da despesa é obrigatória.
5. A categoria da despesa é obrigatória.
6. O total da fatura é obrigatório.
7. O NIF deve ser validado de acordo com as regras portuguesas, quando aplicável.
8. A data de vencimento não deve ser anterior à data de emissão, salvo situações justificadas.
9. O valor pago não pode ultrapassar o total da fatura, exceto quando existir uma regra de negócio que permita tratar pagamentos em excesso.
10. O valor em dívida deve ser calculado automaticamente.
11. Uma fatura não pode ser considerada paga se existir saldo em dívida.
12. As notas de crédito devem ser identificadas como documentos de natureza diferente das faturas normais.
13. Os campos específicos devem aparecer apenas quando forem relevantes para o tipo de despesa.
14. Não permitir duplicação indevida de documentos para o mesmo fornecedor.
15. Preservar o histórico de alterações e pagamentos.

Não aplicar regras fiscais rígidas sem considerar as exceções previstas na legislação portuguesa.

## 4. Estrutura da base de dados

Analisa a base de dados existente e implementa as tabelas ou entidades necessárias.

Como referência, considerar:

- invoices: dados gerais das faturas.
- suppliers: fornecedores.
- invoice_items: linhas de produtos ou serviços.
- invoice_taxes: discriminação dos impostos.
- invoice_payments: pagamentos.
- expense_categories: categorias de despesas.
- invoice_attachments: anexos.
- recurring_expenses: configurações de despesas recorrentes.

Criar os relacionamentos e as chaves estrangeiras adequadas.

Considerar:

- Auditoria de criação e atualização.
- Identificação do utilizador responsável pelo registo.
- Isolamento dos dados por empresa/organização, caso a aplicação seja multiempresa.
- Exclusão lógica quando apropriado.
- Integridade referencial.
- Índices para pesquisas frequentes.

Não duplicar entidades que já existam no projeto.

## 5. Interface e experiência do utilizador

Criar uma interface moderna, profissional e intuitiva.

A página deve:

- Respeitar o design system existente.
- Ser responsiva para desktop, tablet e dispositivos móveis.
- Utilizar componentes reutilizáveis.
- Apresentar o formulário dividido em secções.
- Utilizar campos condicionais.
- Ter validação visual dos campos obrigatórios.
- Apresentar mensagens claras de erro e sucesso.
- Permitir guardar como rascunho, caso seja compatível com a arquitetura existente.
- Permitir editar e consultar faturas registadas.
- Apresentar um resumo financeiro antes de guardar.

Organizar visualmente o formulário em:

1. Identificação.
2. Fornecedor.
3. Despesa.
4. Valores e impostos.
5. Pagamentos.
6. Anexos.

Evitar apresentar todos os campos ao mesmo tempo quando não forem necessários.

## 6. Listagem e pesquisa de faturas

Além do formulário de registo, criar ou integrar uma página de listagem de faturas.

Apresentar as seguintes colunas:

- Número da fatura.
- Fornecedor.
- NIF.
- Tipo de despesa.
- Data de emissão.
- Data de vencimento.
- Valor total.
- Valor pago.
- Valor em dívida.
- Estado do pagamento.

Implementar filtros por:

- Fornecedor.
- NIF.
- Número da fatura.
- Tipo de documento.
- Tipo de despesa.
- Categoria.
- Estado do pagamento.
- Data de emissão.
- Data de vencimento.
- Período de referência.

Permitir visualizar, editar e consultar os documentos anexados.

## 7. Indicadores e resumo financeiro

Se a aplicação já possuir dashboard financeiro, integrar os indicadores. Caso contrário, preparar a estrutura para futura integração.

Indicadores sugeridos:

- Total de faturas registadas.
- Total de despesas do mês.
- Total de faturas pendentes.
- Total de faturas pagas.
- Total de faturas parcialmente pagas.
- Total em dívida.
- Despesas por categoria.
- Despesas por fornecedor.

## 8. Considerações legais e fiscais de Portugal

A aplicação destina-se ao registo de faturas recebidas em Portugal.

Considerar as disposições aplicáveis do Código do IVA, especialmente:

- Artigo 29.º: obrigações de faturação.
- Artigo 36.º: prazos e formalidades das faturas.
- Artigo 40.º: faturas simplificadas.

Considerar também os diferentes regimes de IVA, isenções e situações de autoliquidação.

Distinguir os campos necessários para a gestão interna dos elementos legalmente obrigatórios que devem constar no documento original.

Não assumir que a aplicação é um software certificado de faturação, uma vez que o objetivo principal é o registo e a gestão de documentos recebidos.

## 9. Qualidade e testes

Implementar:

- Validações frontend e backend.
- Tratamento de erros.
- Proteção contra submissões duplicadas.
- Testes dos cálculos de IVA.
- Testes de pagamentos parciais.
- Testes de atualização do estado da fatura.
- Testes de campos dinâmicos.
- Testes de permissões e acesso aos documentos.

Garantir que os dados financeiros são persistidos corretamente.

## 10. Instruções finais de implementação

Segue esta ordem de trabalho:

1. Analisar o projeto existente.
2. Identificar os componentes e serviços reutilizáveis.
3. Apresentar uma breve proposta de implementação.
4. Implementar a estrutura de dados e respetivas migrações.
5. Implementar backend, APIs e validações.
6. Implementar a interface de registo.
7. Implementar a listagem, pesquisa e filtros.
8. Implementar anexos e pagamentos.
9. Executar testes e corrigir eventuais problemas.
10. Apresentar um resumo dos ficheiros criados e alterados.

**Não te limites a criar um protótipo visual.** Quero uma funcionalidade efetivamente integrada com a aplicação e a base de dados existente.

Não utilizar dados fictícios na implementação final, exceto em testes.

Não remover funcionalidades existentes nem alterar partes não relacionadas com este módulo.

Se alguma informação sobre a arquitetura ou regras de negócio for indispensável e não puder ser identificada no projeto, apresenta a dúvida antes de tomar decisões que possam comprometer a implementação.

O resultado esperado é um módulo completo, funcional, escalável e preparado para gerir diferentes tipos de faturas emitidas em Portugal.
