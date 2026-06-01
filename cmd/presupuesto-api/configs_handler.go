package main

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/pierocristi/monthly-budget-calculator/internal/config"
)

// handlerListar sirve GET /api/configs.
func handlerListar(repo *config.RepoJSON) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		configs, err := repo.Listar()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if configs == nil {
			configs = []config.ConfigMensual{}
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(configs)
	}
}

// handlerSubconfigs maneja /api/configs/{algo}:
//   - "resuelta"   → GET ?mes=YYYY-MM
//   - "{YYYY-MM}"  → PUT | DELETE
func handlerSubconfigs(repo *config.RepoJSON) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		resto := strings.TrimPrefix(r.URL.Path, "/api/configs/")
		resto = strings.Trim(resto, "/")
		if resto == "" {
			http.Error(w, "Not found", http.StatusNotFound)
			return
		}
		if resto == "resuelta" {
			handleResuelta(w, r, repo)
			return
		}
		handleItem(w, r, repo, resto)
	}
}

func handleResuelta(w http.ResponseWriter, r *http.Request, repo *config.RepoJSON) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	mesStr := r.URL.Query().Get("mes")
	if mesStr == "" {
		http.Error(w, "missing ?mes=YYYY-MM", http.StatusBadRequest)
		return
	}
	mes, err := config.ParseMes(mesStr)
	if err != nil {
		http.Error(w, "mes inválido, esperado YYYY-MM", http.StatusBadRequest)
		return
	}
	resuelta, err := repo.ParaMes(mes)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resuelta)
}

func handleItem(w http.ResponseWriter, r *http.Request, repo *config.RepoJSON, mesDesde string) {
	if _, err := config.ParseMes(mesDesde); err != nil {
		http.Error(w, "mesDesde inválido en la ruta, esperado YYYY-MM", http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodPut:
		var payload struct {
			Porcentajes          map[string]float64 `json:"porcentajes"`
			PorcentajeParaGastos float64            `json:"porcentajeParaGastos"`
			DiaDeCorteCredito    int                `json:"diaDeCorteCredito"`
			TasaCambioUSD        float64            `json:"tasaCambioUSD"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, "body inválido: "+err.Error(), http.StatusBadRequest)
			return
		}
		c := config.ConfigMensual{
			MesDesde:             mesDesde,
			Porcentajes:          payload.Porcentajes,
			PorcentajeParaGastos: payload.PorcentajeParaGastos,
			DiaDeCorteCredito:    payload.DiaDeCorteCredito,
			TasaCambioUSD:        payload.TasaCambioUSD,
		}
		if err := repo.Guardar(c); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(c)

	case http.MethodDelete:
		if err := repo.Borrar(mesDesde); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusNoContent)

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}
