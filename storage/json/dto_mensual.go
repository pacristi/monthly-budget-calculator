package config

import (
	"fmt"
	"time"

	"presupuesto/internal/presupuesto"
)

// ConfigMensual es la representación persistida de una config: el mes desde
// el que aplica y los tres valores. Es el tipo de transporte/storage; el dominio
// usa presupuesto.ConfigPresupuesto.
type ConfigMensual struct {
	MesDesde string `json:"mesDesde"` // formato "YYYY-MM"
	// Porcentajes asigna un % del sueldo a cada categoría (por id). Formato nuevo.
	Porcentajes          map[string]float64 `json:"porcentajes,omitempty"`
	PorcentajeParaGastos float64            `json:"porcentajeParaGastos,omitempty"` // formato viejo (lectura retrocompat)
	DiaDeCorteCredito    int                `json:"diaDeCorteCredito"`
	TasaCambioUSD        float64            `json:"tasaCambioUSD"`
}

// CategoriaGastoLegacy es el id de categoría al que se mapea el viejo
// porcentajeParaGastos cuando se lee una config en formato pre-categorías.
const CategoriaGastoLegacy = presupuesto.CategoriaPorDefecto

// Validar chequea las invariantes de una config.
func (c ConfigMensual) Validar() error {
	if _, err := ParseMes(c.MesDesde); err != nil {
		return fmt.Errorf("mesDesde inválido (%q): %w", c.MesDesde, err)
	}
	if c.PorcentajeParaGastos < 0 || c.PorcentajeParaGastos > 1 {
		return fmt.Errorf("porcentajeParaGastos debe estar en [0, 1], es %v", c.PorcentajeParaGastos)
	}
	if c.DiaDeCorteCredito < 1 || c.DiaDeCorteCredito > 31 {
		return fmt.Errorf("diaDeCorteCredito debe estar en [1, 31], es %v", c.DiaDeCorteCredito)
	}
	if c.TasaCambioUSD <= 0 {
		return fmt.Errorf("tasaCambioUSD debe ser > 0, es %v", c.TasaCambioUSD)
	}
	return nil
}

// ParseMes parsea un string "YYYY-MM" a time.Time normalizado al día 1.
func ParseMes(s string) (time.Time, error) {
	return time.Parse("2006-01", s)
}

// FormatMes formatea un time.Time como "YYYY-MM".
func FormatMes(t time.Time) string {
	return t.Format("2006-01")
}
