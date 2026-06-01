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
	client         *Client
	overrides      []shared.Override
	reglas         []presupuesto.Regla
	patronesSueldo []string
	rutaManuales   string
	resolvedor     presupuesto.ResolvedorConfig
}

func NewAdapter(rutaJson string, rutaDivisiones string, reglas []presupuesto.Regla, rutaSueldo string, rutaManuales string, resolvedor presupuesto.ResolvedorConfig) *Adapter {
	overrides, _ := shared.LeerOverrides(rutaDivisiones)
	patronesSueldo, _ := shared.LeerPatronesSueldo(rutaSueldo)

	return &Adapter{
		client:         NewClient(rutaJson),
		overrides:      overrides,
		reglas:         reglas,
		patronesSueldo: patronesSueldo,
		rutaManuales:   rutaManuales,
		resolvedor:     resolvedor,
	}
}

func (a *Adapter) ObtenerSueldoBase(periodo presupuesto.PeriodoPresupuestario) (float64, error) {
	movimientos, err := a.client.Fetch()
	if err != nil {
		return 0, err
	}

	for _, mov := range movimientos {
		if shared.CoincidePatronSueldo(mov.Descripcion, a.patronesSueldo) {
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
		// 1. Filtrar positivos (abonos)
		if mov.Monto > 0 {
			continue
		}

		// 2. Parsear fecha (necesaria para resolver la config del mes del movimiento)
		fechaTransaccion, err := time.Parse("02-01-2006", mov.Fecha)
		if err != nil {
			continue
		}
		fechaISO := fechaTransaccion.Format("2006-01-02")

		// 3. Clasificar: override manual > regla por patrón > categoría default.
		//    Los movimientos clasificados como ignorados no cuentan en ningún lado.
		overrideCat := shared.CategoriaOverride(fechaISO, mov.Monto, mov.Descripcion, a.overrides)
		categoria := presupuesto.Clasificar(mov.Descripcion, overrideCat, a.reglas, presupuesto.CategoriaPorDefecto)
		if categoria == presupuesto.Ignorado {
			continue
		}

		cfg, err := a.resolvedor.ParaMes(fechaTransaccion)
		if err != nil {
			return nil, fmt.Errorf("resolviendo config para %s: %w", mov.Fecha, err)
		}

		// 4. Aplicar override (en cruda) y luego normalizar a CLP con tasa del mes
		montoCrudo := shared.AplicarOverrides(mov.Monto, fechaISO, mov.Descripcion, a.overrides)
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
			CategoriaID: categoria,
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
			CategoriaID: presupuesto.CategoriaPorDefecto,
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
				miParte = o.MiParte
				break
			}
		}

		overrideCat := shared.CategoriaOverride(fechaISO, m.Monto, m.Descripcion, a.overrides)
		categoria := presupuesto.Clasificar(m.Descripcion, overrideCat, a.reglas, presupuesto.CategoriaPorDefecto)

		isUSD := float64(int64(m.Monto)) != m.Monto

		result = append(result, presupuesto.Movimiento{
			Fecha:       fecha,
			Descripcion: m.Descripcion,
			Monto:       m.Monto,
			IsUSD:       isUSD,
			MiParte:     miParte,
			CategoriaID: categoria,
		})
	}
	return result, nil
}
