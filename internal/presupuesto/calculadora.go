package presupuesto

type Calculadora struct {
	proveedor  ProveedorFinanciero
	porcentaje float64 // Inyectado desde config/env
}

// NewCalculadora crea una nueva instancia de la calculadora.
func NewCalculadora(proveedor ProveedorFinanciero, porcentaje float64) *Calculadora {
	return &Calculadora{
		proveedor:  proveedor,
		porcentaje: porcentaje,
	}
}

// CalcularDisponible resuelve: (X * porcentaje) - Y
func (c *Calculadora) CalcularDisponible(periodo PeriodoPresupuestario) (float64, error) {
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

	return (sueldo * c.porcentaje) - cargaMensualTotal, nil
}
