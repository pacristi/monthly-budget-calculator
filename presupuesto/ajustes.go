package presupuesto

// Override representa ajustes manuales del usuario sobre un movimiento. La
// clave principal es MovimientoID; la terna legacy se conserva solo para leer y
// migrar ajustes antiguos desde divisiones.json. Dos ajustes ortogonales pueden
// convivir en el mismo registro:
//   - MiParte: lo que efectivamente me tocó pagar (split de gasto compartido),
//     en la misma moneda y signo que el monto original. nil = no hay split;
//     un puntero a 0 significa "No contar".
//   - Categoria: el id de categoría asignado a mano. "" = sin override de categoría.
//   - Alias: descripción visible en UI. "" = mostrar descripción original.
type Override struct {
	MovimientoID  string   `json:"movimientoId,omitempty"`
	Fecha         string   `json:"fecha"` // Formato ISO: yyyy-mm-dd
	MontoOriginal float64  `json:"montoOriginal"`
	Descripcion   string   `json:"descripcion"`
	MiParte       *float64 `json:"miParte,omitempty"`
	Categoria     string   `json:"categoria,omitempty"`
	Alias         string   `json:"alias,omitempty"`
}

// AplicarOverrides devuelve el monto crudo a imputar: "mi parte" si hay un
// override de split registrado para movimientoID, o el monto original tal cual.
// La terna legacy queda como fallback temporal para migrar datos antiguos.
func AplicarOverrides(movimientoID string, montoOriginal float64, fechaISO string, descripcion string, overrides []Override) float64 {
	if o, ok := buscarOverride(movimientoID, fechaISO, montoOriginal, descripcion, overrides); ok {
		if o.MiParte != nil {
			return *o.MiParte
		}
		return montoOriginal
	}
	return montoOriginal
}

// CategoriaOverride devuelve el id de categoría asignado a mano a un movimiento
// persistido, o "" si no hay override de categoría.
func CategoriaOverride(movimientoID string, fechaISO string, montoOriginal float64, descripcion string, overrides []Override) string {
	if o, ok := buscarOverride(movimientoID, fechaISO, montoOriginal, descripcion, overrides); ok {
		return o.Categoria
	}
	return ""
}

// MiParteOverride devuelve el ajuste de split para un movimiento persistido, o
// nil si no hay override. La terna legacy queda como fallback temporal.
func MiParteOverride(movimientoID string, fechaISO string, montoOriginal float64, descripcion string, overrides []Override) *float64 {
	if o, ok := buscarOverride(movimientoID, fechaISO, montoOriginal, descripcion, overrides); ok {
		return o.MiParte
	}
	return nil
}

// DescripcionOverride devuelve la descripción visual asignada por el usuario,
// o "" si el movimiento no tiene alias.
func DescripcionOverride(movimientoID string, fechaISO string, montoOriginal float64, descripcion string, overrides []Override) string {
	if o, ok := buscarOverride(movimientoID, fechaISO, montoOriginal, descripcion, overrides); ok {
		if o.Alias == "" && o.MovimientoID != "" && o.Descripcion != "" && o.Descripcion != descripcion {
			return o.Descripcion
		}
		return o.Alias
	}
	return ""
}

func buscarOverride(movimientoID string, fechaISO string, montoOriginal float64, descripcion string, overrides []Override) (Override, bool) {
	if movimientoID != "" {
		for _, o := range overrides {
			if o.MovimientoID == movimientoID {
				return o, true
			}
		}
	}

	for _, o := range overrides {
		if o.Descripcion == "" {
			continue
		}
		if o.Fecha == fechaISO && o.MontoOriginal == montoOriginal && o.Descripcion == descripcion {
			return o, true
		}
	}
	return Override{}, false
}

func mismoMovimiento(a, b Override) bool {
	return a.MovimientoID != "" && a.MovimientoID == b.MovimientoID
}

func mismaTerna(a, b Override) bool {
	return a.Fecha == b.Fecha && a.MontoOriginal == b.MontoOriginal && a.Descripcion == b.Descripcion
}

// MismoOverrideObjetivo indica si dos overrides identifican al mismo
// movimiento: por MovimientoID si ambos lo tienen, o por la terna legacy
// (fecha + monto + descripción) como fallback. Lo usa el repo de I/O
// (definiciones/json) para decidir si actualiza un registro existente o
// agrega uno nuevo al guardar.
func MismoOverrideObjetivo(a, b Override) bool {
	return mismoMovimiento(a, b) || mismaTerna(a, b)
}
