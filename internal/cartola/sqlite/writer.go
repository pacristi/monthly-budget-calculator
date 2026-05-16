package sqlite

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/pierocristi/monthly-budget-calculator/internal/cartola/ingest"
)

// Writer inserta MovimientoBruto en sqlite aplicando dedup count-based
// por la llave (banco, source, fecha, monto, descripcion).
//
// El campo Origen identifica de qué pipeline viene la inserción
// ("xlsx", "obchile") y se persiste por trazabilidad — no participa
// en la llave de dedup.
type Writer struct {
	db     *sql.DB
	origen string
}

// NewWriter construye un Writer. `origen` etiqueta cada inserción con
// la fuente que la produjo.
func NewWriter(db *sql.DB, origen string) *Writer {
	return &Writer{db: db, origen: origen}
}

// InsertarConDedup inserta los movimientos del batch aplicando dedup
// count-based: para cada llave (banco, source, fecha, monto, descripcion)
// que aparece N veces en el batch, compara con cuántos hay ya en BD
// y solo inserta la diferencia.
//
// Retorna la cantidad de filas efectivamente insertadas.
func (w *Writer) InsertarConDedup(batch []ingest.MovimientoBruto) (int, error) {
	grouped := groupByDedupKey(batch)
	fechaCarga := time.Now().UTC().Format(time.RFC3339)
	total := 0

	for _, group := range grouped {
		first := group[0]
		dbCount, err := w.countByKey(first)
		if err != nil {
			return total, fmt.Errorf("count para llave (%s,%s,%s,%v,%s): %w",
				first.Banco, first.Source, first.Fecha.Format("2006-01-02"), first.Monto, first.Descripcion, err)
		}
		toInsert := len(group) - dbCount
		if toInsert <= 0 {
			continue
		}
		for _, m := range group[:toInsert] {
			if err := w.insertOne(m, fechaCarga); err != nil {
				return total, fmt.Errorf("insert: %w", err)
			}
			total++
		}
	}
	return total, nil
}

type dedupKey struct {
	banco, source, fecha, descripcion string
	monto                             float64
}

func keyOf(m ingest.MovimientoBruto) dedupKey {
	return dedupKey{
		banco:       m.Banco,
		source:      m.Source,
		fecha:       m.Fecha.Format("2006-01-02"),
		monto:       m.Monto,
		descripcion: m.Descripcion,
	}
}

func groupByDedupKey(batch []ingest.MovimientoBruto) map[dedupKey][]ingest.MovimientoBruto {
	out := map[dedupKey][]ingest.MovimientoBruto{}
	for _, m := range batch {
		k := keyOf(m)
		out[k] = append(out[k], m)
	}
	return out
}

func (w *Writer) countByKey(m ingest.MovimientoBruto) (int, error) {
	var n int
	err := w.db.QueryRow(`SELECT COUNT(*) FROM movimientos
		WHERE banco = ? AND source = ? AND fecha = ? AND monto = ? AND descripcion = ?`,
		m.Banco, m.Source, m.Fecha.Format("2006-01-02"), m.Monto, m.Descripcion,
	).Scan(&n)
	return n, err
}

func (w *Writer) insertOne(m ingest.MovimientoBruto, fechaCarga string) error {
	raw := m.Raw
	if raw == nil {
		raw = map[string]any{}
	}
	rawJSON, err := json.Marshal(raw)
	if err != nil {
		return fmt.Errorf("marshal raw: %w", err)
	}
	isUSD := 0
	if m.IsUSD {
		isUSD = 1
	}
	_, err = w.db.Exec(`INSERT INTO movimientos
		(banco, source, fecha, monto, descripcion, is_usd, cuotas, raw, origen, fecha_carga)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		m.Banco, m.Source, m.Fecha.Format("2006-01-02"), m.Monto, m.Descripcion,
		isUSD, m.Cuotas, string(rawJSON), w.origen, fechaCarga,
	)
	return err
}
