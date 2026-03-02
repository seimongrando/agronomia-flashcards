# Agronomia – Flashcards

Plataforma de estudos por repetição espaçada voltada para cursos de Agronomia. Backend em **Go** + **PostgreSQL**, frontend estático embutido no binário (sem CDN, zero dependências de JS em runtime).

---

## Funcionalidades

### Para Alunos
- **Estudo com repetição espaçada (SM-2)** — algoritmo clássico com intervalos crescentes para memorização eficiente.
- **Decks pessoais** — criação e gerenciamento de cards próprios, com tipagem visual por cor.
- **Turmas** — entrar em turmas via código de convite e estudar os decks atribuídos pelo professor.
- **Dashboard de progresso** — histórico de sessões, taxa de acerto, cards difíceis e streaks de estudo.
- **Modo estudante** (para admin/professor) — simular a experiência do aluno sem alterar papéis no banco.
- **PWA** — instalável em dispositivos móveis via Service Worker.

### Para Professores
- **Gestão de conteúdo** — criar, editar, ativar/desativar e excluir decks e cards.
- **Tipos de card** — Conceito, Processo, Aplicação e Comparação (com indicação visual por cor).
- **Importação CSV** — upload em lote com preview, detecção de erros e UPSERT seguro.
- **Exportação CSV** — download de qualquer deck com todos os campos.
- **Turmas** — criar turmas, gerar código de convite, atribuir/remover decks, monitorar alunos.
- **Relatórios** — visão geral e por turma: taxa de acerto, cards difíceis, alunos ativos, desempenho por deck.
- **Encerramento automático** — definir data de expiração por deck.

### Para Administradores
- **Gestão de usuários** — promover/rebaixar papéis (student, professor, admin).
- **Acesso total** — visualizar e gerenciar qualquer deck, turma ou usuário.
- **Exclusão em lote** — deletar múltiplos decks de uma vez.

---

## Rodar local em 2 minutos

### Pré-requisitos

