// Package banco_de_chile contiene los parsers de bchile (cartolas xls y JSON
// del scraper), todos produciendo el DTO canónico ingest.MovimientoBruto.
package banco_de_chile

import "github.com/pierocristi/monthly-budget-calculator/internal/cartola/ingest"

// ParserXLSX abstrae la lectura de un archivo .xls de un banco
// específico para un tipo de cuenta específico. Cada combinación
// (banco, source) tiene su propia implementación porque los formatos
// difieren.
type ParserXLSX interface {
	Banco() string
	Source() string
	Parsear(path string, año int) ([]ingest.MovimientoBruto, error)
}
