// Package ingesta orquesta la persistencia de movimientos. Es
// agnóstico al banco: recibe MovimientoBruto ya canónico (lo produce el
// paquete de cada banco) y delega el almacenamiento al repositorio recibido.
package ingesta

import (
	"presupuesto/internal/cartola/ingest"
	"presupuesto/internal/cartola/ingest/bchile"
	"presupuesto/internal/cartola/shared"
)

// RepositorioMovimientos persiste movimientos canónicos con la política de
// deduplicación propia del repositorio concreto.
type RepositorioMovimientos interface {
	InsertarConDedup([]ingest.MovimientoBruto) (int, error)
}

// Persistir vuelca los movimientos al repositorio recibido.
func Persistir(brutos []ingest.MovimientoBruto, repo RepositorioMovimientos) (int, error) {
	return repo.InsertarConDedup(brutos)
}

// DesdeScraper lee el current.json de bchile y persiste el liquidado. Los
// movimientos provisorios (unbilled) no se persisten: viven en la capa en vivo.
func DesdeScraper(jsonPath string, repo RepositorioMovimientos) (int, error) {
	brutos, err := bchile.LeerScraper(jsonPath)
	if err != nil {
		return 0, err
	}

	liquidado := brutos[:0]
	for _, b := range brutos {
		if shared.EsProvisorio(b.Source) {
			continue
		}
		liquidado = append(liquidado, b)
	}
	return Persistir(liquidado, repo)
}
