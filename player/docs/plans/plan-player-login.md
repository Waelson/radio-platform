# Plano de Implementação — Sistema de Login do Player

> **Contexto profissional:** Este plano é elaborado com base em experiência em sistemas de automação de rádio broadcast, incluindo a arquitetura do Zetta (RCS Systems), com atenção especial às particularidades de ambientes de operação ao vivo onde interrupções e fricção de UX têm custo operacional direto.

---

## 1. Explicação do Problema

O Player atualmente não possui nenhum mecanismo de autenticação. Qualquer pessoa com acesso físico ou de rede à máquina pode:

- Interromper ou alterar a programação ao vivo
- Remover itens da fila de reprodução
- Acionar o modo PANIC
- Conectar ou desconectar streams de Icecast/SHOUTcast
- Acessar e modificar configurações de botoneira

Em estações de rádio, o console de automação é o coração da operação. Uma interrupção de 5 segundos ao vivo pode resultar em perda de concessão junto à Anatel, multas contratuais de clientes de publicidade e dano à reputação da emissora. O acesso irrestrito ao Player é, portanto, um risco operacional crítico.

Adicionalmente, sem autenticação não há rastreabilidade de ações: não se sabe qual operador derrubou uma fila, acionou o PANIC ou alterou o volume master.

---

## 2. Solução Proposta

Implementar autenticação baseada em **JWT (JSON Web Token)** com os seguintes pilares:

| Pilar | Decisão |
|-------|---------|
| Gestão de usuários | Library Service (único ponto de verdade) |
| Autenticação | Player faz POST para Library → recebe JWT |
| Armazenamento do token | `electron.safeStorage` (criptografado no SO) |
| Sessão | Token com TTL de 8h (turno de trabalho) + refresh silencioso |
| Troca de senha | Player (tela de login) e Library (painel admin) |
| Reset via Player | Código de 6 dígitos enviado por e-mail, TTL 15 min |
| Reset via Library | Senha default definida pelo operador, troca forçada no login |
| Rate limit de reset | Mínimo 60s entre envios por e-mail |
| Troca de usuário | Operador atual encerra sessão → UI trava → novo operador faz login sem interromper o playout |

---

## 3. Pesquisa de Mercado

### 3.1 Internacional

**Zetta (RCS Systems)**
- Login local com usuário/senha. Não usa SSO por padrão.
- Perfis de acesso granulares: DJ, Operator, Music Director, Admin.
- Sessão fica ativa enquanto o software estiver aberto (sem TTL explícito).
- Reset de senha feito pelo admin via console de gerenciamento separado.
- **Lição:** Simplicidade no login, controle de acesso granular é mais importante que sofisticação de auth.

**Dalet Galaxy**
- Integração com Active Directory / LDAP para ambientes corporativos.
- Perfis por módulo (rádio, TV, digital).
- SSO entre aplicações do suite.
- **Lição:** Em redes com AD, integração LDAP é valorizada. Para PMEs, auth própria é suficiente.

**WideOrbit Automation for Radio**
- Login web com JWT, refresh automático.
- Roles: Admin, Operator, Read-Only.
- Histórico de ações auditável por usuário.
- **Lição:** Auditoria de ações por usuário é feature esperada em sistemas profissionais.

**AzuraCast (open source)**
- Autenticação própria com bcrypt + JWT.
- Suporte a 2FA via TOTP (Google Authenticator).
- Reset de senha por e-mail com link tokenizado.
- **Lição:** E-mail de reset com código/link é o padrão esperado pelo mercado.

**SAM Broadcaster (Spacial)**
- Usuário/senha local, sem gestão sofisticada.
- Usado em pequenas rádios, modelo mais simples.
- **Lição:** Para o perfil da RadioFlow, superar o SAM é o benchmark mínimo.

**Selector (GSelector / RCS)**
- Autenticação integrada ao Zetta em redes maiores.
- Stand-alone usa login simples sem expiração.

### 3.2 Nacional

**Megasat / Radiosity**
- Login simples, senha armazenada em banco local sem criptografia em versões antigas.
- Sem reset por e-mail — reset manual pelo técnico.
- **Lição:** Oportunidade de diferenciação com reset por e-mail e bcrypt.

**SoundRadix / sistemas web brasileiros**
- Tendem a usar auth baseada em sessões PHP/Node.
- Sem padronização de UX para login.

### 3.3 Síntese

