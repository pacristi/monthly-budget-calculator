package sqlite

import (
	"database/sql"
	"time"

	"presupuesto/movimientos"
)

// Cargos lee todos los movimientos con monto < 0, ordenados por
// fecha ASC, id ASC. Selecciona el set de columnas suficiente para
// servir tanto a ObtenerGastosValidos (necesita instrumento, moneda,
// cuotas_totales) como a ObtenerMovimientos (necesita is_usd); el
// adapter decide qué campos usa y en qué orden los consume (puede
// invertir el slice para DESC sin duplicar la query).
func Cargos(db *sql.DB) ([]movimientos.Persistido, error) {
	rows, err := db.Query(`SELECT id, fecha, monto, descripcion, is_usd, instrumento, moneda, cuotas_totales
		FROM movimientos WHERE monto < 0 ORDER BY fecha ASC, id ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanPersistidos(rows)
}

// Abonos lee los movimientos con monto > 0 dentro del rango [desde, hasta],
// ordenados por fecha DESC (el llamador busca el primer match y se detiene).
func Abonos(db *sql.DB, desde, hasta time.Time) ([]movimientos.Persistido, error) {
	rows, err := db.Query(`SELECT id, fecha, monto, descripcion, is_usd, instrumento, moneda, cuotas_totales
		FROM movimientos WHERE fecha BETWEEN ? AND ? AND monto > 0 ORDER BY fecha DESC`,
		desde.Format("2006-01-02"), hasta.Format("2006-01-02"),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanPersistidos(rows)
}

func scanPersistidos(rows *sql.Rows) ([]movimientos.Persistido, error) {
	var out []movimientos.Persistido
	for rows.Next() {
		var id int64
		var fechaISO, descripcion, instrumento, moneda string
		var monto float64
		var isUSDInt, cuotasTotales int
		if err := rows.Scan(&id, &fechaISO, &monto, &descripcion, &isUSDInt, &instrumento, &moneda, &cuotasTotales); err != nil {
			return nil, err
		}
		fecha, err := time.Parse("2006-01-02", fechaISO)
		if err != nil {
			continue
		}
		out = append(out, movimientos.Persistido{
			ID: id,
			MovimientoBruto: movimientos.MovimientoBruto{
				Fecha:         fecha,
				Monto:         monto,
				Descripcion:   descripcion,
				IsUSD:         isUSDInt != 0,
				Instrumento:   movimientos.Instrumento(instrumento),
				Moneda:        movimientos.Moneda(moneda),
				CuotasTotales: cuotasTotales,
			},
		})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}
