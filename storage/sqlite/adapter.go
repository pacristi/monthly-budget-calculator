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
	rows, err := a.db.Query(`SELECT fecha, monto, descripcion FROM movimientos
		WHERE fecha BETWEEN ? AND ? AND monto > 0
		ORDER BY fecha DESC`,
		ini.Format("2006-01-02"), fin.Format("2006-01-02"),
	)
	if err != nil {
		return 0, false, err
	}
	defer rows.Close()
	for rows.Next() {
		var fecha, descripcion string
		var monto float64
		if err := rows.Scan(&fecha, &monto, &descripcion); err != nil {
			return 0, false, err
		}
		if shared.CoincidePatronSueldo(descripcion, a.patronesSueldo) {
			return math.Abs(monto), true, nil
		}
	}
	return 0, false, rows.Err()
}

// ObtenerGastosValidos lee TODOS los movimientos con monto < 0 (cargos),
// excluye ignorables, aplica overrides y normalización USD, y los
// devuelve como Gasto. La política de corte usa el instrumento canónico
// persistido. Suma también los gastos manuales del JSON.
func (a *Adapter) ObtenerGastosValidos(_ presupuesto.PeriodoPresupuestario) ([]presupuesto.Gasto, error) {
	rows, err := a.db.Query(`SELECT id, fecha, monto, descripcion, instrumento, moneda, cuotas_totales
		FROM movimientos WHERE monto < 0 ORDER BY fecha ASC, id ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var gastos []presupuesto.Gasto
	for rows.Next() {
		var id int
		var fechaISO, descripcion, instrumento, moneda string
		var monto float64
		var cuotasTotales int
		if err := rows.Scan(&id, &fechaISO, &monto, &descripcion, &instrumento, &moneda, &cuotasTotales); err != nil {
			return nil, err
		}
		fechaTransaccion, err := time.Parse("2006-01-02", fechaISO)
		if err != nil {
			continue
		}

		movimientoID := fmt.Sprintf("sql-%d", id)

		// Clasificar: override manual > regla por patrón > categoría default.
		overrideCat := presupuesto.CategoriaOverride(movimientoID, fechaISO, monto, descripcion, a.overrides)
		categoria := presupuesto.Clasificar(descripcion, overrideCat, a.reglas, presupuesto.CategoriaPorDefecto)
		if categoria == presupuesto.Ignorado {
			continue
		}

		cfg, err := a.resolvedor.ParaMes(fechaTransaccion)
		if err != nil {
			return nil, fmt.Errorf("resolviendo config %s: %w", fechaISO, err)
		}

		montoCrudo := presupuesto.AplicarOverrides(movimientoID, monto, fechaISO, descripcion, a.overrides)
		montoImputado := math.Abs(shared.NormalizarMonto(montoCrudo, movimientos.Moneda(moneda) == movimientos.MonedaUSD, cfg.TasaCambioUSD))

		tipo := presupuesto.Debito
		diaCorte := 0
		if movimientos.Instrumento(instrumento) == movimientos.InstrumentoTarjetaCredito {
			tipo = presupuesto.Credito
			diaCorte = cfg.DiaDeCorteCredito
		}

		gastos = append(gastos, presupuesto.Gasto{
			ID:               movimientoID,
			Descripcion:      descripcion,
			MontoImputado:    montoImputado,
			Cuotas:           cuotasTotales,
			FechaTransaccion: fechaTransaccion,
			PoliticaCorte: presupuesto.PoliticaCorte{
				Tipo:       tipo,
				DiaDeCorte: diaCorte,
			},
			CategoriaID: categoria,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, err
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
	rows, err := a.db.Query(`SELECT id, fecha, monto, descripcion, is_usd
		FROM movimientos WHERE monto < 0 ORDER BY fecha DESC, id DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []presupuesto.Movimiento
	for rows.Next() {
		var id int
		var fechaISO, descripcion string
		var monto float64
		var isUSDInt int
		if err := rows.Scan(&id, &fechaISO, &monto, &descripcion, &isUSDInt); err != nil {
			return nil, err
		}
		fecha, err := time.Parse("2006-01-02", fechaISO)
		if err != nil {
			continue
		}

		movimientoID := fmt.Sprintf("sql-%d", id)
		miParte := presupuesto.MiParteOverride(movimientoID, fechaISO, monto, descripcion, a.overrides)

		overrideCat := presupuesto.CategoriaOverride(movimientoID, fechaISO, monto, descripcion, a.overrides)
		categoria := presupuesto.Clasificar(descripcion, overrideCat, a.reglas, presupuesto.CategoriaPorDefecto)

		out = append(out, presupuesto.Movimiento{
			ID:          movimientoID,
			Fecha:       fecha,
			Descripcion: descripcion,
			Monto:       monto,
			IsUSD:       isUSDInt != 0,
			MiParte:     miParte,
			CategoriaID: categoria,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, err
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
