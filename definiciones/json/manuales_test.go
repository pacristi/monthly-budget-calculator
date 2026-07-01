package defjson

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"presupuesto/presupuesto"
)

// resolvedorFijo es un ResolvedorConfig de prueba que siempre devuelve la
// misma config, sin importar el mes consultado.
type resolvedorFijo struct {
	cfg presupuesto.ConfigPresupuesto
	err error
}

func (r resolvedorFijo) ParaMes(mes time.Time) (presupuesto.ConfigPresupuesto, error) {
	return r.cfg, r.err
}

func TestRepoGastosManuales_RutaVaciaDevuelveVacio(t *testing.T) {
	repo := NewRepoGastosManuales("", resolvedorFijo{})

	gastos, err := repo.Listar()
	if err != nil {
		t.Fatalf("no esperaba error, obtuve %v", err)
	}
	if len(gastos) != 0 {
		t.Fatalf("esperaba 0 gastos, obtuve %d", len(gastos))
	}
}

func TestRepoGastosManuales_ArchivoAusenteToleraSinError(t *testing.T) {
	ruta := filepath.Join(t.TempDir(), "no-existe.json")
	repo := NewRepoGastosManuales(ruta, resolvedorFijo{})

	gastos, err := repo.Listar()
	if err != nil {
		t.Fatalf("esperaba tolerar archivo ausente sin error, obtuve %v", err)
	}
	if len(gastos) != 0 {
		t.Fatalf("esperaba 0 gastos, obtuve %d", len(gastos))
	}
}

func TestRepoGastosManuales_MapeaDebito(t *testing.T) {
	ruta := filepath.Join(t.TempDir(), "manuales.json")
	contenido := `[{"id":"manual-1","descripcion":"Arriendo","montoTotal":350000,"cuotasTotales":1,"fechaInicio":"01-05-2025","tipoPago":"debito"}]`
	if err := os.WriteFile(ruta, []byte(contenido), 0644); err != nil {
		t.Fatalf("preparando fixture: %v", err)
	}

	cfg := presupuesto.ConfigPresupuesto{DiaDeCorteCredito: 25}
	repo := NewRepoGastosManuales(ruta, resolvedorFijo{cfg: cfg})

	gastos, err := repo.Listar()
	if err != nil {
		t.Fatalf("no esperaba error, obtuve %v", err)
	}
	if len(gastos) != 1 {
		t.Fatalf("esperaba 1 gasto, obtuve %d", len(gastos))
	}
	g := gastos[0]
	if g.ID != "manual-1" {
		t.Errorf("ID: esperaba manual-1, obtuve %q", g.ID)
	}
	if g.Descripcion != "Arriendo" {
		t.Errorf("Descripcion: esperaba Arriendo, obtuve %q", g.Descripcion)
	}
	if g.MontoImputado != 350000 {
		t.Errorf("MontoImputado: esperaba 350000, obtuve %v", g.MontoImputado)
	}
	if g.Cuotas != 1 {
		t.Errorf("Cuotas: esperaba 1, obtuve %d", g.Cuotas)
	}
	if g.PoliticaCorte.Tipo != presupuesto.Debito {
		t.Errorf("PoliticaCorte.Tipo: esperaba Debito, obtuve %v", g.PoliticaCorte.Tipo)
	}
	if g.PoliticaCorte.DiaDeCorte != 0 {
		t.Errorf("PoliticaCorte.DiaDeCorte: esperaba 0 (débito no usa corte), obtuve %d", g.PoliticaCorte.DiaDeCorte)
	}
	if g.CategoriaID != presupuesto.CategoriaPorDefecto {
		t.Errorf("CategoriaID: esperaba %q, obtuve %q", presupuesto.CategoriaPorDefecto, g.CategoriaID)
	}
	wantFecha := time.Date(2025, time.May, 1, 0, 0, 0, 0, time.UTC)
	if !g.FechaTransaccion.Equal(wantFecha) {
		t.Errorf("FechaTransaccion: esperaba %v, obtuve %v", wantFecha, g.FechaTransaccion)
	}
}

