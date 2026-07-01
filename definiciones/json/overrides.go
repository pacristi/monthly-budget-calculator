package defjson

import (
	"encoding/json"
	"os"

	"presupuesto/presupuesto"
)

// LeerOverrides lee el archivo de ajustes locales, si existe.
func LeerOverrides(ruta string) ([]presupuesto.Override, error) {
	if ruta == "" {
		return []presupuesto.Override{}, nil
	}
	data, err := os.ReadFile(ruta)
	if err != nil {
		if os.IsNotExist(err) {
			return []presupuesto.Override{}, nil
		}
		return []presupuesto.Override{}, nil
	}
	var overrides []presupuesto.Override
	if err := json.Unmarshal(data, &overrides); err != nil {
		return nil, err
	}
	return overrides, nil
}

// GuardarMiParte inserta o actualiza el ajuste de split de un movimiento,
// preservando su categoría manual si ya existía.
func GuardarMiParte(ruta string, override presupuesto.Override) error {
	return guardarOverride(ruta, override, func(actual *presupuesto.Override, nuevo presupuesto.Override) {
		actual.MiParte = nuevo.MiParte
	})
}

// GuardarCategoria inserta o actualiza la categoría manual de un movimiento,
// preservando su split si ya existía. Categoria="" limpia la categoría manual.
func GuardarCategoria(ruta string, override presupuesto.Override) error {
	return guardarOverride(ruta, override, func(actual *presupuesto.Override, nuevo presupuesto.Override) {
		actual.Categoria = nuevo.Categoria
	})
}

// GuardarAlias inserta o actualiza la descripción visible del movimiento,
// preservando sus ajustes contables. Alias="" limpia el alias.
func GuardarAlias(ruta string, override presupuesto.Override) error {
	return guardarOverride(ruta, override, func(actual *presupuesto.Override, nuevo presupuesto.Override) {
		actual.Alias = nuevo.Alias
	})
}

func guardarOverride(ruta string, override presupuesto.Override, aplicar func(*presupuesto.Override, presupuesto.Override)) error {
	overrides, err := LeerOverrides(ruta)
	if err != nil {
		overrides = []presupuesto.Override{}
	}

	found := false
	for i := range overrides {
		if presupuesto.MismoOverrideObjetivo(overrides[i], override) {
			aplicar(&overrides[i], override)
			if override.MovimientoID != "" {
				overrides[i].MovimientoID = override.MovimientoID
			}
			found = true
			break
		}
	}
	if !found {
		aplicar(&override, override)
		if override.MiParte == nil && override.Categoria == "" && override.Alias == "" {
			return escribirOverrides(ruta, overrides)
		}
		overrides = append(overrides, override)
	}

	return escribirOverrides(ruta, overrides)
}

func escribirOverrides(ruta string, overrides []presupuesto.Override) error {
	data, err := json.MarshalIndent(overrides, "", "    ")
	if err != nil {
		return err
	}
	return escribirArchivoAtomico(ruta, data)
}
