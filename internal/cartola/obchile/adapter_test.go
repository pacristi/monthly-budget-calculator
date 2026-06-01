package obchile

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/pierocristi/monthly-budget-calculator/internal/presupuesto"
)

// resolvedorFake devuelve la misma config para cualquier mes.
type resolvedorFake struct {
	cfg presupuesto.ConfigPresupuesto
}

func (r resolvedorFake) ParaMes(_ time.Time) (presupuesto.ConfigPresupuesto, error) {
	return r.cfg, nil
}

func nuevoResolvedorFake(tasa float64, diaCorte int) presupuesto.ResolvedorConfig {
	return resolvedorFake{cfg: presupuesto.ConfigPresupuesto{
		PorcentajeParaGastos: 0.25,
		DiaDeCorteCredito:    diaCorte,
		TasaCambioUSD:        tasa,
		HeredadaDe:           "2026-01",
	}}
}

func TestObtenerGastosValidos_ConGastosManuales(t *testing.T) {
	tempDir := t.TempDir()
	scraperJsonPath := filepath.Join(tempDir, "scraper.json")
	scraperContent := `{
	  "success": true,
	  "movements": [
		{
		  "date": "10-05-2026",
		  "description": "COMPRA SUPERMERCADO",
		  "amount": -50000,
		  "source": "DEBIT",
		  "installments": ""
		}
	  ]
	}`
	err := os.WriteFile(scraperJsonPath, []byte(scraperContent), 0644)
	if err != nil {
		t.Fatalf("No se pudo crear json de scraper: %v", err)
	}

	manualesJsonPath := filepath.Join(tempDir, "manuales.json")
	manualesContent := `[
	  {
		"id": "man-1",
		"descripcion": "Gasto Manual Test",
		"montoTotal": 100000,
		"cuotasTotales": 2,
		"fechaInicio": "15-05-2026",
		"tipoPago": "credito"
	  }
	]`
	err = os.WriteFile(manualesJsonPath, []byte(manualesContent), 0644)
	if err != nil {
		t.Fatalf("No se pudo crear json de manuales: %v", err)
	}

	adapter := NewAdapter(scraperJsonPath, "", nil, "", manualesJsonPath, nuevoResolvedorFake(900.0, 25))

	periodo := presupuesto.PeriodoPresupuestario{
		Inicio: time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC),
		Fin:    time.Date(2026, 5, 31, 23, 59, 59, 0, time.UTC),
	}

	gastos, err := adapter.ObtenerGastosValidos(periodo)
	if err != nil {
		t.Fatalf("ObtenerGastosValidos retornó error: %v", err)
	}

	if len(gastos) != 2 {
		t.Fatalf("Se esperaban 2 gastos, se obtuvieron %d", len(gastos))
	}

	gastoScraper := gastos[0]
	if gastoScraper.Descripcion != "COMPRA SUPERMERCADO" {
		t.Errorf("Descripción scraper incorrecta: %s", gastoScraper.Descripcion)
	}

	gastoManual := gastos[1]
	if gastoManual.ID != "man-1" {
		t.Errorf("ID manual incorrecto: %s", gastoManual.ID)
	}
	if gastoManual.PoliticaCorte.Tipo != presupuesto.Credito {
		t.Errorf("Tipo de pago manual incorrecto")
	}
}

func escribirScraper(t *testing.T, dir string, movimientos string) string {
	t.Helper()
	ruta := filepath.Join(dir, "scraper.json")
	contenido := `{"success": true, "movements": [` + movimientos + `]}`
	if err := os.WriteFile(ruta, []byte(contenido), 0644); err != nil {
		t.Fatalf("escribiendo scraper: %v", err)
	}
	return ruta
}

func gastoPorDescripcion(gastos []presupuesto.Gasto, desc string) (presupuesto.Gasto, bool) {
	for _, g := range gastos {
		if g.Descripcion == desc {
			return g, true
		}
	}
	return presupuesto.Gasto{}, false
}

