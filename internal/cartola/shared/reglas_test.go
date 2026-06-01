package shared

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/pierocristi/monthly-budget-calculator/internal/presupuesto"
)

func TestMigrarExclusionesAReglas(t *testing.T) {
	exclusiones := []string{"fintual", "pago tarjeta de credito"}
	reglas := MigrarExclusionesAReglas(exclusiones)

	if len(reglas) != 2 {
		t.Fatalf("esperaba 2 reglas, obtuve %d", len(reglas))
	}
	for _, r := range reglas {
		if r.Destino != presupuesto.Ignorado {
			t.Errorf("una exclusión migrada debería tener destino %q, got %q (patrón %q)", presupuesto.Ignorado, r.Destino, r.Patron)
		}
	}
	if reglas[0].Patron != "fintual" {
		t.Errorf("patrón mal migrado: got %q", reglas[0].Patron)
	}
}

func TestCargarReglas_PrefiereReglasSobreExclusiones(t *testing.T) {
	dir := t.TempDir()
	rutaReglas := filepath.Join(dir, "reglas.json")
	rutaExclusiones := filepath.Join(dir, "exclusiones.json")
	os.WriteFile(rutaReglas, []byte(`[{"patron":"fintual","destino":"inversion"}]`), 0644)
	os.WriteFile(rutaExclusiones, []byte(`["pago tarjeta"]`), 0644)

	reglas, err := CargarReglas(rutaReglas, rutaExclusiones)
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}
	if len(reglas) != 1 || reglas[0].Destino != "inversion" {
		t.Errorf("debería usar reglas.json (no migrar exclusiones), got %+v", reglas)
	}
}

func TestCargarReglas_MigraExclusionesSiNoHayReglas(t *testing.T) {
	dir := t.TempDir()
	rutaReglas := filepath.Join(dir, "reglas.json") // no existe
	rutaExclusiones := filepath.Join(dir, "exclusiones.json")
	os.WriteFile(rutaExclusiones, []byte(`["fintual"]`), 0644)

	reglas, err := CargarReglas(rutaReglas, rutaExclusiones)
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}
	if len(reglas) != 1 || reglas[0].Patron != "fintual" || reglas[0].Destino != presupuesto.Ignorado {
		t.Errorf("debería migrar exclusiones a reglas ignorado, got %+v", reglas)
	}
}
