package bchile

import (
	"fmt"
	"strings"
	"time"

	"github.com/extrame/xls"

	"presupuesto/internal/cartola/ingest"
)

// TCNacional parsea cartolas mensuales de la tarjeta de crédito
// nacional de Banco de Chile. Las fechas vienen completas (dd/mm/yyyy),
// los montos vienen positivos (hay que asignar signo: cargo o pago), y
// hay filas informativas que filtrar (cuota 00/N y categoría "Información").
type TCNacional struct{}

func NewTCNacional() *TCNacional {
	return &TCNacional{}
}

func (p *TCNacional) Banco() string  { return "bchile" }
func (p *TCNacional) Source() string { return "tc_nacional" }

// Parsear ignora el año (las fechas vienen completas).
func (p *TCNacional) Parsear(path string, _ int) ([]ingest.MovimientoBruto, error) {
	filas, err := leerFilasTCN(path)
	if err != nil {
		return nil, fmt.Errorf("leyendo %s: %w", path, err)
	}
	return filasTCNAMovimientos(filas)
}

type filaTCN struct {
	categoria   string
	fecha       string
	descripcion string
	cuotas      string
	monto       float64
}

func leerFilasTCN(path string) ([]filaTCN, error) {
	wb, err := xls.Open(path, "utf-8")
	if err != nil {
		return nil, err
	}
	sheet := wb.GetSheet(0)
	if sheet == nil {
		return nil, fmt.Errorf("sin hoja 0")
	}

	var out []filaTCN
	headerFound := false
	for r := 0; r <= int(sheet.MaxRow); r++ {
		row := safeRow(sheet, r)
		if row == nil {
			continue
		}
		col1 := strings.TrimSpace(row.Col(1))
		if !headerFound {
			if col1 == "Categoría" {
				headerFound = true
			}
			continue
		}
		out = append(out, filaTCN{
			categoria:   strings.TrimSpace(row.Col(1)),
			fecha:       strings.TrimSpace(row.Col(2)),
			descripcion: strings.TrimSpace(row.Col(3)),
			cuotas:      strings.TrimSpace(row.Col(6)),
			monto:       parseFloat(row.Col(7)),
		})
	}
	return out, nil
}

func filasTCNAMovimientos(filas []filaTCN) ([]ingest.MovimientoBruto, error) {
	var out []ingest.MovimientoBruto

	for _, f := range filas {
		if esFilaTCNVacia(f) {
			continue
		}
		// NOTA: el banco repite la misma compra en cuotas cada mes con
		// cuota "M/N" (mismo monto total y fecha de origen). NO filtramos
		// aquí porque a veces la fila informativa "00/N" no se emite (por
		// ejemplo cuando la compra y su primera facturación caen en el mismo
		// mes). La deduplicación de compras en cuotas la maneja el writer
		// del sqlite, que ve el batch completo de todos los archivos.

		fecha, err := time.Parse("02/01/2006", f.fecha)
		if err != nil {
			continue
		}

		monto := f.monto
		if !esPagoTC(f.descripcion) {
			monto = -monto
		}
		if monto == 0 {
			continue
		}

		cuotaActual, cuotasTotales := parseCuotas(f.cuotas)
		// En el xlsx cada fila trae el monto de UNA cuota, no el total.
		representa := ingest.MontoRepresentaTotal
		if cuotasTotales > 1 {
			representa = ingest.MontoRepresentaCuota
		}

		out = append(out, ingest.MovimientoBruto{
			Banco:           "bchile",
			Source:          "tc_nacional",
			Fecha:           fecha,
			Monto:           monto,
			Descripcion:     f.descripcion,
			Instrumento:     ingest.InstrumentoTarjetaCredito,
			Moneda:          ingest.MonedaCLP,
			MontoRepresenta: representa,
			CuotaActual:     cuotaActual,
			CuotasTotales:   cuotasTotales,
			IsUSD:           false,
			Cuotas:          f.cuotas,
			Raw: map[string]any{
				"categoria":   f.categoria,
				"fecha_xls":   f.fecha,
				"descripcion": f.descripcion,
				"cuotas":      f.cuotas,
				"monto_xls":   f.monto,
			},
		})
	}
	return out, nil
}

func esFilaTCNVacia(f filaTCN) bool {
	return f.categoria == "" && f.fecha == "" && f.descripcion == "" && f.monto == 0
}

// esPagoTC detecta si una descripción corresponde a un pago del titular
// hacia la tarjeta de crédito (abono), basándose en el patrón "pago"
// seguido por alguno de los marcadores TEF/Pesos/Dolar/Manual usados
// por Banco de Chile.
func esPagoTC(descripcion string) bool {
	d := strings.ToLower(strings.TrimSpace(descripcion))
	if !strings.HasPrefix(d, "pago ") {
		return false
	}
	return strings.Contains(d, "tef") ||
		strings.Contains(d, "pesos") ||
		strings.Contains(d, "dolar") ||
		strings.Contains(d, "manual")
}
