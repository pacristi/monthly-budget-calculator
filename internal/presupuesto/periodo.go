package presupuesto

import "time"

// PeriodoPresupuestario define el rango de tiempo evaluado.
type PeriodoPresupuestario struct {
	Inicio time.Time
	Fin    time.Time
}
