package presupuesto

import (
	"testing"
	"time"
)

type fakeProveedor struct {
	sueldo float64
	gastos []Gasto
}

func (f fakeProveedor) ObtenerSueldoBase(PeriodoPresupuestario) (float64, error) {
	return f.sueldo, nil
}
func (f fakeProveedor) ObtenerGastosValidos(PeriodoPresupuestario) ([]Gasto, error) {
	return f.gastos, nil
}

type fakeResolvedor struct{ cfg ConfigPresupuesto }

func (f fakeResolvedor) ParaMes(time.Time) (ConfigPresupuesto, error) { return f.cfg, nil }

func periodoDe(year int, month time.Month) PeriodoPresupuestario {
	inicio := time.Date(year, month, 1, 0, 0, 0, 0, time.UTC)
	return PeriodoPresupuestario{Inicio: inicio, Fin: inicio.AddDate(0, 1, 0).Add(-time.Nanosecond)}
}

func gastoEn(id, categoriaID string, monto float64, dia int) Gasto {
	return Gasto{
		ID:               id,
		MontoImputado:    monto,
		Cuotas:           1,
		FechaTransaccion: time.Date(2026, time.May, dia, 0, 0, 0, 0, time.UTC),
		PoliticaCorte:    PoliticaCorte{Tipo: Debito},
		CategoriaID:      categoriaID,
	}
}

func TestCalcularResumen_UnaCategoriaLimite(t *testing.T) {
	periodo := periodoDe(2026, time.May)
	prov := fakeProveedor{
		sueldo: 1000,
		gastos: []Gasto{
			gastoEn("1", "gasto", 100, 5),
			gastoEn("2", "gasto", 50, 10),
		},
	}
	res := fakeResolvedor{cfg: ConfigPresupuesto{Porcentajes: map[string]float64{"gasto": 0.25}}}
	categorias := []Categoria{{ID: "gasto", Nombre: "Gasto", Tipo: Limite}}

	calc := NewCalculadora(prov, res)
	resumen, err := calc.CalcularResumen(periodo, categorias)
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}

	if resumen.Sueldo != 1000 {
		t.Errorf("Sueldo = %v, want 1000", resumen.Sueldo)
	}
	if len(resumen.Categorias) != 1 {
		t.Fatalf("Categorias = %d, want 1", len(resumen.Categorias))
	}
	c := resumen.Categorias[0]
	if c.CategoriaID != "gasto" {
		t.Errorf("CategoriaID = %q, want gasto", c.CategoriaID)
	}
	if c.Presupuesto != 250 {
		t.Errorf("Presupuesto = %v, want 250", c.Presupuesto)
	}
	if c.Acumulado != 150 {
		t.Errorf("Acumulado = %v, want 150", c.Acumulado)
	}
	if resumen.SinAsignar != 750 {
		t.Errorf("SinAsignar = %v, want 750", resumen.SinAsignar)
	}
}

func TestCalcularResumen_MultiplesCategorias(t *testing.T) {
	periodo := periodoDe(2026, time.May)
	prov := fakeProveedor{
		sueldo: 1000,
		gastos: []Gasto{
			gastoEn("1", "gasto", 100, 5),
			gastoEn("2", "ahorro", 200, 10),
		},
	}
	res := fakeResolvedor{cfg: ConfigPresupuesto{Porcentajes: map[string]float64{
		"gasto":  0.25,
		"ahorro": 0.50,
		// "inversion" sin porcentaje declarado este mes
	}}}
	categorias := []Categoria{
		{ID: "gasto", Nombre: "Gasto", Tipo: Limite},
		{ID: "ahorro", Nombre: "Ahorro", Tipo: Meta},
		{ID: "inversion", Nombre: "Inversión", Tipo: Meta},
	}

	resumen, err := NewCalculadora(prov, res).CalcularResumen(periodo, categorias)
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}

	porID := make(map[string]ResultadoCategoria)
	for _, c := range resumen.Categorias {
		porID[c.CategoriaID] = c
	}

	if g := porID["gasto"]; g.Tipo != Limite || g.Presupuesto != 250 || g.Acumulado != 100 {
		t.Errorf("gasto = %+v, want tipo=limite presupuesto=250 acumulado=100", g)
	}
	if a := porID["ahorro"]; a.Tipo != Meta || a.Presupuesto != 500 || a.Acumulado != 200 {
		t.Errorf("ahorro = %+v, want tipo=meta presupuesto=500 acumulado=200", a)
	}
	if i := porID["inversion"]; i.Presupuesto != 0 || i.Acumulado != 0 {
		t.Errorf("inversion = %+v, want presupuesto=0 acumulado=0", i)
	}
	if resumen.SinAsignar != 250 {
		t.Errorf("SinAsignar = %v, want 250 (1000 * (1 - 0.75))", resumen.SinAsignar)
	}
}
