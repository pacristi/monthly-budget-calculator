ifneq (,$(wildcard ./.env))
    include .env
    export
endif

# Variables
APP_NAME=presupuesto-cli
MAIN_PATH=./cmd/presupuesto-cli
BIN_DIR=./bin

.PHONY: help setup ingest run build test clean all

# Comando por defecto si solo escribes 'make'
help: ## Muestra esta ayuda
	@egrep -h '\s##\s' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

setup: ## Prepara el entorno (crea carpetas, instala dependencias de Node y Go)
	@echo "🛠️ Configurando el entorno..."
	@mkdir -p data/archive
	@touch data/divisiones.json
	@cd ingest && npm install
	@go mod tidy
	@echo "✅ Entorno listo."

ingest: ## Ejecuta el scraper de Node para actualizar current.json
	@echo "🏦 Obteniendo cartola del banco..."
	@cd ingest && node scraper.js

run: ## Ejecuta la calculadora en Go sin compilar el binario
	@echo "🧮 Calculando presupuesto..."
	@go run $(MAIN_PATH) data/current.json data/divisiones.json

serve: ## Levanta el servidor web con el dashboard
	@echo "🌐 Levantando servidor en puerto 8085..."
	@go run ./cmd/presupuesto-api data/current.json data/divisiones.json

build: ## Compila el binario de Go para producción
	@echo "🔨 Compilando binario..."
	@mkdir -p $(BIN_DIR)
	@go build -o $(BIN_DIR)/$(APP_NAME) $(MAIN_PATH)
	@echo "✅ Binario generado en $(BIN_DIR)/$(APP_NAME)"

test: ## Ejecuta los tests unitarios del dominio y shared
	@echo "🧪 Corriendo tests..."
	@go test -v ./internal/...

clean: ## Limpia binarios (NO toca la carpeta data)
	@echo "🧹 Limpiando..."
	@rm -rf $(BIN_DIR)

all: ingest run ## Flujo completo: trae datos frescos y luego calcula