package shared

import (
	"strconv"
	"strings"
)

// EsGastoIgnorable aplica las reglas de SRE para no ensuciar el presupuesto con movimientos patrimoniales.
func EsGastoIgnorable(descripcion string) bool {
	desc := strings.ToLower(descripcion)

	// Reglas de exclusión:
	exclusiones := []string{
		"cargo por pago tc",
		"pago tarjeta de credito",
		"fintual",
		"racional",
	}

	for _, e := range exclusiones {
		if strings.Contains(desc, e) {
			return true
		}
	}

	return false
}

// ParsearCuotas extrae la cantidad total de cuotas del formato "01/03".
func ParsearCuotas(installments string) int {
	if installments == "" {
		return 1
	}
	parts := strings.Split(installments, "/")
	if len(parts) == 2 {
		cuotas, err := strconv.Atoi(parts[1])
		if err == nil && cuotas > 0 {
			return cuotas
		}
	}
	return 1
}
