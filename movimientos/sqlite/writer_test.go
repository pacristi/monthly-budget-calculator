package sqlite

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"presupuesto/movimientos"
)

func setupDB(t *testing.T) *Writer {
	t.Helper()
	db := openMemDB(t)
	if err := Up(db); err != nil {
		t.Fatalf("Up: %v", err)
	}
	return NewWriter(db, "test")
}

func mov(fechaISO string, monto float64, desc string) movimientos.MovimientoBruto {
	f, _ := time.Parse("2006-01-02", fechaISO)
	return movimientos.MovimientoBruto{
		Banco:           "bchile",
		Source:          "cta_corriente",
		Fecha:           f,
		Monto:           monto,
		Descripcion:     desc,
		Instrumento:     movimientos.InstrumentoCuentaCorriente,
		Moneda:          movimientos.MonedaCLP,
		MontoRepresenta: movimientos.MontoRepresentaTotal,
		CuotasTotales:   1,
	}
}

// movUnbilled construye un movimiento provisorio (unbilled) de tarjeta de
// crédito, tal como lo entrega el scraper de OBCL antes de facturarse.
func movUnbilled(fechaISO string, monto float64, desc string) movimientos.MovimientoBruto {
	m := mov(fechaISO, monto, desc)
	m.Source = "credit_card_unbilled"
	m.Instrumento = movimientos.InstrumentoTarjetaCredito
	return m
}

func movCuotas(fechaISO string, monto float64, desc, cuotas string) movimientos.MovimientoBruto {
	m := mov(fechaISO, monto, desc)
	m.Source = "tc_nacional"
	m.Instrumento = movimientos.InstrumentoTarjetaCredito
	m.Cuotas = cuotas
	var actual, total int
	if _, err := fmt.Sscanf(cuotas, "%d/%d", &actual, &total); err != nil {
		panic(err)
	}
	m.CuotaActual = actual
	m.CuotasTotales = total
	if total > 1 {
		m.MontoRepresenta = movimientos.MontoRepresentaCuota
	} else {
		m.MontoRepresenta = movimientos.MontoRepresentaTotal
	}
	return m
}

