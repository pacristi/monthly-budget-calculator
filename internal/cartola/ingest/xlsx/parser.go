// Package xlsx contiene los parsers de cartolas en formato xls de los
// distintos bancos / tipos de cuenta. Todos producen el DTO canónico
// MovimientoBruto.
package xlsx

import "github.com/pierocristi/monthly-budget-calculator/internal/cartola/ingest"

// ParserCartolaXLSX abstrae la lectura de un archivo .xls de un banco
// específico para un tipo de cuenta específico. Cada combinación
// (banco, source) tiene su propia implementación porque los formatos
// difieren.
type ParserCartolaXLSX interface {
	Banco() string
	Source() string
	Parsear(path string, año int) ([]ingest.MovimientoBruto, error)
}
