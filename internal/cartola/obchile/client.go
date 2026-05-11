package obchile

import (
	"encoding/json"
	"os"
)

// Client lee y parsea los archivos de OBCL
type Client struct {
	rutaJson string
}

func NewClient(rutaJson string) *Client {
	return &Client{rutaJson: rutaJson}
}

// Fetch lee el archivo JSON de disco y lo aplana en un arreglo de movimientos.
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
	if len(resp.Movements) > 0 {
		movimientos = append(movimientos, resp.Movements...)
	}
	for _, acc := range resp.Accounts {
		movimientos = append(movimientos, acc.Movements...)
	}
	for _, cc := range resp.CreditCards {
		movimientos = append(movimientos, cc.Movements...)
	}
	return movimientos, nil
}
