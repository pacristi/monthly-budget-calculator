package bchile

import (
	"testing"

	"presupuesto/internal/cartola/ingest"
)

func TestCtaCorriente_HechosCanonicos(t *testing.T) {
	movs, _ := filasAMovimientos([]filaCC{
		{fecha: "02/01", descripcion: "TRASPASO A:X", canal: "INTERNET", cargo: 10000},
	}, 2025)
	m := movs[0]

	if m.Instrumento != ingest.InstrumentoCuentaCorriente {
		t.Errorf("Instrumento = %q, quiero %q", m.Instrumento, ingest.InstrumentoCuentaCorriente)
	}
	if m.Moneda != ingest.MonedaCLP {
		t.Errorf("Moneda = %q, quiero %q", m.Moneda, ingest.MonedaCLP)
	}
	if m.MontoRepresenta != ingest.MontoRepresentaTotal {
		t.Errorf("MontoRepresenta = %q, quiero %q", m.MontoRepresenta, ingest.MontoRepresentaTotal)
	}
	if m.CuotaActual != 1 || m.CuotasTotales != 1 {
		t.Errorf("cuotas = (%d,%d), quiero (1,1)", m.CuotaActual, m.CuotasTotales)
	}
	if m.IsUSD || m.Cuotas != "" {
		t.Errorf("legacy mal: IsUSD=%v Cuotas=%q", m.IsUSD, m.Cuotas)
	}
}

func TestTCInternacional_HechosCanonicos(t *testing.T) {
	movs, _ := filasTCIAMovimientos([]filaTCI{
		{categoria: "Total  Compras", fecha: "01/08/2025", descripcion: "GOOGLE CLOUD", montoUSD: 0.1},
	})
	m := movs[0]

	if m.Instrumento != ingest.InstrumentoTarjetaCredito {
		t.Errorf("Instrumento = %q, quiero %q", m.Instrumento, ingest.InstrumentoTarjetaCredito)
	}
	if m.Moneda != ingest.MonedaUSD {
		t.Errorf("Moneda = %q, quiero %q", m.Moneda, ingest.MonedaUSD)
	}
	if m.MontoRepresenta != ingest.MontoRepresentaTotal {
		t.Errorf("MontoRepresenta = %q, quiero %q", m.MontoRepresenta, ingest.MontoRepresentaTotal)
	}
	if m.CuotaActual != 1 || m.CuotasTotales != 1 {
		t.Errorf("cuotas = (%d,%d), quiero (1,1)", m.CuotaActual, m.CuotasTotales)
	}
	if !m.IsUSD || m.Cuotas != "" {
		t.Errorf("legacy mal: IsUSD=%v Cuotas=%q", m.IsUSD, m.Cuotas)
	}
}

func TestTCNacional_HechosCanonicos(t *testing.T) {
	filas := []filaTCN{
		{categoria: "Total Compras en Cuotas", fecha: "07/01/2025", descripcion: "SKY", cuotas: "02/03", monto: 36124},
		{categoria: "Total de Pagos", fecha: "16/01/2025", descripcion: "BANCHILE SEGUROS", cuotas: "01/01", monto: 3074},
	}
	movs, _ := filasTCNAMovimientos(filas)

	t.Run("compra en cuotas 02/03 -> el monto representa una cuota", func(t *testing.T) {
		m := movs[0]
		if m.Instrumento != ingest.InstrumentoTarjetaCredito {
			t.Errorf("Instrumento = %q, quiero %q", m.Instrumento, ingest.InstrumentoTarjetaCredito)
		}
		if m.Moneda != ingest.MonedaCLP {
			t.Errorf("Moneda = %q, quiero %q", m.Moneda, ingest.MonedaCLP)
		}
		if m.CuotaActual != 2 || m.CuotasTotales != 3 {
			t.Errorf("cuotas = (%d,%d), quiero (2,3)", m.CuotaActual, m.CuotasTotales)
		}
		if m.MontoRepresenta != ingest.MontoRepresentaCuota {
			t.Errorf("MontoRepresenta = %q, quiero %q (CuotasTotales>1)", m.MontoRepresenta, ingest.MontoRepresentaCuota)
		}
		if m.IsUSD || m.Cuotas != "02/03" {
			t.Errorf("legacy mal: IsUSD=%v Cuotas=%q", m.IsUSD, m.Cuotas)
		}
	})

	t.Run("compra sin cuotas 01/01 -> el monto representa el total", func(t *testing.T) {
		m := movs[1]
		if m.CuotaActual != 1 || m.CuotasTotales != 1 {
			t.Errorf("cuotas = (%d,%d), quiero (1,1)", m.CuotaActual, m.CuotasTotales)
		}
		if m.MontoRepresenta != ingest.MontoRepresentaTotal {
			t.Errorf("MontoRepresenta = %q, quiero %q (CuotasTotales==1)", m.MontoRepresenta, ingest.MontoRepresentaTotal)
		}
		if m.Cuotas != "01/01" {
			t.Errorf("legacy Cuotas = %q, quiero %q", m.Cuotas, "01/01")
		}
	})
}
