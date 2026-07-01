package json

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

func TestGuardarMiPartePreservaCategoria(t *testing.T) {
	ruta := t.TempDir() + "/overrides.json"

	if err := GuardarCategoria(ruta, Override{
		Fecha:         "2025-05-15",
		MontoOriginal: -3500,
		Descripcion:   "Starbucks café AM",
		Categoria:     "cafes",
	}); err != nil {
		t.Fatalf("guardando categoría: %v", err)
	}

	if err := GuardarMiParte(ruta, Override{
		Fecha:         "2025-05-15",
		MontoOriginal: -3500,
		Descripcion:   "Starbucks café AM",
		MiParte:       ptrF(-1750),
	}); err != nil {
		t.Fatalf("guardando mi parte: %v", err)
	}

	overrides, err := LeerOverrides(ruta)
	if err != nil {
		t.Fatalf("leyendo overrides: %v", err)
	}
	if len(overrides) != 1 {
		t.Fatalf("esperaba 1 override, obtuve %d", len(overrides))
	}
	if overrides[0].Categoria != "cafes" {
		t.Fatalf("GuardarMiParte debe preservar categoría, obtuvo %q", overrides[0].Categoria)
	}
	if overrides[0].MiParte == nil || *overrides[0].MiParte != -1750 {
		t.Fatalf("MiParte no guardada correctamente: %+v", overrides[0].MiParte)
	}
}

func TestGuardarAliasPreservaAjustesContables(t *testing.T) {
	ruta := t.TempDir() + "/overrides.json"
	miParte := -1750.0

	if err := GuardarMiParte(ruta, Override{
		MovimientoID:  "sql-42",
		Fecha:         "2025-05-15",
		MontoOriginal: -3500,
		Descripcion:   "STARBUCKS PROVIDENCIA",
		MiParte:       &miParte,
		Categoria:     "cafes",
	}); err != nil {
		t.Fatalf("guardando mi parte: %v", err)
	}

	if err := GuardarAlias(ruta, Override{
		MovimientoID:  "sql-42",
		Fecha:         "2025-05-15",
		MontoOriginal: -3500,
		Descripcion:   "STARBUCKS PROVIDENCIA",
		Alias:         "Café",
	}); err != nil {
		t.Fatalf("guardando alias: %v", err)
	}

	overrides, err := LeerOverrides(ruta)
	if err != nil {
		t.Fatalf("leyendo overrides: %v", err)
	}
	if len(overrides) != 1 {
		t.Fatalf("esperaba 1 override, obtuve %d", len(overrides))
	}
	if overrides[0].Descripcion != "STARBUCKS PROVIDENCIA" {
		t.Fatalf("la descripción original debe quedar intacta, obtuvo %q", overrides[0].Descripcion)
	}
	if overrides[0].Alias != "Café" {
		t.Fatalf("alias no guardado correctamente: %+v", overrides[0])
	}
	if overrides[0].Categoria != "cafes" {
		t.Fatalf("GuardarAlias debe preservar categoría, obtuvo %q", overrides[0].Categoria)
	}
	if overrides[0].MiParte == nil || *overrides[0].MiParte != -1750 {
		t.Fatalf("GuardarAlias debe preservar miParte, obtuvo %+v", overrides[0].MiParte)
	}
}

func TestGuardarMiParteMatcheaPorMovimientoIDAunqueCambieDescripcion(t *testing.T) {
	ruta := t.TempDir() + "/overrides.json"

	if err := GuardarCategoria(ruta, Override{
		MovimientoID:  "sql-42",
		Fecha:         "2025-05-15",
		MontoOriginal: -3500,
		Descripcion:   "Descripcion original",
		Categoria:     "cafes",
	}); err != nil {
		t.Fatalf("guardando categoría: %v", err)
	}

	if err := GuardarMiParte(ruta, Override{
		MovimientoID:  "sql-42",
		Fecha:         "2025-05-15",
		MontoOriginal: -3500,
		Descripcion:   "Alias editado",
		MiParte:       ptrF(-1750),
	}); err != nil {
		t.Fatalf("guardando mi parte: %v", err)
	}

	overrides, err := LeerOverrides(ruta)
	if err != nil {
		t.Fatalf("leyendo overrides: %v", err)
	}
	if len(overrides) != 1 {
		t.Fatalf("esperaba 1 override, obtuve %d", len(overrides))
	}
	if overrides[0].Descripcion != "Descripcion original" {
		t.Fatalf("la descripción original debe quedar intacta, obtuvo %q", overrides[0].Descripcion)
	}
	if overrides[0].Categoria != "cafes" {
		t.Fatalf("GuardarMiParte debe preservar categoría, obtuvo %q", overrides[0].Categoria)
	}
	if overrides[0].MiParte == nil || *overrides[0].MiParte != -1750 {
		t.Fatalf("MiParte no guardada correctamente: %+v", overrides[0].MiParte)
	}
}

