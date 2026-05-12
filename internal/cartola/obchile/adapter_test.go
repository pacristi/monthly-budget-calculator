package obchile

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/pierocristi/monthly-budget-calculator/internal/presupuesto"
)

func TestObtenerGastosValidos_ConGastosManuales(t *testing.T) {
	// 1. Crear un JSON temporal para el scraper
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

	// 2. Crear un JSON temporal para gastos manuales
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

	// 3. Instanciar el Adapter con ambas rutas
	adapter := NewAdapter(scraperJsonPath, "", manualesJsonPath, 900.0, 25)

	// 4. Ejecutar ObtenerGastosValidos
	periodo := presupuesto.PeriodoPresupuestario{
		Inicio: time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC),
		Fin:    time.Date(2026, 5, 31, 23, 59, 59, 0, time.UTC),
	}

	gastos, err := adapter.ObtenerGastosValidos(periodo)
	if err != nil {
		t.Fatalf("ObtenerGastosValidos retornó error: %v", err)
	}

	// 5. Validar que vengan ambos gastos
	if len(gastos) != 2 {
		t.Fatalf("Se esperaban 2 gastos, se obtuvieron %d", len(gastos))
	}

	// Validar scraper
	gastoScraper := gastos[0]
	if gastoScraper.Descripcion != "COMPRA SUPERMERCADO" {
		t.Errorf("Descripción scraper incorrecta: %s", gastoScraper.Descripcion)
	}

	// Validar manual
	gastoManual := gastos[1]
	if gastoManual.ID != "man-1" {
		t.Errorf("ID manual incorrecto: %s", gastoManual.ID)
	}
	if gastoManual.Descripcion != "Gasto Manual Test" {
		t.Errorf("Descripción manual incorrecta: %s", gastoManual.Descripcion)
	}
	if gastoManual.MontoImputado != 100000 {
		t.Errorf("Monto manual incorrecto: %f", gastoManual.MontoImputado)
	}
	if gastoManual.Cuotas != 2 {
		t.Errorf("Cuotas manuales incorrectas: %d", gastoManual.Cuotas)
	}
	if gastoManual.PoliticaCorte.Tipo != presupuesto.Credito {
		t.Errorf("Tipo de pago manual incorrecto")
	}
}

func TestLeerGastosManuales_ArchivoNoExiste(t *testing.T) {
	// Si el archivo no existe, debe manejarse gracefully y retornar slice vacío
	adapter := NewAdapter("scraper_dummy.json", "", "ruta_inexistente.json", 900.0, 25)
	
	gastosManuales := adapter.leerGastosManuales()
	if len(gastosManuales) != 0 {
		t.Errorf("Se esperaba 0 gastos al no existir archivo, se obtuvieron %d", len(gastosManuales))
	}
}
