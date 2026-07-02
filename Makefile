ifneq (,$(wildcard ./.env))
    include .env
    export
endif

MAIN_CLI=./cmd/cli
MAIN_API=./cmd/api
BIN_DIR=./bin

.PHONY: help start ingest serve test build clean \
        sqlite-init ingest-sqlite ingest-xlsx-cta-corriente ingest-xlsx-tc-nacional ingest-xlsx-tc-internacional

help: ## Muestra esta ayuda
	@egrep -h '\s##\s' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'

# ======================================================================
# Modo simple (recomendado primera vez)
# ======================================================================

start: ## Wizard: configura .env, instala deps y levanta el dashboard
	@bash scripts/start.sh

ingest: ## Trae la cartola del día y la vuelca al sqlite (idempotente)
	@cd ingesta/open-banking-chile && node scraper.js
	@go run $(MAIN_CLI) ingestar obchile --db data/movimientos.db --json data/current.json

serve: ## Levanta el dashboard web en http://localhost:8085
	@go run $(MAIN_API) --divisiones data/divisiones.json --provisorio data/current.json

test: ## Corre los tests unitarios
	@go test ./...

# ======================================================================
# Modo avanzado: histórico con cartolas .xls + sqlite
# Sólo necesario si querés ver meses pasados o no querés depender solo
# del scraper diario. Ver sección "Avanzado" del README.
# ======================================================================

sqlite-init: ## (avanzado) Inicializa la BD sqlite vacía
	@go run $(MAIN_CLI) sqlite-init --db data/movimientos.db

ingest-sqlite: ## (avanzado) Scraper + volcado al sqlite (idempotente)
	@cd ingesta/open-banking-chile && node scraper.js
	@go run $(MAIN_CLI) ingestar obchile --db data/movimientos.db --json data/current.json

ingest-xlsx-cta-corriente: ## (avanzado) Carga cartolas .xls de cuenta corriente. Args: AÑO=2025 DIR="ruta"
	@test -n "$(AÑO)" || (echo 'Falta AÑO. Uso: make ingest-xlsx-cta-corriente AÑO=2025 DIR="..."' && exit 1)
	@test -n "$(DIR)" || (echo 'Falta DIR. Uso: make ingest-xlsx-cta-corriente AÑO=2025 DIR="..."' && exit 1)
	@go run $(MAIN_CLI) ingestar xlsx --banco bchile --tipo cta-corriente --año $(AÑO) --dir "$(DIR)" --db data/movimientos.db

ingest-xlsx-tc-nacional: ## (avanzado) Carga cartolas .xls de TC nacional. Args: DIR="ruta"
	@test -n "$(DIR)" || (echo 'Falta DIR. Uso: make ingest-xlsx-tc-nacional DIR="..."' && exit 1)
	@go run $(MAIN_CLI) ingestar xlsx --banco bchile --tipo tc-nacional --dir "$(DIR)" --db data/movimientos.db

ingest-xlsx-tc-internacional: ## (avanzado) Carga cartolas .xls de TC internacional. Args: DIR="ruta"
	@test -n "$(DIR)" || (echo 'Falta DIR. Uso: make ingest-xlsx-tc-internacional DIR="..."' && exit 1)
	@go run $(MAIN_CLI) ingestar xlsx --banco bchile --tipo tc-internacional --dir "$(DIR)" --db data/movimientos.db

# ======================================================================
# Utilitarios
# ======================================================================

build: ## Compila los binarios
	@mkdir -p $(BIN_DIR)
	@go build -o $(BIN_DIR)/presupuesto-cli $(MAIN_CLI)
	@go build -o $(BIN_DIR)/presupuesto-api $(MAIN_API)

clean: ## Limpia binarios (NO toca data/)
	@rm -rf $(BIN_DIR)
