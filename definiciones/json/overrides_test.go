package defjson

import (
	"testing"

	"presupuesto/presupuesto"
)

func ptrF(f float64) *float64 { return &f }

func TestGuardarMiPartePreservaCategoria(t *testing.T) {
	ruta := t.TempDir() + "/overrides.json"

	if err := GuardarCategoria(ruta, presupuesto.Override{
		Fecha:         "2025-05-15",
		MontoOriginal: -3500,
		Descripcion:   "Starbucks café AM",
		Categoria:     "cafes",
	}); err != nil {
		t.Fatalf("guardando categoría: %v", err)
	}

	if err := GuardarMiParte(ruta, presupuesto.Override{
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

	if err := GuardarMiParte(ruta, presupuesto.Override{
		MovimientoID:  "sql-42",
		Fecha:         "2025-05-15",
		MontoOriginal: -3500,
		Descripcion:   "STARBUCKS PROVIDENCIA",
		MiParte:       &miParte,
		Categoria:     "cafes",
	}); err != nil {
		t.Fatalf("guardando mi parte: %v", err)
	}

	if err := GuardarAlias(ruta, presupuesto.Override{
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

	if err := GuardarCategoria(ruta, presupuesto.Override{
		MovimientoID:  "sql-42",
		Fecha:         "2025-05-15",
		MontoOriginal: -3500,
		Descripcion:   "Descripcion original",
		Categoria:     "cafes",
	}); err != nil {
		t.Fatalf("guardando categoría: %v", err)
	}

	if err := GuardarMiParte(ruta, presupuesto.Override{
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

	if err := GuardarCategoria(ruta, presupuesto.Override{
		MovimientoID:  "sql-1",
		Fecha:         "2025-05-15",
		MontoOriginal: -3500,
		Descripcion:   "Starbucks café AM",
		Categoria:     "cafes",
	}); err != nil {
		t.Fatalf("guardando categoría: %v", err)
	}

	if err := GuardarMiParte(ruta, presupuesto.Override{
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

	if err := GuardarMiParte(ruta, presupuesto.Override{
		Fecha:         "2025-05-15",
		MontoOriginal: -3500,
		Descripcion:   "Starbucks café AM",
		MiParte:       ptrF(-1750),
		Categoria:     "cafes",
	}); err != nil {
		t.Fatalf("guardando mi parte: %v", err)
	}
	if err := GuardarCategoria(ruta, presupuesto.Override{
		Fecha:         "2025-05-15",
		MontoOriginal: -3500,
		Descripcion:   "Starbucks café AM",
		Categoria:     "cafes",
	}); err != nil {
		t.Fatalf("guardando categoría: %v", err)
	}

	if err := GuardarCategoria(ruta, presupuesto.Override{
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
