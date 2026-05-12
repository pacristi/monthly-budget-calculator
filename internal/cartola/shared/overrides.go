package shared

import (
	"encoding/json"
	"os"
)

// Override representa lo que efectivamente me tocó pagar de un movimiento,
// en la misma moneda y signo que el monto original (cruda).
type Override struct {
	Fecha         string  `json:"fecha"`
	MontoOriginal float64 `json:"montoOriginal"`
	MiParte       float64 `json:"miParte"`
}

// LeerOverrides lee el archivo de reglas locales, si existe.
func LeerOverrides(ruta string) ([]Override, error) {
	if ruta == "" {
		return []Override{}, nil
	}
	data, err := os.ReadFile(ruta)
	if err != nil {
		// Toleramos que no exista el archivo de overrides
		return []Override{}, nil
	}
	var overrides []Override
	if err := json.Unmarshal(data, &overrides); err != nil {
		return nil, err
	}
	return overrides, nil
}

// AplicarOverrides devuelve el monto crudo a imputar: "mi parte" si hay
// override registrado para (fecha, montoOriginal), o el monto original tal cual.
func AplicarOverrides(montoOriginal float64, fechaCruda string, overrides []Override) float64 {
	for _, o := range overrides {
		if o.Fecha == fechaCruda && o.MontoOriginal == montoOriginal {
			return o.MiParte
		}
	}
	return montoOriginal
}