func TestInsertarConDedup_InsertSimple(t *testing.T) {
	w := setupDB(t)

	insertados, err := w.InsertarConDedup([]movimientos.MovimientoBruto{
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

func TestInsertarConDedup_PersisteFactsCanonicos(t *testing.T) {
	w := setupDB(t)
	f, _ := time.Parse("2006-01-02", "2025-05-15")
	m := movimientos.MovimientoBruto{
		Banco: "bchile", Source: "tc_nacional", Fecha: f, Monto: -1000,
		Descripcion: "Café", Instrumento: movimientos.InstrumentoTarjetaCredito,
		Moneda: movimientos.MonedaUSD, CuotasTotales: 1, MontoRepresenta: movimientos.MontoRepresentaTotal,
	}

	if n, err := w.InsertarConDedup([]movimientos.MovimientoBruto{m}); err != nil || n != 1 {
		t.Fatalf("InsertarConDedup: n=%d err=%v", n, err)
	}

	var instrumento, moneda string
	var cuotasTotales int
	if err := w.db.QueryRow(`SELECT instrumento, moneda, cuotas_totales FROM movimientos WHERE descripcion = 'Café'`).
		Scan(&instrumento, &moneda, &cuotasTotales); err != nil {
		t.Fatalf("query facts: %v", err)
	}
	if instrumento != string(movimientos.InstrumentoTarjetaCredito) || moneda != string(movimientos.MonedaUSD) || cuotasTotales != 1 {
		t.Fatalf("facts = (%q, %q, %d)", instrumento, moneda, cuotasTotales)
	}
}

func TestInsertarConDedup_FactsCanonicosFaltantesRetornaError(t *testing.T) {
	w := setupDB(t)
	f, _ := time.Parse("2006-01-02", "2025-05-15")
	m := movimientos.MovimientoBruto{
		Banco: "bchile", Source: "cta_corriente", Fecha: f, Monto: -1000,
		Descripcion: "Café", Moneda: movimientos.MonedaCLP, CuotasTotales: 1,
		MontoRepresenta: movimientos.MontoRepresentaTotal,
	}

	n, err := w.InsertarConDedup([]movimientos.MovimientoBruto{m})
	if err == nil {
		t.Fatal("esperaba error por Instrumento faltante")
	}
	if n != 0 {
		t.Errorf("no debería insertar con facts canónicos incompletos, obtuvo %d", n)
	}
	if !strings.Contains(err.Error(), "sin Instrumento") {
		t.Errorf("error = %v, esperaba contexto de Instrumento faltante", err)
	}
}

func TestInsertarConDedup_NoDuplicaEnDosCorridas(t *testing.T) {
	w := setupDB(t)

	batch := []movimientos.MovimientoBruto{
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
	batch := []movimientos.MovimientoBruto{
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
	if n, _ := w.InsertarConDedup([]movimientos.MovimientoBruto{
		mov("2025-05-15", -3500, "Starbucks"),
	}); n != 1 {
		t.Fatalf("setup día 1: esperaba 1 insert, obtuve %d", n)
	}

	// Día 2: el scraper ya ve 2 cafés. Solo debe insertar 1 (el delta).
	n, err := w.InsertarConDedup([]movimientos.MovimientoBruto{
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
	batch := []movimientos.MovimientoBruto{
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
	batch := []movimientos.MovimientoBruto{
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
	batch := []movimientos.MovimientoBruto{
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
	batch := []movimientos.MovimientoBruto{
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

	batch := []movimientos.MovimientoBruto{
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

func TestInsertarConDedup_CompraEnCuotas_DedupContraCuotasLegacyPersistidas(t *testing.T) {
	w := setupDB(t)
	f, _ := time.Parse("2006-01-02", "2025-01-07")

	legacy := movCuotas("2025-01-07", -108372, "SKY AIRLINE", "01/03")
	if err := w.insertOne(w.db, legacy, f.Format(time.RFC3339)); err != nil {
		t.Fatalf("insert legacy: %v", err)
	}

	n, err := w.InsertarConDedup([]movimientos.MovimientoBruto{
		movCuotas("2025-01-07", -36124, "SKY AIRLINE", "01/03"),
		movCuotas("2025-01-07", -36124, "SKY AIRLINE", "02/03"),
		movCuotas("2025-01-07", -36124, "SKY AIRLINE", "03/03"),
	})
	if err != nil {
		t.Fatalf("InsertarConDedup: %v", err)
	}
	if n != 0 {
		t.Errorf("esperaba dedup contra fila legacy persistida, obtuve %d inserts", n)
	}
}

func TestInsertarConDedup_CompraEnCuotas_FactsCanonicosFaltantesRetornaError(t *testing.T) {
	w := setupDB(t)
	f, _ := time.Parse("2006-01-02", "2025-01-07")
	m := movimientos.MovimientoBruto{
		Banco: "bchile", Source: "tc_nacional", Fecha: f, Monto: -36124,
		Descripcion: "SKY AIRLINE", Cuotas: "01/03", CuotaActual: 1, CuotasTotales: 3,
	}

	n, err := w.InsertarConDedup([]movimientos.MovimientoBruto{m})
	if err == nil {
		t.Fatal("esperaba error por MontoRepresenta faltante")
	}
	if n != 0 {
		t.Errorf("no debería insertar con facts canónicos incompletos, obtuvo %d", n)
	}
	if !strings.Contains(err.Error(), "sin MontoRepresenta") {
		t.Errorf("error = %v, esperaba contexto de MontoRepresenta faltante", err)
	}
}

func TestInsertarConDedup_DedupCrossSource_TCNacionalVsCreditCard(t *testing.T) {
	w := setupDB(t)

	// El xlsx mensual entrega tc_nacional; el scraper entrega
	// credit_card_billed. Misma compra, misma descripción, distinto source:
	// debe contar como una.
	f, _ := time.Parse("2006-01-02", "2026-04-15")
	fromXlsx := movimientos.MovimientoBruto{
		Banco: "bchile", Source: "tc_nacional", Fecha: f, Monto: -20260,
		Descripcion: "DL RAPPI CHILE RAPP LAS CONDES", Cuotas: "01/01",
		Instrumento: movimientos.InstrumentoTarjetaCredito, Moneda: movimientos.MonedaCLP,
		CuotaActual: 1, CuotasTotales: 1, MontoRepresenta: movimientos.MontoRepresentaTotal,
	}
	fromScraper := movimientos.MovimientoBruto{
		Banco: "bchile", Source: "credit_card_billed", Fecha: f, Monto: -20260,
		Descripcion: "DL RAPPI CHILE RAPP LAS CONDES", Cuotas: "",
		Instrumento: movimientos.InstrumentoTarjetaCredito, Moneda: movimientos.MonedaCLP,
		CuotasTotales: 1, MontoRepresenta: movimientos.MontoRepresentaTotal,
	}

	if n, _ := w.InsertarConDedup([]movimientos.MovimientoBruto{fromXlsx}); n != 1 {
		t.Fatalf("inserta xlsx: esperaba 1, obtuve %d", n)
	}
	if n, _ := w.InsertarConDedup([]movimientos.MovimientoBruto{fromScraper}); n != 0 {
		t.Errorf("scraper repetido: esperaba 0 nuevas, obtuve %d", n)
	}
}

func TestInsertarConDedup_DedupCrossSource_CtaCorrienteVsAccountConCasing(t *testing.T) {
	w := setupDB(t)

	// xlsx en mayúsculas, scraper en Title Case.
	f, _ := time.Parse("2006-01-02", "2026-04-20")
	upper := movimientos.MovimientoBruto{
		Banco: "bchile", Source: "cta_corriente", Fecha: f, Monto: -31800,
		Descripcion: "TRASPASO A:Bruno Cristi",
		Instrumento: movimientos.InstrumentoCuentaCorriente, Moneda: movimientos.MonedaCLP,
		CuotasTotales: 1, MontoRepresenta: movimientos.MontoRepresentaTotal,
	}
	titleCase := movimientos.MovimientoBruto{
		Banco: "bchile", Source: "account", Fecha: f, Monto: -31800,
		Descripcion: "Traspaso A:Bruno Cristi",
		Instrumento: movimientos.InstrumentoCuentaCorriente, Moneda: movimientos.MonedaCLP,
		CuotasTotales: 1, MontoRepresenta: movimientos.MontoRepresentaTotal,
	}

	if n, _ := w.InsertarConDedup([]movimientos.MovimientoBruto{upper}); n != 1 {
		t.Fatalf("inserta xlsx: esperaba 1, obtuve %d", n)
	}
	if n, _ := w.InsertarConDedup([]movimientos.MovimientoBruto{titleCase}); n != 0 {
		t.Errorf("scraper con casing distinto: esperaba 0 nuevas, obtuve %d", n)
	}
}

func TestInsertarConDedup_CompraEnCuotas_Scraper_NoMultiplicaMonto(t *testing.T) {
	// El scraper de obchile entrega el monto TOTAL en cada fila de cuotas
	// (al revés del xlsx). El writer NO debe multiplicarlo.
	db := openMemDB(t)
	if err := Up(db); err != nil {
		t.Fatalf("Up: %v", err)
	}
	w := NewWriter(db, "obchile")

	f, _ := time.Parse("2006-01-02", "2025-01-07")
	scraper := movimientos.MovimientoBruto{
		Banco: "bchile", Source: "credit_card_billed", Fecha: f,
		Monto: -108372, Descripcion: "SKY AIRLINE", Cuotas: "1/3",
		Instrumento: movimientos.InstrumentoTarjetaCredito, Moneda: movimientos.MonedaCLP,
		CuotaActual: 1, CuotasTotales: 3, MontoRepresenta: movimientos.MontoRepresentaTotal,
	}
	n, err := w.InsertarConDedup([]movimientos.MovimientoBruto{scraper})
	if err != nil {
		t.Fatalf("InsertarConDedup: %v", err)
	}
	if n != 1 {
		t.Fatalf("esperaba 1 inserción, obtuve %d", n)
	}

	var monto float64
	var cuotas string
	db.QueryRow(`SELECT monto, cuotas FROM movimientos WHERE descripcion='SKY AIRLINE'`).
		Scan(&monto, &cuotas)
	if monto != -108372 {
		t.Errorf("monto del scraper no debe multiplicarse: esperaba -108372, obtuve %v", monto)
	}
	if cuotas != "00/03" {
		t.Errorf("cuotas: esperaba 00/03, obtuve %q", cuotas)
	}
}

func TestInsertarConDedup_DedupCrossSource_CompraEnCuotas(t *testing.T) {
	w := setupDB(t)

	// El xlsx trae una compra en cuotas como tc_nacional. El scraper la
	// puede traer como credit_card_billed (sin cuotas o con formato propio).
	// Acá modelamos el caso conservador: la fila del scraper viene como
	// "01/01" pero el monto y la descripción coinciden con UNA de las
	// cuotas guardadas como compra total — no debería duplicar.
	f, _ := time.Parse("2006-01-02", "2025-01-07")
	xlsxCuotas := []movimientos.MovimientoBruto{
		{Banco: "bchile", Source: "tc_nacional", Fecha: f, Monto: -36124,
			Descripcion: "SKY AIRLINE", Cuotas: "01/03", Instrumento: movimientos.InstrumentoTarjetaCredito, Moneda: movimientos.MonedaCLP,
			CuotaActual: 1, CuotasTotales: 3, MontoRepresenta: movimientos.MontoRepresentaCuota},
		{Banco: "bchile", Source: "tc_nacional", Fecha: f, Monto: -36124,
			Descripcion: "SKY AIRLINE", Cuotas: "02/03", Instrumento: movimientos.InstrumentoTarjetaCredito, Moneda: movimientos.MonedaCLP,
			CuotaActual: 2, CuotasTotales: 3, MontoRepresenta: movimientos.MontoRepresentaCuota},
		{Banco: "bchile", Source: "tc_nacional", Fecha: f, Monto: -36124,
			Descripcion: "SKY AIRLINE", Cuotas: "03/03", Instrumento: movimientos.InstrumentoTarjetaCredito, Moneda: movimientos.MonedaCLP,
			CuotaActual: 3, CuotasTotales: 3, MontoRepresenta: movimientos.MontoRepresentaCuota},
	}
	if n, _ := w.InsertarConDedup(xlsxCuotas); n != 1 {
		t.Fatalf("inserta cuotas: esperaba 1 (total), obtuve %d", n)
	}

	// El scraper trae la misma compra como simple con monto total. La
	// llave (banco, fecha, monto, descripcion_norm) hace match con la
	// fila guardada (que tiene monto total 108372). No debe duplicar.
	totalScraper := movimientos.MovimientoBruto{
		Banco: "bchile", Source: "credit_card_billed", Fecha: f, Monto: -108372,
		Descripcion: "Sky Airline", // distinto casing
		Instrumento: movimientos.InstrumentoTarjetaCredito, Moneda: movimientos.MonedaCLP,
		CuotasTotales: 1, MontoRepresenta: movimientos.MontoRepresentaTotal,
	}
	if n, _ := w.InsertarConDedup([]movimientos.MovimientoBruto{totalScraper}); n != 0 {
		t.Errorf("scraper con monto total: esperaba 0 nuevas, obtuve %d", n)
	}
}

// --- Paso 1: unbilled a sqlite ---

func TestInsertarConDedup_UnbilledSeInsertan(t *testing.T) {
	w := setupDB(t)

	n, err := w.InsertarConDedup([]movimientos.MovimientoBruto{
		movUnbilled("2025-05-20", -5000, "Starbucks Provisorio"),
	})
	if err != nil {
		t.Fatalf("InsertarConDedup: %v", err)
	}
	if n != 1 {
		t.Errorf("esperaba 1 insertado, obtuve %d", n)
	}

	var total int
	if err := w.db.QueryRow(`SELECT COUNT(*) FROM movimientos WHERE source = 'credit_card_unbilled'`).Scan(&total); err != nil {
		t.Fatalf("count: %v", err)
	}
	if total != 1 {
		t.Errorf("esperaba 1 fila unbilled en BD, obtuve %d", total)
	}
}

func TestInsertarConDedup_SegundoSnapshotUnbilled_ReemplazaAlAnterior(t *testing.T) {
	w := setupDB(t)

	// Primer snapshot: el scraper ve 2 unbilled.
	if n, err := w.InsertarConDedup([]movimientos.MovimientoBruto{
		movUnbilled("2025-05-20", -5000, "Starbucks Provisorio"),
		movUnbilled("2025-05-21", -1200, "Uber Provisorio"),
	}); err != nil || n != 2 {
		t.Fatalf("primer snapshot: n=%d err=%v (esperaba 2, nil)", n, err)
	}

	// Segundo snapshot: distinto (el "Uber" desaparece, aparece un "Netflix"
	// nuevo, "Starbucks" ya no está). Simula el drift típico de unbilled.
	n, err := w.InsertarConDedup([]movimientos.MovimientoBruto{
		movUnbilled("2025-05-22", -8900, "Netflix Provisorio"),
	})
	if err != nil {
		t.Fatalf("segundo snapshot: %v", err)
	}
	if n != 1 {
		t.Errorf("segundo snapshot: esperaba 1 insertado, obtuve %d", n)
	}

	var total int
	if err := w.db.QueryRow(`SELECT COUNT(*) FROM movimientos WHERE source = 'credit_card_unbilled'`).Scan(&total); err != nil {
		t.Fatalf("count: %v", err)
	}
	if total != 1 {
		t.Errorf("esperaba que el snapshot anterior fuera reemplazado por completo; total unbilled = %d", total)
	}

	var desc string
	if err := w.db.QueryRow(`SELECT descripcion FROM movimientos WHERE source = 'credit_card_unbilled'`).Scan(&desc); err != nil {
		t.Fatalf("query descripcion: %v", err)
	}
	if desc != "Netflix Provisorio" {
		t.Errorf("esperaba que sobreviviera el unbilled del snapshot nuevo, obtuve %q", desc)
	}
}

func TestInsertarConDedup_DeleteUnbilled_NoAfectaLiquidados(t *testing.T) {
	w := setupDB(t)

	// Un liquidado (billed) ya persistido en una corrida anterior.
	if n, err := w.InsertarConDedup([]movimientos.MovimientoBruto{
		mov("2025-05-10", -20000, "Restorant Facturado"),
	}); err != nil || n != 1 {
		t.Fatalf("setup billed: n=%d err=%v", n, err)
	}

	// Nueva corrida: llega un unbilled distinto y el mismo billed de antes
	// (el scraper lo sigue reportando cada corrida). El billed no debe
	// duplicarse por el dedup existente; el DELETE de unbilled no debe
	// tocarlo.
	n, err := w.InsertarConDedup([]movimientos.MovimientoBruto{
		mov("2025-05-10", -20000, "Restorant Facturado"),
		movUnbilled("2025-05-20", -5000, "Starbucks Provisorio"),
	})
	if err != nil {
		t.Fatalf("InsertarConDedup: %v", err)
	}
	if n != 1 {
		t.Errorf("esperaba 1 insertado (solo el unbilled nuevo, billed deduplicado), obtuve %d", n)
	}

	var totalBilled int
	if err := w.db.QueryRow(`SELECT COUNT(*) FROM movimientos WHERE descripcion = 'Restorant Facturado'`).Scan(&totalBilled); err != nil {
		t.Fatalf("count billed: %v", err)
	}
	if totalBilled != 1 {
		t.Errorf("el liquidado no debe duplicarse: esperaba 1, obtuve %d", totalBilled)
	}

	var totalUnbilled int
	if err := w.db.QueryRow(`SELECT COUNT(*) FROM movimientos WHERE source = 'credit_card_unbilled'`).Scan(&totalUnbilled); err != nil {
		t.Fatalf("count unbilled: %v", err)
	}
	if totalUnbilled != 1 {
		t.Errorf("esperaba 1 unbilled en BD, obtuve %d", totalUnbilled)
	}
}

func TestInsertarConDedup_UnbilledRepetidoEnMismoBatch_NoDedupSeInsertanAmbos(t *testing.T) {
	w := setupDB(t)

	// El diseño no aplica dedup a provisorios: la garantía de no-duplicados
	// para unbilled la da el scraper (billed xor unbilled por snapshot), no
	// el writer. Documentamos ese comportamiento explícitamente: mismo
	// unbilled repetido dos veces en el MISMO batch inserta dos filas.
	dup := movUnbilled("2025-05-20", -5000, "Starbucks Provisorio")
	n, err := w.InsertarConDedup([]movimientos.MovimientoBruto{dup, dup})
	if err != nil {
		t.Fatalf("InsertarConDedup: %v", err)
	}
	if n != 2 {
		t.Errorf("esperaba 2 insertados (sin dedup para provisorios), obtuve %d", n)
	}

	var total int
	if err := w.db.QueryRow(`SELECT COUNT(*) FROM movimientos WHERE source = 'credit_card_unbilled'`).Scan(&total); err != nil {
		t.Fatalf("count: %v", err)
	}
	if total != 2 {
		t.Errorf("esperaba 2 filas unbilled en BD (documentando ausencia de dedup), obtuve %d", total)
	}
}

func TestInsertarConDedup_ErrorEnInsertProvisorio_HaceRollbackCompleto(t *testing.T) {
	w := setupDB(t)

	// Un billed previo ya persistido, para verificar que el rollback no
	// deja rastros del DELETE tampoco.
	if n, err := w.InsertarConDedup([]movimientos.MovimientoBruto{
		mov("2025-05-10", -20000, "Restorant Facturado"),
	}); err != nil || n != 1 {
		t.Fatalf("setup billed: n=%d err=%v", n, err)
	}
	if n, err := w.InsertarConDedup([]movimientos.MovimientoBruto{
		movUnbilled("2025-05-20", -5000, "Starbucks Provisorio"),
	}); err != nil || n != 1 {
		t.Fatalf("setup unbilled: n=%d err=%v", n, err)
	}

	// Este batch trae un unbilled inválido (sin Instrumento) que hará
	// fallar insertOne a mitad de la transacción. El DELETE de unbilled ya
	// habría corrido; si no hay rollback, el Starbucks anterior quedaría
	// borrado sin que el nuevo batch se persista — inconsistencia visible.
	invalido := movimientos.MovimientoBruto{
		Banco: "bchile", Source: "credit_card_unbilled",
		Fecha: mustParseDate(t, "2025-05-25"), Monto: -1000, Descripcion: "Invalido",
		Moneda: movimientos.MonedaCLP, CuotasTotales: 1, MontoRepresenta: movimientos.MontoRepresentaTotal,
		// Instrumento deliberadamente vacío para forzar el error en insertOne.
	}

	_, err := w.InsertarConDedup([]movimientos.MovimientoBruto{invalido})
	if err == nil {
		t.Fatal("esperaba error por Instrumento faltante en el provisorio")
	}

	var totalUnbilled int
	if err := w.db.QueryRow(`SELECT COUNT(*) FROM movimientos WHERE source = 'credit_card_unbilled'`).Scan(&totalUnbilled); err != nil {
		t.Fatalf("count unbilled: %v", err)
	}
	if totalUnbilled != 1 {
		t.Errorf("rollback incompleto: el DELETE de unbilled no debió persistir sin el commit; esperaba 1 (el Starbucks previo intacto), obtuve %d", totalUnbilled)
	}

	var desc string
	if err := w.db.QueryRow(`SELECT descripcion FROM movimientos WHERE source = 'credit_card_unbilled'`).Scan(&desc); err != nil {
		t.Fatalf("query descripcion: %v", err)
	}
	if desc != "Starbucks Provisorio" {
		t.Errorf("esperaba que el unbilled previo sobreviviera al rollback, obtuve %q", desc)
	}

	var totalBilled int
	if err := w.db.QueryRow(`SELECT COUNT(*) FROM movimientos WHERE descripcion = 'Restorant Facturado'`).Scan(&totalBilled); err != nil {
		t.Fatalf("count billed: %v", err)
	}
	if totalBilled != 1 {
		t.Errorf("el billed no debió verse afectado por el rollback, obtuve %d", totalBilled)
	}
}

func mustParseDate(t *testing.T, iso string) time.Time {
	t.Helper()
	f, err := time.Parse("2006-01-02", iso)
	if err != nil {
		t.Fatalf("parse fecha %q: %v", iso, err)
	}
	return f
}

func TestDescripcionCanonica_Normalizacion(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"PAYU   UBER TRIP        SANTIAGO     CL", "PAYU UBER TRIP"},
		{"PAYU *UBER TRIP COMPRAS", "PAYU UBER TRIP"},
		{"Traspaso A:Jose", "TRASPASO A JOSE"},
		{"Traspaso A:Maria", "TRASPASO A MARIA"},
		{"MERPAGOCABIFY2622YXGNCKTLas Condes   CL", "MERPAGOCABIFY2622YXGNCKTLAS CONDES"},
		{"UBER *LIME HELP.UBE COMPRAS INT.VI", "UBER LIME HELP UBE"},
	}

	for _, tt := range tests {
		got := descripcionCanonica(tt.in)
		if got != tt.want {
			t.Errorf("descripcionCanonica(%q) = %q; want %q", tt.in, got, tt.want)
		}
	}
}
