package obchile

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"strings"
	"time"

	"github.com/pierocristi/monthly-budget-calculator/internal/cartola/shared"
	"github.com/pierocristi/monthly-budget-calculator/internal/presupuesto"
)

// Adapter implementa presupuesto.ProveedorFinanciero para OBCL.
type Adapter struct {
	client       *Client
	overrides    []shared.Override
	exclusiones  []string
	rutaManuales string
	resolvedor   presupuesto.ResolvedorConfig
}

func NewAdapter(rutaJson string, rutaDivisiones string, rutaExclusiones string, rutaManuales string, resolvedor presupuesto.ResolvedorConfig) *Adapter {
	overrides, _ := shared.LeerOverrides(rutaDivisiones)
	exclusiones, _ := shared.LeerExclusiones(rutaExclusiones)

	return &Adapter{
		client:       NewClient(rutaJson),
		overrides:    overrides,
		exclusiones:  exclusiones,
		rutaManuales: rutaManuales,
		resolvedor:   resolvedor,
	}
}

func (a *Adapter) ObtenerSueldoBase(periodo presupuesto.PeriodoPresupuestario) (float64, error) {
	movimientos, err := a.client.Fetch()
	if err != nil {
		return 0, err
	}

	for _, mov := range movimientos {
		desc := strings.ToLower(mov.Descripcion)
		if strings.Contains(desc, "pago de sueldos") || strings.Contains(desc, "pago:de sueldos") {
			return math.Abs(mov.Monto), nil
		}
	}

	return 0, fmt.Errorf("sueldo no encontrado")
}

func (a *Adapter) ObtenerGastosValidos(periodo presupuesto.PeriodoPresupuestario) ([]presupuesto.Gasto, error) {
	movimientos, err := a.client.Fetch()
	if err != nil {
		return nil, err
	}

	var gastos []presupuesto.Gasto

	for i, mov := range movimientos {
		// 1. Filtrar positivos (abonos) y exclusiones
		if mov.Monto > 0 || shared.EsGastoIgnorable(mov.Descripcion, a.exclusiones) {
			continue
		}

		// 2. Parsear fecha (necesaria para resolver la config del mes del movimiento)
		fechaTransaccion, err := time.Parse("02-01-2006", mov.Fecha)
		if err != nil {
			continue
		}

		cfg, err := a.resolvedor.ParaMes(fechaTransaccion)
		if err != nil {
			return nil, fmt.Errorf("resolviendo config para %s: %w", mov.Fecha, err)
		}

		// 3. Aplicar override (en cruda) y luego normalizar a CLP con tasa del mes
		montoCrudo := shared.AplicarOverrides(mov.Monto, fechaTransaccion.Format("2006-01-02"), mov.Descripcion, a.overrides)
		montoImputado := math.Abs(shared.NormalizarMonto(montoCrudo, cfg.TasaCambioUSD))

		// 4. Determinar política de corte (día de corte del mes del movimiento)
		tipoPago := presupuesto.Debito
		diaCorte := 0
		sourceLower := strings.ToLower(mov.Source)
		if strings.Contains(sourceLower, "credito") || strings.Contains(sourceLower, "credit_card") {
			tipoPago = presupuesto.Credito
			diaCorte = cfg.DiaDeCorteCredito
		}

		cuotas := shared.ParsearCuotas(mov.Installments)

		gastos = append(gastos, presupuesto.Gasto{
			ID:               fmt.Sprintf("mov-%s-%d", fechaTransaccion.Format("20060102"), i),
			Descripcion:      mov.Descripcion,
			MontoImputado:    montoImputado,
			Cuotas:           cuotas,
			FechaTransaccion: fechaTransaccion,
			PoliticaCorte: presupuesto.PoliticaCorte{
				Tipo:       tipoPago,
				DiaDeCorte: diaCorte,
			},
		})
	}

	gastosManuales, err := a.leerGastosManuales()
	if err != nil {
		return nil, err
	}
	gastos = append(gastos, gastosManuales...)

	return gastos, nil
}

func (a *Adapter) leerGastosManuales() ([]presupuesto.Gasto, error) {
	if a.rutaManuales == "" {
		return nil, nil
	}

	f, err := os.Open(a.rutaManuales)
	if err != nil {
		return nil, nil
	}
	defer f.Close()

	bytes, err := io.ReadAll(f)
	if err != nil {
		return nil, nil
	}

	var dtos []GastoManualDTO
	if err := json.Unmarshal(bytes, &dtos); err != nil {
		return nil, nil
	}

	var gastos []presupuesto.Gasto
	for _, dto := range dtos {
		fechaTransaccion, err := time.Parse("02-01-2006", dto.FechaInicio)
		if err != nil {
			continue
		}

		cfg, err := a.resolvedor.ParaMes(fechaTransaccion)
		if err != nil {
			return nil, fmt.Errorf("resolviendo config para gasto manual %s: %w", dto.ID, err)
		}

		tipoPago := presupuesto.Debito
		diaCorte := 0
		if strings.ToLower(dto.TipoPago) == "credito" {
			tipoPago = presupuesto.Credito
			diaCorte = cfg.DiaDeCorteCredito
		}

		gastos = append(gastos, presupuesto.Gasto{
			ID:               dto.ID,
			Descripcion:      dto.Descripcion,
			MontoImputado:    dto.MontoTotal,
			Cuotas:           dto.CuotasTotales,
			FechaTransaccion: fechaTransaccion,
			PoliticaCorte: presupuesto.PoliticaCorte{
				Tipo:       tipoPago,
				DiaDeCorte: diaCorte,
			},
		})
	}

	return gastos, nil
}

func (a *Adapter) ObtenerMovimientos() ([]presupuesto.Movimiento, error) {
	movs, err := a.client.Fetch()
	if err != nil {
		return nil, err
	}

	result := make([]presupuesto.Movimiento, 0, len(movs))
	for _, m := range movs {
		if m.Monto >= 0 {
			continue
		}

		fecha, err := time.Parse("02-01-2006", m.Fecha)
		if err != nil {
			continue
		}
		fechaISO := fecha.Format("2006-01-02")

		var miParte *float64
		for _, o := range a.overrides {
			if o.Descripcion == "" {
				continue
			}
			if o.Fecha == fechaISO && o.MontoOriginal == m.Monto && o.Descripcion == m.Descripcion {
				v := o.MiParte
				miParte = &v
				break
			}
		}

		isUSD := float64(int64(m.Monto)) != m.Monto

		result = append(result, presupuesto.Movimiento{
			Fecha:       fecha,
			Descripcion: m.Descripcion,
			Monto:       m.Monto,
			IsUSD:       isUSD,
			MiParte:     miParte,
		})
	}
	return result, nil
}
