package presupuesto

import (
	"testing"
	"time"
)

func TestProyectorPresupuesto_Proyectar(t *testing.T) {
	proyector := NewProyectorPresupuesto()

	// Helper para crear fechas de manera sencilla
	fecha := func(year int, month time.Month, day int) time.Time {
		return time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
	}

	tests := []struct {
		name               string
		gastos             []Gasto
		mesBase            time.Time
		mesesHaciaAdelante int
		want               []ProyeccionMensual
	}{
		{
			name:               "Sin gastos debe retornar 0 en todos los meses proyectados",
			gastos:             []Gasto{},
			mesBase:            fecha(2026, time.May, 1),
			mesesHaciaAdelante: 3,
			want: []ProyeccionMensual{
				{Anio: 2026, Mes: time.May, TotalComprometido: 0},
				{Anio: 2026, Mes: time.June, TotalComprometido: 0},
				{Anio: 2026, Mes: time.July, TotalComprometido: 0},
			},
		},
		{
			name: "Gastos de 1 cuota no deben proyectarse a meses futuros",
			gastos: []Gasto{
				{
					ID:               "1",
					MontoImputado:    1000,
					Cuotas:           1,
					FechaTransaccion: fecha(2026, time.May, 10),
					PoliticaCorte:    PoliticaCorte{Tipo: Debito},
				},
			},
			mesBase:            fecha(2026, time.May, 1),
			mesesHaciaAdelante: 3,
			want: []ProyeccionMensual{
				{Anio: 2026, Mes: time.May, TotalComprometido: 1000},
				{Anio: 2026, Mes: time.June, TotalComprometido: 0},
				{Anio: 2026, Mes: time.July, TotalComprometido: 0},
			},
		},
		{
			name: "Gastos de 12 cuotas pasando de un año a otro",
			gastos: []Gasto{
				{
					ID:               "2",
					MontoImputado:    12000,
					Cuotas:           12,
					FechaTransaccion: fecha(2026, time.November, 15),
					PoliticaCorte:    PoliticaCorte{Tipo: Debito},
				},
			},
			mesBase:            fecha(2026, time.November, 1),
			mesesHaciaAdelante: 5,
			want: []ProyeccionMensual{
				{Anio: 2026, Mes: time.November, TotalComprometido: 1000},
				{Anio: 2026, Mes: time.December, TotalComprometido: 1000},
				{Anio: 2027, Mes: time.January, TotalComprometido: 1000},
				{Anio: 2027, Mes: time.February, TotalComprometido: 1000},
				{Anio: 2027, Mes: time.March, TotalComprometido: 1000},
			},
		},
		{
			name: "Gastos antiguos cuyas cuotas ya caducaron",
			gastos: []Gasto{
				{
					ID:               "3",
					MontoImputado:    3000,
					Cuotas:           3,
					FechaTransaccion: fecha(2026, time.January, 5),
					PoliticaCorte:    PoliticaCorte{Tipo: Debito},
				},
			},
			mesBase:            fecha(2026, time.May, 1),
			mesesHaciaAdelante: 2,
			want: []ProyeccionMensual{
				// Enero, Febrero, Marzo fueron los meses de pago. Mayo ya no tiene cuota.
				{Anio: 2026, Mes: time.May, TotalComprometido: 0},
				{Anio: 2026, Mes: time.June, TotalComprometido: 0},
			},
		},
		{
			name: "Múltiples gastos superpuestos",
			gastos: []Gasto{
				{
					ID:               "4",
					MontoImputado:    600,
					Cuotas:           3,
					FechaTransaccion: fecha(2026, time.April, 15),
					PoliticaCorte:    PoliticaCorte{Tipo: Debito},
				}, // Paga en Abril(200), Mayo(200), Junio(200)
				{
					ID:               "5",
					MontoImputado:    1000,
					Cuotas:           2,
					FechaTransaccion: fecha(2026, time.May, 20),
					PoliticaCorte:    PoliticaCorte{Tipo: Debito},
				}, // Paga en Mayo(500), Junio(500)
			},
			mesBase:            fecha(2026, time.May, 1),
			mesesHaciaAdelante: 3,
			want: []ProyeccionMensual{
				{Anio: 2026, Mes: time.May, TotalComprometido: 700},   // 200 (gasto 4) + 500 (gasto 5)
				{Anio: 2026, Mes: time.June, TotalComprometido: 700},  // 200 (gasto 4) + 500 (gasto 5)
				{Anio: 2026, Mes: time.July, TotalComprometido: 0},    // Ya pagaron ambos
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := proyector.Proyectar(tt.gastos, tt.mesBase, tt.mesesHaciaAdelante)

			if len(got) != len(tt.want) {
				t.Fatalf("Proyectar() retornó %d meses, se esperaban %d", len(got), len(tt.want))
			}

			for i := range got {
				if got[i].Anio != tt.want[i].Anio || got[i].Mes != tt.want[i].Mes || got[i].TotalComprometido != tt.want[i].TotalComprometido {
					t.Errorf("Proyectar() para el mes %d = %v, se esperaba %v", i, got[i], tt.want[i])
				}
			}
		})
	}
}