| Sistema | Auth | Reset | Roles | Auditoria |
|---------|------|-------|-------|-----------|
| Zetta | Local | Admin | Sim | Parcial |
| Dalet | LDAP/Local | Admin | Sim | Sim |
| WideOrbit | JWT | E-mail | Sim | Sim |
| AzuraCast | JWT+2FA | E-mail | Sim | Sim |
| SAM Broadcaster | Local | Manual | Não | Não |
| RadioFlow (proposta) | JWT | E-mail + Admin | Futuro | Futuro |

---

## 4. Esboço das Telas (ASCII)

### Tela 1 — Login Principal

```
┌─────────────────────────────────────────────────────────────────┐
│                                                                 │
│                    ░░ RadioFlow ░░                              │
│                  ──── Player ────                               │
│                                                                 │
│              ┌─────────────────────────────┐                   │
│              │  E-mail                     │                   │
│              │  ┌───────────────────────┐  │                   │
│              │  │ operador@radio.com.br │  │                   │
│              │  └───────────────────────┘  │                   │
│              │                             │                   │
│              │  Senha                      │                   │
│              │  ┌───────────────────────┐  │                   │
│              │  │ ••••••••••••          │  │                   │
│              │  └───────────────────────┘  │                   │
│              │                             │                   │
│              │  [ Entrar ]                 │                   │
│              │                             │                   │
│              │  Esqueci minha senha →      │                   │
│              └─────────────────────────────┘                   │
│                                                                 │
│              ● Engine conectado   ● Library conectado           │
└─────────────────────────────────────────────────────────────────┘
```

### Tela 2 — Esqueci a Senha (solicitar código)

```
┌─────────────────────────────────────────────────────────────────┐
│  ← Voltar                                                       │
│                                                                 │
│                  Recuperar Senha                                │
│                                                                 │
│   Informe seu e-mail cadastrado. Enviaremos um código           │
│   de verificação válido por 15 minutos.                         │
│                                                                 │
│              ┌─────────────────────────────┐                   │
│              │  ┌───────────────────────┐  │                   │
│              │  │ operador@radio.com.br │  │                   │
│              │  └───────────────────────┘  │                   │
│              │                             │                   │
│              │  [ Enviar código ]          │                   │
│              │                             │                   │
│              │  ⏱ Aguarde 45s para reenviar│   ← rate limit   │
│              └─────────────────────────────┘                   │
└─────────────────────────────────────────────────────────────────┘
```

### Tela 3 — Inserir Código de Verificação

```
┌─────────────────────────────────────────────────────────────────┐
│  ← Voltar                                                       │
│                                                                 │
│               Código de Verificação                             │
│                                                                 │
│   Enviamos um código de 6 dígitos para                          │
│   ope****@radio.com.br                                          │
│                                                                 │
│              ┌─────────────────────────────┐                   │
│              │   [ 3 ][ 8 ][ 4 ][ 1 ][ 9 ][ 2 ]               │
│              │                             │                   │
│              │  [ Verificar ]              │                   │
│              │                             │                   │
│              │  Não recebeu? Reenviar →    │                   │
│              └─────────────────────────────┘                   │
│                                                                 │
│   ⏱ Código expira em 12:43                                     │
└─────────────────────────────────────────────────────────────────┘
```

### Tela 4 — Nova Senha

```
┌─────────────────────────────────────────────────────────────────┐
│                                                                 │
│                  Definir Nova Senha                             │
│                                                                 │
│              ┌─────────────────────────────┐                   │
│              │  Nova senha                 │                   │
│              │  ┌───────────────────────┐  │                   │
│              │  │ ••••••••••••          │  │                   │
│              │  └───────────────────────┘  │                   │
│              │                             │                   │
│              │  Confirmar nova senha       │                   │
│              │  ┌───────────────────────┐  │                   │
│              │  │ ••••••••••••          │  │                   │
│              │  └───────────────────────┘  │                   │
│              │                             │                   │
│              │  ✓ Mínimo 8 caracteres      │                   │
│              │  ✓ Uma letra maiúscula       │                   │
│              │  ✗ Um número                │                   │
│              │                             │                   │
│              │  [ Salvar nova senha ]      │                   │
│              └─────────────────────────────┘                   │
└─────────────────────────────────────────────────────────────────┘
```

### Tela 6 — Troca de Usuário (Handover de Turno)

