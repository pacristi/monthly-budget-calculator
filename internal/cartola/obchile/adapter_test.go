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

	adapter := NewAdapter(scraperJsonPath, "", "", "", manualesJsonPath, nuevoResolvedorFake(900.0, 25))

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

func TestLeerGastosManuales_ArchivoNoExiste(t *testing.T) {
	adapter := NewAdapter("scraper_dummy.json", "", "", "", "ruta_inexistente.json", nuevoResolvedorFake(900.0, 25))

	gastosManuales, err := adapter.leerGastosManuales()
	if err != nil {
		t.Errorf("error inesperado: %v", err)
	}
	if len(gastosManuales) != 0 {
		t.Errorf("Se esperaba 0 gastos al no existir archivo, se obtuvieron %d", len(gastosManuales))
	}
}
