package presupuesto

type Calculadora struct {
	proveedor  ProveedorFinanciero
	resolvedor ResolvedorConfig
}

// NewCalculadora crea una nueva instancia de la calculadora.
// El porcentaje aplicable se resuelve del mes del periodo evaluado.
func NewCalculadora(proveedor ProveedorFinanciero, resolvedor ResolvedorConfig) *Calculadora {
	return &Calculadora{
		proveedor:  proveedor,
		resolvedor: resolvedor,
	}
}

// CalcularDisponible resuelve: (X * porcentajeDelMes) - Y
func (c *Calculadora) CalcularDisponible(periodo PeriodoPresupuestario) (float64, error) {
	cfg, err := c.resolvedor.ParaMes(periodo.Inicio)
	if err != nil {
		return 0, err
	}

	sueldo, err := c.proveedor.ObtenerSueldoBase(periodo)
	if err != nil {
		return 0, err
	}

	gastos, err := c.proveedor.ObtenerGastosValidos(periodo)
	if err != nil {
		return 0, err
	}

	var cargaMensualTotal float64
	for _, gasto := range gastos {
		cargaMensualTotal += gasto.CalcularCargaParaPeriodo(periodo)
	}

	return (sueldo * cfg.PorcentajeParaGastos) - cargaMensualTotal, nil
}
