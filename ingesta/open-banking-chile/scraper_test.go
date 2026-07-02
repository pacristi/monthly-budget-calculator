package openbankingchile_test

import (
	"os"
	"path/filepath"
	"testing"

	openbankingchile "presupuesto/ingesta/open-banking-chile"
)

// TestEjecutarScraperResuelveOutputPathAAbsoluto reproduce el bug real: el
// proceso node arranca con su cwd cambiado a Dir (la carpeta del scraper), así
// que si OutputPath viajara relativo tal cual, se resolvería contra Dir en
// vez de contra el cwd de quien invoca Ejecutar (el operador/flag
// --provisorio lo piensa relativo a la raíz del repo). Por eso Ejecutar debe
// convertir OutputPath a absoluto ANTES de pasarlo como env var.
func TestEjecutarScraperResuelveOutputPathAAbsoluto(t *testing.T) {
	scraperDir := t.TempDir()
	markerPath := filepath.Join(scraperDir, "output-env.txt")
	script := filepath.Join(scraperDir, "print-output-path.js")
	if err := os.WriteFile(script, []byte("import fs from 'fs'; fs.writeFileSync(process.argv[2], process.env.SCRAPER_OUTPUT_PATH || '');"), 0o600); err != nil {
		t.Fatalf("creando script: %v", err)
	}

	scraper := openbankingchile.EjecutarScraper{
		Dir:        scraperDir, // cwd del proceso node — DISTINTO de donde corre el test
		Script:     script,
		Args:       []string{markerPath},
		OutputPath: "data/custom-current.json", // relativo, como lo pasa app.Config
	}
	if err := scraper.Ejecutar(); err != nil {
		t.Fatalf("Ejecutar: %v", err)
	}

	got, err := os.ReadFile(markerPath)
	if err != nil {
		t.Fatalf("leyendo marker: %v", err)
	}
	gotPath := string(got)

	if !filepath.IsAbs(gotPath) {
		t.Fatalf("SCRAPER_OUTPUT_PATH debe ser absoluto (no depender del cwd del proceso node), got %q", gotPath)
	}

	wantDir, err := filepath.Abs("data")
	if err != nil {
		t.Fatalf("filepath.Abs: %v", err)
	}
	if gotDir := filepath.Dir(gotPath); gotDir != wantDir {
		t.Fatalf("SCRAPER_OUTPUT_PATH resuelto contra el cwd equivocado: got dir %q, want %q (relativo al cwd de este proceso, no de Dir=%q)", gotDir, wantDir, scraperDir)
	}
}
