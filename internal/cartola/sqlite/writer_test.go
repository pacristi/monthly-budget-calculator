package sqlite

import (
	"testing"
	"time"

	"github.com/pierocristi/monthly-budget-calculator/internal/cartola/ingest"
)

func setupDB(t *testing.T) *Writer {
	t.Helper()
	db := openMemDB(t)
	if err := Up(db); err != nil {
		t.Fatalf("Up: %v", err)
	}
	return NewWriter(db, "test")
}

func mov(fechaISO string, monto float64, desc string) ingest.MovimientoBruto {
	f, _ := time.Parse("2006-01-02", fechaISO)
	return ingest.MovimientoBruto{
		Banco:       "bchile",
		Source:      "cta_corriente",
		Fecha:       f,
		Monto:       monto,
		Descripcion: desc,
	}
}

func movCuotas(fechaISO string, monto float64, desc, cuotas string) ingest.MovimientoBruto {
	m := mov(fechaISO, monto, desc)
	m.Source = "tc_nacional"
	m.Cuotas = cuotas
	return m
}

func TestInsertarConDedup_InsertSimple(t *testing.T) {
	w := setupDB(t)

	insertados, err := w.InsertarConDedup([]ingest.MovimientoBruto{
		mov("2025-05-15", -1000, "Café"),
		mov("2025-05-15", -2000, "Almuerzo"),
	})
	if err != nil {
		t.Fatalf("InsertarConDedup: %v", err)
	}
	if insertados != 2 {
		t.Errorf("esperaba 2 insertados, obtuve %d", insertados)
	}
}

func TestInsertarConDedup_NoDuplicaEnDosCorridas(t *testing.T) {
	w := setupDB(t)

	batch := []ingest.MovimientoBruto{
		mov("2025-05-15", -1000, "Café"),
		mov("2025-05-15", -2000, "Almuerzo"),
	}

	if n, err := w.InsertarConDedup(batch); err != nil || n != 2 {
		t.Fatalf("primera corrida: n=%d err=%v (esperaba 2, nil)", n, err)
	}
	if n, err := w.InsertarConDedup(batch); err != nil || n != 0 {
		t.Fatalf("segunda corrida: n=%d err=%v (esperaba 0, nil)", n, err)
	}
}

func TestInsertarConDedup_DobleCafe_InsertaDosFilas(t *testing.T) {
	w := setupDB(t)

	// Mismo (fecha, monto, descripcion) repetido — dos cafés idénticos.
	batch := []ingest.MovimientoBruto{
		mov("2025-05-15", -3500, "Starbucks"),
		mov("2025-05-15", -3500, "Starbucks"),
	}

	n, err := w.InsertarConDedup(batch)
	if err != nil {
		t.Fatalf("InsertarConDedup: %v", err)
	}
	if n != 2 {
		t.Errorf("esperaba 2 filas insertadas (doble café legítimo), obtuve %d", n)
	}

	// Si se vuelve a correr con el mismo batch, no inserta nada nuevo.
	n2, err := w.InsertarConDedup(batch)
	if err != nil {
		t.Fatalf("segunda corrida: %v", err)
	}
	if n2 != 0 {
		t.Errorf("segunda corrida no debería insertar; obtuve %d", n2)
	}
}

func TestInsertarConDedup_DeltaParcial(t *testing.T) {
	w := setupDB(t)

	// Día 1: el scraper ve 1 café.
	if n, _ := w.InsertarConDedup([]ingest.MovimientoBruto{
		mov("2025-05-15", -3500, "Starbucks"),
	}); n != 1 {
		t.Fatalf("setup día 1: esperaba 1 insert, obtuve %d", n)
	}

	// Día 2: el scraper ya ve 2 cafés. Solo debe insertar 1 (el delta).
	n, err := w.InsertarConDedup([]ingest.MovimientoBruto{
		mov("2025-05-15", -3500, "Starbucks"),
		mov("2025-05-15", -3500, "Starbucks"),
	})
	if err != nil {
		t.Fatalf("día 2: %v", err)
	}
	if n != 1 {
		t.Errorf("delta parcial: esperaba 1 insert nuevo, obtuve %d", n)
	}

	// Verificar total en BD.
	var total int
	if err := w.db.QueryRow(`SELECT COUNT(*) FROM movimientos
		WHERE descripcion = 'Starbucks'`).Scan(&total); err != nil {
		t.Fatalf("query total: %v", err)
	}
	if total != 2 {
		t.Errorf("total en BD debería ser 2, es %d", total)
	}
}

