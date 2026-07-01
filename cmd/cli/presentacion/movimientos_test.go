package presentacion_test

import (
	"testing"
	"time"

	"presupuesto/cmd/cli/presentacion"
	"presupuesto/presupuesto"
)

func TestMovimientosAplicaAliasDesdeOverrides(t *testing.T) {
	fecha := time.Date(2026, time.May, 15, 0, 0, 0, 0, time.UTC)
	movs := []presupuesto.Movimiento{{
		ID:          "sql-1",
		Fecha:       fecha,
		Descripcion: "STARBUCKS PROVIDENCIA",
		Monto:       -3500,
		CategoriaID: "comida",
	}}
	overrides := []presupuesto.Override{{
		MovimientoID:  "sql-1",
		Fecha:         "2026-05-15",
		MontoOriginal: -3500,
		Descripcion:   "STARBUCKS PROVIDENCIA",
		Alias:         "Café",
	}}

	got := presentacion.Movimientos(movs, overrides)

	if len(got) != 1 {
		t.Fatalf("esperaba 1 movimiento, obtuvo %d", len(got))
	}
	if got[0].Descripcion != "Café" {
		t.Fatalf("esperaba descripción visual %q, obtuvo %q", "Café", got[0].Descripcion)
	}
	if got[0].DescripcionOriginal != "STARBUCKS PROVIDENCIA" {
		t.Fatalf("esperaba descripción original %q, obtuvo %q", "STARBUCKS PROVIDENCIA", got[0].DescripcionOriginal)
	}
	if movs[0].Descripcion != "STARBUCKS PROVIDENCIA" {
		t.Fatalf("presupuesto.Movimiento no debe mutar, obtuvo %q", movs[0].Descripcion)
	}
}

func TestMovimientosUsaDescripcionOriginalSinAlias(t *testing.T) {
	fecha := time.Date(2026, time.May, 15, 0, 0, 0, 0, time.UTC)
	movs := []presupuesto.Movimiento{{
		Fecha:       fecha,
		Descripcion: "SUPERMERCADO",
		Monto:       -50000,
	}}

	got := presentacion.Movimientos(movs, nil)

	if got[0].Fecha != "2026-05-15" {
		t.Fatalf("esperaba fecha ISO, obtuvo %q", got[0].Fecha)
	}
	if got[0].Descripcion != "SUPERMERCADO" {
		t.Fatalf("esperaba descripción original, obtuvo %q", got[0].Descripcion)
	}
}
