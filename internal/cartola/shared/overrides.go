package shared

import (
	"encoding/json"
	"os"
)

// Override representa una regla de split para gastos compartidos.
type Override struct {
	Fecha         string  `json:"fecha"`
	MontoOriginal float64 `json:"montoOriginal"`
	DivididoEn    int     `json:"divididoEn"`
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

// AplicarOverrides busca si el monto coincide con una regla de split y retorna el monto final.
func AplicarOverrides(montoImputado float64, montoOriginalCrudo float64, fechaCruda string, overrides []Override) float64 {
	for _, div := range overrides {
		if div.Fecha == fechaCruda && div.MontoOriginal == montoOriginalCrudo {
			if div.DivididoEn > 0 {
				return montoImputado / float64(div.DivididoEn)
			}
			break
		}
	}
	return montoImputado
}
