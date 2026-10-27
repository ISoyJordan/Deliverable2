#!/usr/bin/env bash
#
# Experimentos de rendimiento siguiendo el protocolo del enunciado:
# medir el tiempo de probar N contraseñas inválidas con distintas alternativas.
#
#   Experimento A — backend y escalado: fija la generación (stream) y varía
#                   el backend (native/gpg) y el número de workers. Da la
#                   comparativa de tecnología y la curva de speedup.
#   Experimento B — estrategia de generación: fija el backend más rápido y
#                   varía cómo se producen las candidatas (stream/preload/file).
#
# Uso:   ./bench/bench.sh [N]
#        N = nº de contraseñas inválidas por corrida (por defecto 1000).
#
# Salida: tabla por pantalla + bench/results.csv
#
set -euo pipefail

cd "$(dirname "$0")/.."

N="${1:-1000}"
FILE="${FILE:-archive.gpg}"
BIN="./gpg-cracker"
CSV="bench/results.csv"

# Lista de workers: 1, 2, 4, ... hasta el nº de núcleos (y el propio nº).
CORES="$(nproc)"
WORKER_LIST="1"
w=2
while [ "$w" -lt "$CORES" ]; do WORKER_LIST="$WORKER_LIST $w"; w=$((w * 2)); done
[ "$CORES" -gt 1 ] && WORKER_LIST="$WORKER_LIST $CORES"

[ -x "$BIN" ] || { echo "Compila primero: go build -o gpg-cracker ."; exit 1; }
[ -f "$FILE" ] || { echo "No existe $FILE"; exit 1; }

echo "backend,gen,workers,n,elapsed_ms,rate" > "$CSV"
run() { # backend gen workers
  local line
  line="$("$BIN" -file "$FILE" -backend "$1" -gen "$2" -workers "$3" -bench "$N")"
  echo "  $line"
  echo "$line" | sed -E 's/BENCH backend=([^ ]+) gen=([^ ]+) workers=([^ ]+) n=([^ ]+) elapsed_ms=([^ ]+) rate=([^ ]+)/\1,\2,\3,\4,\5,\6/' >> "$CSV"
}

echo "== Experimento A: backend x workers (gen=stream, n=$N) =="
for backend in native gpg; do
  for workers in $WORKER_LIST; do
    run "$backend" stream "$workers"
  done
done

echo
echo "== Experimento B: estrategia de generación (backend=native, workers=$CORES, n=$N) =="
for gen in stream preload file; do
  run native "$gen" "$CORES"
done

echo
echo "Resultados guardados en $CSV"
if command -v column >/dev/null 2>&1; then
  column -t -s, "$CSV"
else
  cat "$CSV"
fi
