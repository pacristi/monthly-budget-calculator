package bchile

import (
	"testing"

	"presupuesto/internal/cartola/ingest"
)

func TestInstrumentoDeSource(t *testing.T) {
	casos := []struct {
		source string
		quiero ingest.Instrumento
	}{
		{"account", ingest.InstrumentoCuentaCorriente},
		{"credit_card_billed", ingest.InstrumentoTarjetaCredito},
		{"credit_card_unbilled", ingest.InstrumentoTarjetaCredito},
		{"", ingest.InstrumentoDesconocido},
		{"tarjeta_credito_visa", ingest.InstrumentoDesconocido},
		{"algo_que_obcl_invente_manana", ingest.InstrumentoDesconocido},
	}
	for _, c := range casos {
		if got := instrumentoDeSource(c.source); got != c.quiero {
			t.Errorf("instrumentoDeSource(%q) = %q, quiero %q", c.source, got, c.quiero)
		}
	}
}

func TestMonedaDeMonto(t *testing.T) {
	casos := []struct {
		monto  float64
		quiero ingest.Moneda
	}{
		{6000, ingest.MonedaCLP},
		{-50000, ingest.MonedaCLP},
		{0, ingest.MonedaCLP},
		{78.57, ingest.MonedaUSD},
		{-2.56, ingest.MonedaUSD},
	}
	for _, c := range casos {
		if got := monedaDeMonto(c.monto); got != c.quiero {
			t.Errorf("monedaDeMonto(%v) = %q, quiero %q", c.monto, got, c.quiero)
		}
	}
}

func TestScraperABruto_HechosCanonicos(t *testing.T) {
	casos := []struct {
		nombre              string
		dto                 MovimientoDTO
		quiereInstrumento   ingest.Instrumento
		quiereMoneda        ingest.Moneda
		quiereRepresenta    ingest.MontoRepresenta
		quiereCuotaActual   int
		quiereCuotasTotales int
		quiereIsUSD         bool
		quiereCuotas        string
	}{
		{
			nombre:              "cuenta corriente en CLP",
			dto:                 MovimientoDTO{Fecha: "15-05-2026", Descripcion: "Traspaso A:X", Monto: -6000, Source: "account", Installments: ""},
			quiereInstrumento:   ingest.InstrumentoCuentaCorriente,
			quiereMoneda:        ingest.MonedaCLP,
			quiereRepresenta:    ingest.MontoRepresentaTotal,
			quiereCuotaActual:   1,
			quiereCuotasTotales: 1,
			quiereIsUSD:         false,
			quiereCuotas:        "",
		},
		{
			nombre:              "tarjeta credito CLP en cuotas (monto es el total)",
			dto:                 MovimientoDTO{Fecha: "13-05-2026", Descripcion: "Starbucks", Monto: -36000, Source: "credit_card_billed", Installments: "01/12"},
			quiereInstrumento:   ingest.InstrumentoTarjetaCredito,
			quiereMoneda:        ingest.MonedaCLP,
			quiereRepresenta:    ingest.MontoRepresentaTotal,
			quiereCuotaActual:   1,
			quiereCuotasTotales: 12,
			quiereIsUSD:         false,
			quiereCuotas:        "01/12",
		},
		{
			nombre:              "tarjeta credito en USD (monto con decimal)",
			dto:                 MovimientoDTO{Fecha: "13-05-2026", Descripcion: "UBER *LIME", Monto: -2.56, Source: "credit_card_unbilled", Installments: ""},
			quiereInstrumento:   ingest.InstrumentoTarjetaCredito,
			quiereMoneda:        ingest.MonedaUSD,
			quiereRepresenta:    ingest.MontoRepresentaTotal,
			quiereCuotaActual:   1,
			quiereCuotasTotales: 1,
			quiereIsUSD:         true,
			quiereCuotas:        "",
		},
	}

	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			b, err := scraperABruto(c.dto)
			if err != nil {
				t.Fatalf("scraperABruto: %v", err)
			}
			if b.Instrumento != c.quiereInstrumento {
				t.Errorf("Instrumento = %q, quiero %q", b.Instrumento, c.quiereInstrumento)
			}
			if b.Moneda != c.quiereMoneda {
				t.Errorf("Moneda = %q, quiero %q", b.Moneda, c.quiereMoneda)
			}
			if b.MontoRepresenta != c.quiereRepresenta {
				t.Errorf("MontoRepresenta = %q, quiero %q", b.MontoRepresenta, c.quiereRepresenta)
			}
			if b.CuotaActual != c.quiereCuotaActual || b.CuotasTotales != c.quiereCuotasTotales {
				t.Errorf("cuotas = (%d,%d), quiero (%d,%d)", b.CuotaActual, b.CuotasTotales, c.quiereCuotaActual, c.quiereCuotasTotales)
			}
			if b.IsUSD != c.quiereIsUSD || b.Cuotas != c.quiereCuotas {
				t.Errorf("legacy mal: IsUSD=%v Cuotas=%q", b.IsUSD, b.Cuotas)
			}
		})
	}
}
