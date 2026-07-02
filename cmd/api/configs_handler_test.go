package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	defjson "presupuesto/definiciones/json"
	"presupuesto/presupuesto"
)

// repoStore adapta un *defjson.RepoJSON a la interfaz configStore que consumen
// los handlers, traduciendo los nombres de método (Listar→Configs,
// ParaMes→ConfigResuelta, etc.). Es el mismo mapeo que *app.App hace en prod.
type repoStore struct {
	repo *defjson.RepoJSON
}

func (s repoStore) Configs() ([]defjson.ConfigMensual, error) { return s.repo.Listar() }
func (s repoStore) ConfigResuelta(mes time.Time) (presupuesto.ConfigPresupuesto, error) {
	return s.repo.ParaMes(mes)
}
func (s repoStore) GuardarConfig(c defjson.ConfigMensual) error { return s.repo.Guardar(c) }
func (s repoStore) BorrarConfig(mesDesde string) error          { return s.repo.Borrar(mesDesde) }

func nuevoStore(t *testing.T) repoStore {
	t.Helper()
	return repoStore{repo: defjson.NewRepoJSON(filepath.Join(t.TempDir(), "configs.json"))}
}

func TestHandlerListar_VacioRetornaArrayVacio(t *testing.T) {
	store := nuevoStore(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/configs", nil)

	handlerListar(store)(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: %d", rec.Code)
	}
	if got := rec.Body.String(); got != "[]\n" {
		t.Errorf("body inesperado: %q", got)
	}
}

func TestHandlerListar_DevuelveConfigs(t *testing.T) {
	store := nuevoStore(t)
	_ = store.GuardarConfig(defjson.ConfigMensual{MesDesde: "2026-01", PorcentajeParaGastos: 0.25, DiaDeCorteCredito: 25, TasaCambioUSD: 950})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/configs", nil)
	handlerListar(store)(rec, req)

	var got []defjson.ConfigMensual
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(got) != 1 || got[0].MesDesde != "2026-01" {
		t.Errorf("lista inesperada: %+v", got)
	}
}

func TestHandlerPut_CreaConfig(t *testing.T) {
	store := nuevoStore(t)
	_ = store.GuardarConfig(defjson.ConfigMensual{MesDesde: "2026-01", PorcentajeParaGastos: 0.25, DiaDeCorteCredito: 25, TasaCambioUSD: 950})

	body := `{"porcentajeParaGastos":0.30,"diaDeCorteCredito":25,"tasaCambioUSD":980}`
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/api/configs/2026-05", bytes.NewBufferString(body))
	handlerSubconfigs(store)(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: %d (%s)", rec.Code, rec.Body.String())
	}

	configs, _ := store.Configs()
	if len(configs) != 2 {
		t.Fatalf("se esperaban 2 configs, got %d", len(configs))
	}
}

func TestHandlerPut_RechazaPayloadInvalido(t *testing.T) {
	store := nuevoStore(t)
	body := `{"porcentajeParaGastos":2.0,"diaDeCorteCredito":25,"tasaCambioUSD":950}`
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/api/configs/2026-05", bytes.NewBufferString(body))
	handlerSubconfigs(store)(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status: %d (%s)", rec.Code, rec.Body.String())
	}
}

func TestHandlerPut_MesInvalidoEnRuta(t *testing.T) {
	store := nuevoStore(t)
	body := `{"porcentajeParaGastos":0.25,"diaDeCorteCredito":25,"tasaCambioUSD":950}`
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/api/configs/2026-13", bytes.NewBufferString(body))
	handlerSubconfigs(store)(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status: %d", rec.Code)
	}
}

func TestHandlerDelete_FallaSiEsLaUnica(t *testing.T) {
	store := nuevoStore(t)
	_ = store.GuardarConfig(defjson.ConfigMensual{MesDesde: "2026-01", PorcentajeParaGastos: 0.25, DiaDeCorteCredito: 25, TasaCambioUSD: 950})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/api/configs/2026-01", nil)
	handlerSubconfigs(store)(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status: %d", rec.Code)
	}
}

func TestHandlerDelete_Ok(t *testing.T) {
	store := nuevoStore(t)
	_ = store.GuardarConfig(defjson.ConfigMensual{MesDesde: "2026-01", PorcentajeParaGastos: 0.25, DiaDeCorteCredito: 25, TasaCambioUSD: 950})
	_ = store.GuardarConfig(defjson.ConfigMensual{MesDesde: "2026-05", PorcentajeParaGastos: 0.30, DiaDeCorteCredito: 25, TasaCambioUSD: 980})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/api/configs/2026-05", nil)
	handlerSubconfigs(store)(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status: %d (%s)", rec.Code, rec.Body.String())
	}
	configs, _ := store.Configs()
	if len(configs) != 1 {
		t.Errorf("delete no surtió efecto: %+v", configs)
	}
}

func TestHandlerResuelta_Ok(t *testing.T) {
	store := nuevoStore(t)
	_ = store.GuardarConfig(defjson.ConfigMensual{MesDesde: "2026-01", PorcentajeParaGastos: 0.25, DiaDeCorteCredito: 25, TasaCambioUSD: 950})
	_ = store.GuardarConfig(defjson.ConfigMensual{MesDesde: "2026-05", PorcentajeParaGastos: 0.30, DiaDeCorteCredito: 25, TasaCambioUSD: 980})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/configs/resuelta?mes=2026-08", nil)
	handlerSubconfigs(store)(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: %d (%s)", rec.Code, rec.Body.String())
	}
	var got presupuesto.ConfigPresupuesto
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.HeredadaDe != "2026-05" || got.PorcentajeParaGastos != 0.30 {
		t.Errorf("resolución inesperada: %+v", got)
	}
}

func TestHandlerResuelta_MesFaltante(t *testing.T) {
	store := nuevoStore(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/configs/resuelta", nil)
	handlerSubconfigs(store)(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status: %d", rec.Code)
	}
}
