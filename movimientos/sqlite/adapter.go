package sqlite

import (
	"database/sql"
	"fmt"
	"math"
	"time"

	"presupuesto/cmd/cli/presentacion"
	defjson "presupuesto/definiciones/json"
	"presupuesto/movimientos"
	"presupuesto/movimientos/shared"
	"presupuesto/presupuesto"
)

// Adapter implementa presupuesto.ProveedorFinanciero leyendo movimientos
// crudos desde sqlite. Aplica los mismos filtros, normalizaciones y
// overrides que el adapter legacy obchile, pero contra la BD en lugar
// de un JSON puntual.
type Adapter struct {
	db             *sql.DB
	overrides      []presupuesto.Override
	reglas         []presupuesto.Regla
	patronesSueldo []string
	manuales       *defjson.RepoGastosManuales
	resolvedor     presupuesto.ResolvedorConfig
}

func NewAdapter(db *sql.DB, rutaDivisiones string, reglas []presupuesto.Regla, rutaSueldo, rutaManuales string, resolvedor presupuesto.ResolvedorConfig) *Adapter {
	overrides, _ := defjson.LeerOverrides(rutaDivisiones)
	patronesSueldo, _ := defjson.LeerListaStrings(rutaSueldo)
	return &Adapter{
		db:             db,
		overrides:      overrides,
		reglas:         reglas,
		patronesSueldo: patronesSueldo,
		manuales:       defjson.NewRepoGastosManuales(rutaManuales, resolvedor),
		resolvedor:     resolvedor,
	}
}

// ObtenerSueldoBase busca el sueldo que financia el periodo.
// Típicamente, el sueldo que se gasta en un mes (ej. Mayo) se deposita
// a finales del mes anterior (ej. 30 de Abril), o en los primeros días del mes.
// Por lo tanto, buscamos el último sueldo depositado en la ventana:
// [Inicio del mes anterior, Inicio del mes actual + 10 días].
func (a *Adapter) ObtenerSueldoBase(periodo presupuesto.PeriodoPresupuestario) (float64, error) {
	iniBusqueda := periodo.Inicio.AddDate(0, -1, 0) // mes anterior
	finBusqueda := periodo.Inicio.AddDate(0, 0, 10) // hasta el día 11 del mes actual

	if monto, ok, err := a.buscarSueldoEnRango(iniBusqueda, finBusqueda); err != nil {
		return 0, err
	} else if ok {
		return monto, nil
	}

	return 0, fmt.Errorf("sueldo no encontrado para el periodo %s — %s (buscado entre %s y %s)",
		periodo.Inicio.Format("2006-01-02"), periodo.Fin.Format("2006-01-02"),
		iniBusqueda.Format("2006-01-02"), finBusqueda.Format("2006-01-02"))
}

// buscarSueldoEnRango busca el último movimiento positivo dentro del rango
// [ini, fin] cuya descripción matchea alguno de los patrones de sueldo.
// Si la lista de patrones está vacía, retorna (0, false, nil) — el llamador
// reportará que no hay sueldo, en lugar de fallar de forma confusa.
//
// El match se hace en Go porque sqlite UPPER/LOWER no maneja unicode bien;
// además permite múltiples patrones sin armar SQL dinámico.
func (a *Adapter) buscarSueldoEnRango(ini, fin time.Time) (float64, bool, error) {
	if len(a.patronesSueldo) == 0 {
		return 0, false, nil
	}
	abonos, err := Abonos(a.db, ini, fin)
	if err != nil {
		return 0, false, err
	}
	for _, m := range abonos {
		if shared.CoincidePatronSueldo(m.Descripcion, a.patronesSueldo) {
			return math.Abs(m.Monto), true, nil
		}
	}
	return 0, false, nil
}

