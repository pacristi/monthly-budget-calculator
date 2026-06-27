package shared

import "testing"

func ptrF(f float64) *float64 { return &f }

func TestAplicarOverrides_MatchPorFechaMontoYDescripcion(t *testing.T) {
	overrides := []Override{
		{Fecha: "2025-05-15", MontoOriginal: -3500, Descripcion: "Starbucks café AM", MiParte: ptrF(-1750)},
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
			{Fecha: "2025-05-15", MontoOriginal: -3500, Descripcion: "", MiParte: ptrF(-1750)},
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

	t.Run("MiParte=0 (No contar) sigue imputando 0", func(t *testing.T) {
		noContar := []Override{
			{Fecha: "2025-05-15", MontoOriginal: -3500, Descripcion: "Starbucks café AM", MiParte: ptrF(0)},
		}
		got := AplicarOverrides(-3500, "2025-05-15", "Starbucks café AM", noContar)
		if got != 0 {
			t.Errorf("MiParte=0 debería imputar 0 (No contar), obtuve %v", got)
		}
	})

	t.Run("MiParte nil (override solo de categoría) NO toca el monto", func(t *testing.T) {
		soloCategoria := []Override{
			{Fecha: "2025-05-15", MontoOriginal: -3500, Descripcion: "Starbucks café AM", MiParte: nil, Categoria: "ahorro"},
		}
		got := AplicarOverrides(-3500, "2025-05-15", "Starbucks café AM", soloCategoria)
		if got != -3500 {
			t.Errorf("override sin MiParte no debería cambiar el monto; esperaba -3500, obtuve %v", got)
		}
	})
}

func TestNombreOverride_MatchPorFechaMontoYDescripcion(t *testing.T) {
	overrides := []Override{
		{Fecha: "2025-05-15", MontoOriginal: -3500, Descripcion: "SBX123", Nombre: "Starbucks"},
	}

	got := NombreOverride("2025-05-15", -3500, "SBX123", overrides)
	if got != "Starbucks" {
		t.Fatalf("esperaba Starbucks, obtuve %q", got)
	}

	got = NombreOverride("2025-05-15", -3500, "OTRO", overrides)
	if got != "" {
		t.Fatalf("no debería matchear otra descripción, obtuve %q", got)
	}
}
