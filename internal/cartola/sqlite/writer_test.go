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

func TestInsertarConDedup_CompraEnCuotas_SoloSeImportaUnaVez(t *testing.T) {
	w := setupDB(t)

	// Misma compra SKY AIRLINE en 3 cuotas: aparece como 00/03 en la cartola
	// de enero y como 01/03, 02/03, 03/03 en febrero/marzo/abril, todas con
	// el mismo monto total y misma fecha de compra.
	batch := []ingest.MovimientoBruto{
		movCuotas("2025-01-07", -36124, "SKY AIRLINE", "00/03"),
		movCuotas("2025-01-07", -36124, "SKY AIRLINE", "01/03"),
		movCuotas("2025-01-07", -36124, "SKY AIRLINE", "02/03"),
		movCuotas("2025-01-07", -36124, "SKY AIRLINE", "03/03"),
	}
	n, err := w.InsertarConDedup(batch)
	if err != nil {
		t.Fatalf("InsertarConDedup: %v", err)
	}
	if n != 1 {
		t.Errorf("esperaba 1 inserción (compra única), obtuve %d", n)
	}

	// El representante elegido debería ser el "00/03" (compra original).
	var cuotas string
	w.db.QueryRow(`SELECT cuotas FROM movimientos WHERE descripcion = 'SKY AIRLINE'`).Scan(&cuotas)
	if cuotas != "00/03" {
		t.Errorf("representante: esperaba 00/03, obtuve %q", cuotas)
	}
}

func TestInsertarConDedup_CompraEnCuotas_SinCero_EligeMenorM(t *testing.T) {
	w := setupDB(t)

	// El banco a veces NO emite la fila "00/N" (si la compra se factura el
	// mismo mes que se hizo). En ese caso debe ganar la cuota con menor M.
	batch := []ingest.MovimientoBruto{
		movCuotas("2025-02-18", -31313, "BAR ALONSO", "03/03"),
		movCuotas("2025-02-18", -31313, "BAR ALONSO", "01/03"),
		movCuotas("2025-02-18", -31313, "BAR ALONSO", "02/03"),
	}
	n, err := w.InsertarConDedup(batch)
	if err != nil {
		t.Fatalf("InsertarConDedup: %v", err)
	}
	if n != 1 {
		t.Errorf("esperaba 1 inserción, obtuve %d", n)
	}

	var cuotas string
	w.db.QueryRow(`SELECT cuotas FROM movimientos WHERE descripcion = 'BAR ALONSO'`).Scan(&cuotas)
	if cuotas != "01/03" {
		t.Errorf("sin 00/N debe ganar la M menor (01/03), obtuve %q", cuotas)
	}
}

func TestInsertarConDedup_CompraEnCuotas_EsIdempotente(t *testing.T) {
	w := setupDB(t)

	batch := []ingest.MovimientoBruto{
		movCuotas("2025-01-07", -36124, "SKY AIRLINE", "00/03"),
		movCuotas("2025-01-07", -36124, "SKY AIRLINE", "01/03"),
		movCuotas("2025-01-07", -36124, "SKY AIRLINE", "02/03"),
	}
	if n, _ := w.InsertarConDedup(batch); n != 1 {
		t.Fatalf("primera corrida: esperaba 1, obtuve %d", n)
	}
	if n, _ := w.InsertarConDedup(batch); n != 0 {
		t.Errorf("segunda corrida: esperaba 0, obtuve %d", n)
	}
}
