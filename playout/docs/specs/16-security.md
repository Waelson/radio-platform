# 16 — Segurança Local

## Objetivo

Proteger o Engine contra comandos indevidos, principalmente quando a UI estiver acessível pela LAN.

## Escopo inicial

Por padrão, o Engine deve escutar apenas em:

```text
127.0.0.1
```

Isso reduz risco de acesso remoto não autorizado.

## CORS

Permitir CORS apenas para origens configuradas.

Exemplo:

```yaml
api:
  cors:
    enabled: true
    allowed_origins:
      - "http://localhost:3000"
      - "http://localhost:5173"
```

## Autenticação

MVP local pode rodar sem autenticação se `host=127.0.0.1`.

Se exposto em LAN:

- exigir token estático local;
- ou autenticação da aplicação UI.

Header sugerido:

```http
Authorization: Bearer <token>
```

## Comandos críticos

Comandos críticos:

- `PANIC`
- `STOP`
- `CLEAR_QUEUE`
- `SHUTDOWN`

Devem gerar eventos e logs sempre.

## Validação de paths

O Engine recebe paths. Isso exige cuidado.

Configurar roots permitidos:

```yaml
security:
  allowed_roots:
    - "/library"
    - "C:\\Radio\\Library"
```

O Engine deve:

- normalizar path;
- impedir path traversal;
- validar existência;
- rejeitar path fora dos roots se `allowed_roots` estiver configurado.

## Admin endpoints

Endpoints administrativos devem ser desabilitados por padrão.

Exemplo:

```yaml
admin:
  shutdown_enabled: false
```

## Logs sensíveis

Não logar tokens.

Paths podem ser logados em ambiente local, mas permitir configurar mascaramento.

## Rede

Recomendado:

- Local: `127.0.0.1`.
- LAN: autenticação obrigatória.
- Internet pública: não suportado na primeira versão.

## Princípio

O Engine deve ser fácil de operar localmente, mas não deve aceitar comandos remotos por acidente.
