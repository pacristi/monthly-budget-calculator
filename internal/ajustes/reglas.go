package ajustes

import (
	"encoding/json"
	"os"

	"presupuesto/internal/presupuesto"
)

// LeerReglas lee el archivo de reglas de categorización, con formato
// `[{"patron":"...","destino":"..."}]`. Tolera archivo inexistente.
func LeerReglas(ruta string) ([]presupuesto.Regla, error) {
	if ruta == "" {
		return nil, nil
	}
	data, err := os.ReadFile(ruta)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var reglas []presupuesto.Regla
	if err := json.Unmarshal(data, &reglas); err != nil {
		return nil, err
	}
	return reglas, nil
}

// EscribirReglas persiste la lista de reglas como JSON. Sobreescribe si existe.
func EscribirReglas(ruta string, reglas []presupuesto.Regla) error {
	if ruta == "" {
		return nil
	}
	if reglas == nil {
		reglas = []presupuesto.Regla{}
	}
	data, err := json.MarshalIndent(reglas, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(ruta, data, 0644)
}

// MigrarExclusionesAReglas convierte la vieja lista de substrings a ignorar en
// reglas con destino Ignorado, preservando el comportamiento previo.
func MigrarExclusionesAReglas(exclusiones []string) []presupuesto.Regla {
	reglas := make([]presupuesto.Regla, 0, len(exclusiones))
	for _, e := range exclusiones {
		reglas = append(reglas, presupuesto.Regla{Patron: e, Destino: presupuesto.Ignorado})
	}
	return reglas
}

// CargarReglas resuelve las reglas efectivas (lectura retrocompatible): si
// existe el archivo de reglas, lo usa; si no, migra el viejo exclusiones.json.
func CargarReglas(rutaReglas, rutaExclusiones string) ([]presupuesto.Regla, error) {
	reglas, err := LeerReglas(rutaReglas)
	if err != nil {
		return nil, err
	}
	if len(reglas) > 0 {
		return reglas, nil
	}
	exclusiones, err := LeerListaStrings(rutaExclusiones)
	if err != nil {
		return nil, err
	}
	return MigrarExclusionesAReglas(exclusiones), nil
}

func LeerListaStrings(ruta string) ([]string, error) {
	if ruta == "" {
		return nil, nil
	}
	data, err := os.ReadFile(ruta)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var out []string
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// EscribirListaStrings persiste una lista de strings como JSON con
// indentación. Sobreescribe el archivo si ya existe.
func EscribirListaStrings(ruta string, lista []string) error {
	if ruta == "" {
		return nil
	}
	if lista == nil {
		lista = []string{}
	}
	data, err := json.MarshalIndent(lista, "", "    ")
	if err != nil {
		return err
	}
	return os.WriteFile(ruta, data, 0644)
}
