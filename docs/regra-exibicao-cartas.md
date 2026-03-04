# Regra de Exibição de Cartas — Repetição Espaçada (SM-2)

## Visão Geral

A plataforma usa o algoritmo **SM-2** (SuperMemo 2), adaptado, para decidir quando cada carta deve
ser mostrada novamente. O objetivo é exibir a carta no momento certo: antes de você esquecê-la,
mas não cedo demais para não desperdiçar tempo revendo o que você já sabe bem.

Cada resposta do aluno atualiza três valores armazenados por carta:

| Variável | O que representa |
|---|---|
| `interval_days` | Quantos dias até a próxima revisão |
| `ease_factor` (FE) | Facilidade da carta (quanto o intervalo cresce a cada acerto) |
| `streak` | Sequência de respostas não-erradas consecutivas |

---

## Os Três Tipos de Resposta

### ❌ ERREI (`result = 0`)

O aluno não lembrou ou errou a resposta.

```
próximo intervalo = 1 dia          (reinicia, volta amanhã)
novo FE           = FE atual − 0,20
streak            = 0              (reinicia)
```

**Efeito:** A carta aparece **amanhã**, mais vezes, até consolidar o aprendizado.
O fator de facilidade cai, tornando os intervalos futuros menores.

---

### ⚠️ DIFÍCIL (`result = 1`)

O aluno lembrou, mas com esforço.

```
próximo intervalo = intervalo atual × 1,2   (cresce lentamente)
novo FE           = FE atual − 0,15
streak            = streak + 1              (mantém a sequência)
```

**Efeito:** A carta volta **em breve**, com crescimento mais conservador.
O fator de facilidade cai um pouco, sinalizando que a carta precisa de mais atenção.

---

### ✅ ACERTEI (`result = 2`)

O aluno lembrou com facilidade.

```
streak 0 (primeira vez)  → próximo intervalo = 1 dia
streak 1 (segunda vez)   → próximo intervalo = 6 dias
streak ≥ 2               → próximo intervalo = intervalo atual × FE

novo FE  = FE atual + 0,10
streak   = streak + 1
```

**Efeito:** A carta vai ficando cada vez mais espaçada conforme o aluno acerta repetidamente.
O fator de facilidade sobe, fazendo os intervalos crescerem mais rápido.

---

## Fator de Facilidade (FE)

O FE controla a taxa de crescimento dos intervalos e fica sempre entre **1,3** e **2,5**:

- **Valor inicial:** 2,5 (padrão SM-2)
- **Mínimo:** 1,3 — mesmo cartas difíceis continuam crescendo, só que devagar
- **Máximo:** 2,5 — evita que cartas fáceis demorem anos para aparecer

Quanto maior o FE, mais rápido os intervalos crescem após acertos.

---

## Exemplo Prático

Suponha uma carta nova sendo respondida corretamente várias vezes seguidas (FE inicial = 2,5):

| Resposta | Streak | Intervalo calculado | Próxima revisão |
|---|---|---|---|
| ACERTEI | 1 | 1 dia | amanhã |
| ACERTEI | 2 | 6 dias | em 6 dias |
| ACERTEI | 3 | 6 × 2,5 = **15 dias** | em 15 dias |
| ACERTEI | 4 | 15 × 2,6 = **39 dias** | em 39 dias |
| DIFÍCIL | 5 | 39 × 1,2 = **47 dias** | em 47 dias (FE cai para 2,45) |
| ERREI | — | **1 dia** | amanhã (streak e FE reiniciam) |

---

## Quando uma Carta Aparece no Modo "Pendentes"

Uma carta é exibida no modo **"Pendentes"** quando:

```sql
review não existe   →  carta nunca estudada (prioridade máxima)
      OU
next_due ≤ agora    →  prazo de revisão atingido
```

A ordenação é por `next_due` crescente — cartas vencidas há mais tempo aparecem primeiro.

---

## Modos de Estudo

| Modo | Lógica de seleção |
|---|---|
| **Pendentes** (padrão) | Cartas sem review + cartas com `next_due ≤ agora`, ordenadas por prazo |
| **Aleatório** | Qualquer carta do deck, em ordem aleatória, sem repetição na mesma rodada |
| **Erradas** | Cartas com `last_result = 0` (ERREI) nos últimos 7 dias; se não houver, cai para Pendentes |

> **Nota — Modo Aleatório:** O sistema rastreia as cartas exibidas na sessão e as exclui das
> próximas requisições. Ao terminar todas as cartas do deck, exibe a tela "Rodada concluída!" e
> recomeça do zero com novo embaralhamento.

---

## Resumo Visual

```
                        ┌─────────────────────────────────────┐
                        │         Carta é exibida             │
                        └───────────────┬─────────────────────┘
                                        │
              ┌─────────────────────────┼─────────────────────────┐
              │                         │                         │
         ❌ ERREI                   ⚠️ DIFÍCIL                ✅ ACERTEI
              │                         │                         │
   intervalo = 1 dia         intervalo × 1,2              streak 0 → 1 dia
   FE − 0,20                 FE − 0,15                    streak 1 → 6 dias
   streak = 0                streak + 1                   streak ≥ 2 → × FE
              │                         │                   FE + 0,10
              │                         │                   streak + 1
              └────────────────┬────────┘                         │
                               │                                  │
                        Carta salva com                    Carta salva com
                      novo intervalo curto               intervalo crescente
                      (aparece em breve)                 (aparece mais tarde)
```

---

## Implementação

O algoritmo está em [`internal/service/study.go`](../internal/service/study.go), função `Schedule`.  
A seleção de cartas está em [`internal/repository/study.go`](../internal/repository/study.go),
funções `NextDueCard`, `NextRandomCard` e `NextWrongCard`.
