package app

import (
	"presupuesto/ingesta"
	obcl "presupuesto/ingesta/open-banking-chile"
)

// Refrescar corre el scraper de Open Banking Chile (dejando su salida en
// provisorioPath) y, si persistir es true, ingesta ese snapshot a sqlite.
// Devuelve cuántos movimientos nuevos quedaron almacenados (0 si !persistir).
// (traducido de cmd/cli/sync.go CasoDeUso.Ejecutar, sin la interfaz
// banco.Scraper — la orquestación vive acá directo).
func (a *App) Refrescar(persistir bool) (int, error) {
	scraper := obcl.EjecutarScraper{
		Dir:        "ingest",
		Script:     "scraper.js",
		OutputPath: a.provisorioPath,
	}
	if err := scraper.Ejecutar(); err != nil {
		return 0, err
	}
	if !persistir {
		return 0, nil
	}
	return ingesta.Ingestar(obcl.NuevaOpenBankingChile(a.provisorioPath), a.writer)
}
