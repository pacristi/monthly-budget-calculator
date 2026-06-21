package banco_de_chile

import (
	"fmt"
	"strings"
	"time"

	"github.com/extrame/xls"

	"github.com/pierocristi/monthly-budget-calculator/internal/cartola/ingest"
)

// BchileTCInternacional parsea cartolas mensuales de la tarjeta de
// crédito internacional de Banco de Chile. Los montos vienen en USD;
// el parser marca cada movimiento con IsUSD=true.
type BchileTCInternacional struct{}

func NewBchileTCInternacional() *BchileTCInternacional {
	return &BchileTCInternacional{}
}

func (p *BchileTCInternacional) Banco() string  { return "bchile" }
func (p *BchileTCInternacional) Source() string { return "tc_internacional" }

func (p *BchileTCInternacional) Parsear(path string, _ int) ([]ingest.MovimientoBruto, error) {
	filas, err := leerFilasTCI(path)
	if err != nil {
		return nil, fmt.Errorf("leyendo %s: %w", path, err)
	}
	return filasTCIAMovimientos(filas)
}

type filaTCI struct {
	categoria   string
	fecha       string
	descripcion string
	pais        string
	montoOrigen float64
	montoUSD    float64
}

func leerFilasTCI(path string) ([]filaTCI, error) {
	wb, err := xls.Open(path, "utf-8")
	if err != nil {
		return nil, err
	}
	sheet := wb.GetSheet(0)
	if sheet == nil {
		return nil, fmt.Errorf("sin hoja 0")
	}

	var out []filaTCI
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
		out = append(out, filaTCI{
			categoria:   strings.TrimSpace(row.Col(1)),
			fecha:       strings.TrimSpace(row.Col(3)),
			descripcion: strings.TrimSpace(row.Col(4)),
			pais:        strings.TrimSpace(row.Col(6)),
			montoOrigen: parseFloat(row.Col(7)),
			montoUSD:    parseFloat(row.Col(8)),
		})
	}
	return out, nil
}

func filasTCIAMovimientos(filas []filaTCI) ([]ingest.MovimientoBruto, error) {
	var out []ingest.MovimientoBruto

	for _, f := range filas {
		if esFilaTCIVacia(f) {
			continue
		}
		if strings.Contains(strings.ToLower(f.categoria), "información") {
			continue
		}

		fecha, err := time.Parse("02/01/2006", f.fecha)
		if err != nil {
			continue
		}

		// Convención del banco para TC internacional (distinta a la TC
		// nacional): las compras vienen positivas y los pagos a la TC
		// vienen negativos. Negar todo deja cargos como negativos y
		// abonos como positivos, alineado con la convención del proyecto.
		monto := -f.montoUSD
		if monto == 0 {
			continue
		}

		out = append(out, ingest.MovimientoBruto{
			Banco:           "bchile",
			Source:          "tc_internacional",
			Fecha:           fecha,
			Monto:           monto,
			Descripcion:     f.descripcion,
			Instrumento:     ingest.InstrumentoTarjetaCredito,
			Moneda:          ingest.MonedaUSD,
			MontoRepresenta: ingest.MontoRepresentaTotal,
			CuotaActual:     1,
			CuotasTotales:   1,
			IsUSD:           true,
			Cuotas:          "",
			Raw: map[string]any{
				"categoria":           f.categoria,
				"fecha_xls":           f.fecha,
				"descripcion":         f.descripcion,
				"pais":                f.pais,
				"monto_moneda_origen": f.montoOrigen,
				"monto_usd":           f.montoUSD,
			},
		})
	}
	return out, nil
}

func esFilaTCIVacia(f filaTCI) bool {
	return f.categoria == "" && f.fecha == "" && f.descripcion == "" && f.montoUSD == 0
}
