package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"

	"github.com/pierocristi/monthly-budget-calculator/internal/cartola/ingesta"
)

// ejecutarScraper corre el scraper de Node (`ingest/scraper.js`), que trae la
// cartola del día y sobrescribe data/current.json. Equivale a `make ingest`.
// Es una variable para poder stubbearla en los tests sin invocar Node.
var ejecutarScraper = func() error {
	cmd := exec.Command("node", "scraper.js")
	cmd.Dir = "ingest"
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("scraper: %v: %s", err, out)
	}
	return nil
}

// volcarASqlite ingesta el current.json recién scrapeado al sqlite (modo
// avanzado). Equivale al segundo paso de `make ingest-sqlite`. Variable para
// stubbearla en tests.
var volcarASqlite = func(jsonPath, dbPath string) (int, error) {
	return ingesta.DesdeScraper(jsonPath, dbPath)
}

// handleRefresh dispara una ingesta nueva desde el dashboard, equivalente a
// `make ingest` (modo simple) o `make ingest-sqlite` (modo avanzado). El server
// ya sabe en qué modo está por la variable global `proveedor`, así que el
// cliente no necesita indicarlo: el endpoint bifurca solo.
//
// Es síncrono: el request queda abierto mientras corre el scraper (segundos).
// El scraper de bchile es silencioso (RUT+clave, sin 2FA interactivo), así que
// no requiere intervención del usuario.
//
// Seguridad: este endpoint dispara un proceso del sistema. Es seguro solo
// mientras el server escuche en localhost; no exponer el puerto a la red.
func handleRefresh(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if err := ejecutarScraper(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	nuevos := 0
	if proveedor == "sqlite" || proveedor == "compuesto" {
		n, err := volcarASqlite("data/current.json", dbPath)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		nuevos = n
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"status": "ok",
		"modo":   proveedor,
		"nuevos": nuevos,
	})
}
