package app

import (
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	_ "modernc.org/sqlite"

	"presupuesto/movimientos/sqlite"
	"presupuesto/presupuesto"
)

// nuevaAppTest arma un App sobre un directorio temporal con una sqlite de
// archivo real y archivos de estado semilla. Devuelve el App y el path de la BD
// para insertar movimientos directamente.
func nuevaAppTest(t *testing.T) (*App, string) {
	t.Helper()
	dir := t.TempDir()

	// Configs: 100% del sueldo a la categoría "gasto" (límite), tasa USD 900.
	configs := []map[string]any{{
		"mesDesde":          "2026-01",
		"porcentajes":       map[string]float64{"gasto": 1.0},
		"diaDeCorteCredito": 5,
		"tasaCambioUSD":     900.0,
	}}
	escribirJSON(t, filepath.Join(dir, "configs.json"), configs)

	categorias := []presupuesto.Categoria{
		{ID: "gasto", Nombre: "Gasto", Tipo: presupuesto.Limite},
		{ID: "ahorro", Nombre: "Ahorro", Tipo: presupuesto.Meta},
	}
	escribirJSON(t, filepath.Join(dir, "categorias.json"), categorias)

	escribirJSON(t, filepath.Join(dir, "sueldo.json"), []string{"REMUNERACION"})
	escribirJSON(t, filepath.Join(dir, "reglas.json"), []presupuesto.Regla{})

	cfg := Config{
		ConfigsPath:     filepath.Join(dir, "configs.json"),
		CategoriasPath:  filepath.Join(dir, "categorias.json"),
		ReglasPath:      filepath.Join(dir, "reglas.json"),
		ExclusionesPath: filepath.Join(dir, "exclusiones.json"),
		DivisionesPath:  filepath.Join(dir, "divisiones.json"),
		SueldoPath:      filepath.Join(dir, "sueldo.json"),
		ManualesPath:    filepath.Join(dir, "manuales.json"),
		DBPath:          filepath.Join(dir, "mov.db"),
		ProvisorioPath:  filepath.Join(dir, "current.json"),
	}

	a, err := New(cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(func() { a.Close() })
	return a, cfg.DBPath
}

func escribirJSON(t *testing.T, ruta string, v any) {
	t.Helper()
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		t.Fatalf("marshal %s: %v", ruta, err)
	}
	if err := os.WriteFile(ruta, data, 0o644); err != nil {
		t.Fatalf("write %s: %v", ruta, err)
	}
}

// insertarMov inserta un movimiento canónico simple en la BD del App.
func insertarMov(t *testing.T, dbPath, fecha string, monto float64, desc string) {
	t.Helper()
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()
	_, err = db.Exec(`INSERT INTO movimientos
		(banco, source, fecha, monto, descripcion, is_usd, cuotas, raw, origen, fecha_carga, instrumento, moneda, cuotas_totales)
		VALUES ('bchile', 'account', ?, ?, ?, 0, '01/01', '{}', 'test', '2026-05-15T00:00:00Z', 'cuenta_corriente', 'CLP', 1)`,
		fecha, monto, desc)
	if err != nil {
		t.Fatalf("insert: %v", err)
	}
}

func mayo2026() time.Time {
	return time.Date(2026, time.May, 15, 0, 0, 0, 0, time.UTC)
}

func TestResumenDelMes(t *testing.T) {
	a, dbPath := nuevaAppTest(t)

	insertarMov(t, dbPath, "2026-05-03", -100, "SUPERMERCADO")
	insertarMov(t, dbPath, "2026-05-10", -50, "FARMACIA")
	insertarMov(t, dbPath, "2026-04-30", 1000, "REMUNERACION MENSUAL") // sueldo (ventana)

	res, err := a.ResumenDelMes(mayo2026())
	if err != nil {
		t.Fatalf("ResumenDelMes: %v", err)
	}

	if res.Sueldo != 1000 {
		t.Errorf("Sueldo = %v, want 1000", res.Sueldo)
	}
	if len(res.Categorias) != 2 {
		t.Fatalf("Categorias = %d, want 2", len(res.Categorias))
	}
	var gastoCat CategoriaResumen
	for _, c := range res.Categorias {
		if c.ID == "gasto" {
			gastoCat = c
		}
	}
	if gastoCat.Presupuesto != 1000 { // 1000 * 1.0
		t.Errorf("gasto.Presupuesto = %v, want 1000", gastoCat.Presupuesto)
	}
	if gastoCat.Acumulado != 150 {
		t.Errorf("gasto.Acumulado = %v, want 150", gastoCat.Acumulado)
	}
	// Detalle: solo categorías límite con carga > 0. Ambos cargos caen en "gasto"
	// (default), que es límite.
	if len(res.Gastos) != 2 {
		t.Errorf("Gastos detalle = %d, want 2", len(res.Gastos))
	}
	if res.Config.TasaCambioUSD != 900 {
		t.Errorf("Config.TasaCambioUSD = %v, want 900", res.Config.TasaCambioUSD)
	}
}

