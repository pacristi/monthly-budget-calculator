package main

import (
	"encoding/json"
	"net/http"
)

// modoRefresh es el valor fijo del campo `modo` de la respuesta de /api/refresh.
// El frontend (web/app.js) ya no ramifica por modo — se conserva el campo por
// compatibilidad de shape, con un único modo posible: sqlite.
const modoRefresh = "sqlite"

// refrescador es la superficie mínima que necesita el handler de refresh. La
// satisface *app.App; en tests se usa un fake.
type refrescador interface {
	Refrescar(persistir bool) (int, error)
}

// handleRefresh dispara una ingesta nueva desde el dashboard (equivalente a
// `make ingest`). Con un solo modo (sqlite), refrescar SIEMPRE persiste.
//
// Es síncrono: el request queda abierto mientras corre el scraper (segundos).
//
// Seguridad: este endpoint dispara un proceso del sistema. Es seguro solo
// mientras el server escuche en localhost; no exponer el puerto a la red.
func (deps apiDeps) handleRefresh(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	nuevos, err := deps.refresh.Refrescar(true)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"status": "ok",
		"modo":   modoRefresh,
		"nuevos": nuevos,
	})
}