```
┌─────────────────────────────────────────────────────────────────┐
│                                                                 │
│   🔒  Sessão encerrada                                          │
│       João Silva encerrou o turno.                              │
│       Faça login para continuar.                                │
│                                                                 │
│   ┄┄┄ Playout em execução ┄┄┄ (fundo bloqueado/desfocado)      │
│                                                                 │
│              ┌─────────────────────────────┐                   │
│              │  E-mail                     │                   │
│              │  ┌───────────────────────┐  │                   │
│              │  │ novo@radio.com.br     │  │                   │
│              │  └───────────────────────┘  │                   │
│              │                             │                   │
│              │  Senha                      │                   │
│              │  ┌───────────────────────┐  │                   │
│              │  │ ••••••••••••          │  │                   │
│              │  └───────────────────────┘  │                   │
│              │                             │                   │
│              │  [ Entrar ]                 │                   │
│              │                             │                   │
│              │  Esqueci minha senha →      │                   │
│              └─────────────────────────────┘                   │
│                                                                 │
│   ● Playout ativo — programação não foi interrompida            │
└─────────────────────────────────────────────────────────────────┘
```

### Tela 5 — Troca de Senha Forçada (pós-reset pelo admin)

```
┌─────────────────────────────────────────────────────────────────┐
│                                                                 │
│   ⚠ Você está usando uma senha temporária.                     │
│      Defina uma nova senha para continuar.                      │
│                                                                 │
│              ┌─────────────────────────────┐                   │
│              │  Senha temporária           │                   │
│              │  ┌───────────────────────┐  │                   │
│              │  │ ••••••••••••          │  │                   │
│              │  └───────────────────────┘  │                   │
│              │                             │                   │
│              │  Nova senha                 │                   │
│              │  ┌───────────────────────┐  │                   │
│              │  │ ••••••••••••          │  │                   │
│              │  └───────────────────────┘  │                   │
│              │                             │                   │
│              │  Confirmar nova senha       │                   │
│              │  ┌───────────────────────┐  │                   │
│              │  │ ••••••••••••          │  │                   │
│              │  └───────────────────────┘  │                   │
│              │                             │                   │
│              │  [ Salvar e entrar ]        │                   │
│              └─────────────────────────────┘                   │
└─────────────────────────────────────────────────────────────────┘
```

---

## 5. Esboço da Arquitetura (ASCII)

```
┌─────────────────────────────────────────────────────────────────────┐
│                        PLAYER (Electron)                            │
│                                                                     │
│  ┌─────────────────────┐      ┌──────────────────────────────────┐ │
│  │   LoginOverlay.html  │      │         player.html              │ │
│  │  (tela de login)    │      │   (interface principal)          │ │
│  │                     │      │                                  │ │
│  │  - login form       │      │  Só renderiza após auth válida   │ │
│  │  - forgot password  │      │                                  │ │
│  │  - reset code form  │      │                                  │ │
│  │  - new password form│      │                                  │ │
│  └────────┬────────────┘      └──────────────┬───────────────────┘ │
│           │                                  │                      │
│           ▼                                  ▼                      │
│  ┌────────────────────────────────────────────────────────────────┐ │
│  │                   session-manager.js                           │ │
│  │  - Armazena JWT em safeStorage                                 │ │
│  │  - Refresh silencioso a cada 7h                                │ │
│  │  - Expõe currentUser() e isAuthenticated()                     │ │
│  │  - Rate limit de reset: Map<email, timestamp>                  │ │
│  └──────────────────────────────┬─────────────────────────────────┘ │
└─────────────────────────────────┼───────────────────────────────────┘
                                  │ HTTP
                    ┌─────────────▼──────────────┐
                    │     Library Service          │
                    │                             │
                    │  POST /v1/auth/login        │
                    │  POST /v1/auth/refresh      │
                    │  POST /v1/auth/logout       │
                    │  POST /v1/auth/reset-request│
                    │  POST /v1/auth/reset-verify │
                    │  POST /v1/auth/reset-confirm│
                    │  POST /v1/auth/change-pwd   │
                    │                             │
                    │  ┌─────────────────────┐   │
                    │  │    user_store.go     │   │
                    │  │  - bcrypt hash       │   │
                    │  │  - reset_code table  │   │
                    │  │  - force_change flag │   │
                    │  └─────────────────────┘   │
                    │                             │
                    │  ┌─────────────────────┐   │
                    │  │    mailer.go         │   │
                    │  │  SMTP → e-mail       │   │
                    │  └─────────────────────┘   │
                    └─────────────────────────────┘
```

---

## 6. Decisões Arquiteturais

### DA-01 — JWT stateless (sem sessão no servidor)
**Decisão:** O Library não mantém tabela de sessões ativas. O JWT é auto-contido com claims de usuário, role e expiração.
**Justificativa:** O Player é uma aplicação Electron local. A validação stateful exigiria round-trip ao Library em cada request protegido, adicionando latência ao hot path do playout. Com JWT, o Player valida o token localmente (verificação de assinatura + expiração) sem chamar o Library.
**Trade-off:** Logout imediato não invalida o token no servidor. Mitigado com TTL curto (8h) e refresh token de revogação futura.

