package obcl

import (
	"os"
	"path/filepath"
	"testing"

	"presupuesto/internal/cartola/canonico"
)

func TestInstrumentoDeSource(t *testing.T) {
	casos := []struct {
		source string
		quiero canonico.Instrumento
	}{
		{"account", canonico.InstrumentoCuentaCorriente},
		{"credit_card_billed", canonico.InstrumentoTarjetaCredito},
		{"credit_card_unbilled", canonico.InstrumentoTarjetaCredito},
		{"", canonico.InstrumentoDesconocido},
		{"cta_corriente", canonico.InstrumentoDesconocido},
		{"tc_nacional", canonico.InstrumentoDesconocido},
		{"tc_internacional", canonico.InstrumentoDesconocido},
		{"tarjeta_credito_visa", canonico.InstrumentoDesconocido},
		{"algo_que_obcl_invente_manana", canonico.InstrumentoDesconocido},
	}
	for _, c := range casos {
		if got := InstrumentoDeSource(c.source); got != c.quiero {
			t.Errorf("InstrumentoDeSource(%q) = %q, quiero %q", c.source, got, c.quiero)
		}
	}
}

func TestMonedaDeMonto(t *testing.T) {
	casos := []struct {
		monto  float64
		quiero canonico.Moneda
	}{
		{6000, canonico.MonedaCLP},
		{-50000, canonico.MonedaCLP},
		{0, canonico.MonedaCLP},
		{78.57, canonico.MonedaUSD},
		{-2.56, canonico.MonedaUSD},
	}
	for _, c := range casos {
		if got := MonedaDeMonto(c.monto); got != c.quiero {
			t.Errorf("MonedaDeMonto(%v) = %q, quiero %q", c.monto, got, c.quiero)
		}
	}
}

func TestScraperABruto_HechosCanonicos(t *testing.T) {
	casos := []struct {
		nombre              string
		dto                 MovimientoDTO
		quiereInstrumento   canonico.Instrumento
		quiereMoneda        canonico.Moneda
		quiereRepresenta    canonico.MontoRepresenta
		quiereCuotaActual   int
		quiereCuotasTotales int
		quiereIsUSD         bool
		quiereCuotas        string
	}{
		{
			nombre:              "cuenta corriente en CLP",
			dto:                 MovimientoDTO{Banco: "bchile", Fecha: "15-05-2026", Descripcion: "Traspaso A:X", Monto: -6000, Source: "account", Installments: ""},
			quiereInstrumento:   canonico.InstrumentoCuentaCorriente,
			quiereMoneda:        canonico.MonedaCLP,
			quiereRepresenta:    canonico.MontoRepresentaTotal,
			quiereCuotaActual:   1,
			quiereCuotasTotales: 1,
			quiereIsUSD:         false,
			quiereCuotas:        "",
		},
		{
			nombre:              "tarjeta credito CLP en cuotas (monto es el total)",
			dto:                 MovimientoDTO{Banco: "bchile", Fecha: "13-05-2026", Descripcion: "Starbucks", Monto: -36000, Source: "credit_card_billed", Installments: "01/12"},
			quiereInstrumento:   canonico.InstrumentoTarjetaCredito,
			quiereMoneda:        canonico.MonedaCLP,
			quiereRepresenta:    canonico.MontoRepresentaTotal,
			quiereCuotaActual:   1,
			quiereCuotasTotales: 12,
			quiereIsUSD:         false,
			quiereCuotas:        "01/12",
		},
		{
			nombre:              "tarjeta credito en USD (monto con decimal)",
			dto:                 MovimientoDTO{Banco: "bchile", Fecha: "13-05-2026", Descripcion: "UBER *LIME", Monto: -2.56, Source: "credit_card_unbilled", Installments: ""},
			quiereInstrumento:   canonico.InstrumentoTarjetaCredito,
			quiereMoneda:        canonico.MonedaUSD,
			quiereRepresenta:    canonico.MontoRepresentaTotal,
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
			if b.Banco != c.dto.Banco {
				t.Errorf("Banco = %q, quiero %q", b.Banco, c.dto.Banco)
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

func TestLeerScraper_UsaBancoDelJSON(t *testing.T) {
	jsonPath := filepath.Join(t.TempDir(), "current.json")
	if err := os.WriteFile(jsonPath, []byte(`{
  "success": true,
  "bank": "estado",
  "movements": [],
  "accounts": [{"movements": [
    {"date": "15-05-2026", "description": "Movimiento", "amount": -1000, "source": "account", "installments": ""}
  ]}],
  "creditCards": []
}`), 0644); err != nil {
		t.Fatalf("escribiendo JSON temporal: %v", err)
	}

	movs, err := LeerScraper(jsonPath)
	if err != nil {
		t.Fatalf("LeerScraper: %v", err)
	}
	if len(movs) != 1 {
		t.Fatalf("movimientos: esperaba 1, obtuve %d", len(movs))
	}
	if movs[0].Banco != "estado" {
		t.Errorf("Banco = %q, quiero estado", movs[0].Banco)
	}
}
