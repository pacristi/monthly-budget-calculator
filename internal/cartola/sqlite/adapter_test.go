package sqlite

import (
	"database/sql"
	"os"
	"testing"
	"time"

	_ "modernc.org/sqlite"

	"github.com/pierocristi/monthly-budget-calculator/internal/presupuesto"
)

type fakeResolvedor struct {
	tasaUSD    float64
	diaCorteCC int
	porcGastos float64
}

func (r fakeResolvedor) ParaMes(t time.Time) (presupuesto.ConfigPresupuesto, error) {
	return presupuesto.ConfigPresupuesto{
		TasaCambioUSD:        r.tasaUSD,
		DiaDeCorteCredito:    r.diaCorteCC,
		PorcentajeParaGastos: r.porcGastos,
		HeredadaDe:           t.Format("2006-01"),
	}, nil
}

func setupAdapterDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	if err := Up(db); err != nil {
		t.Fatalf("Up: %v", err)
	}
	return db
}

func insertarMov(t *testing.T, db *sql.DB, fecha string, monto float64, desc, source string, isUSD bool, cuotas string) {
	t.Helper()
	isUSDInt := 0
	if isUSD {
		isUSDInt = 1
	}
	_, err := db.Exec(`INSERT INTO movimientos
		(banco, source, fecha, monto, descripcion, is_usd, cuotas, raw, origen, fecha_carga)
		VALUES ('bchile', ?, ?, ?, ?, ?, ?, '{}', 'test', '2026-05-15T00:00:00Z')`,
		source, fecha, monto, desc, isUSDInt, cuotas)
	if err != nil {
		t.Fatalf("insert: %v", err)
	}
}

func periodoMayo2026() presupuesto.PeriodoPresupuestario {
	inicio := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	fin := inicio.AddDate(0, 1, 0).Add(-time.Nanosecond)
	return presupuesto.PeriodoPresupuestario{Inicio: inicio, Fin: fin}
}

func TestAdapter_ObtenerSueldoBase_EncuentraSueldoDelPeriodo(t *testing.T) {
	db := setupAdapterDB(t)
	insertarMov(t, db, "2026-05-15", 1500000, "PAGO DE SUELDOS Mayo", "cta_corriente", false, "")
	insertarMov(t, db, "2026-04-15", 1400000, "PAGO DE SUELDOS Abril", "cta_corriente", false, "")

	a := NewAdapter(db, "", "", fakeResolvedor{tasaUSD: 950, diaCorteCC: 22, porcGastos: 0.5})

	sueldo, err := a.ObtenerSueldoBase(periodoMayo2026())
	if err != nil {
		t.Fatalf("ObtenerSueldoBase: %v", err)
	}
	if sueldo != 1500000 {
		t.Errorf("esperaba 1500000 (mayo), obtuve %v", sueldo)
	}
}

func TestAdapter_ObtenerSueldoBase_NoEncontradoEsError(t *testing.T) {
	db := setupAdapterDB(t)
	a := NewAdapter(db, "", "", fakeResolvedor{tasaUSD: 950, diaCorteCC: 22, porcGastos: 0.5})
	if _, err := a.ObtenerSueldoBase(periodoMayo2026()); err == nil {
		t.Error("esperaba error por sueldo no encontrado")
	}
}

func TestAdapter_ObtenerGastosValidos_FiltraAbonosYIgnorables(t *testing.T) {
	db := setupAdapterDB(t)
	insertarMov(t, db, "2026-05-10", -10000, "STARBUCKS", "cta_corriente", false, "")
	insertarMov(t, db, "2026-05-15", 1500000, "PAGO DE SUELDOS", "cta_corriente", false, "")         // abono, skip
	insertarMov(t, db, "2026-05-12", -50000, "PAGO TARJETA DE CREDITO", "cta_corriente", false, "") // ignorable, skip
	insertarMov(t, db, "2026-05-14", -30000, "Fintual ahorro", "cta_corriente", false, "")          // ignorable, skip

	a := NewAdapter(db, "", "", fakeResolvedor{tasaUSD: 950, diaCorteCC: 22, porcGastos: 0.5})
	gastos, err := a.ObtenerGastosValidos(periodoMayo2026())
	if err != nil {
		t.Fatalf("ObtenerGastosValidos: %v", err)
	}
	if len(gastos) != 1 {
		t.Fatalf("esperaba 1 gasto válido, obtuve %d", len(gastos))
	}
	if gastos[0].Descripcion != "STARBUCKS" {
		t.Errorf("quedó el gasto equivocado: %q", gastos[0].Descripcion)
	}
}