func TestRepoGastosManuales_MapeaCreditoConDiaDeCorte(t *testing.T) {
	ruta := filepath.Join(t.TempDir(), "manuales.json")
	contenido := `[{"id":"manual-2","descripcion":"TV a cuotas","montoTotal":120000,"cuotasTotales":6,"fechaInicio":"15-03-2025","tipoPago":"credito"}]`
	if err := os.WriteFile(ruta, []byte(contenido), 0644); err != nil {
		t.Fatalf("preparando fixture: %v", err)
	}

	cfg := presupuesto.ConfigPresupuesto{DiaDeCorteCredito: 25}
	repo := NewRepoGastosManuales(ruta, resolvedorFijo{cfg: cfg})

	gastos, err := repo.Listar()
	if err != nil {
		t.Fatalf("no esperaba error, obtuve %v", err)
	}
	if len(gastos) != 1 {
		t.Fatalf("esperaba 1 gasto, obtuve %d", len(gastos))
	}
	g := gastos[0]
	if g.PoliticaCorte.Tipo != presupuesto.Credito {
		t.Errorf("PoliticaCorte.Tipo: esperaba Credito, obtuve %v", g.PoliticaCorte.Tipo)
	}
	if g.PoliticaCorte.DiaDeCorte != 25 {
		t.Errorf("PoliticaCorte.DiaDeCorte: esperaba 25, obtuve %d", g.PoliticaCorte.DiaDeCorte)
	}
	if g.Cuotas != 6 {
		t.Errorf("Cuotas: esperaba 6, obtuve %d", g.Cuotas)
	}
}

func TestRepoGastosManuales_IgnoraFechaInvalida(t *testing.T) {
	ruta := filepath.Join(t.TempDir(), "manuales.json")
	contenido := `[
		{"id":"malo","descripcion":"Fecha rota","montoTotal":1000,"cuotasTotales":1,"fechaInicio":"no-es-fecha","tipoPago":"debito"},
		{"id":"bueno","descripcion":"Fecha ok","montoTotal":2000,"cuotasTotales":1,"fechaInicio":"01-01-2025","tipoPago":"debito"}
	]`
	if err := os.WriteFile(ruta, []byte(contenido), 0644); err != nil {
		t.Fatalf("preparando fixture: %v", err)
	}

	repo := NewRepoGastosManuales(ruta, resolvedorFijo{})

	gastos, err := repo.Listar()
	if err != nil {
		t.Fatalf("no esperaba error, obtuve %v", err)
	}
	if len(gastos) != 1 {
		t.Fatalf("esperaba filtrar el registro con fecha inválida y quedarse con 1, obtuve %d", len(gastos))
	}
	if gastos[0].ID != "bueno" {
		t.Errorf("esperaba conservar el gasto con fecha válida, obtuve %q", gastos[0].ID)
	}
}

func TestRepoGastosManuales_ArchivoConJSONInvalidoToleraSinError(t *testing.T) {
	ruta := filepath.Join(t.TempDir(), "manuales.json")
	if err := os.WriteFile(ruta, []byte("{no es json valido"), 0644); err != nil {
		t.Fatalf("preparando fixture: %v", err)
	}

	repo := NewRepoGastosManuales(ruta, resolvedorFijo{})

	gastos, err := repo.Listar()
	if err != nil {
		t.Fatalf("esperaba tolerar JSON inválido sin error, obtuve %v", err)
	}
	if len(gastos) != 0 {
		t.Fatalf("esperaba 0 gastos, obtuve %d", len(gastos))
	}
}

func TestRepoGastosManuales_PropagaErrorDelResolvedor(t *testing.T) {
	ruta := filepath.Join(t.TempDir(), "manuales.json")
	contenido := `[{"id":"manual-1","descripcion":"Arriendo","montoTotal":350000,"cuotasTotales":1,"fechaInicio":"01-05-2025","tipoPago":"debito"}]`
	if err := os.WriteFile(ruta, []byte(contenido), 0644); err != nil {
		t.Fatalf("preparando fixture: %v", err)
	}

	boom := os.ErrInvalid
	repo := NewRepoGastosManuales(ruta, resolvedorFijo{err: boom})

	_, err := repo.Listar()
	if err == nil {
		t.Fatal("esperaba que se propague el error del resolvedor")
	}
}
