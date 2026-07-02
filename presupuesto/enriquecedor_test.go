package presupuesto_test

import (
	"strings"
	"testing"
	"time"

	"presupuesto/movimientos"
	"presupuesto/presupuesto"
)

type resolvedorFake struct {
	tasaUSD    float64
	diaCorteCC int
}

func (r resolvedorFake) ParaMes(t time.Time) (presupuesto.ConfigPresupuesto, error) {
	return presupuesto.ConfigPresupuesto{
		TasaCambioUSD:     r.tasaUSD,
		DiaDeCorteCredito: r.diaCorteCC,
		HeredadaDe:        t.Format("2006-01"),
	}, nil
}

func cargo(id int64, fecha string, monto float64, desc string, instrumento movimientos.Instrumento, moneda movimientos.Moneda, cuotasTotales int) movimientos.Persistido {
	return movimientos.Persistido{
		ID: id,
		MovimientoBruto: movimientos.MovimientoBruto{
			Fecha:         fechaDe(fecha),
			Monto:         monto,
			Descripcion:   desc,
			Instrumento:   instrumento,
			Moneda:        moneda,
			IsUSD:         moneda == movimientos.MonedaUSD,
			CuotasTotales: cuotasTotales,
		},
	}
}

func abono(id int64, fecha string, monto float64, desc string) movimientos.Persistido {
	return cargo(id, fecha, monto, desc, movimientos.InstrumentoCuentaCorriente, movimientos.MonedaCLP, 1)
}

func fechaDe(iso string) time.Time {
	t, err := time.Parse("2006-01-02", iso)
	if err != nil {
		panic(err)
	}
	return t
}

func periodoMayo2026() presupuesto.PeriodoPresupuestario {
	inicio := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	return presupuesto.PeriodoPresupuestario{Inicio: inicio, Fin: inicio.AddDate(0, 1, 0).Add(-time.Nanosecond)}
}

func gastoPorDesc(gastos []presupuesto.Gasto, desc string) (presupuesto.Gasto, bool) {
	for _, g := range gastos {
		if g.Descripcion == desc {
			return g, true
		}
	}
	return presupuesto.Gasto{}, false
}

func ptr(f float64) *float64 { return &f }

func ptrB(b bool) *bool { return &b }

