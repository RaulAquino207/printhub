# Minimal / Modern — Design System para agentes

**Versão:** 1.0  
**Data:** 2026-10-22  
**Status:** padrão recomendado, adaptável entre projetos  
**Referência visual:** shadcn/ui — linguagem minimalista/clássica (Vega como ponto de partida), sem copiar a identidade visual de outro produto.

> **Regra central:** a interface deve parecer um produto digital cuidadosamente desenhado: limpa, legível, discreta, consistente e funcional. Priorize clareza e usabilidade; evite efeitos decorativos.

Este documento é **uma especificação de equipe/projeto**, não um preset oficial fornecido pelo shadcn/ui. Os tokens, tamanhos e convenções abaixo são decisões nossas para garantir consistência entre agentes.

## 1. Stack e princípios

- **Frontend recomendado:** React + TypeScript; Next.js para aplicações que se beneficiem dele.
- **Estilização:** Tailwind CSS; utilizar variáveis CSS/tokens semânticos no tema do shadcn/ui.
- **Componentes:** shadcn/ui (componentes instalados no repositório). Antes de implementar, inspecione os componentes existentes e o `components.json`; não pressuponha uma API específica se o projeto já tem uma configuração.
- **Ícones:** Lucide; normalmente 16–20 px e traço uniforme.
- **Fonte:** Geist Sans (preferencial) ou Inter; monospace só para código, identificadores e dados técnicos.
- **Estilo base:** neutro, predominantemente preto/cinza/branco. Light como aparência inicial; dark mode se fizer sentido.
- **Prioridades:** (1) acessibilidade e fluxo, (2) hierarquia, (3) consistência, (4) responsividade, (5) detalhes estéticos.

**Regras inegociáveis:**
1. Utilizar componentes shadcn/ui já existentes, não reinventar botões, selects, diálogos ou tabelas.
2. Usar tokens como `bg-background`, `text-foreground`, `bg-card`, `text-muted-foreground`, `border-border`, `ring-ring`; não espalhar hexadecimais pelas telas.
3. Não criar uma paleta nova a cada funcionalidade.
4. Evitar gradientes, glassmorphism, neon, sombras pesadas, emojis como ícones de UI, animações excessivas e páginas lotadas de cards.
5. Se houver design system preexistente, adaptar esta especificação ao projeto antes de migrar componentes: não quebrar interfaces existentes para impor o padrão.

## 2. Identidade visual — tokens

### 2.1 Cores semânticas — modo claro

| Papel | Valor de referência | Uso |
|---|---|---|
| `background` | `#FAFAFA` | Fundo principal da aplicação |
| `foreground` | `#171717` | Texto primário |
| `card` / `popover` | `#FFFFFF` | Superfícies elevadas |
| `primary` | `#171717` | Ação primária e seleção forte |
| `primary-foreground` | `#FFFFFF` | Texto sobre primary |
| `secondary` / `muted` | `#F5F5F5` | Plano de fundo secundário |
| `secondary-foreground` | `#262626` | Texto secundário forte |
| `muted-foreground` | `#737373` | Labels, metadados e ajuda |
| `accent` | `#F5F5F5` | Hover/seleção suave |
| `border` / `input` | `#E5E5E5` | Contornos e divisores |
| `ring` | `#737373` | Foco visível |
| `destructive` | `#DC2626` | Ações destrutivas |

**Cores de status** (usar em texto/ícone e fundos sutis; nunca como única indicação): success `#15803D`, warning `#B45309`, info `#2563EB`, error `#DC2626`. Para badges, usar variantes suaves. Uma cor de marca opcional deve ser discreta e usada com parcimônia, jamais em todos os cartões.

### 2.2 Cores semânticas — modo escuro