### DA-02 — safeStorage do Electron para armazenamento do JWT
**Decisão:** Usar `electron.safeStorage.encryptString()` para gravar o JWT em disco.
**Justificativa:** localStorage do Electron não é criptografado. safeStorage usa a keychain do SO (Keychain no macOS, DPAPI no Windows, libsecret no Linux), vinculando o token ao usuário do SO.
**Trade-off:** Não portável entre máquinas — intencional, já que o Player é instalado por estação.

### DA-03 — Cadastro apenas no Library
**Decisão:** Nenhuma tela de cadastro no Player. Operadores são provisionados pelo administrador no Library.
**Justificativa:** Consistente com o modelo broadcast, onde o accesso ao console é controlado pelo director de operações, não auto-provisionado. Replica o modelo do Zetta e Dalet.

### DA-04 — Reset duplo (admin + e-mail)
**Decisão:** Dois fluxos de reset independentes, sem acoplamento entre si.
**Justificativa:** Emissoras pequenas podem não ter SMTP configurado — o reset via admin (senha default) garante recuperação sem dependência externa. E-mail é o fluxo preferencial para operadores que esquecem a senha sem precisar do admin.

### DA-05 — Rate limit em memória no processo principal do Electron
**Decisão:** O controle de "mínimo 60s entre envios" é gerido por um `Map<email, lastSentAt>` no `session-manager.js` no processo principal.
**Justificativa:** Evita round-trip ao Library para verificar rate limit. O Library também valida no lado do servidor como segunda camada de defesa.

### DA-06 — Login como overlay fullscreen, não nova janela
**Decisão:** O login é um `<div>` overlay sobre o `player.html`, não uma janela Electron separada.
**Justificativa:** Evita flash de janela e complexidade de IPC entre janelas. O player.html existe no DOM mas fica `pointer-events: none; opacity: 0` até a autenticação ser concluída — não renderiza dados da fila antes do login.

### DA-07 — Troca de usuário não interrompe o playout
**Decisão:** Durante a troca de usuário (handover de turno), o overlay de login é exibido sobre o player com `backdrop-filter: blur`, mas o WebSocket permanece conectado, o playout engine continua tocando e nenhum comando de parada é enviado.
**Justificativa:** Em rádio ao vivo, a transição de turno não pode causar silêncio. O modelo é análogo ao "workstation lock" do Zetta: a tela trava, mas o áudio continua. O Engine é um processo separado (`playout/`) e independente do estado de autenticação do Player.
**Trade-off:** Durante a janela de troca (overlay aberto), nenhum operador pode intervir manualmente na fila. Mitigado pelo fato de que a troca deve ser feita em momento de baixo risco operacional.

---

## 7. Novos Componentes

### Player (Electron)

| Componente | Arquivo | Responsabilidade |
|-----------|---------|-----------------|
| Login Overlay | `player/login-overlay.js` | Renderiza as telas de login no DOM do player.html (T1–T6) |
| Session Manager | `player/session-manager.js` | JWT storage, refresh, rate limit de reset, troca de usuário |
| User Menu | `player/player.html` (update) | Avatar/nome do usuário logado com opção "Trocar usuário" |
| Auth Preload Bridge | `player/preload.js` (update) | Expõe `authAPI` via contextBridge |
| Auth IPC Handlers | `player/main.js` (update) | IPC handlers: `auth:login`, `auth:logout`, `auth:switch-user`, `auth:reset-request`, etc. |

### Library Service (Go)

| Componente | Arquivo | Responsabilidade |
|-----------|---------|-----------------|
| User Store | `library/internal/store/user_store.go` | CRUD de usuários, bcrypt, force_change flag |
| Auth Handlers | `library/internal/api/handlers/auth.go` | Endpoints de login, reset, change-password |
| Mailer | `library/internal/mailer/mailer.go` | Envio de e-mail via SMTP |
| JWT Middleware | `library/internal/api/middleware/jwt.go` | Validação de JWT nas rotas protegidas |
| Reset Code Store | (dentro de user_store.go) | Tabela `password_reset_codes` com TTL |

### Banco de Dados (SQLite — Library)

