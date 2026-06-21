package banco_de_chile

import "testing"

func TestFilasAMovimientos_BasicoCargoYAbono(t *testing.T) {
	filas := []filaCC{
		{fecha: "02/01", descripcion: "TRASPASO A:X", canal: "INTERNET", cargo: 10000, abono: 0},
		{fecha: "06/01", descripcion: "SUELDO", canal: "OFICINA", cargo: 0, abono: 1500000},
	}

	movs, err := filasAMovimientos(filas, 2025)
	if err != nil {
		t.Fatalf("filasAMovimientos: %v", err)
	}
	if len(movs) != 2 {
		t.Fatalf("esperaba 2 movimientos, obtuve %d", len(movs))
	}

	if movs[0].Monto != -10000 {
		t.Errorf("cargo debería ser negativo: %v", movs[0].Monto)
	}
	if movs[1].Monto != 1500000 {
		t.Errorf("abono debería ser positivo: %v", movs[1].Monto)
	}

	if movs[0].Banco != "bchile" || movs[0].Source != "cta_corriente" {
		t.Errorf("banco/source mal: %s/%s", movs[0].Banco, movs[0].Source)
	}

	if y, m, d := movs[0].Fecha.Date(); y != 2025 || m != 1 || d != 2 {
		t.Errorf("fecha mal: %v", movs[0].Fecha)
	}

	if movs[0].Descripcion != "TRASPASO A:X" {
		t.Errorf("descripción mal: %q", movs[0].Descripcion)
	}

	if movs[0].Cuotas != "" || movs[0].IsUSD {
		t.Errorf("cta corriente no debería tener cuotas ni USD: cuotas=%q isUSD=%v", movs[0].Cuotas, movs[0].IsUSD)
	}
}

func TestFilasAMovimientos_FiltraSaldoInicial(t *testing.T) {
	filas := []filaCC{
		{fecha: "30/12", descripcion: "SALDO INICIAL", canal: "", cargo: 0, abono: 0},
		{fecha: "02/01", descripcion: "TRASPASO A:X", canal: "INTERNET", cargo: 10000, abono: 0},
	}

	movs, _ := filasAMovimientos(filas, 2025)
	if len(movs) != 1 {
		t.Errorf("esperaba 1 mov (SALDO INICIAL filtrado), obtuve %d", len(movs))
	}
	if movs[0].Descripcion == "SALDO INICIAL" {
		t.Error("SALDO INICIAL no debería haberse incluido")
	}
}

func TestFilasAMovimientos_FiltraFilasVacias(t *testing.T) {
	filas := []filaCC{
		{fecha: "", descripcion: "", canal: "", cargo: 0, abono: 0},
		{fecha: "02/01", descripcion: "X", canal: "", cargo: 100, abono: 0},
	}
	movs, _ := filasAMovimientos(filas, 2025)
	if len(movs) != 1 {
		t.Errorf("esperaba 1, obtuve %d", len(movs))
	}
}

func TestFilasAMovimientos_FechaInvalidaSeIgnora(t *testing.T) {
	filas := []filaCC{
		{fecha: "no-fecha", descripcion: "X", canal: "", cargo: 100, abono: 0},
		{fecha: "02/01", descripcion: "Y", canal: "", cargo: 200, abono: 0},
	}
	movs, _ := filasAMovimientos(filas, 2025)
	if len(movs) != 1 {
		t.Errorf("esperaba 1 (fila con fecha inválida ignorada), obtuve %d", len(movs))
	}
	if movs[0].Descripcion != "Y" {
		t.Errorf("debería haber pasado la fila Y; pasó: %q", movs[0].Descripcion)
	}
}

func TestFilasAMovimientos_RawIncluyeCamposCrudos(t *testing.T) {
	filas := []filaCC{
		{fecha: "02/01", descripcion: "X", canal: "INTERNET", cargo: 10000, abono: 0},
	}
	movs, _ := filasAMovimientos(filas, 2025)
	if movs[0].Raw["canal"] != "INTERNET" {
		t.Errorf("raw.canal: esperaba 'INTERNET', obtuve %v", movs[0].Raw["canal"])
	}
	if movs[0].Raw["fecha_xls"] != "02/01" {
		t.Errorf("raw.fecha_xls: esperaba '02/01', obtuve %v", movs[0].Raw["fecha_xls"])
	}
}

func TestParser_BancoYSource(t *testing.T) {
	p := NewBchileCuentaCorriente()
	if p.Banco() != "bchile" {
		t.Errorf("banco: %s", p.Banco())
	}
	if p.Source() != "cta_corriente" {
		t.Errorf("source: %s", p.Source())
	}
}
