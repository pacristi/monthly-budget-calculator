// Package compuesto implementa un ProveedorFinanciero que compone dos
// capas: la liquidada (hechos persistidos en sqlite) y la provisoria
// (proyección viva del último scrape). Los conjuntos son disjuntos por
// estado (un movimiento es billed xor unbilled en un snapshot), así que el
// merge es una concatenación sin riesgo de doble conteo.
package compuesto

import "github.com/pierocristi/monthly-budget-calculator/internal/presupuesto"

// Adapter compone una capa liquidada y una provisoria.
type Adapter struct {
	liquidado  presupuesto.ProveedorFinanciero
	provisorio presupuesto.ProveedorFinanciero
}

// NewAdapter construye el Compuesto a partir de los dos proveedores.
func NewAdapter(liquidado, provisorio presupuesto.ProveedorFinanciero) *Adapter {
	return &Adapter{liquidado: liquidado, provisorio: provisorio}
}

// NewDesdeFuentes construye el proveedor de lectura a partir de sus fuentes.
// El liquidado siempre está; el provisorio se compone solo si existe. Sin
// fuente de scrape (usuario solo-xlsx) devuelve el liquidado solo, sin
// reventar.
func NewDesdeFuentes(liquidado, provisorio presupuesto.ProveedorFinanciero) presupuesto.ProveedorFinanciero {
	if provisorio == nil {
		return liquidado
	}
	return NewAdapter(liquidado, provisorio)
}

// ObtenerSueldoBase se delega a la capa liquidada: el sueldo es un abono de
// cuenta ya asentado; el provisorio (cargos de TC no facturados) no lo
// contiene.
func (a *Adapter) ObtenerSueldoBase(periodo presupuesto.PeriodoPresupuestario) (float64, error) {
	return a.liquidado.ObtenerSueldoBase(periodo)
}

// ObtenerGastosValidos concatena los gastos de ambas capas.
func (a *Adapter) ObtenerGastosValidos(periodo presupuesto.PeriodoPresupuestario) ([]presupuesto.Gasto, error) {
	liq, err := a.liquidado.ObtenerGastosValidos(periodo)
	if err != nil {
		return nil, err
	}
	prov, err := a.provisorio.ObtenerGastosValidos(periodo)
	if err != nil {
		return nil, err
	}
	out := make([]presupuesto.Gasto, 0, len(liq)+len(prov))
	out = append(out, liq...)
	out = append(out, prov...)
	return out, nil
}

// ObtenerMovimientos concatena los movimientos de ambas capas.
func (a *Adapter) ObtenerMovimientos() ([]presupuesto.Movimiento, error) {
	liq, err := a.liquidado.ObtenerMovimientos()
	if err != nil {
		return nil, err
	}
	prov, err := a.provisorio.ObtenerMovimientos()
	if err != nil {
		return nil, err
	}
	out := make([]presupuesto.Movimiento, 0, len(liq)+len(prov))
	out = append(out, liq...)
	out = append(out, prov...)
	return out, nil
}