```sql
-- Nova tabela: users
CREATE TABLE users (
    id          TEXT PRIMARY KEY,
    email       TEXT UNIQUE NOT NULL,
    name        TEXT NOT NULL,
    password_hash TEXT NOT NULL,       -- bcrypt
    role        TEXT NOT NULL DEFAULT 'operator',
    force_change_pwd INTEGER NOT NULL DEFAULT 0,
    created_at  TEXT NOT NULL,
    updated_at  TEXT NOT NULL
);

-- Nova tabela: password_reset_codes
CREATE TABLE password_reset_codes (
    id          TEXT PRIMARY KEY,
    user_id     TEXT NOT NULL REFERENCES users(id),
    code        TEXT NOT NULL,          -- 6 dígitos, bcrypt hash
    expires_at  TEXT NOT NULL,
    used        INTEGER NOT NULL DEFAULT 0,
    created_at  TEXT NOT NULL
);
```

---

## 8. Fluxo de Telas

```
                    ┌─────────────┐
                    │  Inicializa  │
                    │   Player    │
                    └──────┬──────┘
                           │
                    ¿ JWT válido?
                    ┌──────┴──────┐
                   Não           Sim
                    │             │
                    ▼             ▼
            ┌──────────┐   ┌────────────────────────────┐
            │  [T1]    │   │  Player Principal          │
            │  LOGIN   │   │                            │
            └────┬─────┘   │  [👤 João Silva ▾]         │
                 │         │   └─ Trocar usuário ──────┐│
      ┌──────────┤         └──────────────┬────────────┘│
      │          │                        │             │
   Sucesso   Esqueci              Clica "Trocar"        │
      │       senha                       │             │
      │          │                        ▼             │
      ▼          ▼               [T6] HANDOVER          │
 ¿ force_    [T2] Inserir        (overlay blur,         │
  change?     e-mail              playout continua)     │
      │          │                        │             │
     Sim         ▼               Novo login bem-sucedido│
      │       [T3] Inserir                │             │
      ▼         código           Player Principal ◄─────┘
  [T5]           │               (novo usuário ativo)
  Troca          ▼
  Forçada    [T4] Nova
      │         senha
      │          │
      ▼          ▼
  Player     LOGIN [T1]
  Principal  (auto-login
              após reset)
```

---

## 9. Fluxo entre Componentes

### Login bem-sucedido

```
Player UI (T1)
    │ submit email+senha
    ▼
session-manager.js
    │ ipcRenderer.invoke('auth:login', {email, password})
    ▼
main.js (IPC handler)
    │ POST /v1/auth/login → Library
    ▼
Library: auth.go → user_store.go
    │ bcrypt.CompareHashAndPassword
    │ jwt.Sign({sub, email, name, role, exp})
    ▼
main.js
    │ safeStorage.encryptString(token) → grava em disco
    ▼
session-manager.js
    │ resolve { ok: true, user: {...}, forceChangePwd }
    ▼
login-overlay.js
    │ se forceChangePwd → mostra T5
    │ senão → esconde overlay, player.html inicia fetchQueue()
```

### Reset por e-mail (Player)

```
Player UI (T2)
    │ submit email
    ▼
session-manager.js
    │ verifica Map<email, lastSentAt>
    │ se (now - lastSentAt) < 60s → erro "aguarde Xs"
    │ senão → ipcRenderer.invoke('auth:reset-request', {email})
    ▼
main.js → POST /v1/auth/reset-request → Library
    ▼
Library: auth.go
    │ verifica rate limit no banco (segunda camada)
    │ gera código 6 dígitos
    │ bcrypt.Hash(code) → grava em password_reset_codes (TTL 15min)
    │ mailer.Send(email, code)
    ▼
Player UI (T3)
    │ submit código
    ▼
main.js → POST /v1/auth/reset-verify → Library
    ▼
Library: auth.go
    │ valida código + TTL + not used
    │ retorna reset_token temporário (JWT com scope=reset, TTL 10min)
    ▼
Player UI (T4)
    │ submit nova senha + reset_token
    ▼
main.js → POST /v1/auth/reset-confirm → Library
    ▼
Library: auth.go
    │ valida reset_token
    │ bcrypt.Hash(nova senha) → atualiza users.password_hash
    │ marca código como used
    ▼
Player → auto-login com nova senha
```

### Troca de Usuário (Handover de Turno)

```
Player Principal
    │ Operador A clica em "Trocar usuário" no menu de perfil
    ▼
session-manager.js
    │ clearSession() — apaga JWT do safeStorage
    │ NÃO envia logout ao servidor (playout não é afetado)
    │ NÃO desconecta WebSocket (playout continua)
    ▼
login-overlay.js
    │ exibe [T6] com nome do operador anterior
    │ player.html fica com blur + pointer-events: none
    │ (fila e controles inacessíveis)
    ▼
Operador B insere e-mail + senha em [T6]
    ▼
session-manager.js → ipcRenderer.invoke('auth:login', {...})
    ▼
main.js → POST /v1/auth/login → Library
    ▼
Library: retorna JWT do Operador B
    ▼
session-manager.js
    │ safeStorage.encryptString(tokenB) → salva em disco
    ▼
login-overlay.js
    │ esconde overlay
    │ player.html atualiza nome/avatar para Operador B
    │ player.html restaura pointer-events + sem blur
    │ playout nunca foi interrompido
```

