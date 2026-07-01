package openbankingchile

import (
	"encoding/json"
	"os"
)

// MovimientoDTO representa cada transaccion individual en el JSON de OBCL.
type MovimientoDTO struct {
	Banco        string  `json:"bank"`
	Fecha        string  `json:"date"`
	Descripcion  string  `json:"description"`
	Monto        float64 `json:"amount"`
	Source       string  `json:"source"`
	Installments string  `json:"installments"`
}

// Client lee y aplana el JSON del scraper OBCL.
type Client struct {
	rutaJSON string
}

func NewClient(rutaJSON string) *Client {
	return &Client{rutaJSON: rutaJSON}
}

func (c *Client) Fetch() ([]MovimientoDTO, error) {
	data, err := os.ReadFile(c.rutaJSON)
	if err != nil {
		return nil, err
	}
	var resp scraperResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}

	movimientos := make([]MovimientoDTO, 0, len(resp.Movements))
	movimientos = appendMovements(movimientos, resp.Bank, resp.Movements)
	for _, acc := range resp.Accounts {
		movimientos = appendMovements(movimientos, resp.Bank, acc.Movements)
	}
	for _, cc := range resp.CreditCards {
		movimientos = appendMovements(movimientos, resp.Bank, cc.Movements)
	}
	return movimientos, nil
}

// scraperResponse es la raiz del JSON entregado por Open Banking Chile.
type scraperResponse struct {
	Success     bool            `json:"success"`
	Bank        string          `json:"bank"`
	Movements   []MovimientoDTO `json:"movements"`
	Accounts    []account       `json:"accounts"`
	CreditCards []creditCard    `json:"creditCards"`
}

type account struct {
	Balance   float64         `json:"balance"`
	Movements []MovimientoDTO `json:"movements"`
}

type creditCard struct {
	Label     string          `json:"label"`
	Movements []MovimientoDTO `json:"movements"`
}

func appendMovements(out []MovimientoDTO, bank string, movimientos []MovimientoDTO) []MovimientoDTO {
	for _, m := range movimientos {
		if m.Banco == "" {
			m.Banco = bank
		}
		out = append(out, m)
	}
	return out
}
