package presupuesto

import "time"

// Movimiento representa una transacción cruda tal como la entrega un
// proveedor financiero, con los overrides locales aplicados si corresponde.
type Movimiento struct {
	Fecha       time.Time
	Descripcion string
	Monto       float64
	IsUSD       bool
	MiParte     *float64 // nil cuando no hay override registrado
	CategoriaID string   // categoría efectiva (override > regla > default); Ignorado si no cuenta
}