### Reset via Library (admin)

```
Admin no Library UI
    │ POST /v1/users/{id}/reset-password
    ▼
Library: user_store.go
    │ define senha = config.DefaultResetPassword
    │ bcrypt.Hash(defaultPwd)
    │ users.force_change_pwd = 1
    ▼
Operador faz login no Player com senha default
    ▼
Library responde JWT com claim force_change_pwd = true
    ▼
Player mostra [T5] Troca Forçada
    │ valida senha temporária + nova senha
    ▼
Library: auth.go → atualiza hash, force_change_pwd = 0
    ▼
Player inicia normalmente
```

---

## 10. Riscos e Mitigações

| # | Risco | Probabilidade | Impacto | Mitigação |
|---|-------|--------------|---------|-----------|
| R01 | Operador esquece senha em horário de pico sem acesso ao e-mail | Alta | Crítico | Admin pode resetar via Library com senha default sem e-mail |
| R02 | SMTP não configurado → reset por e-mail falha silenciosamente | Média | Alto | UI exibe mensagem clara; fluxo admin sempre disponível. Validar config SMTP na inicialização do Library |
| R03 | JWT roubado do disco (acesso físico à máquina) | Baixa | Alto | safeStorage vincula ao usuário do SO; TTL curto de 8h; logout ao fechar o Player |
| R04 | Brute force de código de reset | Média | Alto | Código bcrypt-hasheado; máximo 5 tentativas por código; TTL 15min; after 5 tentativas invalida o código |
| R05 | Player fica sem acesso ao Library (rede down) durante turno | Média | Médio | JWT válido por 8h funciona offline para operação contínua. Refresh só quando necessário |
| R06 | Usuário admin excluído acidentalmente sem outros admins | Baixa | Crítico | Library garante sempre ao menos 1 usuário admin (validação no DELETE) |
| R07 | Rate limit contornado reiniciando o Player | Baixa | Baixo | Library valida rate limit também no servidor (dupla camada) |
| R08 | Token expirado durante operação ao vivo | Baixa | Alto | Refresh silencioso 1h antes da expiração; banner discreto de "sessão expirando" |
| R09 | Operador esquece de fazer handover e sai — outro operador fica sem acesso | Média | Alto | Menu "Trocar usuário" proeminente; futura feature de timeout de inatividade que dispara T6 automaticamente |
| R10 | Playout para durante janela de handover por falta de itens na fila | Média | Crítico | Não é responsabilidade do sistema de auth — o playout engine tem modo PANIC e auto-silence. Documentar procedimento operacional de handover em horário de baixo risco |
| R11 | Operador B faz login mas herda estado visual desatualizado do Operador A | Baixa | Baixo | Após handover, player.html executa `fetchQueue()` e `fetchStatus()` para forçar atualização do estado |

---

## 11. Regras de Negócio

### RN-01 — Cadastro
- Cadastro somente no Library (painel admin).
- Campos obrigatórios: nome, e-mail, senha inicial, role.
- E-mail único no sistema.
- Senha inicial definida pelo admin; `force_change_pwd = true` até troca.

### RN-02 — Login
- Autenticação por e-mail + senha.
- Senha verificada via bcrypt.
- JWT gerado com TTL de 8h, assinado com HMAC-SHA256.
- Claims obrigatórios: `sub` (user_id), `email`, `name`, `role`, `exp`, `force_change_pwd`.
- Se `force_change_pwd = true`, Player redireciona para T5 antes de liberar acesso.

### RN-03 — Troca de Senha (Player)
- Disponível apenas após login autenticado OU com reset_token válido.
- Nova senha: mínimo 8 caracteres, ao menos 1 maiúscula, ao menos 1 número.
- Nova senha não pode ser igual à atual.
- Confirmação de nova senha deve coincidir.

### RN-04 — Reset por E-mail (Player)
- Formulário disponível sem autenticação prévia.
- Rate limit: mínimo 60 segundos entre solicitações para o mesmo e-mail.
- Código: 6 dígitos numéricos, gerado com `crypto/rand`.
- Código armazenado como bcrypt hash no banco.
- TTL do código: 15 minutos.
- Máximo 5 tentativas de verificação por código; após isso, código invalidado.
- Após reset bem-sucedido, código marcado como `used = 1`.
- Após confirmar nova senha, Player executa login automático com as novas credenciais.
- Resposta da API de envio é sempre `200 OK` independente de o e-mail existir (evita user enumeration).

