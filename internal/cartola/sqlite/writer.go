package sqlite

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
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

// InsertarConDedup inserta los movimientos del batch evitando duplicaciones.
//
// Trabaja en dos pasadas:
//
//  1. Compras en cuotas (cuota "M/N" con N>1): se agrupan por
//     (banco, source, fecha, descripcion, N) — sin el monto, porque el
//     banco a veces ajusta ±1 peso entre las últimas cuotas. De cada
//     grupo se inserta UNA sola fila (la "00/N" si existe, si no la de
//     menor M); si ya hay alguna fila con esa llave en BD, se omite.
//
//  2. Movimientos simples (sin cuotas o "01/01"): dedup count-based por
//     la llave clásica (banco, source, fecha, monto, descripcion). El
//     batch puede tener M y la BD N; se inserta max(0, M-N). Cubre el
//     caso "doble café".
//
// Retorna la cantidad de filas efectivamente insertadas.
func (w *Writer) InsertarConDedup(batch []ingest.MovimientoBruto) (int, error) {
	fechaCarga := time.Now().UTC().Format(time.RFC3339)
	total := 0

	enCuotas, simples := separarPorTipoDeCuota(batch)

	n, err := w.insertarCompraEnCuotas(enCuotas, fechaCarga)
	if err != nil {
		return total, err
	}
	total += n

	n, err = w.insertarSimples(simples, fechaCarga)
	if err != nil {
		return total, err
	}
	total += n

	return total, nil
}

// separarPorTipoDeCuota divide el batch en dos: filas que pertenecen a
// una compra en N>1 cuotas y filas "simples" (sin cuotas o "01/01").
func separarPorTipoDeCuota(batch []ingest.MovimientoBruto) (enCuotas, simples []ingest.MovimientoBruto) {
	for _, m := range batch {
		if cuotaConTotalMayorAUno(m.Cuotas) {
			enCuotas = append(enCuotas, m)
		} else {
			simples = append(simples, m)
		}
	}
	return
}

type cuotaCompraKey struct {
	banco, source, fecha, descripcion string
	totalCuotas                       int
}

func cuotaKeyOf(m ingest.MovimientoBruto) cuotaCompraKey {
	_, n, _ := parseCuotas(m.Cuotas)
	return cuotaCompraKey{
		banco:       m.Banco,
		source:      m.Source,
		fecha:       m.Fecha.Format("2006-01-02"),
		descripcion: m.Descripcion,
		totalCuotas: n,
	}
}

func (w *Writer) insertarCompraEnCuotas(batch []ingest.MovimientoBruto, fechaCarga string) (int, error) {
	grupos := map[cuotaCompraKey][]ingest.MovimientoBruto{}
	for _, m := range batch {
		k := cuotaKeyOf(m)
		grupos[k] = append(grupos[k], m)
	}

	total := 0
	for k, group := range grupos {
		yaExiste, err := w.compraEnCuotasYaEnBD(k)
		if err != nil {
			return total, fmt.Errorf("chequeando compra en cuotas (%s,%s,%s,%s,N=%d): %w",
				k.banco, k.source, k.fecha, k.descripcion, k.totalCuotas, err)
		}
		if yaExiste {
			continue
		}
		representante, ok := construirRepresentanteCompra(group)
		if !ok {
			// Solo había filas "00/N" (informativas, sin facturar). No
			// insertamos nada todavía; cuando aparezca al menos una cuota
			// facturada cargaremos la compra.
			continue
		}
		if err := w.insertOne(representante, fechaCarga); err != nil {
			return total, fmt.Errorf("insert compra en cuotas: %w", err)
		}
		total++
	}
	return total, nil
}