func TestGuardarMiParteActualizaMovimientoIDViejoPorTerna(t *testing.T) {
	ruta := t.TempDir() + "/overrides.json"

	if err := GuardarCategoria(ruta, Override{
		MovimientoID:  "sql-1",
		Fecha:         "2025-05-15",
		MontoOriginal: -3500,
		Descripcion:   "Starbucks café AM",
		Categoria:     "cafes",
	}); err != nil {
		t.Fatalf("guardando categoría: %v", err)
	}

	if err := GuardarMiParte(ruta, Override{
		MovimientoID:  "sql-50",
		Fecha:         "2025-05-15",
		MontoOriginal: -3500,
		Descripcion:   "Starbucks café AM",
		MiParte:       ptrF(-1750),
	}); err != nil {
		t.Fatalf("guardando mi parte: %v", err)
	}

	overrides, err := LeerOverrides(ruta)
	if err != nil {
		t.Fatalf("leyendo overrides: %v", err)
	}
	if len(overrides) != 1 {
		t.Fatalf("esperaba actualizar override existente, obtuve %d", len(overrides))
	}
	if overrides[0].MovimientoID != "sql-50" {
		t.Fatalf("esperaba movimientoId actualizado a sql-50, obtuvo %q", overrides[0].MovimientoID)
	}
	if overrides[0].Categoria != "cafes" {
		t.Fatalf("GuardarMiParte debe preservar categoría, obtuvo %q", overrides[0].Categoria)
	}
	if overrides[0].MiParte == nil || *overrides[0].MiParte != -1750 {
		t.Fatalf("MiParte no guardada correctamente: %+v", overrides[0].MiParte)
	}
}

func TestGuardarCategoriaPermiteLimpiarCategoria(t *testing.T) {
	ruta := t.TempDir() + "/overrides.json"

	if err := GuardarMiParte(ruta, Override{
		Fecha:         "2025-05-15",
		MontoOriginal: -3500,
		Descripcion:   "Starbucks café AM",
		MiParte:       ptrF(-1750),
		Categoria:     "cafes",
	}); err != nil {
		t.Fatalf("guardando mi parte: %v", err)
	}
	if err := GuardarCategoria(ruta, Override{
		Fecha:         "2025-05-15",
		MontoOriginal: -3500,
		Descripcion:   "Starbucks café AM",
		Categoria:     "cafes",
	}); err != nil {
		t.Fatalf("guardando categoría: %v", err)
	}

	if err := GuardarCategoria(ruta, Override{
		Fecha:         "2025-05-15",
		MontoOriginal: -3500,
		Descripcion:   "Starbucks café AM",
		Categoria:     "",
	}); err != nil {
		t.Fatalf("limpiando categoría: %v", err)
	}

	overrides, err := LeerOverrides(ruta)
	if err != nil {
		t.Fatalf("leyendo overrides: %v", err)
	}
	if len(overrides) != 1 {
		t.Fatalf("esperaba 1 override, obtuve %d", len(overrides))
	}
	if overrides[0].Categoria != "" {
		t.Fatalf("GuardarCategoria debe permitir limpiar categoría, obtuvo %q", overrides[0].Categoria)
	}
	if overrides[0].MiParte == nil || *overrides[0].MiParte != -1750 {
		t.Fatalf("GuardarCategoria debe preservar MiParte, obtuvo %+v", overrides[0].MiParte)
	}
}
