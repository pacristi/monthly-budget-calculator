package obchile

// GastoManualDTO representa un gasto ingresado manualmente por el usuario.
type GastoManualDTO struct {
	ID            string  `json:"id"`
	Descripcion   string  `json:"descripcion"`
	MontoTotal    float64 `json:"montoTotal"`
	CuotasTotales int     `json:"cuotasTotales"`
	FechaInicio   string  `json:"fechaInicio"` // Formato esperado: dd-mm-yyyy
	TipoPago      string  `json:"tipoPago"`    // "debito" o "credito"
}
