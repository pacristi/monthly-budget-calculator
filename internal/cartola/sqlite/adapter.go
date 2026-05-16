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
	db           *sql.DB
	overrides    []shared.Override
	rutaManuales string
	resolvedor   presupuesto.ResolvedorConfig
}

func NewAdapter(db *sql.DB, rutaDivisiones, rutaManuales string, resolvedor presupuesto.ResolvedorConfig) *Adapter {
	overrides, _ := shared.LeerOverrides(rutaDivisiones)
	return &Adapter{
		db:           db,
		overrides:    overrides,
		rutaManuales: rutaManuales,
		resolvedor:   resolvedor,
	}
}

// ObtenerSueldoBase busca el primer movimiento dentro del periodo cuya
// descripción contiene "pago de sueldos" o "pago:de sueldos" y devuelve
// su monto en valor absoluto.
func (a *Adapter) ObtenerSueldoBase(periodo presupuesto.PeriodoPresupuestario) (float64, error) {
	fechaIni := periodo.Inicio.Format("2006-01-02")
	fechaFin := periodo.Fin.Format("2006-01-02")
	row := a.db.QueryRow(`SELECT monto FROM movimientos
		WHERE fecha BETWEEN ? AND ?
		  AND (LOWER(descripcion) LIKE '%pago de sueldos%' OR LOWER(descripcion) LIKE '%pago:de sueldos%')
		ORDER BY fecha DESC LIMIT 1`,
		fechaIni, fechaFin,
	)
	var monto float64
	if err := row.Scan(&monto); err != nil {
		if err == sql.ErrNoRows {
			return 0, fmt.Errorf("sueldo no encontrado en periodo %s — %s", fechaIni, fechaFin)
		}
		return 0, err
	}
	return math.Abs(monto), nil
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
		if shared.EsGastoIgnorable(descripcion) {
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
