package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"presupuesto/app"
	"presupuesto/presupuesto"
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
	dir := t.TempDir()
	divisionesPath := filepath.Join(dir, "divisiones.json")
	a, err := app.New(app.Config{
		ConfigsPath:     filepath.Join(dir, "configs.json"),
		CategoriasPath:  filepath.Join(dir, "categorias.json"),
		ReglasPath:      filepath.Join(dir, "reglas.json"),
		ExclusionesPath: filepath.Join(dir, "exclusiones.json"),
		DivisionesPath:  divisionesPath,
		SueldoPath:      filepath.Join(dir, "sueldo.json"),
		ManualesPath:    filepath.Join(dir, "manuales.json"),
		DBPath:          filepath.Join(dir, "mov.db"),
		ProvisorioPath:  filepath.Join(dir, "current.json"),
	})
	if err != nil {
		t.Fatalf("app.New: %v", err)
	}
	t.Cleanup(func() { a.Close() })
	deps := apiDeps{app: a}

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

	// Verifica que el override efectivamente se persistió en DivisionesPath
	// con los valores del body, no solo que el handler respondió 200.
	data, err := os.ReadFile(divisionesPath)
	if err != nil {
		t.Fatalf("leyendo %s: %v", divisionesPath, err)
	}
	var overrides []presupuesto.Override
	if err := json.Unmarshal(data, &overrides); err != nil {
		t.Fatalf("decodificando overrides persistidos: %v", err)
	}
	if len(overrides) != 1 {
		t.Fatalf("se esperaba 1 override persistido, got %d: %+v", len(overrides), overrides)
	}
	persistido := overrides[0]
	if persistido.MovimientoID != "sql-1" {
		t.Errorf("MovimientoID persistido = %q, want %q", persistido.MovimientoID, "sql-1")
	}
	if persistido.Fecha != "2026-06-30" {
		t.Errorf("Fecha persistida = %q, want %q", persistido.Fecha, "2026-06-30")
	}
	if persistido.MontoOriginal != -3 {
		t.Errorf("MontoOriginal persistido = %v, want %v", persistido.MontoOriginal, -3.0)
	}
	if persistido.Descripcion != "WINDSCRIBE" {
		t.Errorf("Descripcion persistida = %q, want %q", persistido.Descripcion, "WINDSCRIBE")
	}
	if persistido.EsUSD == nil || !*persistido.EsUSD {
		t.Errorf("EsUSD persistido = %v, want true", persistido.EsUSD)
	}
}

// TestHandleMovimientoMoneda_ErrorDeAppEs500 fuerza un error real en la capa
// de persistencia (no un fake inyectable, porque apiDeps.app es *app.App
// concreto): apunta DivisionesPath a una ruta cuyo directorio padre es un
// archivo regular, así que escribirArchivoAtomico falla al hacer MkdirAll
// sobre ese "directorio". Esto ejercita la rama 500 documentada en
// handleMovimientoMoneda.
func TestHandleMovimientoMoneda_ErrorDeAppEs500(t *testing.T) {
	dir := t.TempDir()

	bloqueador := filepath.Join(dir, "bloqueador")
	if err := os.WriteFile(bloqueador, []byte("no soy un directorio"), 0644); err != nil {
		t.Fatalf("creando archivo bloqueador: %v", err)
	}
	divisionesPath := filepath.Join(bloqueador, "divisiones.json")

	a, err := app.New(app.Config{
		ConfigsPath:     filepath.Join(dir, "configs.json"),
		CategoriasPath:  filepath.Join(dir, "categorias.json"),
		ReglasPath:      filepath.Join(dir, "reglas.json"),
		ExclusionesPath: filepath.Join(dir, "exclusiones.json"),
		DivisionesPath:  divisionesPath,
		SueldoPath:      filepath.Join(dir, "sueldo.json"),
		ManualesPath:    filepath.Join(dir, "manuales.json"),
		DBPath:          filepath.Join(dir, "mov.db"),
		ProvisorioPath:  filepath.Join(dir, "current.json"),
	})
	if err != nil {
		t.Fatalf("app.New: %v", err)
	}
	t.Cleanup(func() { a.Close() })
	deps := apiDeps{app: a}

	body := `{"movimientoId":"sql-1","fecha":"2026-06-30","montoOriginal":-3,"descripcion":"WINDSCRIBE","esUSD":true}`
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/movimientos/moneda", bytes.NewBufferString(body))
	deps.handleMovimientoMoneda(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status: %d (%s), want %d", rec.Code, rec.Body.String(), http.StatusInternalServerError)
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