### RN-05 — Reset via Library (Admin)
- Disponível para usuários com role `admin`.
- Define `password_hash` como bcrypt da senha default do sistema.
- Define `force_change_pwd = 1`.
- Senha default configurada em `library/config.yaml` (`auth.default_reset_password`).
- Não envia e-mail.

### RN-06 — Sessão e Refresh
- JWT válido por 8 horas.
- Player tenta refresh silencioso quando restar menos de 1 hora para expirar.
- Ao fechar o Player, sessão persiste (token no disco) — operador pode reabrir sem relogar dentro do TTL.
- Logout explícito apaga o token do disco (não invalida no servidor — trade-off DA-01).

### RN-07 — Roles (fase inicial)
- `admin`: acesso total ao Library.
- `operator`: acesso ao Player. Sem restrições internas por enquanto.
- Expansão de roles por tela/funcionalidade é escopo de fase futura.

### RN-09 — Troca de Usuário (Handover de Turno)
- Disponível apenas quando há um usuário autenticado (botão "Trocar usuário" no menu de perfil do player).
- Ao acionar, o JWT atual é removido do `safeStorage` imediatamente.
- O WebSocket com o playout engine **não é desconectado** — a programação continua ao vivo.
- O overlay [T6] é exibido com o nome do operador que encerrou o turno e um formulário de login.
- Os controles do player ficam completamente inacessíveis durante o handover (blur + `pointer-events: none`).
- O novo operador deve fazer login com suas próprias credenciais.
- Após login bem-sucedido, o player atualiza nome/avatar e executa `fetchQueue()` + `fetchStatus()`.
- Não é possível cancelar a troca de usuário após iniciada — o operador anterior precisaria de suas credenciais para retomar.
- Se o novo operador também clicar em "Trocar usuário", o mesmo fluxo se repete.

### RN-08 — Proteção de rotas no Library
- Todos os endpoints do Library (exceto `/v1/health`, `/v1/auth/login`, `/v1/auth/reset-*`) exigem JWT válido no header `Authorization: Bearer <token>`.

---

## 12. Testes Unitários

### Library — user_store_test.go

```go
TestCreateUser_Success
TestCreateUser_DuplicateEmail_Error
TestAuthenticateUser_CorrectPassword_ReturnsUser
TestAuthenticateUser_WrongPassword_Error
TestAuthenticateUser_UserNotFound_Error
TestForceChangePwd_SetOnCreate_WhenAdminSetsDefault
TestChangePassword_Success
TestChangePassword_SameAsCurrentPassword_Error
TestChangePassword_WeakPassword_Error
```

### Library — reset_code_test.go

```go
TestCreateResetCode_GeneratesSixDigits
TestCreateResetCode_StoresBcryptHash
TestVerifyResetCode_ValidCode_Success
TestVerifyResetCode_ExpiredCode_Error
TestVerifyResetCode_UsedCode_Error
TestVerifyResetCode_WrongCode_Error
TestVerifyResetCode_MaxAttempts_InvalidatesCode
TestRateLimit_SecondRequestWithin60s_Rejected
TestRateLimit_SecondRequestAfter60s_Allowed
```

### Library — auth_handler_test.go

```go
TestLoginEndpoint_ValidCredentials_ReturnsJWT
TestLoginEndpoint_InvalidCredentials_Returns401
TestLoginEndpoint_ForceChangePwd_ClaimPresent
TestResetRequest_UnknownEmail_Returns200 (evita user enumeration)
TestResetRequest_KnownEmail_SendsEmail
TestResetRequest_RateLimitExceeded_Returns429
TestResetVerify_ValidCode_ReturnsResetToken
TestResetVerify_InvalidCode_Returns400
TestResetConfirm_ValidResetToken_UpdatesPassword
TestResetConfirm_ExpiredResetToken_Returns401
TestChangePassword_Authenticated_Success
TestChangePassword_WrongCurrentPassword_Error
```

### Player — session-manager.test.js

```js
test_login_storesJWTInSafeStorage
test_login_forceChangePwd_returnsFlag
test_isAuthenticated_validToken_true
test_isAuthenticated_expiredToken_false
test_logout_clearsStorage
test_resetRequest_withinRateLimit_throws
test_resetRequest_afterRateLimit_callsIPC
test_refresh_calledWhenExpiresInLessThan1h
test_switchUser_clearsSessionWithoutDisconnectingWebSocket
test_switchUser_overlayShownWithPreviousUserName
test_switchUser_newLoginRestoresFullAccess
test_switchUser_fetchQueueCalledAfterNewLogin
```

