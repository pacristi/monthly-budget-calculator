package presupuesto

import "time"

// ConfigPresupuesto son los valores de configuración resueltos para un mes
// específico, junto con metadata de qué declaración los originó.
type ConfigPresupuesto struct {
	PorcentajeParaGastos float64 `json:"porcentajeParaGastos"`
	DiaDeCorteCredito    int     `json:"diaDeCorteCredito"`
	TasaCambioUSD        float64 `json:"tasaCambioUSD"`
	HeredadaDe           string  `json:"heredadaDe"`
}

// ResolvedorConfig es el puerto que el dominio usa para obtener la configuración
// aplicable a un mes. La implementación (archivo JSON, BD, etc.) vive en infra.
type ResolvedorConfig interface {
	ParaMes(mes time.Time) (ConfigPresupuesto, error)
}