func TestProyecciones(t *testing.T) {
	a, dbPath := nuevaAppTest(t)
	insertarMov(t, dbPath, "2026-05-03", -100, "SUPERMERCADO")

	proys, err := a.Proyecciones(mayo2026(), 3)
	if err != nil {
		t.Fatalf("Proyecciones: %v", err)
	}
	if len(proys) != 3 {
		t.Fatalf("Proyecciones = %d, want 3", len(proys))
	}
	if proys[0].Mes != "Mayo" || proys[0].MesNum != 5 || proys[0].Anio != 2026 {
		t.Errorf("proy[0] = %+v, want Mayo/5/2026", proys[0])
	}
}

func TestMovimientos(t *testing.T) {
	a, dbPath := nuevaAppTest(t)
	insertarMov(t, dbPath, "2026-05-03", -100, "SUPERMERCADO")
	insertarMov(t, dbPath, "2026-05-10", -50, "FARMACIA")

	movs, err := a.Movimientos()
	if err != nil {
		t.Fatalf("Movimientos: %v", err)
	}
	if len(movs) != 2 {
		t.Fatalf("Movimientos = %d, want 2", len(movs))
	}
	// Orden preservado de la query (fecha ASC, id ASC).
	if movs[0].Descripcion != "SUPERMERCADO" || movs[1].Descripcion != "FARMACIA" {
		t.Errorf("orden inesperado: %q, %q", movs[0].Descripcion, movs[1].Descripcion)
	}
	if movs[0].Fecha != "2026-05-03" {
		t.Errorf("Fecha = %q, want 2026-05-03", movs[0].Fecha)
	}
	if movs[0].MiParte != nil {
		t.Errorf("MiParte = %v, want nil", *movs[0].MiParte)
	}
}

// TestGuardarAliasRefrescaCache verifica que tras GuardarAlias el cache de
// overrides se recarga y las lecturas reflejan el alias sin reconstruir el App.
func TestGuardarAliasRefrescaCache(t *testing.T) {
	a, dbPath := nuevaAppTest(t)
	insertarMov(t, dbPath, "2026-05-03", -100, "COMERCIO XYZ 123")

	movs, _ := a.Movimientos()
	id := movs[0].ID

	err := a.GuardarAlias(presupuesto.Override{
		MovimientoID: id,
		Fecha:        "2026-05-03",
		Alias:        "Netflix",
	})
	if err != nil {
		t.Fatalf("GuardarAlias: %v", err)
	}

	movs, err = a.Movimientos()
	if err != nil {
		t.Fatalf("Movimientos post-alias: %v", err)
	}
	if movs[0].Descripcion != "Netflix" {
		t.Errorf("Descripcion = %q, want Netflix (cache no refrescado)", movs[0].Descripcion)
	}
	if movs[0].DescripcionOriginal != "COMERCIO XYZ 123" {
		t.Errorf("DescripcionOriginal = %q, want COMERCIO XYZ 123", movs[0].DescripcionOriginal)
	}
}

// TestGuardarMiParteRefrescaCache verifica que el split se refleja en las
// lecturas tras guardarlo.
func TestGuardarMiParteRefrescaCache(t *testing.T) {
	a, dbPath := nuevaAppTest(t)
	insertarMov(t, dbPath, "2026-05-03", -100, "CENA COMPARTIDA")

	movs, _ := a.Movimientos()
	id := movs[0].ID

	mitad := 50.0
	if err := a.GuardarMiParte(presupuesto.Override{
		MovimientoID: id,
		Fecha:        "2026-05-03",
		MiParte:      &mitad,
	}); err != nil {
		t.Fatalf("GuardarMiParte: %v", err)
	}

	movs, _ = a.Movimientos()
	if movs[0].MiParte == nil || *movs[0].MiParte != 50 {
		t.Errorf("MiParte = %v, want 50", movs[0].MiParte)
	}
}

// TestGuardarReglasRefrescaCache verifica que una regla nueva reclasifica los
// movimientos en las lecturas siguientes.
func TestGuardarReglasRefrescaCache(t *testing.T) {
	a, dbPath := nuevaAppTest(t)
	insertarMov(t, dbPath, "2026-05-03", -100, "BANCO ESTADO AHORRO")

	if err := a.GuardarReglas([]presupuesto.Regla{
		{Patron: "AHORRO", Destino: "ahorro"},
	}); err != nil {
		t.Fatalf("GuardarReglas: %v", err)
	}

	movs, _ := a.Movimientos()
	if movs[0].CategoriaID != "ahorro" {
		t.Errorf("CategoriaID = %q, want ahorro (regla no aplicada)", movs[0].CategoriaID)
	}
}

func TestConfigResueltaYCRUD(t *testing.T) {
	a, _ := nuevaAppTest(t)

	cfg, err := a.ConfigResuelta(mayo2026())
	if err != nil {
		t.Fatalf("ConfigResuelta: %v", err)
	}
	if cfg.TasaCambioUSD != 900 {
		t.Errorf("TasaCambioUSD = %v, want 900", cfg.TasaCambioUSD)
	}

	configs, err := a.Configs()
	if err != nil {
		t.Fatalf("Configs: %v", err)
	}
	if len(configs) == 0 {
		t.Error("Configs vacío, want al menos 1")
	}
}

// verifica que el driver sqlite embebido corre migraciones (smoke).
func TestNewMigra(t *testing.T) {
	a, dbPath := nuevaAppTest(t)
	_ = a
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()
	if _, err := sqlite.Cargos(db); err != nil {
		t.Fatalf("Cargos tras migrar: %v", err)
	}
}