func TestInsertarConDedup_CompraEnCuotas_GuardaMontoTotal(t *testing.T) {
	w := setupDB(t)

	// SKY AIRLINE en 3 cuotas. En el xlsx cada fila tiene el monto de la
	// CUOTA (no del total). Banco repite la misma fecha de origen en las 3.
	// Esperamos: 1 fila guardada con monto = 3 × 36124 = 108372.
	batch := []ingest.MovimientoBruto{
		movCuotas("2025-01-07", -36124, "SKY AIRLINE", "01/03"),
		movCuotas("2025-01-07", -36124, "SKY AIRLINE", "02/03"),
		movCuotas("2025-01-07", -36124, "SKY AIRLINE", "03/03"),
	}
	n, err := w.InsertarConDedup(batch)
	if err != nil {
		t.Fatalf("InsertarConDedup: %v", err)
	}
	if n != 1 {
		t.Errorf("esperaba 1 inserción, obtuve %d", n)
	}

	var monto float64
	var cuotas string
	w.db.QueryRow(`SELECT monto, cuotas FROM movimientos WHERE descripcion = 'SKY AIRLINE'`).
		Scan(&monto, &cuotas)
	if monto != -108372 {
		t.Errorf("monto guardado: esperaba -108372 (total), obtuve %v", monto)
	}
	if cuotas != "00/03" {
		t.Errorf("cuotas guardadas: esperaba 00/03 (señalando monto total), obtuve %q", cuotas)
	}
}

func TestInsertarConDedup_CompraEnCuotas_AjusteRedondeo(t *testing.T) {
	w := setupDB(t)

	// BAR ALONSO con ajuste de redondeo: 31313 + 31313 + 31314 = 93940.
	// Cuando tenemos las N cuotas, el total es la suma exacta.
	batch := []ingest.MovimientoBruto{
		movCuotas("2025-02-18", -31313, "BAR ALONSO", "01/03"),
		movCuotas("2025-02-18", -31313, "BAR ALONSO", "02/03"),
		movCuotas("2025-02-18", -31314, "BAR ALONSO", "03/03"),
	}
	if _, err := w.InsertarConDedup(batch); err != nil {
		t.Fatalf("InsertarConDedup: %v", err)
	}

	var monto float64
	w.db.QueryRow(`SELECT monto FROM movimientos WHERE descripcion = 'BAR ALONSO'`).Scan(&monto)
	if monto != -93940 {
		t.Errorf("monto exacto: esperaba -93940, obtuve %v", monto)
	}
}

func TestInsertarConDedup_CompraEnCuotas_SoloPrimeraCuotaEstima(t *testing.T) {
	w := setupDB(t)

	// Carga parcial: sólo conocemos la cuota 1/12. Estimamos total = cuota×N.
	batch := []ingest.MovimientoBruto{
		movCuotas("2025-01-15", -10000, "ALGO", "01/12"),
	}
	if _, err := w.InsertarConDedup(batch); err != nil {
		t.Fatalf("InsertarConDedup: %v", err)
	}

	var monto float64
	w.db.QueryRow(`SELECT monto FROM movimientos WHERE descripcion = 'ALGO'`).Scan(&monto)
	if monto != -120000 {
		t.Errorf("estimación con 1 cuota: esperaba -120000, obtuve %v", monto)
	}
}

func TestInsertarConDedup_CompraEnCuotas_SoloInformativaNoInserta(t *testing.T) {
	w := setupDB(t)

	// Sólo viene la fila informativa 00/N (la compra aún no se facturó).
	// No debemos insertar: el motor proyectaría cuotas que no existen.
	batch := []ingest.MovimientoBruto{
		movCuotas("2025-01-07", -36124, "SKY AIRLINE", "00/03"),
	}
	n, err := w.InsertarConDedup(batch)
	if err != nil {
		t.Fatalf("InsertarConDedup: %v", err)
	}
	if n != 0 {
		t.Errorf("esperaba 0 inserciones (sólo informativa), obtuve %d", n)
	}
}

func TestInsertarConDedup_CompraEnCuotas_EsIdempotente(t *testing.T) {
	w := setupDB(t)

	batch := []ingest.MovimientoBruto{
		movCuotas("2025-01-07", -36124, "SKY AIRLINE", "01/03"),
		movCuotas("2025-01-07", -36124, "SKY AIRLINE", "02/03"),
		movCuotas("2025-01-07", -36124, "SKY AIRLINE", "03/03"),
	}
	if n, _ := w.InsertarConDedup(batch); n != 1 {
		t.Fatalf("primera corrida: esperaba 1, obtuve %d", n)
	}
	if n, _ := w.InsertarConDedup(batch); n != 0 {
		t.Errorf("segunda corrida: esperaba 0, obtuve %d", n)
	}
}
