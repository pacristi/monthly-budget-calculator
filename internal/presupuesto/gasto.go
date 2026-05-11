package presupuesto

import "time"

// Gasto representa una transacción.
type Gasto struct {
	ID               string
	Descripcion      string  // Para identificar visualmente el gasto
	MontoImputado    float64 // Ya calculado: Monto Total * Porcentaje
	Cuotas           int     // Default 1
	FechaTransaccion time.Time
	PoliticaCorte    PoliticaCorte
}

// CalcularCargaParaPeriodo determina cuánto dinero recae en el periodo consultado.
func (g *Gasto) CalcularCargaParaPeriodo(periodo PeriodoPresupuestario) float64 {
	// Normalizamos la fecha de la transacción al primer día del mes para evitar
	// problemas de salto de mes en Go (ej. 31 de marzo + 1 mes = 1 de mayo)
	inicioImputacion := time.Date(g.FechaTransaccion.Year(), g.FechaTransaccion.Month(), 1, 0, 0, 0, 0, g.FechaTransaccion.Location())

	// 1. Evaluar g.FechaTransaccion vs g.PoliticaCorte.DiaDeCorte
	if g.PoliticaCorte.Tipo == Credito {
		mesesAtraso := 0 // Considerar en el mes actual por defecto
		if g.FechaTransaccion.Day() > g.PoliticaCorte.DiaDeCorte {
			mesesAtraso = 1 // Si pasa el día de corte, se patea al mes siguiente
		}
		inicioImputacion = inicioImputacion.AddDate(0, mesesAtraso, 0)
	}

	// 2. Determinar mes de inicio de imputación
	mesInicio := inicioImputacion
	mesPeriodo := time.Date(periodo.Inicio.Year(), periodo.Inicio.Month(), 1, 0, 0, 0, 0, periodo.Inicio.Location())

	// 3. Proyectar temporalmente las 'g.Cuotas'
	cuotas := g.Cuotas
	if cuotas < 1 {
		cuotas = 1
	}

	diferenciaMeses := (mesPeriodo.Year()-mesInicio.Year())*12 + int(mesPeriodo.Month()-mesInicio.Month())

	// 4. Si 'periodo' hace match con la proyección: retorna g.MontoImputado / float64(g.Cuotas)
	if diferenciaMeses >= 0 && diferenciaMeses < cuotas {
		return g.MontoImputado / float64(cuotas)
	}

	// 5. Si no hay match: retorna 0.0
	return 0.0
}
