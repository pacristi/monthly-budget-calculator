package shared

import "testing"

func TestAplicarOverrides_MatchPorFechaMontoYDescripcion(t *testing.T) {
	overrides := []Override{
		{Fecha: "2025-05-15", MontoOriginal: -3500, Descripcion: "Starbucks café AM", MiParte: -1750},
	}

	t.Run("matchea cuando fecha, monto y descripción coinciden", func(t *testing.T) {
		got := AplicarOverrides(-3500, "2025-05-15", "Starbucks café AM", overrides)
		if got != -1750 {
			t.Errorf("esperaba -1750 (mi parte), obtuve %v", got)
		}
	})

	t.Run("NO matchea si la descripción es distinta", func(t *testing.T) {
		got := AplicarOverrides(-3500, "2025-05-15", "Starbucks café PM", overrides)
		if got != -3500 {
			t.Errorf("esperaba -3500 (monto original sin override), obtuve %v", got)
		}
	})

	t.Run("NO matchea si el override tiene descripción vacía", func(t *testing.T) {
		emptyDescOverrides := []Override{
			{Fecha: "2025-05-15", MontoOriginal: -3500, Descripcion: "", MiParte: -1750},
		}
		got := AplicarOverrides(-3500, "2025-05-15", "Starbucks café AM", emptyDescOverrides)
		if got != -3500 {
			t.Errorf("override con Descripcion vacía no debería matchear; esperaba -3500, obtuve %v", got)
		}
	})

	t.Run("NO matchea si la descripción del movimiento es vacía", func(t *testing.T) {
		got := AplicarOverrides(-3500, "2025-05-15", "", overrides)
		if got != -3500 {
			t.Errorf("movimiento con descripcion vacía no debería matchear un override con descripción; esperaba -3500, obtuve %v", got)
		}
	})
}
