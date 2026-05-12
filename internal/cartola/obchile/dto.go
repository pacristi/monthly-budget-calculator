package obchile

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

// GastoManualDTO representa un gasto ingresado manualmente por el usuario.
type GastoManualDTO struct {
	ID            string  `json:"id"`
	Descripcion   string  `json:"descripcion"`
	MontoTotal    float64 `json:"montoTotal"`
	CuotasTotales int     `json:"cuotasTotales"`
	FechaInicio   string  `json:"fechaInicio"` // Formato esperado: dd-mm-yyyy
	TipoPago      string  `json:"tipoPago"`    // "debito" o "credito"
}