// ObtenerGastosValidos lee TODOS los movimientos con monto < 0 (cargos),
// excluye ignorables, aplica overrides y normalización USD, y los
// devuelve como Gasto. La política de corte usa el instrumento canónico
// persistido. Suma también los gastos manuales del JSON.
func (a *Adapter) ObtenerGastosValidos(_ presupuesto.PeriodoPresupuestario) ([]presupuesto.Gasto, error) {
	cargos, err := Cargos(a.db)
	if err != nil {
		return nil, err
	}

	var gastos []presupuesto.Gasto
	for _, m := range cargos {
		fechaISO := m.Fecha.Format("2006-01-02")
		movimientoID := fmt.Sprintf("sql-%d", m.ID)

		// Clasificar: override manual > regla por patrón > categoría default.
		overrideCat := presupuesto.CategoriaOverride(movimientoID, fechaISO, m.Monto, m.Descripcion, a.overrides)
		categoria := presupuesto.Clasificar(m.Descripcion, overrideCat, a.reglas, presupuesto.CategoriaPorDefecto)
		if categoria == presupuesto.Ignorado {
			continue
		}

		cfg, err := a.resolvedor.ParaMes(m.Fecha)
		if err != nil {
			return nil, fmt.Errorf("resolviendo config %s: %w", fechaISO, err)
		}

		montoCrudo := presupuesto.AplicarOverrides(movimientoID, m.Monto, fechaISO, m.Descripcion, a.overrides)
		montoImputado := math.Abs(shared.NormalizarMonto(montoCrudo, m.Moneda == movimientos.MonedaUSD, cfg.TasaCambioUSD))

		tipo := presupuesto.Debito
		diaCorte := 0
		if m.Instrumento == movimientos.InstrumentoTarjetaCredito {
			tipo = presupuesto.Credito
			diaCorte = cfg.DiaDeCorteCredito
		}

		gastos = append(gastos, presupuesto.Gasto{
			ID:               movimientoID,
			Descripcion:      m.Descripcion,
			MontoImputado:    montoImputado,
			Cuotas:           m.CuotasTotales,
			FechaTransaccion: m.Fecha,
			PoliticaCorte: presupuesto.PoliticaCorte{
				Tipo:       tipo,
				DiaDeCorte: diaCorte,
			},
			CategoriaID: categoria,
		})
	}

	manuales, err := a.manuales.Listar()
	if err != nil {
		return nil, err
	}
	gastos = append(gastos, manuales...)
	return gastos, nil
}

// ObtenerMovimientos devuelve solo los movimientos con monto negativo
// (cargos) aplicando overrides al campo MiParte.
func (a *Adapter) ObtenerMovimientos() ([]presupuesto.Movimiento, error) {
	cargos, err := Cargos(a.db)
	if err != nil {
		return nil, err
	}

	var out []presupuesto.Movimiento
	for _, m := range cargos {
		fechaISO := m.Fecha.Format("2006-01-02")
		movimientoID := fmt.Sprintf("sql-%d", m.ID)
		miParte := presupuesto.MiParteOverride(movimientoID, fechaISO, m.Monto, m.Descripcion, a.overrides)

		overrideCat := presupuesto.CategoriaOverride(movimientoID, fechaISO, m.Monto, m.Descripcion, a.overrides)
		categoria := presupuesto.Clasificar(m.Descripcion, overrideCat, a.reglas, presupuesto.CategoriaPorDefecto)

		out = append(out, presupuesto.Movimiento{
			ID:          movimientoID,
			Fecha:       m.Fecha,
			Descripcion: m.Descripcion,
			Monto:       m.Monto,
			IsUSD:       m.IsUSD,
			MiParte:     miParte,
			CategoriaID: categoria,
		})
	}

	// Cargos() ordena fecha ASC, id ASC; ObtenerMovimientos necesita
	// fecha DESC, id DESC — invertimos en Go en vez de duplicar la query.
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out, nil
}

func (a *Adapter) PresentarMovimientos() ([]presentacion.Movimiento, error) {
	movs, err := a.ObtenerMovimientos()
	if err != nil {
		return nil, err
	}
	return presentacion.Movimientos(movs, a.overrides), nil
}
