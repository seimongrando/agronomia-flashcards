# Privacy Policy (LGPD Baseline)

> **Aplicável a:** FlashCards — plataforma de estudo por repetição espaçada  
> **Última atualização:** 2026-02-27  
> **Base legal:** Art. 7º, inciso IX da Lei Geral de Proteção de Dados (LGPD — Lei nº 13.709/2018)  
> — legítimo interesse para prestação do serviço educacional.

---

## 1. Dados Coletados

Coletamos **exclusivamente** o mínimo necessário para autenticação e prestação do serviço:

| Dado | Origem | Finalidade |
|------|--------|-----------|
| `sub` (identificador Google) | Google OAuth | Identificação única e segura da conta |
| `email` | Google OAuth | Controle de acesso e atribuição de roles por administrador |
| `name` | Google OAuth | Exibição do nome na interface |
| `picture` (URL da foto de perfil) | Google OAuth | Exibição do avatar na interface |

**Não coletamos** senhas, dados de pagamento, localização, histórico de navegação
externo ou qualquer outro dado além dos listados acima.

---

## 2. Como os Dados são Usados

- **Autenticação:** o identificador Google (`sub`) vincula a sessão JWT ao registro
  do usuário no banco de dados.
- **Controle de acesso (RBAC):** o e-mail é usado pelo administrador para identificar
  contas ao atribuir roles (`student`, `professor`, `admin`). O e-mail **não é exibido
  publicamente** a outros usuários.
- **Interface:** nome e URL da foto são exibidos apenas para o próprio usuário.
- **Auditoria interna:** operações administrativas (alteração de roles, importações CSV)
  são registradas com o `user_id` (UUID interno), **nunca com e-mail ou nome**.

---

## 3. Armazenamento e Retenção

| Dado | Local | Retenção |
|------|-------|---------|
| Perfil do usuário (`sub`, `email`, `name`, `picture_url`) | PostgreSQL | Enquanto a conta existir |
| Roles do usuário | PostgreSQL | Enquanto a conta existir |
| Histórico de revisões (cards estudados) | PostgreSQL | Enquanto a conta existir |
| Token de sessão (JWT) | Cookie `HttpOnly` no navegador do usuário | 24 horas (expira automaticamente) |
| Logs de acesso HTTP | stdout do servidor | Conforme política de infraestrutura (recomendado: ≤ 30 dias) |

Os logs da aplicação **não contêm PII** — registram apenas `user_id` (UUID), método,
caminho, status HTTP e duração.

---

## 4. Compartilhamento de Dados

Os dados **não são compartilhados** com terceiros, exceto:

- **Google LLC:** para autenticação via OAuth 2.0. Consulte a
  [Política de Privacidade do Google](https://policies.google.com/privacy).
- **Infraestrutura de hospedagem:** o provedor de cloud onde o serviço está hospedado
  processa os dados conforme seu próprio DPA (Data Processing Agreement).

---

## 5. Segurança

As medidas técnicas aplicadas estão detalhadas em `SECURITY.md`. Resumidamente:

- Sessão armazenada em cookie `HttpOnly; SameSite=Lax; Secure` (produção).
- Comunicação via HTTPS em produção.
- Validação de CSRF em todas as requisições mutadoras.
- Acesso ao banco de dados restrito por autenticação e rede privada.

---

## 6. Direitos do Titular (LGPD Art. 18)

Você tem o direito de:

| Direito | Como exercer |
|---------|-------------|
| **Acesso** — saber quais dados temos sobre você | Consulte `/api/me` ou envie solicitação por e-mail |
| **Correção** — corrigir dados incompletos ou inexatos | Atualize seu perfil Google (a sincronização ocorre no próximo login) |
| **Eliminação** — excluir seus dados da plataforma | Envie solicitação por e-mail (ver abaixo) |
| **Portabilidade** | Disponível mediante solicitação |
| **Revogação do consentimento** | Entre em contato para exclusão da conta |

### Solicitação de Exclusão de Dados

Para solicitar a exclusão completa dos seus dados (conta, histórico de revisões
e roles), envie um e-mail para:

**📧 `privacidade@<seu-dominio>.com.br`**

Inclua no e-mail:
- Endereço de e-mail cadastrado.
- Assunto: "Solicitação de Exclusão de Dados — LGPD".

Responderemos e concluiremos a exclusão em até **15 dias corridos**.

---

## 7. Encarregado de Proteção de Dados (DPO)

Conforme Art. 41 da LGPD:

- **Nome:** `<Nome do DPO ou responsável>`
- **E-mail:** `dpo@<seu-dominio>.com.br`

---

## 8. Alterações nesta Política

Esta política pode ser atualizada para refletir mudanças no serviço ou na
legislação. A data de "última atualização" no topo do documento será modificada
em cada revisão. Mudanças significativas serão comunicadas por e-mail ou aviso
na interface.

---

## 9. Contato

Para dúvidas sobre privacidade e proteção de dados:  
**📧 `privacidade@<seu-dominio>.com.br`**
