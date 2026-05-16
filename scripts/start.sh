#!/usr/bin/env bash
# Wizard de setup + arranque para usuarios nuevos.
# Crea .env y los configs personales si no existen, después levanta el dashboard.

set -euo pipefail

cd "$(dirname "$0")/.."

# Carga .env si existe. Lo hacemos al inicio Y después de crearlo, así
# `node scraper.js` y `go run` ven BANCO_RUT, BANCO_PASS y BANCO_ID como
# variables de entorno (igual que cuando se corre por Makefile, que tiene
# `include .env; export`).
load_env() {
  if [ -f .env ]; then
    set -a
    # shellcheck disable=SC1091
    source .env
    set +a
  fi
}

load_env

echo "Calculadora de Presupuesto Mensual"
echo "===================================="
echo

# 1. .env
if [ ! -f .env ]; then
  echo "No encontré .env. Vamos a crearlo."
  echo
  read -p "Banco (default: bchile): " bid
  bid="${bid:-bchile}"
  read -p "Tu RUT (ej. 12.345.678-9): " rut
  read -s -p "Tu clave de internet del banco: " pass
  echo
  cat > .env <<EOF
BANCO_ID=$bid
BANCO_RUT=$rut
BANCO_PASS=$pass
EOF
  echo "OK, .env creado."
  load_env
else
  echo ".env ya existe, no lo toco."
fi
echo

# 2. Configs personales
ensure_copy() {
  local dst="$1"
  local src="$2"
  local label="$3"
  if [ ! -f "$dst" ]; then
    cp "$src" "$dst"
    echo "Creado $dst desde $src — editalo después para personalizar $label."
  fi
}
mkdir -p data
ensure_copy data/exclusiones.json data/exclusiones.example.json "qué gastos ignorar"
ensure_copy data/sueldo.json data/sueldo.example.json "cómo se identifica tu sueldo"
[ -f data/divisiones.json ] || echo "[]" > data/divisiones.json
echo

# 3. Dependencias
if [ ! -d ingest/node_modules ]; then
  echo "Instalando dependencias del scraper..."
  (cd ingest && npm install)
fi
echo "Resolviendo dependencias Go..."
go mod tidy
echo

# 4. Primera ingesta (opcional)
read -p "¿Traer la cartola ahora? (S/n): " ans
ans="${ans:-S}"
if [[ "$ans" =~ ^[SsYy] ]]; then
  echo
  echo "Corriendo scraper..."
  (cd ingest && node scraper.js)
  echo
fi

# 5. Arranque
echo "Listo. Levantando dashboard en http://localhost:8085 ..."
echo "(Ctrl+C para parar.)"
echo
exec go run ./cmd/presupuesto-api data/current.json data/divisiones.json
