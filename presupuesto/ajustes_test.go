package presupuesto

import "testing"

func ptrF(f float64) *float64 { return &f }

func TestAplicarOverrides_PriorizaMovimientoID(t *testing.T) {
	overrides := []Override{
		{MovimientoID: "sql-1", Fecha: "2025-05-15", MontoOriginal: -3500, Descripcion: "Starbucks café AM", MiParte: ptrF(-1750)},
		{MovimientoID: "sql-2", Fecha: "2025-05-15", MontoOriginal: -3500, Descripcion: "Starbucks café AM", MiParte: ptrF(-500)},
	}

	got := AplicarOverrides("sql-2", -3500, "2025-05-15", "Starbucks café AM", overrides)
	if got != -500 {
		t.Fatalf("esperaba override por movimiento_id -500, obtuve %v", got)
	}
}

func TestAplicarOverrides_FallbackLegacyConMovimientoIDViejo(t *testing.T) {
	overrides := []Override{
		{MovimientoID: "sql-1", Fecha: "2025-05-15", MontoOriginal: -3500, Descripcion: "Starbucks café AM", MiParte: ptrF(-1750)},
	}

	got := AplicarOverrides("sql-50", -3500, "2025-05-15", "Starbucks café AM", overrides)
	if got != -1750 {
		t.Fatalf("esperaba fallback por terna con movimiento_id viejo, obtuve %v", got)
	}
}

func TestAplicarOverrides_FallbackLegacyPorFechaMontoYDescripcion(t *testing.T) {
	overrides := []Override{
		{Fecha: "2025-05-15", MontoOriginal: -3500, Descripcion: "Starbucks café AM", MiParte: ptrF(-1750)},
	}

	t.Run("matchea cuando fecha, monto y descripción coinciden", func(t *testing.T) {
		got := AplicarOverrides("sql-1", -3500, "2025-05-15", "Starbucks café AM", overrides)
		if got != -1750 {
			t.Errorf("esperaba -1750 (mi parte), obtuve %v", got)
		}
	})

	t.Run("NO matchea si la descripción es distinta", func(t *testing.T) {
		got := AplicarOverrides("sql-1", -3500, "2025-05-15", "Starbucks café PM", overrides)
		if got != -3500 {
			t.Errorf("esperaba -3500 (monto original sin override), obtuve %v", got)
		}
	})

	t.Run("NO matchea si el override tiene descripción vacía", func(t *testing.T) {
		emptyDescOverrides := []Override{
			{Fecha: "2025-05-15", MontoOriginal: -3500, Descripcion: "", MiParte: ptrF(-1750)},
		}
		got := AplicarOverrides("sql-1", -3500, "2025-05-15", "Starbucks café AM", emptyDescOverrides)
		if got != -3500 {
			t.Errorf("override con Descripcion vacía no debería matchear; esperaba -3500, obtuve %v", got)
		}
	})

	t.Run("NO matchea si la descripción del movimiento es vacía", func(t *testing.T) {
		got := AplicarOverrides("sql-1", -3500, "2025-05-15", "", overrides)
		if got != -3500 {
			t.Errorf("movimiento con descripcion vacía no debería matchear un override con descripción; esperaba -3500, obtuve %v", got)
		}
	})

	t.Run("MiParte=0 (No contar) sigue imputando 0", func(t *testing.T) {
		noContar := []Override{
			{Fecha: "2025-05-15", MontoOriginal: -3500, Descripcion: "Starbucks café AM", MiParte: ptrF(0)},
		}
		got := AplicarOverrides("sql-1", -3500, "2025-05-15", "Starbucks café AM", noContar)
		if got != 0 {
			t.Errorf("MiParte=0 debería imputar 0 (No contar), obtuve %v", got)
		}
	})

	t.Run("MiParte nil (override solo de categoría) NO toca el monto", func(t *testing.T) {
		soloCategoria := []Override{
			{Fecha: "2025-05-15", MontoOriginal: -3500, Descripcion: "Starbucks café AM", MiParte: nil, Categoria: "ahorro"},
		}
		got := AplicarOverrides("sql-1", -3500, "2025-05-15", "Starbucks café AM", soloCategoria)
		if got != -3500 {
			t.Errorf("override sin MiParte no debería cambiar el monto; esperaba -3500, obtuve %v", got)
		}
	})
}

func ptrB(b bool) *bool { return &b }

func TestMonedaOverride_SinOverrideDevuelveNil(t *testing.T) {
	overrides := []Override{
		{Fecha: "2025-05-15", MontoOriginal: -3, Descripcion: "WINDSCRIBE", Categoria: "software"},
	}
	got := MonedaOverride("sql-1", "2025-05-15", -3, "WINDSCRIBE", overrides)
	if got != nil {
		t.Fatalf("esperaba nil (sin override de moneda), obtuve %v", *got)
	}
}

func TestMonedaOverride_ConOverridePriorizaMovimientoID(t *testing.T) {
	overrides := []Override{
		{MovimientoID: "sql-1", Fecha: "2025-05-15", MontoOriginal: -3, Descripcion: "WINDSCRIBE", EsUSD: ptrB(true)},
	}
	got := MonedaOverride("sql-1", "2025-05-15", -3, "WINDSCRIBE", overrides)
	if got == nil || *got != true {
		t.Fatalf("esperaba override *true, obtuve %v", got)
	}
}

func TestMonedaOverride_FallbackLegacyPorTerna(t *testing.T) {
	overrides := []Override{
		{Fecha: "2025-05-15", MontoOriginal: -3, Descripcion: "WINDSCRIBE", EsUSD: ptrB(true)},
	}
	got := MonedaOverride("sql-99", "2025-05-15", -3, "WINDSCRIBE", overrides)
	if got == nil || *got != true {
		t.Fatalf("esperaba fallback por terna *true, obtuve %v", got)
	}
}

func TestMonedaOverride_PermiteForzarCLP(t *testing.T) {
	overrides := []Override{
		{MovimientoID: "sql-1", Fecha: "2025-05-15", MontoOriginal: -3, Descripcion: "WINDSCRIBE", EsUSD: ptrB(false)},
	}
	got := MonedaOverride("sql-1", "2025-05-15", -3, "WINDSCRIBE", overrides)
	if got == nil || *got != false {
		t.Fatalf("esperaba override *false (forzar CLP), obtuve %v", got)
	}
}
