package parser

import "testing"

func TestTCNacional_FilasAMovimientos_BasicoCargoYAbono(t *testing.T) {
	filas := []filaTCN{
		{categoria: "Total de Pagos, Compras, Cuotas y Avances", fecha: "16/01/2025", descripcion: "BANCHILE SEGUROS", cuotas: "01/01", monto: 3074},
		{categoria: "Total de Pagos, Compras, Cuotas y Avances", fecha: "27/12/2024", descripcion: "Pago Pesos TEF", cuotas: "01/01", monto: 61608},
	}

	movs, err := filasTCNAMovimientos(filas)
	if err != nil {
		t.Fatalf("filasTCNAMovimientos: %v", err)
	}
	if len(movs) != 2 {
		t.Fatalf("esperaba 2 movimientos, obtuve %d", len(movs))
	}

	// Cargo: negativo.
	if movs[0].Monto != -3074 {
		t.Errorf("cargo BANCHILE: esperaba -3074, obtuve %v", movs[0].Monto)
	}
	// Pago: positivo (abono a la TC).
	if movs[1].Monto != 61608 {
		t.Errorf("pago TEF: esperaba +61608, obtuve %v", movs[1].Monto)
	}

	if movs[0].Banco != "bchile" || movs[0].Source != "tc_nacional" {
		t.Errorf("banco/source mal: %s/%s", movs[0].Banco, movs[0].Source)
	}
	if y, m, d := movs[0].Fecha.Date(); y != 2025 || m != 1 || d != 16 {
		t.Errorf("fecha mal: %v", movs[0].Fecha)
	}
	if movs[0].Cuotas != "01/01" {
		t.Errorf("cuotas mal: %q", movs[0].Cuotas)
	}
}

func TestTCNacional_NoFiltraEnElParser_DejaPasarTodasLasCuotas(t *testing.T) {
	// El parser NO debe filtrar cuotas. Es responsabilidad del writer del
	// sqlite agrupar las cuotas repetidas porque solo él ve el batch
	// completo de todos los meses.
	filas := []filaTCN{
		{categoria: "Total Información Compras en Cuotas", fecha: "07/01/2025", descripcion: "SKY", cuotas: "00/03", monto: 36124},
		{categoria: "Total de Pagos, Compras, Cuotas y Avances", fecha: "07/01/2025", descripcion: "SKY", cuotas: "01/03", monto: 36124},
	}
	movs, _ := filasTCNAMovimientos(filas)
	if len(movs) != 2 {
		t.Fatalf("esperaba 2 (sin filtro a nivel parser), obtuve %d", len(movs))
	}
}

func TestTCNacional_FiltraFilasVaciasYFechasInvalidas(t *testing.T) {
	filas := []filaTCN{
		{categoria: "", fecha: "", descripcion: "", cuotas: "", monto: 0},
		{categoria: "Total de Pagos, Compras, Cuotas y Avances", fecha: "no-fecha", descripcion: "X", cuotas: "01/01", monto: 1000},
		{categoria: "Total de Pagos, Compras, Cuotas y Avances", fecha: "02/01/2025", descripcion: "Y", cuotas: "01/01", monto: 2000},
	}
	movs, _ := filasTCNAMovimientos(filas)
	if len(movs) != 1 {
		t.Fatalf("esperaba 1, obtuve %d", len(movs))
	}
	if movs[0].Descripcion != "Y" {
		t.Errorf("quedó la fila equivocada: %q", movs[0].Descripcion)
	}
}

func TestTCNacional_DetectaPagoEnVariantes(t *testing.T) {
	casos := []struct {
		desc      string
		esperaPos bool
	}{
		{"Pago Pesos TEF", true},
		{"Pago Dolar TEF", true},
		{"Pago Manual TEF", true},
		{"pago pesos tef (minúsculas)", true},
		{"PAGO HONORARIOS", false}, // no contiene TEF, ni Pesos como pago
		{"PAGO PESOS TEF", true},
		{"BANCHILE SEGUROS", false},
	}
	for _, c := range casos {
		got := esPagoTC(c.desc)
		if got != c.esperaPos {
			t.Errorf("esPagoTC(%q) = %v, esperaba %v", c.desc, got, c.esperaPos)
		}
	}
}

func TestTCNacional_RawIncluyeCategoria(t *testing.T) {
	filas := []filaTCN{
		{categoria: "Total Cargos, Comisiones, Impuestos", fecha: "22/01/2025", descripcion: "COMISION", cuotas: "01/01", monto: 2688},
	}
	movs, _ := filasTCNAMovimientos(filas)
	if movs[0].Raw["categoria"] != "Total Cargos, Comisiones, Impuestos" {
		t.Errorf("raw.categoria mal: %v", movs[0].Raw["categoria"])
	}
}

func TestTCNacional_ParserMeta(t *testing.T) {
	p := NewTCNacional()
	if p.Banco() != "bchile" {
		t.Errorf("banco: %s", p.Banco())
	}
	if p.Source() != "tc_nacional" {
		t.Errorf("source: %s", p.Source())
	}
}
