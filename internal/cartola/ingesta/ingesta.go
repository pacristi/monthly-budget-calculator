// Package ingesta orquesta la persistencia de movimientos al sqlite. Es
// agnóstico al banco: recibe MovimientoBruto ya canónico (lo produce el
// paquete de cada banco) y se encarga de la BD, el writer y el dedup.
package ingesta

import (
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"

	"github.com/pierocristi/monthly-budget-calculator/internal/cartola/ingest"
	"github.com/pierocristi/monthly-budget-calculator/internal/cartola/ingest/banco_de_chile"
	"github.com/pierocristi/monthly-budget-calculator/internal/cartola/shared"
	sqlitepkg "github.com/pierocristi/monthly-budget-calculator/internal/cartola/sqlite"
)

// Persistir vuelca los movimientos al sqlite en `dbPath` con dedup, aplicando
// migraciones si hace falta. `origen` queda como metadato de la fila.
func Persistir(brutos []ingest.MovimientoBruto, dbPath, origen string) (int, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return 0, fmt.Errorf("abriendo BD %s: %w", dbPath, err)
	}
	defer db.Close()

	if err := sqlitepkg.Up(db); err != nil {
		return 0, fmt.Errorf("aplicando migraciones: %w", err)
	}

	writer := sqlitepkg.NewWriter(db, origen)
	return writer.InsertarConDedup(brutos)
}

// DesdeScraper lee el current.json de bchile y persiste el liquidado. Los
// movimientos provisorios (unbilled) no se persisten: viven en la capa en vivo.
func DesdeScraper(jsonPath, dbPath string) (int, error) {
	brutos, err := banco_de_chile.LeerScraper(jsonPath)
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
	return Persistir(liquidado, dbPath, "obchile")
}