func TestObtenerGastosValidos_ClasificaPorReglas(t *testing.T) {
	dir := t.TempDir()
	scraper := escribirScraper(t, dir, `
		{"date": "10-05-2026", "description": "COMPRA SUPERMERCADO", "amount": -50000, "source": "DEBIT"},
		{"date": "12-05-2026", "description": "Traspaso Fintual", "amount": -200000, "source": "DEBIT"},
		{"date": "15-05-2026", "description": "Pago Tarjeta de Credito", "amount": -300000, "source": "DEBIT"}
	`)
	reglas := []presupuesto.Regla{
		{Patron: "fintual", Destino: "inversion"},
		{Patron: "pago tarjeta", Destino: presupuesto.Ignorado},
	}

	adapter := NewAdapter(scraper, "", reglas, "", "", nuevoResolvedorFake(900.0, 25))
	gastos, err := adapter.ObtenerGastosValidos(presupuesto.PeriodoPresupuestario{})
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}

	// El pago de tarjeta (regla ignorado) no debe aparecer.
	if _, ok := gastoPorDescripcion(gastos, "Pago Tarjeta de Credito"); ok {
		t.Errorf("el movimiento con regla ignorado no debería aparecer como gasto")
	}

	super, ok := gastoPorDescripcion(gastos, "COMPRA SUPERMERCADO")
	if !ok || super.CategoriaID != presupuesto.CategoriaPorDefecto {
		t.Errorf("supermercado sin regla debería ser categoría default %q, got %q", presupuesto.CategoriaPorDefecto, super.CategoriaID)
	}

	fintual, ok := gastoPorDescripcion(gastos, "Traspaso Fintual")
	if !ok || fintual.CategoriaID != "inversion" {
		t.Errorf("fintual debería clasificar como inversion, got %q", fintual.CategoriaID)
	}
}

func TestObtenerGastosValidos_OverrideCategoriaGana(t *testing.T) {
	dir := t.TempDir()
	scraper := escribirScraper(t, dir, `
		{"date": "12-05-2026", "description": "Traspaso Fintual", "amount": -200000, "source": "DEBIT"}
	`)
	divisiones := filepath.Join(dir, "divisiones.json")
	// Override que solo asigna categoría (sin miParte) → no toca el monto.
	contenido := `[{"fecha": "2026-05-12", "montoOriginal": -200000, "descripcion": "Traspaso Fintual", "categoria": "ahorro"}]`
	if err := os.WriteFile(divisiones, []byte(contenido), 0644); err != nil {
		t.Fatalf("escribiendo divisiones: %v", err)
	}
	reglas := []presupuesto.Regla{{Patron: "fintual", Destino: "inversion"}}

	adapter := NewAdapter(scraper, divisiones, reglas, "", "", nuevoResolvedorFake(900.0, 25))
	gastos, err := adapter.ObtenerGastosValidos(presupuesto.PeriodoPresupuestario{})
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}

	fintual, ok := gastoPorDescripcion(gastos, "Traspaso Fintual")
	if !ok {
		t.Fatal("no apareció el gasto fintual")
	}
	if fintual.CategoriaID != "ahorro" {
		t.Errorf("el override de categoría debería ganar sobre la regla: got %q, want ahorro", fintual.CategoriaID)
	}
	if fintual.MontoImputado != 200000 {
		t.Errorf("el override solo-categoría no debería tocar el monto: got %v, want 200000", fintual.MontoImputado)
	}
}

func TestLeerGastosManuales_ArchivoNoExiste(t *testing.T) {
	adapter := NewAdapter("scraper_dummy.json", "", nil, "", "ruta_inexistente.json", nuevoResolvedorFake(900.0, 25))

	gastosManuales, err := adapter.leerGastosManuales()
	if err != nil {
		t.Errorf("error inesperado: %v", err)
	}
	if len(gastosManuales) != 0 {
		t.Errorf("Se esperaba 0 gastos al no existir archivo, se obtuvieron %d", len(gastosManuales))
	}
}
