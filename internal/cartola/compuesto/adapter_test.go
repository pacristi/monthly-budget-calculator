package compuesto

import (
	"errors"
	"testing"

	"presupuesto/internal/presupuesto"
)

// proveedorFake es un ProveedorFinanciero de prueba con datos prefijados.
type proveedorFake struct {
	sueldo      float64
	sueldoErr   error
	gastos      []presupuesto.Gasto
	movimientos []presupuesto.Movimiento
}

func (p proveedorFake) ObtenerSueldoBase(presupuesto.PeriodoPresupuestario) (float64, error) {
	return p.sueldo, p.sueldoErr
}
func (p proveedorFake) ObtenerGastosValidos(presupuesto.PeriodoPresupuestario) ([]presupuesto.Gasto, error) {
	return p.gastos, nil
}
func (p proveedorFake) ObtenerMovimientos() ([]presupuesto.Movimiento, error) {
	return p.movimientos, nil
}

func TestCompuesto_SueldoVieneDeLiquidado(t *testing.T) {
	liquidado := proveedorFake{sueldo: 1500000}
	provisorio := proveedorFake{sueldoErr: errors.New("el provisorio no debería ser consultado por el sueldo")}

	c := NewAdapter(liquidado, provisorio)
	sueldo, err := c.ObtenerSueldoBase(presupuesto.PeriodoPresupuestario{})
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}
	if sueldo != 1500000 {
		t.Errorf("el sueldo debe venir del liquidado: got %v, want 1500000", sueldo)
	}
}

func TestCompuesto_GastosMergeaAmbasCapas(t *testing.T) {
	liquidado := proveedorFake{gastos: []presupuesto.Gasto{{ID: "liq-1", Descripcion: "FACTURADO"}}}
	provisorio := proveedorFake{gastos: []presupuesto.Gasto{{ID: "prov-1", Descripcion: "UNBILLED"}}}

	c := NewAdapter(liquidado, provisorio)
	gastos, err := c.ObtenerGastosValidos(presupuesto.PeriodoPresupuestario{})
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}
	if len(gastos) != 2 {
		t.Fatalf("esperaba 2 gastos (liquidado + provisorio), obtuve %d", len(gastos))
	}
}

func TestNewDesdeFuentes_SinProvisorioDevuelveSoloLiquidado(t *testing.T) {
	liquidado := proveedorFake{gastos: []presupuesto.Gasto{{ID: "liq-1"}}}

	// Sin fuente de scrape (provisorio nil): debe servir solo el liquidado,
	// sin componer ni reventar.
	prov := NewDesdeFuentes(liquidado, nil)
	gastos, err := prov.ObtenerGastosValidos(presupuesto.PeriodoPresupuestario{})
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}
	if len(gastos) != 1 {
		t.Fatalf("sin provisorio esperaba 1 gasto (solo liquidado), obtuve %d", len(gastos))
	}
}

func TestNewDesdeFuentes_ConProvisorioCompone(t *testing.T) {
	liquidado := proveedorFake{gastos: []presupuesto.Gasto{{ID: "liq-1"}}}
	provisorio := proveedorFake{gastos: []presupuesto.Gasto{{ID: "prov-1"}}}

	prov := NewDesdeFuentes(liquidado, provisorio)
	gastos, err := prov.ObtenerGastosValidos(presupuesto.PeriodoPresupuestario{})
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}
	if len(gastos) != 2 {
		t.Fatalf("con provisorio esperaba 2 gastos (compone), obtuve %d", len(gastos))
	}
}

func TestCompuesto_MovimientosMergeaAmbasCapas(t *testing.T) {
	liquidado := proveedorFake{movimientos: []presupuesto.Movimiento{{Descripcion: "FACTURADO"}}}
	provisorio := proveedorFake{movimientos: []presupuesto.Movimiento{{Descripcion: "UNBILLED"}}}

	c := NewAdapter(liquidado, provisorio)
	movs, err := c.ObtenerMovimientos()
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}
	if len(movs) != 2 {
		t.Fatalf("esperaba 2 movimientos (liquidado + provisorio), obtuve %d", len(movs))
	}
}
