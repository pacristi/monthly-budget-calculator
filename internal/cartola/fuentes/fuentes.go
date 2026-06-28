// Package fuentes centraliza la selección de fuentes de movimientos para ingesta.
package fuentes

import (
	"fmt"
	"path/filepath"

	"presupuesto/internal/cartola/ingest"
	"presupuesto/internal/cartola/ingest/bchile"
	"presupuesto/internal/cartola/ingest/obcl"
)

type OpenBankingChile struct {
	JsonPath string
}

func (f OpenBankingChile) LeerMovimientos() ([]ingest.MovimientoBruto, error) {
	brutos, err := obcl.LeerScraper(f.JsonPath)
	if err != nil {
		return nil, err
	}

	liquidado := brutos[:0]
	for _, b := range brutos {
		if obcl.EsProvisorio(b.Source) {
			continue
		}
		liquidado = append(liquidado, b)
	}
	return liquidado, nil
}

type CartolaBancoChile struct {
	Banco string
	Tipo  string
	Anio  int
	Dir   string
}

func (f CartolaBancoChile) LeerMovimientos() ([]ingest.MovimientoBruto, error) {
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

	var batch []ingest.MovimientoBruto
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

func parserXLSX(banco, tipo string) (bchile.ParserXLSX, error) {
	if banco != "bchile" {
		return nil, fmt.Errorf("banco no soportado: %s (solo 'bchile' por ahora)", banco)
	}
	switch tipo {
	case "cta-corriente":
		return bchile.NewCuentaCorriente(), nil
	case "tc-nacional":
		return bchile.NewTCNacional(), nil
	case "tc-internacional":
		return bchile.NewTCInternacional(), nil
	default:
		return nil, fmt.Errorf("tipo no soportado en esta versión: %s (soporta cta-corriente, tc-nacional, tc-internacional)", tipo)
	}
}
