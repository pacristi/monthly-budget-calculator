package obchile

import (
	"encoding/json"
	"os"
)

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
	var resp scraperResponse
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

// scraperResponse es la raíz del JSON entregado por Open Banking Chile.
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
