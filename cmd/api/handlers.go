package main

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"presupuesto/presupuesto"
)

// handleBudget sirve GET /api/budget?year&month. Calcula el mes (hoy por
// defecto; ajustado por year/month si vienen válidos, month en rango 1-12) y
// delega en app.ResumenDelMes. El shape de app.Resumen ya calza con el contrato.
func (deps apiDeps) handleBudget(w http.ResponseWriter, r *http.Request) {
	mes := time.Now()
	if mStr := r.URL.Query().Get("month"); mStr != "" {
		if m, err := strconv.Atoi(mStr); err == nil && m >= 1 && m <= 12 {
			mes = time.Date(mes.Year(), time.Month(m), 1, 0, 0, 0, 0, mes.Location())
		}
	}
	if yStr := r.URL.Query().Get("year"); yStr != "" {
		if y, err := strconv.Atoi(yStr); err == nil {
			mes = time.Date(y, mes.Month(), 1, 0, 0, 0, 0, mes.Location())
		}
	}

	resumen, err := deps.app.ResumenDelMes(mes)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	responderJSON(w, resumen)
}

// handleProjections sirve GET /api/projections?months (default 6). Delega en
// app.Proyecciones desde ahora.
func (deps apiDeps) handleProjections(w http.ResponseWriter, r *http.Request) {
	meses := 6
	if mStr := r.URL.Query().Get("months"); mStr != "" {
		if m, err := strconv.Atoi(mStr); err == nil && m > 0 {
			meses = m
		}
	}

	proyecciones, err := deps.app.Proyecciones(time.Now(), meses)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	responderJSON(w, proyecciones)
}

// handleMovements sirve GET /api/movements. Delega en app.Movimientos.
func (deps apiDeps) handleMovements(w http.ResponseWriter, r *http.Request) {
	movs, err := deps.app.Movimientos()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	responderJSON(w, movs)
}

// handleDivisions sirve POST /api/divisions: persiste el override de "mi parte".
func (deps apiDeps) handleDivisions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var o presupuesto.Override
	if err := json.NewDecoder(r.Body).Decode(&o); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := deps.app.GuardarMiParte(o); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	responderOK(w)
}

// handleMovimientoCategoria sirve POST /api/movimientos/categoria: asigna a mano
// la categoría de un movimiento, preservando el split si ya existía.
func (deps apiDeps) handleMovimientoCategoria(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var o presupuesto.Override
	if err := json.NewDecoder(r.Body).Decode(&o); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := deps.app.GuardarCategoriaDeMovimiento(o); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	responderOK(w)
}

// handleMovimientoAlias sirve POST /api/movimientos/alias: persiste el alias de
// descripción de un movimiento.
func (deps apiDeps) handleMovimientoAlias(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var o presupuesto.Override
	if err := json.NewDecoder(r.Body).Decode(&o); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := deps.app.GuardarAlias(o); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	responderOK(w)
}

// handleCategorias sirve GET (lista) y POST (reemplazo total) de categorías.
func (deps apiDeps) handleCategorias(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		cats, err := deps.app.Categorias()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		responderJSON(w, cats)
	case http.MethodPost:
		var cats []presupuesto.Categoria
		if err := json.NewDecoder(r.Body).Decode(&cats); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := deps.app.GuardarCategorias(cats); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		responderOK(w)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleReglas sirve GET (reglas efectivas, migrando exclusiones legacy) y POST
// (reemplazo total) de reglas de categorización.
func (deps apiDeps) handleReglas(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		reglas, err := deps.app.Reglas()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if reglas == nil {
			reglas = []presupuesto.Regla{}
		}
		responderJSON(w, reglas)
	case http.MethodPost:
		var reglas []presupuesto.Regla
		if err := json.NewDecoder(r.Body).Decode(&reglas); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := deps.app.GuardarReglas(reglas); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		responderOK(w)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleSueldo sirve GET/POST de los patrones de descripción que identifican el
// sueldo (JSON simple `["a","b",...]`).
func (deps apiDeps) handleSueldo(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		lista, err := deps.app.PatronesSueldo()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		responderListaStrings(w, lista)
	case http.MethodPost:
		lista, err := decodeListaStrings(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := deps.app.GuardarPatronesSueldo(lista); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		responderOK(w)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleExclusiones sirve GET/POST de las exclusiones legacy (substrings a
// ignorar), mientras exista la migración a reglas.
func (deps apiDeps) handleExclusiones(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		lista, err := deps.app.Exclusiones()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		responderListaStrings(w, lista)
	case http.MethodPost:
		lista, err := decodeListaStrings(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := deps.app.GuardarExclusiones(lista); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		responderOK(w)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// decodeListaStrings decodifica el body como una lista de strings.
func decodeListaStrings(r *http.Request) ([]string, error) {
	var lista []string
	if err := json.NewDecoder(r.Body).Decode(&lista); err != nil {
		return nil, err
	}
	return lista, nil
}

// responderListaStrings serializa una lista de strings, normalizando nil a `[]`.
func responderListaStrings(w http.ResponseWriter, lista []string) {
	if lista == nil {
		lista = []string{}
	}
	responderJSON(w, lista)
}

// responderJSON escribe v como JSON con Content-Type application/json.
func responderJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

// responderOK escribe la respuesta estándar {"status":"ok"} de las escrituras.
func responderOK(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}
