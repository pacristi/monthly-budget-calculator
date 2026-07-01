package defjson

import (
	"fmt"
	"sort"
	"time"

	"presupuesto/presupuesto"
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
				Porcentajes:          resolverPorcentajes(c),
				PorcentajeParaGastos: c.PorcentajeParaGastos,
				DiaDeCorteCredito:    c.DiaDeCorteCredito,
				TasaCambioUSD:        c.TasaCambioUSD,
				HeredadaDe:           c.MesDesde,
			}, nil
		}
	}

	return presupuesto.ConfigPresupuesto{}, fmt.Errorf("no hay config aplicable para %s (no hay declaraciones <= ese mes)", FormatMes(mesNormalizado))
}

// resolverPorcentajes devuelve el mapa de porcentajes por categoría. Si la
// config trae el formato nuevo (Porcentajes), lo usa tal cual; si solo trae el
// viejo (PorcentajeParaGastos), lo mapea a la categoría de gasto legacy.
func resolverPorcentajes(c ConfigMensual) map[string]float64 {
	if len(c.Porcentajes) > 0 {
		return c.Porcentajes
	}
	return map[string]float64{CategoriaGastoLegacy: c.PorcentajeParaGastos}
}