---

## 13. Detalhamento das Fases

### Fase 0 — Preparação (Pré-requisito)
- [ ] Criar feature branch: `git checkout main && git checkout -b feat/player-login`
- [ ] Criar diretório `player/auth/`
- [ ] Verificar se SQLite migration system está funcional no Library

---

### Fase 1 — Backend: Usuários e Auth no Library

**Objetivo:** Endpoints de login, change-password e estrutura de usuários.

Entregas:
- [ ] Migration: tabela `users`
- [ ] `user_store.go`: CreateUser, Authenticate, ChangePassword, SetForceChange
- [ ] `auth.go` (handler): POST `/v1/auth/login`, POST `/v1/auth/change-password`
- [ ] JWT middleware para rotas protegidas
- [ ] Seed: usuário admin default na primeira execução
- [ ] Testes: `TestCreateUser_*`, `TestAuthenticateUser_*`, `TestLoginEndpoint_*`

---

### Fase 2 — Backend: Reset de Senha

**Objetivo:** Fluxo completo de reset por e-mail e reset via admin.

Entregas:
- [ ] Migration: tabela `password_reset_codes`
- [ ] `mailer.go`: cliente SMTP configurável
- [ ] `user_store.go`: CreateResetCode, VerifyResetCode, MarkCodeUsed
- [ ] `auth.go` (handler): POST `/v1/auth/reset-request`, POST `/v1/auth/reset-verify`, POST `/v1/auth/reset-confirm`
- [ ] `users.go` (handler): POST `/v1/users/{id}/reset-password` (admin)
- [ ] Rate limit no servidor (dupla camada)
- [ ] Testes: todos os `TestReset*`, `TestRateLimit*`

---

### Fase 3 — Protótipos HTML

**Objetivo:** Validar UX antes de integrar no Player.

Entregas:
- [ ] `player/auth/prototype-login.html` — todas as telas navegáveis com identidade visual do player.html
- [ ] Aprovação do fluxo visual antes de prosseguir

---

### Fase 4 — Frontend: Login Overlay no Player

**Objetivo:** Integrar autenticação no Player Electron.

Entregas:
- [ ] `player/auth/login-overlay.js`: gerencia estado das telas (T1–T6)
- [ ] `player/auth/session-manager.js`: JWT storage, refresh, rate limit, troca de usuário
- [ ] `player/preload.js`: expor `authAPI` via contextBridge
- [ ] `player/main.js`: IPC handlers `auth:login`, `auth:logout`, `auth:switch-user`, `auth:reset-*`
- [ ] `player/player.html`: incluir overlay, bloquear UI até auth, menu de perfil com "Trocar usuário"
- [ ] Refresh silencioso 1h antes da expiração
- [ ] Handover: blur + pointer-events none durante T6, fetchQueue após novo login
- [ ] Testes: `session-manager.test.js`

---

### Fase 5 — Integração e Testes End-to-End

**Objetivo:** Garantir fluxo completo funcionando.

Entregas:
- [ ] Teste manual de todos os fluxos (login, reset e-mail, reset admin, troca forçada, refresh, logout, troca de usuário)
- [ ] Verificar comportamento offline (Library indisponível, JWT ainda válido)
- [ ] Verificar comportamento de token expirado
- [ ] Review de segurança: injeção, brute force, user enumeration

---

### Fase 6 — PR e Merge

- [ ] `go test ./...` no Library
- [ ] `go vet ./...` no Library
- [ ] Atualizar README com instruções de configuração SMTP
- [ ] Abrir PR: `feat/player-login` → `main`
- [ ] Merge após aprovação

---

## 14. Protótipos HTML

Os protótipos serão criados em:

```
player/auth/prototype-login.html
```

Seguirão a identidade visual do `player.html`:
- Paleta de cores: fundo `#0a1520`, acentos `#20e6ff` (ciano), texto `#c8d8e0`
- Tipografia: sistema sem-serif, pesos 600–800
- Bordas arredondadas com `border-color: rgba(32,230,255,0.25)`
- Animações suaves (transition 0.14s)
- Glassmorphism nos cards de input

Os protótipos serão funcionais (navegação entre telas via JS puro, sem backend) para validação de UX antes da implementação.

> A criação dos protótipos ocorre na **Fase 3** e é pré-requisito para a Fase 4.