| Papel | Valor de referência |
|---|---|
| `background` | `#0A0A0A` |
| `foreground` | `#FAFAFA` |
| `card` / `popover` | `#171717` |
| `primary` | `#FAFAFA` |
| `primary-foreground` | `#171717` |
| `secondary` / `muted` / `accent` | `#262626` |
| `secondary-foreground` | `#FAFAFA` |
| `muted-foreground` | `#A3A3A3` |
| `border` / `input` | `#333333` |
| `ring` | `#A3A3A3` |
| `destructive` | `#F87171` |

**Cores de status no modo escuro** (adicionado no PrintHub, 2026-10-25): success `#4ADE80`, warning `#FBBF24`, info `#60A5FA`, error = `destructive` `#F87171`. Os tons claros do modo claro têm contraste insuficiente sobre `#0A0A0A`.

O dark mode deve alterar **tokens**, não duplicar toda a UI com estilos independentes.

### 2.3 CSS de tema — integração com shadcn/ui

Para um novo projeto, configure um tema com **base Neutral**, variáveis CSS ativadas e estilo visual clássico/próximo de **Vega** no [shadcn/create](https://ui.shadcn.com/create). Escolha o motor de componentes suportado pelo projeto (Radix/Base UI/React Aria, conforme a versão e necessidade) e mantenha essa escolha consistente.

Em projetos existentes, **preserve o CSS gerado pelo shadcn/ui**, incluindo os mapeamentos `@theme inline` do Tailwind CSS e tokens adicionais. Sobreponha somente os valores semânticos necessários em `:root` / `.dark`. Exemplo de referência:

```css
/* Adicionar ao globals.css existente; não substituir os imports nem @theme inline. */
:root {
  --radius: 0.625rem;
  --background: #fafafa;
  --foreground: #171717;
  --card: #ffffff;
  --card-foreground: #171717;
  --popover: #ffffff;
  --popover-foreground: #171717;
  --primary: #171717;
  --primary-foreground: #ffffff;
  --secondary: #f5f5f5;
  --secondary-foreground: #262626;
  --muted: #f5f5f5;
  --muted-foreground: #737373;
  --accent: #f5f5f5;
  --accent-foreground: #171717;
  --destructive: #dc2626;
  --border: #e5e5e5;
  --input: #e5e5e5;
  --ring: #737373;
}

.dark {
  --background: #0a0a0a;
  --foreground: #fafafa;
  --card: #171717;
  --card-foreground: #fafafa;
  --popover: #171717;
  --popover-foreground: #fafafa;
  --primary: #fafafa;
  --primary-foreground: #171717;
  --secondary: #262626;
  --secondary-foreground: #fafafa;
  --muted: #262626;
  --muted-foreground: #a3a3a3;
  --accent: #262626;
  --accent-foreground: #fafafa;
  --destructive: #f87171;
  --border: #333333;
  --input: #333333;
  --ring: #a3a3a3;
}
```

**Nota técnica:** o código acima é um *override de tokens*, não um `globals.css` completo. As classes do Tailwind só enxergam esses nomes se o projeto já tiver os mapeamentos esperados pelo shadcn/ui. Não misturar esse formato com scaffolds antigos que esperam `hsl(var(--background))` sem adaptar os mapeamentos. Use a documentação oficial correspondente à versão instalada.

## 3. Tipografia

| Elemento | Desktop | Mobile | Peso | Regra |
|---|---|---|---|---|
| Título de página (H1) | 28–32 px | 24–28 px | 600–700 | Um por tela |
| Título de seção (H2) | 20–24 px | 18–20 px | 600 | Curto e descritivo |
| Título de card (H3) | 16–18 px | 16–18 px | 600 | Sem letras todas maiúsculas |
| Corpo | 14–16 px | 14–16 px | 400 | Altura de linha 1.45–1.6 |
| Label | 13–14 px | 13–14 px | 500 | Associado ao input |
| Legenda/metadados | 12–13 px | 12–13 px | 400 | Apenas informações secundárias |

- Usar `font-sans` em toda a interface; títulos com `tracking-tight` apenas quando favorecer legibilidade.
- Não usar mais de duas famílias tipográficas nem misturar várias escalas arbitrárias.
- Nunca depender de texto menor que 12 px para informação essencial.
- Priorizar português claro e microcopy objetiva.

## 4. Espaçamento, layout e elevação

**Escala base:** 4 / 8 / 12 / 16 / 24 / 32 / 48 / 64 px, que equivale a unidades Tailwind como `1`, `2`, `3`, `4`, `6`, `8`, `12`, `16`.

- Gap entre ícone e rótulo: 8 px.
- Gap entre campos relacionados: 12–16 px.
- Padding de cards: 16–24 px.
- Distância entre seções: 24–32 px.
- Padding lateral mobile: 16 px (20 px quando houver espaço).
- Container desktop: `max-w-6xl` ou `max-w-7xl` conforme densidade; centralizado.
- Formulários longos: limitar a largura de leitura (`max-w-xl` ou `max-w-2xl`).
- Preferir **respiro e divisores** a múltiplos cards aninhados.

**Forma:** raio padrão 10 px (`--radius: 0.625rem`); utilizar `rounded-lg`/`rounded-xl` de forma consistente. Bordas `1px` sutis. Sombras mínimas (`shadow-sm`) apenas em menus, popovers e elementos realmente elevados. Nada de sombras dramáticas.

## 5. Componentes e padrões de interação

### Botões
- Primário: fundo `primary`, texto `primary-foreground`; **apenas uma ação primária por contexto**.
- Secundário: `outline` ou `secondary` para ações auxiliares.
- Destrutivo: `destructive` somente para apagar, cancelar definitivamente ou ações similares.
- Ghost: ícones utilitários ou ações de baixa ênfase.
- Altura padrão ~40 px; alvo de toque ideal >=44×44 px no mobile.
- Estados obrigatórios: normal, hover, focus-visible, disabled e loading. Durante uma operação, impedir submissão duplicada.

### Formulários
- Labels acima do campo; nunca usar placeholder como substituto do label.
- Agrupar campos com hierarquia clara e descrição apenas quando necessária.
- Mensagens de validação próximas do campo; indicar solução, não apenas erro.
- Usar `Select`, `Checkbox`, `Switch`, `RadioGroup` e `Input` já presentes no projeto.
- Para ações de risco, confirmar com `AlertDialog` e texto específico.

### Cards e tabelas
- Cards são para informações ou ações que merecem agrupamento; não criar card para cada linha simples.
- Tabelas: títulos claros, alinhamento numérico à direita, ações discretas e paginação quando houver volume.
- Em telas pequenas, substituir tabelas densas por linhas/cards responsivos quando isso realmente ajudar.

### Feedback
- Loading: skeleton para conteúdo, spinner apenas em ações curtas e localizadas.
- Empty state: mensagem direta, explicação curta e uma próxima ação útil.
- Erro: descrição clara, opção de tentar novamente e, quando cabível, detalhe técnico acessível.
- Sucesso: feedback discreto (toast/estado atualizado), sem animação comemorativa excessiva.
- Status: texto + ícone/cor. Não depender só de vermelho/verde.

### Navegação
- Em aplicações pequenas: header simples ou navegação superior.
- Sidebar apenas quando existirem seções suficientes para justificá-la.
- Destaque o item ativo; não lotar a navegação com links raramente usados.
- Fluxo prioritário sempre identificável em até poucos segundos.

## 6. Responsive e acessibilidade

- Começar por mobile; trabalhar em 360–390 px, 768 px e 1280+ px.
- Mobile: coluna única; desktop: duas colunas apenas se houver uma relação clara entre elas.
- Evitar rolagem horizontal inesperada.
- Prever teclado, foco visível, navegação por Tab/Escape e rótulos acessíveis.
- Considerar WCAG AA para contraste de texto, em particular cinza claro sobre branco.
- Em atualizações assíncronas, usar `aria-live`/componentes adequados quando o usuário precisar ser informado.
- Respeitar `prefers-reduced-motion`; transições discretas (aprox. 150–200 ms), sem animações essenciais para entender o estado.
- Quando houver upload, validar formato, tamanho, progresso e falhas; fornecer alternativas ao drag-and-drop com seletor tradicional.

## 7. Composição de telas

**Template de página funcional:**

1. Cabeçalho compacto: título e uma frase de contexto opcional.
2. Uma ação principal bem identificada.
3. Conteúdo principal organizado em uma seção dominante.
4. Opções secundárias menos enfatizadas.
5. Rodapé/feedback de estado quando útil.

**Não adicionar por padrão:** gráficos, métricas, grandes hero banners, sidebar, múltiplos CTAs, ícones puramente decorativos ou dashboards de administração. Só usar se o caso de uso exigir.

## 8. Convenções para agentes de código

Antes de escrever UI, o agente deve:

1. Ler este arquivo, o `components.json`, o tema global e os componentes existentes.
2. Identificar as convenções reais do framework e a versão da biblioteca no repositório.
3. Descrever brevemente o fluxo principal e os estados da tela (normal, loading, vazio, erro, sucesso).
4. Reutilizar os tokens e componentes existentes; não criar estilos divergentes em novas telas.
5. Implementar responsividade e acessibilidade como parte do escopo, não como extras.
6. Executar os testes/lint/build disponíveis e inspecionar visualmente quando houver ambiente de preview.
7. Quando um novo padrão reutilizável for necessário, documentá-lo aqui antes de propagá-lo a outras telas.

**Checklist de aceitação** (passa/falha):

- [ ] A tela é consistente com tokens e tipografia deste guia.
- [ ] Há uma ação principal óbvia; ações secundárias não competem.
- [ ] É utilizável em mobile e desktop sem overflow inesperado.
- [ ] Loading, vazio, erro e sucesso foram considerados.
- [ ] Formulários têm labels, feedback e foco visível.
- [ ] Os componentes shadcn/ui existentes são reutilizados.
- [ ] Não há cores hardcoded espalhadas pela árvore de componentes.
- [ ] Não há enfeites gratuitos nem duplicação de padrões.
- [ ] O agente relatou o que implementou e o que não conseguiu verificar.

## 9. Exemplo de adaptação — PrintHub (não é regra global)

**Produto:** central de impressão privada.  
**Fluxo primário:** selecionar PDF → configurar impressão → enviar → acompanhar status.  
**Telas:** Nova impressão (principal), Fila, Histórico, Configurações.  
**Layout:** em desktop, upload/preview na área principal e configurações em painel lateral; em mobile, formulário empilhado com CTA bem visível.  
**Feedback:** estados de upload, fila, processando, concluído, falhou e impressora offline.  
**Limite:** não implementar opções que a impressora não suporte.  
**Ilustração da impressora** (adicionado em 2026-10-25): miniatura 4:3 (`PrinterImage`, 64×48 px; 48×36 px em listas densas) com fundo branco e borda sutil, inclusive no modo escuro, sempre ao lado do nome da impressora e com `alt` vazio (o nome já informa). Sem imagem cadastrada, usar o ícone `Printer` do Lucide sobre `bg-muted`. A imagem nunca substitui o selo "Simulada".

O projeto PrintHub utiliza a mesma identidade Minimal / Modern, mas cada novo produto poderá adaptar sua navegação e a cor de marca sem modificar os fundamentos.

## 10. Referências oficiais

- shadcn/ui Introduction: https://ui.shadcn.com/docs
- shadcn/ui Theming: https://ui.shadcn.com/docs/theming
- shadcn/create: https://ui.shadcn.com/create
- Instalação Next.js: https://ui.shadcn.com/docs/installation/next
- Configuração `components.json`: https://ui.shadcn.com/docs/components-json

**Regra de atualização:** ao adotar versões novas de shadcn/ui ou Tailwind, verifique a documentação atual antes de reutilizar configurações e comandos deste guia. Os conceitos visuais permanecem; APIs e formatos de configuração podem mudar.
