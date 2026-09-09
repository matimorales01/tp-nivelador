#!/bin/bash
# Genera docker-compose.yaml con N clientes.
# Uso: ./scripts/generate_compose.sh <cantidad_de_clientes>

set -euo pipefail

if [ $# -ne 1 ] || ! [[ "$1" =~ ^[0-9]+$ ]] || [ "$1" -lt 1 ]; then
    echo "Uso: $0 <cantidad_de_clientes>" >&2
    exit 1
fi

N=$1
OUT="docker-compose.yaml"

cat > "$OUT" <<EOF
services:
  server:
    build:
      context: ./services/server
      dockerfile: Dockerfile
    container_name: server
    ports:
      - "5678:5678"
    environment:
      - PYTHONUNBUFFERED=1
      - SERVER_HOST=server
      - SERVER_PORT=5678
      - STORAGE_FILE_PATH=/tmp/bets.csv
EOF

for i in $(seq 0 $((N - 1))); do
    cat >> "$OUT" <<EOF

  client_$i:
    build:
      context: ./services/client
      dockerfile: Dockerfile
    container_name: client_$i
    depends_on:
      - server
    volumes:
      - ./input:/input:ro
      - ./output:/output
    environment:
      - AGENCY_ID=$i
      - SERVER_HOST=server
      - SERVER_PORT=5678
      - INPUT_FILE=/input/input-$i.csv
      - OUTPUT_FILE=/output/output-$i.csv
      - BATCH_SIZE=100
EOF
done

echo "Generado $OUT con $N cliente(s)."
