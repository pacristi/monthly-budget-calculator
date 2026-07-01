package presupuesto

import (
	"fmt"
	"math"
	"strings"
	"time"

	"presupuesto/movimientos"
)

// EnriquecerGastos convierte cargos ya persistidos en gastos del presupuesto.
// Es puro: no toca I/O, recibe todo lo que necesita por parámetro. Aplica, en
// orden, para cada cargo: clasificación (override > regla > default, filtrando
// los Ignorado), override de "mi parte", normalización USD a CLP con la tasa
// del mes del movimiento, y política de corte según el instrumento canónico.
//
// Los gastos manuales NO entran aquí: son responsabilidad de
// definiciones/json.RepoGastosManuales y el caller los anexa aparte.
func EnriquecerGastos(cargos []movimientos.Persistido, overrides []Override, reglas []Regla, resolvedor ResolvedorConfig) ([]Gasto, error) {
	var gastos []Gasto
	for _, m := range cargos {
		fechaISO := m.Fecha.Format("2006-01-02")
		movimientoID := fmt.Sprintf("sql-%d", m.ID)

		overrideCat := CategoriaOverride(movimientoID, fechaISO, m.Monto, m.Descripcion, overrides)
		categoria := Clasificar(m.Descripcion, overrideCat, reglas, CategoriaPorDefecto)
		if categoria == Ignorado {
			continue
		}

		cfg, err := resolvedor.ParaMes(m.Fecha)
		if err != nil {
			return nil, fmt.Errorf("resolviendo config %s: %w", fechaISO, err)
		}

		montoCrudo := AplicarOverrides(movimientoID, m.Monto, fechaISO, m.Descripcion, overrides)
		montoImputado := math.Abs(NormalizarMonto(montoCrudo, m.Moneda == movimientos.MonedaUSD, cfg.TasaCambioUSD))

		tipo := Debito
		diaCorte := 0
		if m.Instrumento == movimientos.InstrumentoTarjetaCredito {
			tipo = Credito
			diaCorte = cfg.DiaDeCorteCredito
		}

		gastos = append(gastos, Gasto{
			ID:               movimientoID,
			Descripcion:      m.Descripcion,
			MontoImputado:    montoImputado,
			Cuotas:           m.CuotasTotales,
			FechaTransaccion: m.Fecha,
			PoliticaCorte: PoliticaCorte{
				Tipo:       tipo,
				DiaDeCorte: diaCorte,
			},
			CategoriaID: categoria,
		})
	}
	return gastos, nil
}

// VistaMovimientos convierte cargos persistidos en la vista de movimientos del
// dominio: aplica el override de "mi parte" y la clasificación (override >
// regla > default). Es puro y preserva el orden del slice de entrada; ordenar
// (p.ej. fecha DESC para la UI) es responsabilidad del caller, que controla el
// orden de la query. No aplica alias/DescripcionOverride: eso es presentación.
func VistaMovimientos(cargos []movimientos.Persistido, overrides []Override, reglas []Regla) []Movimiento {
	var out []Movimiento
	for _, m := range cargos {
		fechaISO := m.Fecha.Format("2006-01-02")
		movimientoID := fmt.Sprintf("sql-%d", m.ID)

		miParte := MiParteOverride(movimientoID, fechaISO, m.Monto, m.Descripcion, overrides)
		overrideCat := CategoriaOverride(movimientoID, fechaISO, m.Monto, m.Descripcion, overrides)
		categoria := Clasificar(m.Descripcion, overrideCat, reglas, CategoriaPorDefecto)

		out = append(out, Movimiento{
			ID:          movimientoID,
			Fecha:       m.Fecha,
			Descripcion: m.Descripcion,
			Monto:       m.Monto,
			IsUSD:       m.IsUSD,
			MiParte:     miParte,
			CategoriaID: categoria,
		})
	}
	return out
}

// VentanaSueldo devuelve el rango de búsqueda del sueldo que financia un
// periodo: el sueldo se deposita típicamente a fin del mes anterior o en los
// primeros días del mes, así que se busca en [inicio del mes anterior, inicio
// del mes + 10 días]. El caller usa este rango para consultar los abonos; la
// misma función lo reusa DetectarSueldo para el mensaje de error, sin duplicar
// la query SQL.
func VentanaSueldo(periodo PeriodoPresupuestario) (ini, fin time.Time) {
	ini = periodo.Inicio.AddDate(0, -1, 0)
	fin = periodo.Inicio.AddDate(0, 0, 10)
	return ini, fin
}

// DetectarSueldo elige, entre los abonos ya obtenidos (ordenados fecha DESC por
// la query), el primero cuya descripción matchea algún patrón de sueldo, y
// devuelve su monto absoluto. Es puro: la ventana de búsqueda y la query viven
// en el caller (ver VentanaSueldo / store.Abonos).
//
// Sin patrones configurados es un error explícito, en vez de "no encontrado"
// confuso. Si ningún abono matchea, el error reporta el periodo y la ventana.
func DetectarSueldo(abonos []movimientos.Persistido, patrones []string, periodo PeriodoPresupuestario) (float64, error) {
	if len(patrones) == 0 {
		return 0, fmt.Errorf("sueldo no encontrado para el periodo %s — %s: no hay patrones de sueldo configurados",
			periodo.Inicio.Format("2006-01-02"), periodo.Fin.Format("2006-01-02"))
	}

	for _, m := range abonos {
		if CoincidePatronSueldo(m.Descripcion, patrones) {
			return math.Abs(m.Monto), nil
		}
	}

	ini, fin := VentanaSueldo(periodo)
	return 0, fmt.Errorf("sueldo no encontrado para el periodo %s — %s (buscado entre %s y %s)",
		periodo.Inicio.Format("2006-01-02"), periodo.Fin.Format("2006-01-02"),
		ini.Format("2006-01-02"), fin.Format("2006-01-02"))
}

// NormalizarMonto convierte montos USD a CLP usando la tasa entregada. Es una
// regla de significado del presupuesto, no infraestructura.
func NormalizarMonto(monto float64, esUSD bool, tasaCambioUSD float64) float64 {
	if esUSD {
		return monto * tasaCambioUSD
	}
	return monto
}

// CoincidePatronSueldo retorna true si descripcion (case-insensitive) contiene
// alguno de los patrones.
func CoincidePatronSueldo(descripcion string, patrones []string) bool {
	desc := strings.ToLower(descripcion)
	for _, p := range patrones {
		if strings.Contains(desc, strings.ToLower(p)) {
			return true
		}
	}
	return false
}