| Ferramenta | Instalação |
|---|---|
| Go 1.23+ | https://go.dev/dl/ |
| Docker + Docker Compose | https://docs.docker.com/get-docker/ |
| `golang-migrate` CLI | `brew install golang-migrate` · [outras formas](https://github.com/golang-migrate/migrate/tree/master/cmd/migrate) |
| Credenciais Google OAuth | veja seção abaixo |

### 1. Configurar variáveis de ambiente

```bash
cp .env.local.example .env.local
```

Edite `.env.local` e preencha **obrigatoriamente**:

```env
GOOGLE_CLIENT_ID=seu-client-id.apps.googleusercontent.com
GOOGLE_CLIENT_SECRET=seu-client-secret
ADMIN_EMAILS=seuemail@gmail.com     # seu email do Google para acesso admin
```

> Como obter credenciais Google OAuth: veja [Configurar Google OAuth](#configurar-google-oauth).

### 2. Subir tudo com um comando

```bash
make dev
```

Este comando executa em sequência: `docker-up → db-wait → migrate-up → run`.

Abra **http://localhost:8080** e clique em *Entrar com Google*.

---

## Configurar Google OAuth

1. Acesse [console.cloud.google.com/apis/credentials](https://console.cloud.google.com/apis/credentials).
2. Crie um projeto (ou selecione um existente).
3. **APIs e Serviços → Credenciais → Criar credencial → ID do cliente OAuth 2.0**.
4. Tipo: **Aplicativo da Web**.
5. Configure:
   - **Origens JavaScript autorizadas:** `http://localhost:8080`
   - **URIs de redirecionamento autorizados:** `http://localhost:8080/auth/google/callback`
6. Copie o **Client ID** e **Client Secret** para `.env.local`.

---

## Papéis e Acesso

| Papel | Permissões |
|---|---|
| `student` | Estudar decks públicos e de turma, criar decks pessoais, ver próprio progresso |
| `professor` | + Gerenciar decks/cards próprios, criar e gerenciar turmas próprias, ver relatórios |
| `admin` | + Gerenciar todos os decks/turmas, gerenciar usuários, acesso a qualquer conteúdo |

**Promoção via `ADMIN_EMAILS`** (automático no primeiro login):

```env
# .env.local
ADMIN_EMAILS=seuemail@gmail.com
```

**Promoção manual via psql:**

```bash
make psql
```

```sql
INSERT INTO user_roles (user_id, role)
  SELECT id, 'professor' FROM users WHERE email = 'email@dominio.com';
```

---

## Páginas

| URL | Acesso | Descrição |
|---|---|---|
| `/` | Todos | Home — decks por matéria/turma |
| `/study.html` | Todos | Estudo com repetição espaçada |
| `/progress.html` | Aluno | Dashboard de progresso pessoal |
| `/classes.html` | Aluno | Turmas e decks pessoais |
| `/my_decks.html` | Aluno | Gerenciar decks pessoais |
| `/my_deck.html` | Aluno | Cards de um deck pessoal |
| `/teach.html` | Professor/Admin | Gerenciar decks e cards |
| `/deck_manage.html` | Professor/Admin | Cards de um deck gerenciado |
| `/classes.html` | Professor/Admin | Gerenciar turmas |
| `/class_manage.html` | Professor/Admin | Detalhe de turma + relatórios |
| `/professor_stats.html` | Professor/Admin | Painel de relatórios geral |
| `/me.html` | Todos | Perfil do usuário |
| `/admin_users.html` | Admin | Gerenciamento de usuários |

---

## Comandos Make

```bash
make help          # lista todos os comandos disponíveis
```

| Comando | Descrição |
|---|---|
| `make dev` | 🚀 Bootstrap completo: DB → migrações → servidor |
| `make run` | Inicia o servidor (lê `.env.local`) |
| `make docker-up` | Sobe o Postgres em background |
| `make docker-up-tools` | Sobe Postgres + Adminer (inspector DB na porta 8081) |
| `make docker-down` | Para os serviços (mantém dados) |
| `make docker-reset` | ⚠ Apaga todos os dados locais (pede confirmação) |
| `make db-wait` | Aguarda o Postgres aceitar conexões |
| `make migrate-up` | Aplica migrações pendentes |
| `make migrate-down` | Desfaz a última migração |
| `make migrate-create name=<slug>` | Cria um novo par de arquivos de migração |
| `make migrate-status` | Mostra a versão atual das migrações |
| `make seed-admin` | Exibe instruções para habilitar acesso admin |
| `make psql` | Abre shell psql dentro do container |
| `make test` | Roda todos os testes com race detector |
| `make lint` | Executa golangci-lint (ou go vet como fallback) |
| `make build` | Compila binário de produção em `bin/server` |
| `make docker-build` | Constrói imagem Docker de produção |

---

## Inspecionar o banco de dados

**Opção A — Adminer (interface web):**

```bash
make docker-up-tools
# Abra: http://localhost:8081
# Sistema: PostgreSQL | Servidor: postgres | Usuário/DB: conforme .env.local
```

**Opção B — psql direto no container:**

```bash
make psql
```

---

## Estrutura do projeto

```
webapp/
├── cmd/server/           # Entrypoint (main.go) — roteamento e composição DI
├── internal/
│   ├── config/           # Configuração via variáveis de ambiente
│   ├── csvparse/         # Parser CSV robusto (UTF-8/BOM, validação, UPSERT)
│   ├── handler/          # HTTP handlers (content, study, class, admin)
│   ├── middleware/        # Auth JWT, CSRF, CORS, rate limit, security headers
│   ├── model/            # DTOs e structs de domínio
│   ├── pagination/       # Cursor-based pagination helpers
│   ├── repository/       # SQL (parametrizado, sem ORM)
│   ├── service/          # Lógica de negócio (SM-2, RBAC, turmas, progresso)
│   └── validate/         # Validação de input (UUID, strings, campos obrigatórios)
├── migrations/           # SQL versionado (golang-migrate)
│   ├── 001_init.up.sql              # Schema base
│   ├── 002_upsert_constraints.up.sql
│   ├── 003_pagination_indexes.up.sql
│   ├── 004_progress.up.sql
│   ├── 005_subject.up.sql
│   ├── 006_private_decks.up.sql     # Decks pessoais de alunos
│   ├── 007_study_streak.up.sql      # Streak de estudos
│   ├── 008_pwa_icons.up.sql
│   ├── 009_home_perf.up.sql         # Índices de performance
│   ├── 010_classes.up.sql           # Turmas e membros
│   └── 011_perf_indexes.up.sql      # Índices adicionais (uploads, reviews)
├── web/                  # Frontend estático embutido no binário (//go:embed)
│   ├── static/css/style.css
│   ├── static/js/        # Um .js por página (sem bundler)
│   └── *.html
├── .env.example          # Referência de todas as variáveis
├── .env.local.example    # Template pronto para desenvolvimento local
├── docker-compose.yml    # Postgres + Adminer
├── Dockerfile            # Build multi-stage de produção
├── Makefile              # Automação de tarefas
└── render.yaml           # Deploy automático no Render.com
```

---

## Arquitetura

**Camadas:** `handler → service → repository`, todas em `internal/`. Handlers não acessam o banco diretamente — passam pelo service que aplica regras de negócio, RBAC e ownership.

**Autenticação:** JWT em cookie `HttpOnly`, obtido via OAuth2 Google. O token carrega `user_id`, `email` e papéis (`roles`). O middleware `RequireAuth` valida a assinatura antes de qualquer handler protegido.

**RBAC por rota:**

| Middleware | Papéis permitidos |
|---|---|
| `authOnly` | Qualquer usuário autenticado |
| `contentMgmt` | `professor`, `admin` |
| `staffOnly` | `professor`, `admin` |
| `adminOnly` | `admin` |

**Stack de middleware** (aplicada na ordem):

1. `RequestID` — geração de ID único por requisição
2. `Logger` — log estruturado JSON (slog)
3. `SecurityHeaders` — CSP, X-Frame-Options, X-Content-Type-Options, Referrer-Policy
4. `CORS` — restrito por `ALLOWED_ORIGINS`
5. `CSRF` — valida Origin/Referer em métodos mutantes
6. `MaxBody` — limita tamanho do body
7. `RateLimit` — token bucket por IP (auth com limite mais restrito)

**Algoritmo SM-2:** `service/study.go` — implementação do algoritmo de repetição espaçada. Ratings: 0=errou, 1=difícil, 2=acertou. Intervalo inicial 1 dia, crescimento exponencial controlado pelo ease factor.

**Paginação:** cursor-based em todos os endpoints de lista, usando `(name, id)` ou `(updated_at, id)` como chave composta para estabilidade de ordenação.

**Endpoints de health:**
- `GET /healthz` — liveness (sempre 200)
- `GET /readyz` — readiness (pinga o banco)

---

## Variáveis de ambiente

Veja [`.env.example`](.env.example) para todas as opções com valores padrão.
Para desenvolvimento local use [`.env.local.example`](.env.local.example) como base.

---

## Deploy (Render)

Conecte o repositório ao [Render](https://render.com). O arquivo `render.yaml` provisiona automaticamente um web service (`agronomia-flashcards`) + Postgres.

Configure as variáveis no dashboard do Render:
- `GOOGLE_CLIENT_ID`, `GOOGLE_CLIENT_SECRET`
- `JWT_SECRET` (string aleatória longa)
- `ADMIN_EMAILS`
- `BASE_URL` (URL pública do serviço, ex: `https://agronomia-flashcards.onrender.com`)

---

## Segurança

- Cookies `HttpOnly`, `SameSite=Lax`, `Secure` (em produção).
- JWT em cookie — nunca exposto em `localStorage`.
- CSRF protegido via validação de `Origin`/`Referer`.
- Ownership verificado no service layer — professores só acessam seus próprios decks e turmas.
- Todas as queries SQL parametrizadas — sem concatenação de strings (zero SQL injection).
- Body limitado por `MAX_BODY_SIZE`. Rate limiting por IP em todas as rotas.
- Campos sensíveis (`google_sub`) marcados `json:"-"` — nunca serializados.
- `CreatedBy` (UUID interno) ocultado em respostas ao aluno.

Veja [`SECURITY.md`](SECURITY.md) e [`PRIVACY.md`](PRIVACY.md) para detalhes.
