package presupuesto

// ProveedorFinanciero es el contrato (Puerto) que los adaptadores deben cumplir.
type ProveedorFinanciero interface {
	ObtenerSueldoBase(periodo PeriodoPresupuestario) (float64, error)
	ObtenerGastosValidos(periodo PeriodoPresupuestario) ([]Gasto, error)
}
