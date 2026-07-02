package app

import (
	"os"
	"path/filepath"
	"testing"
)

// TestRefrescarScraperDirExiste verifica que el Dir/Script que Refrescar le
// pasa a EjecutarScraper todavía exista en disco. EjecutarScraper.Ejecutar
// hace cmd.Dir = s.Dir antes de invocar node, así que un Dir stale rompe
// /api/refresh en runtime sin que ningún test lo note (el bug real que este
// test previene: Paso 5 renombró banco/ a ingesta/ y movió los assets del
// scraper a ingesta/open-banking-chile/, pero Refrescar seguía apuntando al
// ingest/ viejo). No ejecuta node ni el scraper real: solo confirma que la
// ruta es real.
//
// go test corre con cwd = app/, pero la app (go run/binario) corre desde la
// raíz del repo, que es donde Dir se resuelve en runtime. app/ es hija
// directa de la raíz, así que ".." reconstruye esa raíz para el chequeo.
func TestRefrescarScraperDirExiste(t *testing.T) {
	const dir = "ingesta/open-banking-chile"
	const script = "scraper.js"

	repoRoot := ".."
	scraperPath := filepath.Join(repoRoot, dir, script)
	if _, err := os.Stat(scraperPath); err != nil {
		t.Fatalf("Dir %q + Script %q no existen relativos a la raíz del repo (%s): %v", dir, script, scraperPath, err)
	}
}
