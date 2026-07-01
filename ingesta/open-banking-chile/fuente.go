package openbankingchile

import "presupuesto/movimientos"

type OpenBankingChile struct {
	JsonPath string
}

func (f OpenBankingChile) LeerMovimientos() ([]movimientos.MovimientoBruto, error) {
	// Entrega TODO (billed + unbilled) sin filtrar: movimientos/sqlite.Writer
	// decide cómo persistir cada uno (dedup para liquidados, replace
	// completo para provisorios).
	return LeerScraper(f.JsonPath)
}

func NuevaOpenBankingChile(jsonPath string) OpenBankingChile {
	return OpenBankingChile{JsonPath: jsonPath}
}
