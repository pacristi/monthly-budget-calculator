package obchile

import (
	"math"
	"strconv"
	"strings"

	"presupuesto/internal/cartola/ingest"
)

func InstrumentoDeSource(source string) ingest.Instrumento {
	switch source {
	case "account", "cta_corriente":
		return ingest.InstrumentoCuentaCorriente
	case "credit_card_billed", "credit_card_unbilled", "tc_nacional", "tc_internacional":
		return ingest.InstrumentoTarjetaCredito
	default:
		return ingest.InstrumentoDesconocido
	}
}

func EsProvisorio(source string) bool {
	return strings.Contains(strings.ToLower(source), "unbilled")
}

func MonedaDeMonto(monto float64) ingest.Moneda {
	if math.Trunc(monto) != monto {
		return ingest.MonedaUSD
	}
	return ingest.MonedaCLP
}

func CuotasDeInstallments(installments string) (actual, total int) {
	parts := strings.Split(installments, "/")
	if len(parts) != 2 {
		return 1, 1
	}
	a, errA := strconv.Atoi(strings.TrimSpace(parts[0]))
	tot, errTot := strconv.Atoi(strings.TrimSpace(parts[1]))
	if errA != nil || errTot != nil || tot < 1 {
		return 1, 1
	}
	return a, tot
}
