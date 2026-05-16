package sqlite

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"strings"
	"time"

	"github.com/pierocristi/monthly-budget-calculator/internal/cartola/shared"
	"github.com/pierocristi/monthly-budget-calculator/internal/presupuesto"
)

// Adapter implementa presupuesto.ProveedorFinanciero leyendo movimientos
// crudos desde sqlite. Aplica los mismos filtros, normalizaciones y
// overrides que el adapter legacy obchile, pero contra la BD en lugar
// de un JSON puntual.
type Adapter struct {
	db              *sql.DB
	overrides       []shared.Override
	exclusiones     []string
	patronesSueldo  []string
	rutaManuales    string
	resolvedor      presupuesto.ResolvedorConfig
}

func NewAdapter(db *sql.DB, rutaDivisiones, rutaExclusiones, rutaSueldo, rutaManuales string, resolvedor presupuesto.ResolvedorConfig) *Adapter {
	overrides, _ := shared.LeerOverrides(rutaDivisiones)
	exclusiones, _ := shared.LeerExclusiones(rutaExclusiones)
	patronesSueldo, _ := shared.LeerPatronesSueldo(rutaSueldo)
	return &Adapter{
		db:             db,
		overrides:      overrides,
		exclusiones:    exclusiones,
		patronesSueldo: patronesSueldo,
		rutaManuales:   rutaManuales,
		resolvedor:     resolvedor,
	}
}

