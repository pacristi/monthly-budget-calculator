package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"presupuesto/app"
)

// nuevaAppTest arma un *app.App real sobre un directorio temporal, con archivos
// de estado vacíos/semilla — mismo patrón que app.nuevaAppTest, pero usando
// solo la superficie exportada (app.New) porque cmd/api no puede importar los
// helpers de test del paquete app. apiDeps.app es *app.App concreto (no una
// interfaz), así que los handlers se testean contra una app real en vez de un
// fake, siguiendo el mismo criterio ya usado en configs_handler_test.go
// (repoStore real) y refresh_handler_test.go (fake solo para lo que sí es
// interfaz).
func nuevaAppTest(t *testing.T) *app.App {
	t.Helper()
	dir := t.TempDir()

	a, err := app.New(app.Config{
		ConfigsPath:     filepath.Join(dir, "configs.json"),
		CategoriasPath:  filepath.Join(dir, "categorias.json"),
		ReglasPath:      filepath.Join(dir, "reglas.json"),
		ExclusionesPath: filepath.Join(dir, "exclusiones.json"),
		DivisionesPath:  filepath.Join(dir, "divisiones.json"),
		SueldoPath:      filepath.Join(dir, "sueldo.json"),
		ManualesPath:    filepath.Join(dir, "manuales.json"),
		DBPath:          filepath.Join(dir, "mov.db"),
		ProvisorioPath:  filepath.Join(dir, "current.json"),
	})
	if err != nil {
		t.Fatalf("app.New: %v", err)
	}
	t.Cleanup(func() { a.Close() })
	return a
}

func TestHandleMovimientoMoneda_RechazaNoPost(t *testing.T) {
	deps := apiDeps{app: nuevaAppTest(t)}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/movimientos/moneda", nil)
	deps.handleMovimientoMoneda(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status: %d", rec.Code)
	}
}

func TestHandleMovimientoMoneda_PostGuardaYDevuelveOk(t *testing.T) {
	deps := apiDeps{app: nuevaAppTest(t)}

	body := `{"movimientoId":"sql-1","fecha":"2026-06-30","montoOriginal":-3,"descripcion":"WINDSCRIBE","esUSD":true}`
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/movimientos/moneda", bytes.NewBufferString(body))
	deps.handleMovimientoMoneda(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: %d (%s)", rec.Code, rec.Body.String())
	}

	var got map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got["status"] != "ok" {
		t.Fatalf("body inesperado: %v", got)
	}
}

func TestHandleMovimientoMoneda_BodyInvalidoEs400(t *testing.T) {
	deps := apiDeps{app: nuevaAppTest(t)}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/movimientos/moneda", bytes.NewBufferString("{invalido"))
	deps.handleMovimientoMoneda(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status: %d", rec.Code)
	}
}
