package bchile

import (
	"fmt"
	"strings"
	"time"

	"github.com/extrame/xls"

	"presupuesto/internal/cartola/canonico"
)

// CuentaCorriente parsea cartolas mensuales de cuenta corriente de
// Banco de Chile. El formato:
//   - Filas 0-23: encabezado (titular, RUT, saldos, headers de cartola).
//   - Fila ~24: headers de movimientos.
//   - Filas siguientes: movimientos con fecha "dd/mm" (sin año), descripción,
//     canal, cargo (CLP), abono (CLP), saldo.
//
// Como las fechas no traen año, requiere el año explícito al parsear.
type CuentaCorriente struct{}

func NewCuentaCorriente() *CuentaCorriente {
	return &CuentaCorriente{}
}

func (p *CuentaCorriente) Banco() string  { return "bchile" }
func (p *CuentaCorriente) Source() string { return "cta_corriente" }

// Parsear lee el archivo .xls en `path` y devuelve los movimientos.
func (p *CuentaCorriente) Parsear(path string, año int) ([]canonico.MovimientoBruto, error) {
	filas, err := leerFilasCC(path)
	if err != nil {
		return nil, fmt.Errorf("leyendo %s: %w", path, err)
	}
	return filasAMovimientos(filas, año)
}

// filaCC representa una fila cruda de la cartola, ya con cargos y abonos
// parseados a float64 (y a 0 si la celda está vacía).
type filaCC struct {
	fecha       string
	descripcion string
	canal       string
	cargo       float64
	abono       float64
	saldo       float64
}

// leerFilasCC abre el .xls y extrae las filas relevantes (descartando
// el encabezado y la fila de headers).
func leerFilasCC(path string) ([]filaCC, error) {
	wb, err := xls.Open(path, "utf-8")
	if err != nil {
		return nil, err
	}
	sheet := wb.GetSheet(0)
	if sheet == nil {
		return nil, fmt.Errorf("sin hoja 0")
	}

	var out []filaCC
	headerFound := false
	for r := 0; r <= int(sheet.MaxRow); r++ {
		row := safeRow(sheet, r)
		if row == nil {
			continue
		}
		col1 := strings.TrimSpace(row.Col(1))
		if !headerFound {
			if col1 == "Fecha" {
				headerFound = true
			}
			continue
		}
		// Después del header, parseamos filas. Las columnas en extrame/xls
		// para este formato vienen desplazadas: col 0 está siempre vacía y
		// los datos viven en cols 1..6.
		f := filaCC{
			fecha:       strings.TrimSpace(row.Col(1)),
			descripcion: strings.TrimSpace(row.Col(2)),
			canal:       strings.TrimSpace(row.Col(3)),
			cargo:       parseFloat(row.Col(4)),
			abono:       parseFloat(row.Col(5)),
			saldo:       parseFloat(row.Col(6)),
		}
		out = append(out, f)
	}
	return out, nil
}

func parseFloat(s string) float64 {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	// extrame/xls devuelve números como string con punto decimal en celdas tipo number.
	var v float64
	_, err := fmt.Sscanf(s, "%f", &v)
	if err != nil {
		return 0
	}
	return v
}

// filasAMovimientos es la capa pura: convierte filas crudas a MovimientoBruto
// aplicando filtros (SALDO INICIAL, filas vacías, fechas inválidas) y la
// regla de signo (cargo negativo, abono positivo).
func filasAMovimientos(filas []filaCC, año int) ([]canonico.MovimientoBruto, error) {
	var out []canonico.MovimientoBruto

	for _, f := range filas {
		if esFilaVacia(f) {
			continue
		}
		if strings.EqualFold(f.descripcion, "SALDO INICIAL") {
			continue
		}

		fecha, err := time.Parse("02/01/2006", fmt.Sprintf("%s/%d", f.fecha, año))
		if err != nil {
			// Fechas no parseables (ej: filas-resumen) se ignoran silenciosamente.
			continue
		}

		monto := f.abono - f.cargo
		if monto == 0 {
			continue
		}

		out = append(out, canonico.MovimientoBruto{
			Banco:           "bchile",
			Source:          "cta_corriente",
			Fecha:           fecha,
			Monto:           monto,
			Descripcion:     f.descripcion,
			Instrumento:     canonico.InstrumentoCuentaCorriente,
			Moneda:          canonico.MonedaCLP,
			MontoRepresenta: canonico.MontoRepresentaTotal,
			CuotaActual:     1,
			CuotasTotales:   1,
			IsUSD:           false,
			Cuotas:          "",
			Raw: map[string]any{
				"fecha_xls":   f.fecha,
				"descripcion": f.descripcion,
				"canal":       f.canal,
				"cargo":       f.cargo,
				"abono":       f.abono,
				"saldo":       f.saldo,
			},
		})
	}
	return out, nil
}

func esFilaVacia(f filaCC) bool {
	return f.fecha == "" && f.descripcion == "" && f.cargo == 0 && f.abono == 0
}
