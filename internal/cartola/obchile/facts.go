package obchile

import (
	"presupuesto/internal/cartola/ingest"
	"presupuesto/internal/cartola/obchile/facts"
)

func InstrumentoDeSource(source string) ingest.Instrumento {
	return facts.InstrumentoDeSource(source)
}

func MonedaDeMonto(monto float64) ingest.Moneda {
	return facts.MonedaDeMonto(monto)
}

func CuotasDeInstallments(installments string) (actual, total int) {
	return facts.CuotasDeInstallments(installments)
}
