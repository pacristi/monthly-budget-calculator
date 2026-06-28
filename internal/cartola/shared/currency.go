package shared

// NormalizarMonto convierte montos USD a CLP usando la tasa entregada.
func NormalizarMonto(monto float64, esUSD bool, tasaCambioUSD float64) float64 {
	if esUSD {
		return monto * tasaCambioUSD
	}
	return monto
}
