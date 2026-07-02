package presupuesto

type Calculadora struct {
	sueldo     float64
	gastos     []Gasto
	resolvedor ResolvedorConfig
}

// NewCalculadora crea una nueva instancia de la calculadora. El sueldo y los
// gastos del periodo se resuelven fuera (store + dominio) y se inyectan
// directo; el porcentaje aplicable se resuelve del mes del periodo evaluado.
func NewCalculadora(sueldo float64, gastos []Gasto, resolvedor ResolvedorConfig) *Calculadora {
	return &Calculadora{
		sueldo:     sueldo,
		gastos:     gastos,
		resolvedor: resolvedor,
	}
}

// ResultadoCategoria es el estado de una categoría en un periodo: cuánto se le
// asignó del sueldo (Presupuesto) y cuánto flujo real cayó en ella (Acumulado).
type ResultadoCategoria struct {
	CategoriaID string
	Nombre      string
	Tipo        TipoCategoria
	Presupuesto float64 // sueldo * %
	Acumulado   float64 // suma de la carga de los movimientos de esta categoría
}

// ResumenPresupuesto es el desglose por categoría de un periodo.
type ResumenPresupuesto struct {
	Sueldo     float64
	Categorias []ResultadoCategoria
	SinAsignar float64 // sueldo no cubierto por la suma de porcentajes
}

// CalcularResumen arma el desglose por categoría: para cada categoría devuelve
// su presupuesto (sueldo × %) y lo acumulado (flujo real hacia esa categoría).
func (c *Calculadora) CalcularResumen(periodo PeriodoPresupuestario, categorias []Categoria) (ResumenPresupuesto, error) {
	cfg, err := c.resolvedor.ParaMes(periodo.Inicio)
	if err != nil {
		return ResumenPresupuesto{}, err
	}

	acumuladoPorCategoria := make(map[string]float64)
	for _, gasto := range c.gastos {
		acumuladoPorCategoria[gasto.CategoriaID] += gasto.CalcularCargaParaPeriodo(periodo)
	}

	resultados := make([]ResultadoCategoria, 0, len(categorias))
	var sumaPorcentajes float64
	for _, cat := range categorias {
		pct := cfg.Porcentajes[cat.ID]
		sumaPorcentajes += pct
		resultados = append(resultados, ResultadoCategoria{
			CategoriaID: cat.ID,
			Nombre:      cat.Nombre,
			Tipo:        cat.Tipo,
			Presupuesto: c.sueldo * pct,
			Acumulado:   acumuladoPorCategoria[cat.ID],
		})
	}

	return ResumenPresupuesto{
		Sueldo:     c.sueldo,
		Categorias: resultados,
		SinAsignar: c.sueldo * (1 - sumaPorcentajes),
	}, nil
}
