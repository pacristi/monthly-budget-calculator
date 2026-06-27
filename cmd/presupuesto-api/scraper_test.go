package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNodeScraperPasaOutputPathAlProceso(t *testing.T) {
	dir := t.TempDir()
	markerPath := filepath.Join(dir, "output-env.txt")
	script := filepath.Join(dir, "print-output-path.js")
	if err := os.WriteFile(script, []byte("import fs from 'fs'; fs.writeFileSync(process.argv[2], process.env.SCRAPER_OUTPUT_PATH || '');"), 0o600); err != nil {
		t.Fatalf("creando script: %v", err)
	}

	scraper := nodeScraper{
		Dir:        dir,
		Script:     script,
		Args:       []string{markerPath},
		OutputPath: "data/custom-current.json",
	}
	if err := scraper.Ejecutar(); err != nil {
		t.Fatalf("Ejecutar: %v", err)
	}

	got, err := os.ReadFile(markerPath)
	if err != nil {
		t.Fatalf("leyendo marker: %v", err)
	}
	if string(got) != "data/custom-current.json" {
		t.Fatalf("SCRAPER_OUTPUT_PATH: %q", got)
	}
}