// construirRepresentanteCompra arma la fila única que va al sqlite para
// una compra en cuotas. El monto guardado es el TOTAL de la compra
// (no la cuota individual), porque el dominio divide MontoImputado entre
// Cuotas al proyectar al mes.
//
// Estrategia:
//   - Considera sólo las filas con M>=1 (cuotas facturadas). Las "00/N"
//     son informativas, no se han facturado.
//   - Si tenemos las N cuotas facturadas → total = suma exacta.
//   - Si tenemos K<N cuotas → estima total = round(suma/K * N). Se
//     refinará en cargas posteriores (pero como ya guardamos una fila,
//     hoy no se actualiza — ver compraEnCuotasYaEnBD).
//
// Retorna (representante, true) si pudo construirla, (zero, false) si
// el grupo no contiene cuotas facturadas.
func construirRepresentanteCompra(group []ingest.MovimientoBruto) (ingest.MovimientoBruto, bool) {
	var base ingest.MovimientoBruto
	baseM := -1
	var sumCuotas float64
	cuotasFacturadas := 0
	var totalCuotas int

	for _, m := range group {
		mNum, n, ok := parseCuotas(m.Cuotas)
		if !ok {
			continue
		}
		totalCuotas = n
		if mNum == 0 {
			continue // informativa, no factura
		}
		sumCuotas += m.Monto
		cuotasFacturadas++
		if baseM == -1 || mNum < baseM {
			baseM = mNum
			base = m
		}
	}

	if cuotasFacturadas == 0 {
		return ingest.MovimientoBruto{}, false
	}

	// Calcular el total. Si tenemos todas las cuotas, suma exacta. Si no,
	// extrapolar.
	var montoTotal float64
	if cuotasFacturadas == totalCuotas {
		montoTotal = sumCuotas
	} else {
		promedio := sumCuotas / float64(cuotasFacturadas)
		montoTotal = promedio * float64(totalCuotas)
	}

	base.Monto = montoTotal
	// Marcamos las cuotas como "00/N" para señalar que el monto guardado
	// es el TOTAL de la compra (convención: M=0 ↔ monto total).
	base.Cuotas = fmt.Sprintf("00/%02d", totalCuotas)
	return base, true
}

func (w *Writer) compraEnCuotasYaEnBD(k cuotaCompraKey) (bool, error) {
	// Una compra en cuotas se identifica por (banco, source, fecha,
	// descripcion) — el monto puede tener ±1 peso de ajuste, así que
	// no entra en la llave. Filtramos por N total parseando el campo
	// cuotas en Go porque el banco usa padding de ceros ("00/03") y
	// LIKE no se lleva bien con eso.
	rows, err := w.db.Query(`SELECT cuotas FROM movimientos
		WHERE banco = ? AND source = ? AND fecha = ? AND descripcion = ?`,
		k.banco, k.source, k.fecha, k.descripcion,
	)
	if err != nil {
		return false, err
	}
	defer rows.Close()
	for rows.Next() {
		var c string
		if err := rows.Scan(&c); err != nil {
			return false, err
		}
		if _, n, ok := parseCuotas(c); ok && n == k.totalCuotas {
			return true, nil
		}
	}
	return false, rows.Err()
}

func (w *Writer) insertarSimples(batch []ingest.MovimientoBruto, fechaCarga string) (int, error) {
	grouped := groupByDedupKey(batch)
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

// cuotaConTotalMayorAUno reconoce strings tipo "M/N" donde N>1, sin
// restricción sobre M (cubre tanto la fila informativa "00/N" como las
// cuotas facturadas "01/N", "02/N", ...).
func cuotaConTotalMayorAUno(cuotas string) bool {
	_, n, ok := parseCuotas(cuotas)
	return ok && n > 1
}

// parseCuotas extrae M y N del formato "M/N". Retorna ok=false si el
// formato no es parseable.
func parseCuotas(cuotas string) (m, n int, ok bool) {
	parts := strings.Split(cuotas, "/")
	if len(parts) != 2 {
		return 0, 0, false
	}
	if _, err := fmt.Sscanf(parts[0], "%d", &m); err != nil {
		return 0, 0, false
	}
	if _, err := fmt.Sscanf(parts[1], "%d", &n); err != nil {
		return 0, 0, false
	}
	return m, n, true
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