// ObtenerSueldoBase busca el sueldo del periodo. Si no aparece (típico
// los primeros días del mes, antes del pago), cae al sueldo del mes
// anterior como estimación.
//
// Retorna error solo si tampoco hay registro en el mes anterior.
func (a *Adapter) ObtenerSueldoBase(periodo presupuesto.PeriodoPresupuestario) (float64, error) {
	if monto, ok, err := a.buscarSueldoEnRango(periodo.Inicio, periodo.Fin); err != nil {
		return 0, err
	} else if ok {
		return monto, nil
	}

	iniAnterior := periodo.Inicio.AddDate(0, -1, 0)
	finAnterior := periodo.Inicio.Add(-time.Nanosecond)
	if monto, ok, err := a.buscarSueldoEnRango(iniAnterior, finAnterior); err != nil {
		return 0, err
	} else if ok {
		return monto, nil
	}

	return 0, fmt.Errorf("sueldo no encontrado en periodo %s — %s ni en el mes anterior",
		periodo.Inicio.Format("2006-01-02"), periodo.Fin.Format("2006-01-02"))
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
// devuelve como Gasto. La política de corte (débito/crédito y día) se
// deriva del source. Suma también los gastos manuales del JSON.
func (a *Adapter) ObtenerGastosValidos(_ presupuesto.PeriodoPresupuestario) ([]presupuesto.Gasto, error) {
	rows, err := a.db.Query(`SELECT id, fecha, monto, descripcion, source, is_usd, cuotas
		FROM movimientos WHERE monto < 0 ORDER BY fecha ASC, id ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var gastos []presupuesto.Gasto
	for rows.Next() {
		var id int
		var fechaISO, descripcion, source, cuotasStr string
		var monto float64
		var isUSDInt int
		if err := rows.Scan(&id, &fechaISO, &monto, &descripcion, &source, &isUSDInt, &cuotasStr); err != nil {
			return nil, err
		}
		if shared.EsGastoIgnorable(descripcion, a.exclusiones) {
			continue
		}
		fechaTransaccion, err := time.Parse("2006-01-02", fechaISO)
		if err != nil {
			continue
		}
		cfg, err := a.resolvedor.ParaMes(fechaTransaccion)
		if err != nil {
			return nil, fmt.Errorf("resolviendo config %s: %w", fechaISO, err)
		}

		montoCrudo := shared.AplicarOverrides(monto, fechaISO, descripcion, a.overrides)
		montoImputado := math.Abs(shared.NormalizarMonto(montoCrudo, cfg.TasaCambioUSD))

		tipo := presupuesto.Debito
		diaCorte := 0
		if esCredito(source) {
			tipo = presupuesto.Credito
			diaCorte = cfg.DiaDeCorteCredito
		}

		gastos = append(gastos, presupuesto.Gasto{
			ID:               fmt.Sprintf("sql-%d", id),
			Descripcion:      descripcion,
			MontoImputado:    montoImputado,
			Cuotas:           shared.ParsearCuotas(cuotasStr),
			FechaTransaccion: fechaTransaccion,
			PoliticaCorte: presupuesto.PoliticaCorte{
				Tipo:       tipo,
				DiaDeCorte: diaCorte,
			},
		})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	manuales, err := a.leerGastosManuales()
	if err != nil {
		return nil, err
	}
	gastos = append(gastos, manuales...)
	return gastos, nil
}

// ObtenerMovimientos devuelve solo los movimientos con monto negativo
// (cargos) aplicando overrides al campo MiParte.
func (a *Adapter) ObtenerMovimientos() ([]presupuesto.Movimiento, error) {
	rows, err := a.db.Query(`SELECT fecha, monto, descripcion, is_usd
		FROM movimientos WHERE monto < 0 ORDER BY fecha DESC, id DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []presupuesto.Movimiento
	for rows.Next() {
		var fechaISO, descripcion string
		var monto float64
		var isUSDInt int
		if err := rows.Scan(&fechaISO, &monto, &descripcion, &isUSDInt); err != nil {
			return nil, err
		}
		fecha, err := time.Parse("2006-01-02", fechaISO)
		if err != nil {
			continue
		}

		var miParte *float64
		for _, o := range a.overrides {
			if o.Descripcion == "" {
				continue
			}
			if o.Fecha == fechaISO && o.MontoOriginal == monto && o.Descripcion == descripcion {
				v := o.MiParte
				miParte = &v
				break
			}
		}

		out = append(out, presupuesto.Movimiento{
			Fecha:       fecha,
			Descripcion: descripcion,
			Monto:       monto,
			IsUSD:       isUSDInt != 0,
			MiParte:     miParte,
		})
	}
	return out, rows.Err()
}

func (a *Adapter) leerGastosManuales() ([]presupuesto.Gasto, error) {
	if a.rutaManuales == "" {
		return nil, nil
	}
	data, err := os.ReadFile(a.rutaManuales)
	if err != nil {
		return nil, nil // tolerar archivo ausente
	}
	var dtos []manualDTO
	if err := json.Unmarshal(data, &dtos); err != nil {
		return nil, nil
	}
	var out []presupuesto.Gasto
	for _, dto := range dtos {
		fechaTransaccion, err := time.Parse("02-01-2006", dto.FechaInicio)
		if err != nil {
			continue
		}
		cfg, err := a.resolvedor.ParaMes(fechaTransaccion)
		if err != nil {
			return nil, err
		}
		tipo := presupuesto.Debito
		diaCorte := 0
		if strings.ToLower(dto.TipoPago) == "credito" {
			tipo = presupuesto.Credito
			diaCorte = cfg.DiaDeCorteCredito
		}
		out = append(out, presupuesto.Gasto{
			ID:               dto.ID,
			Descripcion:      dto.Descripcion,
			MontoImputado:    dto.MontoTotal,
			Cuotas:           dto.CuotasTotales,
			FechaTransaccion: fechaTransaccion,
			PoliticaCorte:    presupuesto.PoliticaCorte{Tipo: tipo, DiaDeCorte: diaCorte},
		})
	}
	return out, nil
}

type manualDTO struct {
	ID            string  `json:"id"`
	Descripcion   string  `json:"descripcion"`
	MontoTotal    float64 `json:"montoTotal"`
	CuotasTotales int     `json:"cuotasTotales"`
	FechaInicio   string  `json:"fechaInicio"`
	TipoPago      string  `json:"tipoPago"`
}

func esCredito(source string) bool {
	s := strings.ToLower(source)
	return strings.Contains(s, "credito") ||
		strings.Contains(s, "credit_card") ||
		strings.HasPrefix(s, "tc_")
}
