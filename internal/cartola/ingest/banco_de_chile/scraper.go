package banco_de_chile

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"time"

	"github.com/pierocristi/monthly-budget-calculator/internal/cartola/ingest"
)

const banco = "bchile"

// ScraperResponse es la raíz del JSON entregado por Open Banking Chile.
type ScraperResponse struct {
	Success     bool            `json:"success"`
	Bank        string          `json:"bank"`
	Movements   []MovimientoDTO `json:"movements"`
	Accounts    []Account       `json:"accounts"`
	CreditCards []CreditCard    `json:"creditCards"`
}

type Account struct {
	Balance   float64         `json:"balance"`
	Movements []MovimientoDTO `json:"movements"`
}

type CreditCard struct {
	Label     string          `json:"label"`
	Movements []MovimientoDTO `json:"movements"`
}

// MovimientoDTO representa cada transacción individual en el JSON de OBCL.
type MovimientoDTO struct {
	Fecha        string  `json:"date"`
	Descripcion  string  `json:"description"`
	Monto        float64 `json:"amount"`
	Source       string  `json:"source"`
	Installments string  `json:"installments"`
}

// Client lee y aplana el JSON del scraper.
type Client struct {
	rutaJson string
}

func NewClient(rutaJson string) *Client {
	return &Client{rutaJson: rutaJson}
}

func (c *Client) Fetch() ([]MovimientoDTO, error) {
	data, err := os.ReadFile(c.rutaJson)
	if err != nil {
		return nil, err
	}
	var resp ScraperResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}

	var movimientos []MovimientoDTO
	movimientos = append(movimientos, resp.Movements...)
	for _, acc := range resp.Accounts {
		movimientos = append(movimientos, acc.Movements...)
	}
	for _, cc := range resp.CreditCards {
		movimientos = append(movimientos, cc.Movements...)
	}
	return movimientos, nil
}

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

	moneda := monedaDeMonto(d.Monto)
	cuotaActual, cuotasTotales := parseCuotas(d.Installments)

	return ingest.MovimientoBruto{
		Banco:       banco,
		Source:      d.Source,
		Fecha:       fecha,
		Monto:       d.Monto,
		Descripcion: d.Descripcion,
		Instrumento: instrumentoDeSource(d.Source),
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

func instrumentoDeSource(source string) ingest.Instrumento {
	switch source {
	case "account":
		return ingest.InstrumentoCuentaCorriente
	case "credit_card_billed", "credit_card_unbilled":
		return ingest.InstrumentoTarjetaCredito
	default:
		return ingest.InstrumentoDesconocido
	}
}

// monedaDeMonto: el scraper no declara divisa, pero expresa CLP como enteros, así que un monto con parte decimal es USD.
func monedaDeMonto(monto float64) ingest.Moneda {
	if math.Trunc(monto) != monto {
		return ingest.MonedaUSD
	}
	return ingest.MonedaCLP
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
