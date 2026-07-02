package main

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	defjson "presupuesto/definiciones/json"
	"presupuesto/presupuesto"
)

// configStore es la superficie de app/ que necesitan los handlers de configs.
// La satisface *app.App directamente; en tests se usa un fake sobre un
// *defjson.RepoJSON temporal.
type configStore interface {
	Configs() ([]defjson.ConfigMensual, error)
	ConfigResuelta(mes time.Time) (presupuesto.ConfigPresupuesto, error)
	GuardarConfig(c defjson.ConfigMensual) error
	BorrarConfig(mesDesde string) error
}

// handlerListar sirve GET /api/configs.
func handlerListar(store configStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		configs, err := store.Configs()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if configs == nil {
			configs = []defjson.ConfigMensual{}
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(configs)
	}
}

// handlerSubconfigs maneja /api/configs/{algo}:
//   - "resuelta"   → GET ?mes=YYYY-MM
//   - "{YYYY-MM}"  → PUT | DELETE
func handlerSubconfigs(store configStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		resto := strings.TrimPrefix(r.URL.Path, "/api/configs/")
		resto = strings.Trim(resto, "/")
		if resto == "" {
			http.Error(w, "Not found", http.StatusNotFound)
			return
		}
		if resto == "resuelta" {
			handleResuelta(w, r, store)
			return
		}
		handleItem(w, r, store, resto)
	}
}

func handleResuelta(w http.ResponseWriter, r *http.Request, store configStore) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	mesStr := r.URL.Query().Get("mes")
	if mesStr == "" {
		http.Error(w, "missing ?mes=YYYY-MM", http.StatusBadRequest)
		return
	}
	mes, err := defjson.ParseMes(mesStr)
	if err != nil {
		http.Error(w, "mes inválido, esperado YYYY-MM", http.StatusBadRequest)
		return
	}
	resuelta, err := store.ConfigResuelta(mes)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resuelta)
}

func handleItem(w http.ResponseWriter, r *http.Request, store configStore, mesDesde string) {
	if _, err := defjson.ParseMes(mesDesde); err != nil {
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
		c := defjson.ConfigMensual{
			MesDesde:             mesDesde,
			Porcentajes:          payload.Porcentajes,
			PorcentajeParaGastos: payload.PorcentajeParaGastos,
			DiaDeCorteCredito:    payload.DiaDeCorteCredito,
			TasaCambioUSD:        payload.TasaCambioUSD,
		}
		if err := store.GuardarConfig(c); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(c)

	case http.MethodDelete:
		if err := store.BorrarConfig(mesDesde); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusNoContent)

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}
