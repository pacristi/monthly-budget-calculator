package bchile

import "testing"

func TestTCInternacional_FilasAMovimientos_BasicoCargoYAbono(t *testing.T) {
	// Convención del banco para TC internacional:
	// - Compras vienen positivas en el xlsx -> negativas en MovimientoBruto.
	// - Pagos a la TC vienen negativos en el xlsx -> positivos en MovimientoBruto.
	filas := []filaTCI{
		{categoria: "Total  Compras", fecha: "01/08/2025", descripcion: "GOOGLE CLOUD", pais: "ESTADOS UNIDOS", montoOrigen: 0.1, montoUSD: 0.1},
		{categoria: "Total  Pagos", fecha: "15/08/2025", descripcion: "Pago Dolar TEF", pais: "", montoOrigen: -30.97, montoUSD: -30.97},
	}

	movs, err := filasTCIAMovimientos(filas)
	if err != nil {
		t.Fatalf("filasTCIAMovimientos: %v", err)
	}
	if len(movs) != 2 {
		t.Fatalf("esperaba 2 movimientos, obtuve %d", len(movs))
	}

	if movs[0].Monto != -0.1 {
		t.Errorf("compra: esperaba -0.1, obtuve %v", movs[0].Monto)
	}
	if movs[1].Monto != 30.97 {
		t.Errorf("pago: esperaba +30.97, obtuve %v", movs[1].Monto)
	}

	if !movs[0].IsUSD || !movs[1].IsUSD {
		t.Error("ambos deberían ser USD")
	}

	if movs[0].Banco != "bchile" || movs[0].Source != "tc_internacional" {
		t.Errorf("banco/source mal: %s/%s", movs[0].Banco, movs[0].Source)
	}

	if movs[0].Cuotas != "" {
		t.Errorf("TC internacional no debería tener cuotas, obtuve %q", movs[0].Cuotas)
	}

	if y, m, d := movs[0].Fecha.Date(); y != 2025 || m != 8 || d != 1 {
		t.Errorf("fecha mal: %v", movs[0].Fecha)
	}
}

func TestTCInternacional_FiltraCategoriaInformacion(t *testing.T) {
	filas := []filaTCI{
		{categoria: "Total Información de Compras", fecha: "01/08/2025", descripcion: "Info", pais: "", montoOrigen: 50, montoUSD: 50},
		{categoria: "Total  Compras", fecha: "02/08/2025", descripcion: "Real", pais: "USA", montoOrigen: 10, montoUSD: 10},
	}
	movs, _ := filasTCIAMovimientos(filas)
	if len(movs) != 1 {
		t.Fatalf("esperaba 1, obtuve %d", len(movs))
	}
	if movs[0].Descripcion != "Real" {
		t.Errorf("quedó la fila equivocada: %q", movs[0].Descripcion)
	}
}

func TestTCInternacional_FiltraFilasVaciasYFechasInvalidas(t *testing.T) {
	filas := []filaTCI{
		{categoria: "", fecha: "", descripcion: "", montoUSD: 0},
		{categoria: "Total  Compras", fecha: "no-fecha", descripcion: "X", montoUSD: 1},
		{categoria: "Total  Compras", fecha: "02/01/2025", descripcion: "Y", montoUSD: 2},
	}
	movs, _ := filasTCIAMovimientos(filas)
	if len(movs) != 1 {
		t.Fatalf("esperaba 1, obtuve %d", len(movs))
	}
	if movs[0].Descripcion != "Y" {
		t.Errorf("quedó la fila equivocada: %q", movs[0].Descripcion)
	}
}

func TestTCInternacional_RawIncluyePaisYMontoOrigen(t *testing.T) {
	filas := []filaTCI{
		{categoria: "Total  Compras", fecha: "10/05/2026", descripcion: "VIAGOGO", pais: "ESTADOS UNIDOS", montoOrigen: 50, montoUSD: 60.88},
	}
	movs, _ := filasTCIAMovimientos(filas)
	if movs[0].Raw["pais"] != "ESTADOS UNIDOS" {
		t.Errorf("raw.pais mal: %v", movs[0].Raw["pais"])
	}
	if movs[0].Raw["monto_moneda_origen"] != 50.0 {
		t.Errorf("raw.monto_moneda_origen mal: %v", movs[0].Raw["monto_moneda_origen"])
	}
	if movs[0].Raw["categoria"] != "Total  Compras" {
		t.Errorf("raw.categoria mal: %v", movs[0].Raw["categoria"])
	}
}

func TestTCInternacional_ParserMeta(t *testing.T) {
	p := NewTCInternacional()
	if p.Banco() != "bchile" {
		t.Errorf("banco: %s", p.Banco())
	}
	if p.Source() != "tc_internacional" {
		t.Errorf("source: %s", p.Source())
	}
}
