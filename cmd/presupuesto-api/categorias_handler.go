package main

import (
	"encoding/json"
	"net/http"

	"presupuesto/internal/ajustes"
	"presupuesto/internal/presupuesto"
)

// handleCategorias sirve GET (lista) y POST (reemplazo total) de categorías.
func (deps apiDeps) handleCategorias(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		cats, err := deps.app.RepoCategorias.Listar()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(cats)
	case http.MethodPost:
		var cats []presupuesto.Categoria
		if err := json.NewDecoder(r.Body).Decode(&cats); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := deps.app.RepoCategorias.Guardar(cats); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleReglas sirve GET (reglas efectivas, migrando exclusiones legacy) y
// POST (reemplazo total) de reglas de categorización.
func (deps apiDeps) handleReglas(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		reglas, err := ajustes.CargarReglas(deps.app.ReglasPath, deps.app.ExclusionesPath)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if reglas == nil {
			reglas = []presupuesto.Regla{}
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(reglas)
	case http.MethodPost:
		var reglas []presupuesto.Regla
		if err := json.NewDecoder(r.Body).Decode(&reglas); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := ajustes.EscribirReglas(deps.app.ReglasPath, reglas); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleMovimientoCategoria asigna a mano la categoría de un movimiento,
// preservando el split (MiParte) si ya existía. Persiste en el archivo de
// divisiones (mismo registro de override, preferentemente por movimientoId).
func (deps apiDeps) handleMovimientoCategoria(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if deps.app.DivisionesPath == "" {
		http.Error(w, "No divisions file configured", http.StatusBadRequest)
		return
	}

	var req ajustes.Override
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := ajustes.GuardarCategoria(deps.app.DivisionesPath, ajustes.Override{
		MovimientoID:  req.MovimientoID,
		Fecha:         req.Fecha,
		MontoOriginal: req.MontoOriginal,
		Descripcion:   req.Descripcion,
		Categoria:     req.Categoria,
	}); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}
