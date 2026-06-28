package obcl

import (
	"math"
	"strconv"
	"strings"

	"presupuesto/internal/cartola/canonico"
)

func InstrumentoDeSource(source string) canonico.Instrumento {
	switch source {
	case "account":
		return canonico.InstrumentoCuentaCorriente
	case "credit_card_billed", "credit_card_unbilled":
		return canonico.InstrumentoTarjetaCredito
	default:
		return canonico.InstrumentoDesconocido
	}
}

func EsProvisorio(source string) bool {
	return strings.Contains(strings.ToLower(source), "unbilled")
}

func MonedaDeMonto(monto float64) canonico.Moneda {
	if math.Trunc(monto) != monto {
		return canonico.MonedaUSD
	}
	return canonico.MonedaCLP
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
