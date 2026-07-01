package shared

import "presupuesto/presupuesto"

// NormalizarMonto convierte montos USD a CLP usando la tasa entregada. La regla
// canónica vive en presupuesto; esto es un alias temporal para el adapter
// legacy obchile (se elimina con él en Paso 5).
func NormalizarMonto(monto float64, esUSD bool, tasaCambioUSD float64) float64 {
	return presupuesto.NormalizarMonto(monto, esUSD, tasaCambioUSD)
}
