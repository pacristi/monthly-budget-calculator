package ajustes

import (
	"encoding/json"
	"os"
)

// Override representa ajustes manuales del usuario sobre un movimiento,
// identificado por la terna (Fecha, MontoOriginal, Descripcion). Dos ajustes
// ortogonales pueden convivir en el mismo registro:
//   - MiParte: lo que efectivamente me tocó pagar (split de gasto compartido),
//     en la misma moneda y signo que el monto original. nil = no hay split;
//     un puntero a 0 significa "No contar".
//   - Categoria: el id de categoría asignado a mano. "" = sin override de categoría.
type Override struct {
	Fecha         string   `json:"fecha"` // Formato ISO: yyyy-mm-dd
	MontoOriginal float64  `json:"montoOriginal"`
	Descripcion   string   `json:"descripcion"`
	MiParte       *float64 `json:"miParte,omitempty"`
	Categoria     string   `json:"categoria,omitempty"`
}

// LeerOverrides lee el archivo de ajustes locales, si existe.
func LeerOverrides(ruta string) ([]Override, error) {
	if ruta == "" {
		return []Override{}, nil
	}
	data, err := os.ReadFile(ruta)
	if err != nil {
		if os.IsNotExist(err) {
			return []Override{}, nil
		}
		return []Override{}, nil
	}
	var overrides []Override
	if err := json.Unmarshal(data, &overrides); err != nil {
		return nil, err
	}
	return overrides, nil
}

// GuardarOverride inserta o actualiza un ajuste preservando los otros campos
// del mismo movimiento.
func GuardarOverride(ruta string, override Override) error {
	overrides, err := LeerOverrides(ruta)
	if err != nil {
		overrides = []Override{}
	}

	found := false
	for i := range overrides {
		if overrides[i].Fecha == override.Fecha && overrides[i].MontoOriginal == override.MontoOriginal && overrides[i].Descripcion == override.Descripcion {
			if override.MiParte != nil {
				overrides[i].MiParte = override.MiParte
			}
			if override.Categoria != "" {
				overrides[i].Categoria = override.Categoria
			}
			found = true
			break
		}
	}
	if !found {
		overrides = append(overrides, override)
	}

	data, err := json.MarshalIndent(overrides, "", "    ")
	if err != nil {
		return err
	}
	return os.WriteFile(ruta, data, 0644)
}

// AplicarOverrides devuelve el monto crudo a imputar: "mi parte" si hay un
// override de split registrado para (fecha, montoOriginal, descripcion), o el
// monto original tal cual. La fecha debe venir en ISO (yyyy-mm-dd).
func AplicarOverrides(montoOriginal float64, fechaISO string, descripcion string, overrides []Override) float64 {
	for _, o := range overrides {
		if o.Descripcion == "" {
			continue
		}
		if o.Fecha == fechaISO && o.MontoOriginal == montoOriginal && o.Descripcion == descripcion {
			if o.MiParte != nil {
				return *o.MiParte
			}
			return montoOriginal
		}
	}
	return montoOriginal
}

// CategoriaOverride devuelve el id de categoría asignado a mano a un movimiento
// (terna fecha, monto, descripción), o "" si no hay override de categoría.
func CategoriaOverride(fechaISO string, montoOriginal float64, descripcion string, overrides []Override) string {
	for _, o := range overrides {
		if o.Descripcion == "" {
			continue
		}
		if o.Fecha == fechaISO && o.MontoOriginal == montoOriginal && o.Descripcion == descripcion {
			return o.Categoria
		}
	}
	return ""
}
