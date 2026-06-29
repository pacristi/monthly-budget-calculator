package presentacion

import (
	"presupuesto/internal/ajustes"
	"presupuesto/internal/presupuesto"
)

type Presentador interface {
	PresentarMovimientos() ([]Movimiento, error)
}

// Movimiento es la vista estable que comparten las interfaces de usuario.
type Movimiento struct {
	ID                  string   `json:"id"`
	Fecha               string   `json:"fecha"`
	Descripcion         string   `json:"descripcion"`
	DescripcionOriginal string   `json:"descripcionOriginal,omitempty"`
	Monto               float64  `json:"monto"`
	IsUSD               bool     `json:"isUsd"`
	MiParte             *float64 `json:"miParte,omitempty"`
	CategoriaID         string   `json:"categoriaId"`
}

// Movimientos aplica preferencias de presentación sin contaminar el dominio de
// presupuesto ni obligar a cada handler a reconstruir la misma vista.
func Movimientos(movs []presupuesto.Movimiento, overrides []ajustes.Override) []Movimiento {
	vista := make([]Movimiento, 0, len(movs))
	for _, m := range movs {
		descripcion := descripcionMovimiento(m, overrides)
		descripcionOriginal := ""
		if descripcion != m.Descripcion {
			descripcionOriginal = m.Descripcion
		}
		vista = append(vista, Movimiento{
			ID:                  m.ID,
			Fecha:               m.Fecha.Format("2006-01-02"),
			Descripcion:         descripcion,
			DescripcionOriginal: descripcionOriginal,
			Monto:               m.Monto,
			IsUSD:               m.IsUSD,
			MiParte:             m.MiParte,
			CategoriaID:         m.CategoriaID,
		})
	}
	return vista
}

func descripcionMovimiento(m presupuesto.Movimiento, overrides []ajustes.Override) string {
	fechaISO := m.Fecha.Format("2006-01-02")
	if desc := ajustes.DescripcionOverride(m.ID, fechaISO, m.Monto, m.Descripcion, overrides); desc != "" {
		return desc
	}
	return m.Descripcion
}
