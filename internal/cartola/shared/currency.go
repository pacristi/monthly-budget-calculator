package shared

import (
	"math"
)

// NormalizarMonto aplica la heurística del punto decimal para determinar si es USD y pasarlo a CLP.
func NormalizarMonto(monto float64, tasaCambioUSD float64) float64 {
	// Si el monto tiene parte decimal, asumimos que es USD
	if math.Mod(math.Abs(monto), 1) != 0 {
		return monto * tasaCambioUSD
	}
	return monto
}