func TestEnriquecerGastos(t *testing.T) {
	res := resolvedorFake{tasaUSD: 950, diaCorteCC: 22}

	t.Run("filtra abonos ignorables y reglas ignorado", func(t *testing.T) {
		// Los abonos (monto > 0) no llegan a EnriquecerGastos: el caller solo
		// pasa cargos. Aquí probamos el filtrado de reglas Ignorado.
		cargos := []movimientos.Persistido{
			cargo(1, "2026-05-10", -10000, "STARBUCKS", movimientos.InstrumentoCuentaCorriente, movimientos.MonedaCLP, 1),
			cargo(2, "2026-05-12", -50000, "PAGO TARJETA DE CREDITO", movimientos.InstrumentoCuentaCorriente, movimientos.MonedaCLP, 1),
			cargo(3, "2026-05-14", -30000, "Fintual ahorro", movimientos.InstrumentoCuentaCorriente, movimientos.MonedaCLP, 1),
		}
		reglas := []presupuesto.Regla{
			{Patron: "pago tarjeta de credito", Destino: presupuesto.Ignorado},
			{Patron: "fintual", Destino: presupuesto.Ignorado},
		}

		gastos, err := presupuesto.EnriquecerGastos(cargos, nil, reglas, res)
		if err != nil {
			t.Fatalf("EnriquecerGastos: %v", err)
		}
		if len(gastos) != 1 {
			t.Fatalf("esperaba 1 gasto válido, obtuve %d", len(gastos))
		}
		if gastos[0].Descripcion != "STARBUCKS" {
			t.Errorf("quedó el gasto equivocado: %q", gastos[0].Descripcion)
		}
	})

	t.Run("usa instrumento canonico para politica de corte", func(t *testing.T) {
		cargos := []movimientos.Persistido{
			cargo(1, "2026-05-10", -10000, "SOURCE ENGAÑOSO", movimientos.InstrumentoCuentaCorriente, movimientos.MonedaCLP, 1),
			cargo(2, "2026-05-10", -20000, "CREDITO CANONICO", movimientos.InstrumentoTarjetaCredito, movimientos.MonedaCLP, 3),
		}

		gastos, err := presupuesto.EnriquecerGastos(cargos, nil, nil, res)
		if err != nil {
			t.Fatalf("EnriquecerGastos: %v", err)
		}
		if len(gastos) != 2 {
			t.Fatalf("esperaba 2, obtuve %d", len(gastos))
		}

		debito, _ := gastoPorDesc(gastos, "SOURCE ENGAÑOSO")
		credito, _ := gastoPorDesc(gastos, "CREDITO CANONICO")

		if debito.PoliticaCorte.Tipo != presupuesto.Debito {
			t.Error("cuenta_corriente no debería volver crédito")
		}
		if debito.PoliticaCorte.DiaDeCorte != 0 {
			t.Error("débito no debería tener día de corte")
		}
		if credito.PoliticaCorte.Tipo != presupuesto.Credito {
			t.Error("instrumento tarjeta_credito no quedó como crédito")
		}
		if credito.PoliticaCorte.DiaDeCorte != 22 {
			t.Errorf("crédito día de corte: esperaba 22, obtuve %d", credito.PoliticaCorte.DiaDeCorte)
		}
		if credito.Cuotas != 3 {
			t.Errorf("cuotas: esperaba 3, obtuve %d", credito.Cuotas)
		}
	})

	t.Run("normaliza USD a CLP con la tasa del mes", func(t *testing.T) {
		cargos := []movimientos.Persistido{
			cargo(1, "2026-05-10", -10.5, "USD COMPRA", movimientos.InstrumentoTarjetaCredito, movimientos.MonedaUSD, 1),
		}
		gastos, err := presupuesto.EnriquecerGastos(cargos, nil, nil, res)
		if err != nil {
			t.Fatalf("EnriquecerGastos: %v", err)
		}
		if len(gastos) != 1 {
			t.Fatalf("esperaba 1, obtuve %d", len(gastos))
		}
		// |−10.5 * 950| = 9975
		if gastos[0].MontoImputado != 9975 {
			t.Errorf("USD normalizado: esperaba 9975, obtuve %v", gastos[0].MontoImputado)
		}
	})

	t.Run("CLP decimal no se convierte por heuristica", func(t *testing.T) {
		cargos := []movimientos.Persistido{
			cargo(1, "2026-05-10", -10.5, "MI PARTE CLP", movimientos.InstrumentoCuentaCorriente, movimientos.MonedaCLP, 1),
		}
		gastos, err := presupuesto.EnriquecerGastos(cargos, nil, nil, res)
		if err != nil {
			t.Fatalf("EnriquecerGastos: %v", err)
		}
		if len(gastos) != 1 {
			t.Fatalf("esperaba 1, obtuve %d", len(gastos))
		}
		if gastos[0].MontoImputado != 10.5 {
			t.Errorf("CLP decimal no debe convertirse por tasa: esperaba 10.5, obtuve %v", gastos[0].MontoImputado)
		}
	})

	t.Run("aplica override de mi parte al monto", func(t *testing.T) {
		cargos := []movimientos.Persistido{
			cargo(1, "2026-05-10", -10000, "Restaurante", movimientos.InstrumentoCuentaCorriente, movimientos.MonedaCLP, 1),
		}
		overrides := []presupuesto.Override{
			{MovimientoID: "sql-1", Fecha: "2026-05-10", MontoOriginal: -10000, Descripcion: "Restaurante", MiParte: ptr(-4000)},
		}
		gastos, err := presupuesto.EnriquecerGastos(cargos, overrides, nil, res)
		if err != nil {
			t.Fatalf("EnriquecerGastos: %v", err)
		}
		if gastos[0].MontoImputado != 4000 {
			t.Errorf("con override: esperaba 4000, obtuve %v", gastos[0].MontoImputado)
		}
	})

	t.Run("una regla asigna una categoria concreta", func(t *testing.T) {
		cargos := []movimientos.Persistido{
			cargo(1, "2026-05-10", -50000, "COMPRA SUPERMERCADO", movimientos.InstrumentoCuentaCorriente, movimientos.MonedaCLP, 1),
			cargo(2, "2026-05-12", -200000, "Traspaso Fintual", movimientos.InstrumentoCuentaCorriente, movimientos.MonedaCLP, 1),
		}
		reglas := []presupuesto.Regla{{Patron: "fintual", Destino: "inversion"}}

		gastos, err := presupuesto.EnriquecerGastos(cargos, nil, reglas, res)
		if err != nil {
			t.Fatalf("EnriquecerGastos: %v", err)
		}
		super, ok := gastoPorDesc(gastos, "COMPRA SUPERMERCADO")
		if !ok || super.CategoriaID != presupuesto.CategoriaPorDefecto {
			t.Errorf("supermercado sin regla debería ser default %q, got %q", presupuesto.CategoriaPorDefecto, super.CategoriaID)
		}
		fintual, ok := gastoPorDesc(gastos, "Traspaso Fintual")
		if !ok || fintual.CategoriaID != "inversion" {
			t.Errorf("fintual debería clasificar como inversion, got %q", fintual.CategoriaID)
		}
	})

	t.Run("override de moneda fuerza USD aunque el monto sea entero", func(t *testing.T) {
		cargos := []movimientos.Persistido{
			cargo(1, "2026-05-10", -3, "WINDSCRIBE", movimientos.InstrumentoTarjetaCredito, movimientos.MonedaCLP, 1),
		}
		overrides := []presupuesto.Override{
			{MovimientoID: "sql-1", Fecha: "2026-05-10", MontoOriginal: -3, Descripcion: "WINDSCRIBE", EsUSD: ptrB(true)},
		}
		gastos, err := presupuesto.EnriquecerGastos(cargos, overrides, nil, res)
		if err != nil {
			t.Fatalf("EnriquecerGastos: %v", err)
		}
		if len(gastos) != 1 {
			t.Fatalf("esperaba 1 gasto, obtuve %d", len(gastos))
		}
		if want := 3.0 * 950; gastos[0].MontoImputado != want {
			t.Errorf("MontoImputado = %v, want %v (override debe forzar conversión USD)", gastos[0].MontoImputado, want)
		}
	})

	t.Run("override de categoria gana sobre la regla sin tocar el monto", func(t *testing.T) {
		cargos := []movimientos.Persistido{
			cargo(1, "2026-05-12", -200000, "Traspaso Fintual", movimientos.InstrumentoCuentaCorriente, movimientos.MonedaCLP, 1),
		}
		reglas := []presupuesto.Regla{{Patron: "fintual", Destino: "inversion"}}
		overrides := []presupuesto.Override{
			{MovimientoID: "sql-1", Fecha: "2026-05-12", MontoOriginal: -200000, Descripcion: "Traspaso Fintual", Categoria: "ahorro"},
		}

		gastos, err := presupuesto.EnriquecerGastos(cargos, overrides, reglas, res)
		if err != nil {
			t.Fatalf("EnriquecerGastos: %v", err)
		}
		fintual, ok := gastoPorDesc(gastos, "Traspaso Fintual")
		if !ok {
			t.Fatal("no apareció el gasto fintual")
		}
		if fintual.CategoriaID != "ahorro" {
			t.Errorf("el override de categoría debería ganar sobre la regla: got %q, want ahorro", fintual.CategoriaID)
		}
		if fintual.MontoImputado != 200000 {
			t.Errorf("el override solo-categoría no debería tocar el monto: got %v, want 200000", fintual.MontoImputado)
		}
	})
}

