#!/usr/bin/env bash
# start-icecast-test.sh — sobe um servidor Icecast local para testes de streaming
# Uso: ./start-icecast-test.sh [start|stop|logs|status]

set -euo pipefail

CONTAINER="icecast-radioflow-test"
IMAGE="moul/icecast"
PORT=8000
SOURCE_PASSWORD="hackme"
ADMIN_PASSWORD="adminpass"

print_info() {
  echo ""
  echo "  ┌─────────────────────────────────────────────────────┐"
  echo "  │  Icecast rodando em http://127.0.0.1:${PORT}           │"
  echo "  │                                                     │"
  echo "  │  Configure no RadioFlow:                            │"
  echo "  │    Tipo    : Icecast 2                              │"
  echo "  │    Host    : 127.0.0.1                              │"
  echo "  │    Porta   : ${PORT}                                   │"
  echo "  │    Mount   : /stream                                │"
  echo "  │    Senha   : ${SOURCE_PASSWORD}                              │"
  echo "  │    Formato : MP3 / 128 kbps                        │"
  echo "  │                                                     │"
  echo "  │  Ouvir no VLC : http://127.0.0.1:${PORT}/stream       │"
  echo "  │  Admin        : http://127.0.0.1:${PORT}/admin/        │"
  echo "  │  (usuário: admin  |  senha: ${ADMIN_PASSWORD})       │"
  echo "  └─────────────────────────────────────────────────────┘"
  echo ""
}

cmd="${1:-start}"

case "$cmd" in

  start)
    if ! command -v docker &>/dev/null; then
      echo "❌  Docker não encontrado. Instale em https://www.docker.com e tente novamente."
      exit 1
    fi

    if docker ps --format '{{.Names}}' | grep -q "^${CONTAINER}$"; then
      echo "✓  Icecast já está rodando (container: ${CONTAINER})"
      print_info
      exit 0
    fi

    # Remove container parado anterior, se houver
    docker rm -f "${CONTAINER}" &>/dev/null || true

    echo "→  Subindo Icecast..."
    docker run -d \
      --name "${CONTAINER}" \
      -p "${PORT}:8000" \
      -e ICECAST_SOURCE_PASSWORD="${SOURCE_PASSWORD}" \
      -e ICECAST_ADMIN_PASSWORD="${ADMIN_PASSWORD}" \
      -e ICECAST_PASSWORD="${SOURCE_PASSWORD}" \
      -e ICECAST_RELAY_PASSWORD="${SOURCE_PASSWORD}" \
      "${IMAGE}" >/dev/null

    # Aguarda o servidor aceitar conexões (máx 10s)
    echo -n "   Aguardando servidor ficar disponível"
    for i in $(seq 1 10); do
      if curl -s -o /dev/null "http://127.0.0.1:${PORT}/"; then
        echo " ✓"
        break
      fi
      echo -n "."
      sleep 1
      if [ "$i" -eq 10 ]; then
        echo ""
        echo "⚠️   Servidor demorou mais que o esperado. Verifique com: $0 logs"
      fi
    done

    print_info
    ;;

  stop)
    if docker ps --format '{{.Names}}' | grep -q "^${CONTAINER}$"; then
      docker rm -f "${CONTAINER}" >/dev/null
      echo "✓  Icecast parado."
    else
      echo "  Icecast não está rodando."
    fi
    ;;

  logs)
    docker logs -f "${CONTAINER}"
    ;;

  status)
    if docker ps --format '{{.Names}}' | grep -q "^${CONTAINER}$"; then
      echo "✓  Icecast está rodando (container: ${CONTAINER})"
      print_info
    else
      echo "○  Icecast não está rodando. Use: $0 start"
    fi
    ;;

  *)
    echo "Uso: $0 [start|stop|logs|status]"
    echo ""
    echo "  start   Sobe o Icecast (padrão)"
    echo "  stop    Para e remove o container"
    echo "  logs    Exibe os logs em tempo real"
    echo "  status  Verifica se está rodando"
    exit 1
    ;;
esac
