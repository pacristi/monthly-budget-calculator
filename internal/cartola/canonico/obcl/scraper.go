package obcl

import (
	"encoding/json"
	"fmt"
	"time"

	"presupuesto/internal/cartola/canonico"
)

// LeerScraper lee el current.json del scraper OBCL y devuelve los movimientos
// como DTO canonico. No filtra ni toca sqlite.
func LeerScraper(jsonPath string) ([]canonico.MovimientoBruto, error) {
	dtos, err := NewClient(jsonPath).Fetch()
	if err != nil {
		return nil, fmt.Errorf("leyendo JSON %s: %w", jsonPath, err)
	}

	out := make([]canonico.MovimientoBruto, 0, len(dtos))
	for _, d := range dtos {
		b, err := scraperABruto(d)
		if err != nil {
			return nil, fmt.Errorf("mapeando movimiento (fecha=%s desc=%s): %w", d.Fecha, d.Descripcion, err)
		}
		out = append(out, b)
	}
	return out, nil
}

func scraperABruto(d MovimientoDTO) (canonico.MovimientoBruto, error) {
	fecha, err := time.Parse("02-01-2006", d.Fecha)
	if err != nil {
		return canonico.MovimientoBruto{}, fmt.Errorf("fecha %q no parseable: %w", d.Fecha, err)
	}

	raw, err := dtoAMap(d)
	if err != nil {
		return canonico.MovimientoBruto{}, err
	}

	moneda := MonedaDeMonto(d.Monto)
	cuotaActual, cuotasTotales := CuotasDeInstallments(d.Installments)

	return canonico.MovimientoBruto{
		Banco:       d.Banco,
		Source:      d.Source,
		Fecha:       fecha,
		Monto:       d.Monto,
		Descripcion: d.Descripcion,
		Instrumento: InstrumentoDeSource(d.Source),
		Moneda:      moneda,
		// El scraper entrega el monto TOTAL de la compra, tambien en cuotas.
		MontoRepresenta: canonico.MontoRepresentaTotal,
		CuotaActual:     cuotaActual,
		CuotasTotales:   cuotasTotales,
		IsUSD:           moneda == canonico.MonedaUSD,
		Cuotas:          d.Installments,
		Raw:             raw,
	}, nil
}

func dtoAMap(d MovimientoDTO) (map[string]any, error) {
	bytes, err := json.Marshal(d)
	if err != nil {
		return nil, err
	}
	var m map[string]any
	if err := json.Unmarshal(bytes, &m); err != nil {
		return nil, err
	}
	return m, nil
}