func TestDetectarSueldo(t *testing.T) {
	periodo := periodoMayo2026()

	t.Run("sueldo del mes anterior financia el periodo", func(t *testing.T) {
		// El caller entrega los abonos de la ventana ordenados fecha DESC.
		// El sueldo de mayo (fuera de ventana, no lo entregaría el store) no
		// aparece; el de abril sí, y es el que gana.
		abonos := []movimientos.Persistido{
			abono(2, "2026-04-15", 1400000, "PAGO DE SUELDOS Abril"),
		}
		got, err := presupuesto.DetectarSueldo(abonos, []string{"pago de sueldos"}, periodo)
		if err != nil {
			t.Fatalf("DetectarSueldo: %v", err)
		}
		if got != 1400000 {
			t.Errorf("esperaba 1400000 (abril financia mayo), obtuve %v", got)
		}
	})

	t.Run("sueldo que llega los primeros dias del mes", func(t *testing.T) {
		abonos := []movimientos.Persistido{
			abono(1, "2026-05-05", 1400000, "PAGO:DE SUELDOS Atrasado"),
		}
		got, err := presupuesto.DetectarSueldo(abonos, []string{"pago:de sueldos"}, periodo)
		if err != nil {
			t.Fatalf("DetectarSueldo: %v", err)
		}
		if got != 1400000 {
			t.Errorf("esperaba 1400000, obtuve %v", got)
		}
	})

	t.Run("primer match gana (orden DESC del caller)", func(t *testing.T) {
		abonos := []movimientos.Persistido{
			abono(3, "2026-05-05", 1500000, "PAGO DE SUELDOS Mayo"),
			abono(2, "2026-04-15", 1400000, "PAGO DE SUELDOS Abril"),
		}
		got, err := presupuesto.DetectarSueldo(abonos, []string{"pago de sueldos"}, periodo)
		if err != nil {
			t.Fatalf("DetectarSueldo: %v", err)
		}
		if got != 1500000 {
			t.Errorf("esperaba el primer match (1500000), obtuve %v", got)
		}
	})

	t.Run("sin patrones es error", func(t *testing.T) {
		abonos := []movimientos.Persistido{abono(1, "2026-05-15", 1500000, "PAGO DE SUELDOS Mayo")}
		_, err := presupuesto.DetectarSueldo(abonos, nil, periodo)
		if err == nil {
			t.Fatal("sin patrones de sueldo configurados debería ser error explícito")
		}
	})

	t.Run("no encontrado es error con la ventana", func(t *testing.T) {
		_, err := presupuesto.DetectarSueldo(nil, []string{"pago de sueldos"}, periodo)
		if err == nil {
			t.Fatal("esperaba error por sueldo no encontrado")
		}
		// El mensaje reporta la ventana [mes anterior, +10 días].
		if !strings.Contains(err.Error(), "2026-04-01") || !strings.Contains(err.Error(), "2026-05-11") {
			t.Errorf("el error debería reportar la ventana de búsqueda: %v", err)
		}
	})
}

