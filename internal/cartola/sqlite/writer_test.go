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
