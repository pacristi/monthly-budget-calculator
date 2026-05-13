package config

import (
	"fmt"
	"sort"
	"time"

	"github.com/pierocristi/monthly-budget-calculator/internal/presupuesto"
)

// ResolverParaMes encuentra la config aplicable para el mes dado, usando carry-forward:
// devuelve la config con mesDesde más reciente cuyo mesDesde <= mes. Mapea el resultado
// al tipo del dominio presupuesto.ConfigPresupuesto.
func ResolverParaMes(mes time.Time, configs []ConfigMensual) (presupuesto.ConfigPresupuesto, error) {
	if len(configs) == 0 {
		return presupuesto.ConfigPresupuesto{}, fmt.Errorf("no hay configs declaradas")
	}

	mesNormalizado := time.Date(mes.Year(), mes.Month(), 1, 0, 0, 0, 0, time.UTC)

	ordenadas := make([]ConfigMensual, len(configs))
	copy(ordenadas, configs)
	sort.Slice(ordenadas, func(i, j int) bool {
		return ordenadas[i].MesDesde > ordenadas[j].MesDesde
	})

	for _, c := range ordenadas {
		mesDesdeT, err := ParseMes(c.MesDesde)
		if err != nil {
			return presupuesto.ConfigPresupuesto{}, fmt.Errorf("config con mesDesde inválido %q: %w", c.MesDesde, err)
		}
		if !mesDesdeT.After(mesNormalizado) {
			return presupuesto.ConfigPresupuesto{
				PorcentajeParaGastos: c.PorcentajeParaGastos,
				DiaDeCorteCredito:    c.DiaDeCorteCredito,
				TasaCambioUSD:        c.TasaCambioUSD,
				HeredadaDe:           c.MesDesde,
			}, nil
		}
	}

	return presupuesto.ConfigPresupuesto{}, fmt.Errorf("no hay config aplicable para %s (no hay declaraciones <= ese mes)", FormatMes(mesNormalizado))
}
