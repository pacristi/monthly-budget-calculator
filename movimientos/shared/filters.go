package shared

import "strings"

// EsGastoIgnorable devuelve true si la descripción contiene alguna de las
// substrings de `exclusiones` (case-insensitive). Las exclusiones son
// personales: típicamente ahorros, traspasos patrimoniales y pagos de
// tarjeta que ya están contabilizados en otra parte.
func EsGastoIgnorable(descripcion string, exclusiones []string) bool {
	desc := strings.ToLower(descripcion)
	for _, e := range exclusiones {
		if strings.Contains(desc, strings.ToLower(e)) {
			return true
		}
	}
	return false
}

// CoincidePatronSueldo retorna true si descripcion (case-insensitive)
// contiene alguno de los patrones. Igual semántica que EsGastoIgnorable
// pero conceptualmente distinto (identificación vs filtrado).
func CoincidePatronSueldo(descripcion string, patrones []string) bool {
	desc := strings.ToLower(descripcion)
	for _, p := range patrones {
		if strings.Contains(desc, strings.ToLower(p)) {
			return true
		}
	}
	return false
}