func TestAdapter_ObtenerGastosValidos_DetectaCreditoPorSource(t *testing.T) {
	db := setupAdapterDB(t)
	insertarMov(t, db, "2026-05-10", -10000, "DEBITO", "cta_corriente", false, "")
	insertarMov(t, db, "2026-05-10", -20000, "CREDITO BCHL", "tc_nacional", false, "01/01")
	insertarMov(t, db, "2026-05-10", -5000, "USD COMPRA", "tc_internacional", true, "")

	a := NewAdapter(db, "", "", fakeResolvedor{tasaUSD: 950, diaCorteCC: 22, porcGastos: 0.5})
	gastos, _ := a.ObtenerGastosValidos(periodoMayo2026())

	if len(gastos) != 3 {
		t.Fatalf("esperaba 3, obtuve %d", len(gastos))
	}

	porDesc := map[string]presupuesto.Gasto{}
	for _, g := range gastos {
		porDesc[g.Descripcion] = g
	}

	if porDesc["DEBITO"].PoliticaCorte.Tipo != presupuesto.Debito {
		t.Error("DEBITO no quedó como débito")
	}
	if porDesc["CREDITO BCHL"].PoliticaCorte.Tipo != presupuesto.Credito {
		t.Error("CREDITO BCHL no quedó como crédito")
	}
	if porDesc["USD COMPRA"].PoliticaCorte.Tipo != presupuesto.Credito {
		t.Error("tc_internacional no quedó como crédito")
	}
	if porDesc["DEBITO"].PoliticaCorte.DiaDeCorte != 0 {
		t.Error("débito no debería tener día de corte")
	}
	if porDesc["CREDITO BCHL"].PoliticaCorte.DiaDeCorte != 22 {
		t.Errorf("crédito día de corte: esperaba 22, obtuve %d", porDesc["CREDITO BCHL"].PoliticaCorte.DiaDeCorte)
	}
}

func TestAdapter_ObtenerGastosValidos_NormalizaUSD(t *testing.T) {
	db := setupAdapterDB(t)
	insertarMov(t, db, "2026-05-10", -10.5, "USD COMPRA", "tc_internacional", true, "")

	a := NewAdapter(db, "", "", fakeResolvedor{tasaUSD: 950, diaCorteCC: 22, porcGastos: 0.5})
	gastos, _ := a.ObtenerGastosValidos(periodoMayo2026())
	if len(gastos) != 1 {
		t.Fatalf("esperaba 1, obtuve %d", len(gastos))
	}
	// |−10.5 * 950| = 9975
	if gastos[0].MontoImputado != 9975 {
		t.Errorf("USD normalizado: esperaba 9975, obtuve %v", gastos[0].MontoImputado)
	}
}

func TestAdapter_ObtenerGastosValidos_AplicaOverride(t *testing.T) {
	db := setupAdapterDB(t)
	insertarMov(t, db, "2026-05-10", -10000, "Restaurante", "cta_corriente", false, "")

	tmpDir := t.TempDir()
	overridesPath := tmpDir + "/divisiones.json"
	overridesJSON := `[{"fecha":"2026-05-10","montoOriginal":-10000,"descripcion":"Restaurante","miParte":-4000}]`
	if err := os.WriteFile(overridesPath, []byte(overridesJSON), 0644); err != nil {
		t.Fatalf("escribiendo overrides: %v", err)
	}

	a := NewAdapter(db, overridesPath, "", fakeResolvedor{tasaUSD: 950, diaCorteCC: 22, porcGastos: 0.5})
	gastos, _ := a.ObtenerGastosValidos(periodoMayo2026())
	if gastos[0].MontoImputado != 4000 {
		t.Errorf("con override: esperaba 4000, obtuve %v", gastos[0].MontoImputado)
	}
}

func TestAdapter_ObtenerMovimientos_FiltraPositivosYAplicaMiParte(t *testing.T) {
	db := setupAdapterDB(t)
	insertarMov(t, db, "2026-05-10", -10000, "Restaurante", "cta_corriente", false, "")
	insertarMov(t, db, "2026-05-15", 1500000, "SUELDO", "cta_corriente", false, "") // positivo, skip

	tmpDir := t.TempDir()
	overridesPath := tmpDir + "/divisiones.json"
	overridesJSON := `[{"fecha":"2026-05-10","montoOriginal":-10000,"descripcion":"Restaurante","miParte":-4000}]`
	if err := os.WriteFile(overridesPath, []byte(overridesJSON), 0644); err != nil {
		t.Fatalf("escribiendo overrides: %v", err)
	}

	a := NewAdapter(db, overridesPath, "", fakeResolvedor{tasaUSD: 950, diaCorteCC: 22, porcGastos: 0.5})
	movs, err := a.ObtenerMovimientos()
	if err != nil {
		t.Fatalf("ObtenerMovimientos: %v", err)
	}
	if len(movs) != 1 {
		t.Fatalf("esperaba 1 (positivos filtrados), obtuve %d", len(movs))
	}
	if movs[0].MiParte == nil || *movs[0].MiParte != -4000 {
		t.Errorf("MiParte no aplicado: %v", movs[0].MiParte)
	}
}
