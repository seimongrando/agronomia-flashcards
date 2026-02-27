# Agronomia – Flashcards

Backend em Go + Postgres com frontend estático embutido. Repetição espaçada para estudantes de Agronomia.

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

## Acesso Admin

Por padrão, qualquer usuário que fizer login recebe o papel `student`. Para ter acesso de professor/admin:

**Via `ADMIN_EMAILS` (automático no primeiro login):**

```env
# .env.local
ADMIN_EMAILS=seuemail@gmail.com
```

Reinicie o servidor (`make run`) e faça login. Seu email é automaticamente promovido a `admin`.

**Via psql (manual após login):**

```bash
make psql
```

```sql
INSERT INTO user_roles (user_id, role)
  SELECT id, 'admin' FROM users WHERE email = 'seuemail@gmail.com';
-- ou role = 'professor' para acesso às telas de conteúdo sem admin total
```

**Páginas restritas:**
- `/teach.html` — gerenciar decks e cards (professor ou admin)
- `/admin_users.html` — gerenciar usuários e roles (admin apenas)

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
├── cmd/server/           # Entrypoint (main.go)
├── internal/
│   ├── config/           # Configuração via env
│   ├── csvparse/         # Parser CSV robusto (UTF-8, BOM, validação)
│   ├── handler/          # HTTP handlers
│   ├── middleware/        # Auth, CORS, CSRF, rate limit, security headers
│   ├── model/            # Modelos de domínio
│   ├── pagination/       # Cursor-based pagination utilities
│   ├── repository/       # Acesso ao banco (SQL)
│   ├── service/          # Lógica de negócio
│   └── validate/         # Validação de input
├── migrations/           # SQL versionado (golang-migrate)
│   ├── 001_init.up.sql
│   ├── 002_upsert_constraints.up.sql
│   └── 003_pagination_indexes.up.sql
├── web/                  # Frontend estático embutido (embed.go)
│   ├── static/css/
│   ├── static/js/
│   └── *.html
├── .env.example          # Referência de todas as variáveis
├── .env.local.example    # Template pronto para desenvolvimento local
├── docker-compose.yml    # Postgres + Adminer (profile tools)
├── Dockerfile            # Build de produção (multi-stage)
├── Makefile              # Automação de tarefas
└── render.yaml           # Deploy automático no Render.com
```

---

## Arquitetura

**Camadas:** `handler → service → repository` (tudo em `internal/`).

**Stack de middleware** (aplicada na ordem):

1. `RequestID` — geração e propagação de request-id
2. `Logger` — log estruturado JSON (slog)
3. `SecurityHeaders` — CSP, X-Frame-Options, X-Content-Type-Options, Referrer-Policy
4. `CORS` — restrito por `ALLOWED_ORIGINS` (negado por padrão)
5. `CSRF` — valida Origin/Referer em métodos mutantes (POST/PUT/DELETE)
6. `MaxBody` — limita o tamanho do body da requisição
7. `RateLimit` — token bucket por IP (`/auth/*` e `/api/*` com limites diferentes)

**Endpoints de health:**
- `GET /healthz` — liveness (sempre 200)
- `GET /readyz` — readiness (pinga o banco)

**Paginação:** cursor-based em todos os endpoints de lista (`/api/decks`, `/api/content/cards`, `/api/admin/users`).

---

## Variáveis de ambiente

Veja [`.env.example`](.env.example) para todas as opções com valores padrão.
Para desenvolvimento local use [`.env.local.example`](.env.local.example) como base.

---

## Deploy (Render)

Conecte o repositório ao [Render](https://render.com). O arquivo `render.yaml` provisiona automaticamente um web service + Postgres gratuito.

Certifique-se de configurar as variáveis `GOOGLE_CLIENT_ID`, `GOOGLE_CLIENT_SECRET`, `JWT_SECRET` e `ADMIN_EMAILS` nos *Environment Variables* do serviço no dashboard do Render.

---

## Segurança

- Cookies `HttpOnly`, `SameSite=Lax`, `Secure` (em produção).
- JWT em cookie — nunca exposto em `localStorage`.
- CSRF protegido via validação de `Origin`/`Referer`.
- Secrets nunca logados.
- Body limitado por `MAX_BODY_SIZE`.
- Rate limiting por IP em rotas sensíveis.

Veja [`SECURITY.md`](SECURITY.md) e [`PRIVACY.md`](PRIVACY.md) para detalhes.
