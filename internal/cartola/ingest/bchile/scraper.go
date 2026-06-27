package bchile

import (
	"encoding/json"
	"fmt"
	"time"

	"presupuesto/internal/cartola/ingest"
	"presupuesto/internal/cartola/obchile"
)

const banco = "bchile"

type MovimientoDTO = obchile.MovimientoDTO
type Client = obchile.Client

var NewClient = obchile.NewClient

// LeerScraper lee el current.json del scraper y devuelve los movimientos del
// banco como DTO canónico. No filtra (la política de qué se persiste vive en
// el runner de ingesta) ni toca sqlite.
func LeerScraper(jsonPath string) ([]ingest.MovimientoBruto, error) {
	dtos, err := NewClient(jsonPath).Fetch()
	if err != nil {
		return nil, fmt.Errorf("leyendo JSON %s: %w", jsonPath, err)
	}

	out := make([]ingest.MovimientoBruto, 0, len(dtos))
	for _, d := range dtos {
		b, err := scraperABruto(d)
		if err != nil {
			return nil, fmt.Errorf("mapeando movimiento (fecha=%s desc=%s): %w", d.Fecha, d.Descripcion, err)
		}
		out = append(out, b)
	}
	return out, nil
}

func scraperABruto(d MovimientoDTO) (ingest.MovimientoBruto, error) {
	fecha, err := time.Parse("02-01-2006", d.Fecha)
	if err != nil {
		return ingest.MovimientoBruto{}, fmt.Errorf("fecha %q no parseable: %w", d.Fecha, err)
	}

	raw, err := dtoAMap(d)
	if err != nil {
		return ingest.MovimientoBruto{}, err
	}

	moneda := obchile.MonedaDeMonto(d.Monto)
	cuotaActual, cuotasTotales := obchile.CuotasDeInstallments(d.Installments)

	return ingest.MovimientoBruto{
		Banco:       banco,
		Source:      d.Source,
		Fecha:       fecha,
		Monto:       d.Monto,
		Descripcion: d.Descripcion,
		Instrumento: obchile.InstrumentoDeSource(d.Source),
		Moneda:      moneda,
		// El scraper entrega el monto TOTAL de la compra, también en cuotas.
		MontoRepresenta: ingest.MontoRepresentaTotal,
		CuotaActual:     cuotaActual,
		CuotasTotales:   cuotasTotales,
		IsUSD:           moneda == ingest.MonedaUSD,
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
