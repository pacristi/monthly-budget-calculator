package presupuesto

import "time"

// ConfigPresupuesto son los valores de configuración resueltos para un mes
// específico, junto con metadata de qué declaración los originó.
type ConfigPresupuesto struct {
	// Porcentajes es el % del sueldo asignado a cada categoría (por id).
	Porcentajes          map[string]float64 `json:"porcentajes"`
	PorcentajeParaGastos float64            `json:"porcentajeParaGastos"` // legacy (lectura retrocompat)
	DiaDeCorteCredito    int                `json:"diaDeCorteCredito"`
	TasaCambioUSD        float64            `json:"tasaCambioUSD"`
	HeredadaDe           string             `json:"heredadaDe"`
}

// ResolvedorConfig es el puerto que el dominio usa para obtener la configuración
// aplicable a un mes. La implementación (archivo JSON, BD, etc.) vive en infra.
type ResolvedorConfig interface {
	ParaMes(mes time.Time) (ConfigPresupuesto, error)
}