func TestVentanaSueldo(t *testing.T) {
	ini, fin := presupuesto.VentanaSueldo(periodoMayo2026())
	if !ini.Equal(fechaDe("2026-04-01")) {
		t.Errorf("inicio ventana: esperaba 2026-04-01, obtuve %s", ini.Format("2006-01-02"))
	}
	if !fin.Equal(fechaDe("2026-05-11")) {
		t.Errorf("fin ventana: esperaba 2026-05-11, obtuve %s", fin.Format("2006-01-02"))
	}
}

func TestVistaMovimientos(t *testing.T) {
	t.Run("aplica mi parte y clasifica", func(t *testing.T) {
		// El caller filtra positivos (solo pasa cargos) y decide el orden.
		cargos := []movimientos.Persistido{
			cargo(1, "2026-05-10", -10000, "Restaurante", movimientos.InstrumentoCuentaCorriente, movimientos.MonedaCLP, 1),
		}
		overrides := []presupuesto.Override{
			{MovimientoID: "sql-1", Fecha: "2026-05-10", MontoOriginal: -10000, Descripcion: "Restaurante", MiParte: ptr(-4000)},
		}
		reglas := []presupuesto.Regla{{Patron: "restaurante", Destino: "comida"}}

		movs := presupuesto.VistaMovimientos(cargos, overrides, reglas)
		if len(movs) != 1 {
			t.Fatalf("esperaba 1, obtuve %d", len(movs))
		}
		if movs[0].MiParte == nil || *movs[0].MiParte != -4000 {
			t.Errorf("MiParte no aplicado: %v", movs[0].MiParte)
		}
		if movs[0].CategoriaID != "comida" {
			t.Errorf("categoría no aplicada: %q", movs[0].CategoriaID)
		}
		if movs[0].ID != "sql-1" {
			t.Errorf("ID: esperaba sql-1, obtuve %q", movs[0].ID)
		}
	})

	t.Run("override de moneda fuerza IsUSD aunque el monto sea entero", func(t *testing.T) {
		cargos := []movimientos.Persistido{
			cargo(1, "2026-05-10", -3, "WINDSCRIBE", movimientos.InstrumentoTarjetaCredito, movimientos.MonedaCLP, 1),
		}
		overrides := []presupuesto.Override{
			{MovimientoID: "sql-1", Fecha: "2026-05-10", MontoOriginal: -3, Descripcion: "WINDSCRIBE", EsUSD: ptrB(true)},
		}

		movs := presupuesto.VistaMovimientos(cargos, overrides, nil)
		if len(movs) != 1 {
			t.Fatalf("esperaba 1, obtuve %d", len(movs))
		}
		if !movs[0].IsUSD {
			t.Errorf("IsUSD = %v, want true (override debe forzar USD)", movs[0].IsUSD)
		}
	})

	t.Run("preserva el orden de entrada", func(t *testing.T) {
		cargos := []movimientos.Persistido{
			cargo(1, "2026-05-10", -10000, "Primero", movimientos.InstrumentoCuentaCorriente, movimientos.MonedaCLP, 1),
			cargo(2, "2026-05-11", -20000, "Segundo", movimientos.InstrumentoCuentaCorriente, movimientos.MonedaCLP, 1),
		}
		movs := presupuesto.VistaMovimientos(cargos, nil, nil)
		if len(movs) != 2 || movs[0].Descripcion != "Primero" || movs[1].Descripcion != "Segundo" {
			t.Fatalf("VistaMovimientos debería preservar el orden de entrada, got %+v", movs)
		}
	})
}
