package main

import (
	"encoding/json"
	"net/http"
)

type refreshUseCase interface {
	Ejecutar(persistir bool) (int, error)
}

var refrescarDashboard refreshUseCase

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

	nuevos, err := refrescarDashboard.Ejecutar(proveedor == "sqlite" || proveedor == "compuesto")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"status": "ok",
		"modo":   proveedor,
		"nuevos": nuevos,
	})
}
