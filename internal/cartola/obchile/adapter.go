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
	client          *Client
	overrides       []shared.Override
	rutaManuales    string
	tasaCambioUSD   float64
	diaCorteCredito int
}

func NewAdapter(rutaJson string, rutaDivisiones string, rutaManuales string, tasaCambioUSD float64, diaCorteCredito int) *Adapter {
	overrides, _ := shared.LeerOverrides(rutaDivisiones)
	
	return &Adapter{
		client:          NewClient(rutaJson),
		overrides:       overrides,
		rutaManuales:    rutaManuales,
		tasaCambioUSD:   tasaCambioUSD,
		diaCorteCredito: diaCorteCredito,
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
		if mov.Monto > 0 || shared.EsGastoIgnorable(mov.Descripcion) {
			continue
		}

		// 2. Usar shared para moneda y overrides
		montoNormalizado := shared.NormalizarMonto(mov.Monto, a.tasaCambioUSD)
		montoImputado := math.Abs(montoNormalizado)
		montoImputado = shared.AplicarOverrides(montoImputado, mov.Monto, mov.Fecha, a.overrides)

		// 3. Parsear Fecha
		fechaTransaccion, err := time.Parse("02-01-2006", mov.Fecha)
		if err != nil {
			continue // Ignorar fechas inválidas
		}

		// 4. Determinar política de corte
		tipoPago := presupuesto.Debito
		diaCorte := 0
		sourceLower := strings.ToLower(mov.Source)
		if strings.Contains(sourceLower, "credito") || strings.Contains(sourceLower, "credit_card") {
			tipoPago = presupuesto.Credito
			diaCorte = a.diaCorteCredito
		}

		cuotas := shared.ParsearCuotas(mov.Installments)

		// 5. Instanciar Entidad de Dominio
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

	gastosManuales := a.leerGastosManuales()
	gastos = append(gastos, gastosManuales...)

	return gastos, nil
}

func (a *Adapter) leerGastosManuales() []presupuesto.Gasto {
	if a.rutaManuales == "" {
		return nil
	}

	f, err := os.Open(a.rutaManuales)
	if err != nil {
		// Manejar gracefully si el archivo no existe
		return nil
	}
	defer f.Close()

	bytes, err := io.ReadAll(f)
	if err != nil {
		return nil
	}

	var dtos []GastoManualDTO
	if err := json.Unmarshal(bytes, &dtos); err != nil {
		return nil
	}

	var gastos []presupuesto.Gasto
	for _, dto := range dtos {
		fechaTransaccion, err := time.Parse("02-01-2006", dto.FechaInicio)
		if err != nil {
			continue
		}

		tipoPago := presupuesto.Debito
		diaCorte := 0
		if strings.ToLower(dto.TipoPago) == "credito" {
			tipoPago = presupuesto.Credito
			diaCorte = a.diaCorteCredito
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

	return gastos
}
