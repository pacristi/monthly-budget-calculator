package presupuesto

import "time"

// ProyeccionMensual representa la deuda proyectada para un mes específico.
type ProyeccionMensual struct {
	Anio              int
	Mes               time.Month
	TotalComprometido float64
}

// ProyectorPresupuesto se encarga de calcular proyecciones financieras futuras.
type ProyectorPresupuesto struct{}

// NewProyectorPresupuesto crea una nueva instancia del proyector.
func NewProyectorPresupuesto() *ProyectorPresupuesto {
	return &ProyectorPresupuesto{}
}

// Proyectar calcula los compromisos de gastos para los próximos N meses,
// a partir del mes base especificado.
func (p *ProyectorPresupuesto) Proyectar(gastos []Gasto, mesBase time.Time, mesesHaciaAdelante int) []ProyeccionMensual {
	var proyecciones []ProyeccionMensual

	// Normalizar mesBase al día 1
	base := time.Date(mesBase.Year(), mesBase.Month(), 1, 0, 0, 0, 0, mesBase.Location())

	for i := 0; i < mesesHaciaAdelante; i++ {
		mesEvaluar := base.AddDate(0, i, 0)

		// Construir el periodo para el mes a evaluar
		periodo := PeriodoPresupuestario{
			Inicio: mesEvaluar,
			Fin:    mesEvaluar.AddDate(0, 1, 0).Add(-time.Nanosecond), // Último instante del mes
		}

		var totalMes float64
		for _, gasto := range gastos {
			totalMes += gasto.CalcularCargaParaPeriodo(periodo)
		}

		proyecciones = append(proyecciones, ProyeccionMensual{
			Anio:              mesEvaluar.Year(),
			Mes:               mesEvaluar.Month(),
			TotalComprometido: totalMes,
		})
	}

	return proyecciones
}
