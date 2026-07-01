// Package fuentes centraliza la selección de fuentes de movimientos para importar.
package fuentes

import (
	"fmt"
	"path/filepath"

	"presupuesto/movimientos"
	parserBchile "presupuesto/banco/banco-de-chile/parser"
	parserObcl "presupuesto/banco/open-banking-chile/parser"
)

type OpenBankingChile struct {
	JsonPath string
}

func (f OpenBankingChile) LeerMovimientos() ([]movimientos.MovimientoBruto, error) {
	// Entrega TODO (billed + unbilled) sin filtrar: storage/sqlite.Writer
	// decide cómo persistir cada uno (dedup para liquidados, replace
	// completo para provisorios).
	return parserObcl.LeerScraper(f.JsonPath)
}

type CartolaBancoChile struct {
	Banco string
	Tipo  string
	Anio  int
	Dir   string
}

func (f CartolaBancoChile) LeerMovimientos() ([]movimientos.MovimientoBruto, error) {
	parser, err := parserXLSX(f.Banco, f.Tipo)
	if err != nil {
		return nil, err
	}

	archivos, err := filepath.Glob(filepath.Join(f.Dir, "*.xls"))
	if err != nil {
		return nil, fmt.Errorf("listando %s: %w", f.Dir, err)
	}
	if len(archivos) == 0 {
		return nil, fmt.Errorf("ningún .xls en %s", f.Dir)
	}

	var batch []movimientos.MovimientoBruto
	for _, a := range archivos {
		movs, err := parser.Parsear(a, f.Anio)
		if err != nil {
			return nil, fmt.Errorf("parseando %s: %w", a, err)
		}
		batch = append(batch, movs...)
	}
	return batch, nil
}

func NuevaOpenBankingChile(jsonPath string) OpenBankingChile {
	return OpenBankingChile{JsonPath: jsonPath}
}

func NuevaCartolaXLSX(banco, tipo string, anio int, dir string) (CartolaBancoChile, error) {
	if banco == "" || tipo == "" || dir == "" {
		return CartolaBancoChile{}, fmt.Errorf("banco, tipo y dir son obligatorios")
	}
	if _, err := parserXLSX(banco, tipo); err != nil {
		return CartolaBancoChile{}, err
	}
	return CartolaBancoChile{Banco: banco, Tipo: tipo, Anio: anio, Dir: dir}, nil
}

func parserXLSX(banco, tipo string) (parserBchile.ParserXLSX, error) {
	if banco != "bchile" {
		return nil, fmt.Errorf("banco no soportado: %s (solo 'bchile' por ahora)", banco)
	}
	switch tipo {
	case "cta-corriente":
		return parserBchile.NewCuentaCorriente(), nil
	case "tc-nacional":
		return parserBchile.NewTCNacional(), nil
	case "tc-internacional":
		return parserBchile.NewTCInternacional(), nil
	default:
		return nil, fmt.Errorf("tipo no soportado en esta versión: %s (soporta cta-corriente, tc-nacional, tc-internacional)", tipo)
	}
}
