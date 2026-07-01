package shared

import "presupuesto/presupuesto"

// CoincidePatronSueldo retorna true si descripcion (case-insensitive) contiene
// alguno de los patrones. La regla canónica vive en presupuesto; esto es un
// alias temporal para el adapter legacy obchile (se elimina con él en Paso 5).
func CoincidePatronSueldo(descripcion string, patrones []string) bool {
	return presupuesto.CoincidePatronSueldo(descripcion, patrones)
}
