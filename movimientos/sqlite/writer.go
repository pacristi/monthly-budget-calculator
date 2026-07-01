package sqlite

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"
	"unicode"

	parserObcl "presupuesto/ingesta/open-banking-chile"
	"presupuesto/movimientos"
)

// querier es el subconjunto de *sql.DB / *sql.Tx que el writer necesita.
// Permite que los métodos de inserción/consulta corran indistintamente
// contra la conexión suelta o contra una transacción.
type querier interface {
	Exec(query string, args ...any) (sql.Result, error)
	Query(query string, args ...any) (*sql.Rows, error)
	QueryRow(query string, args ...any) *sql.Row
}

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

// GuardarMovimientos persiste movimientos canónicos en sqlite.
func (w *Writer) GuardarMovimientos(batch []movimientos.MovimientoBruto) (int, error) {
	return w.InsertarConDedup(batch)
}

// InsertarConDedup inserta los movimientos del batch evitando duplicaciones,
// dentro de una única transacción:
//
//  1. DELETE FROM movimientos WHERE source LIKE '%unbilled%' — el snapshot
//     provisorio anterior se descarta entero. Los unbilled driftean entre
//     corridas y no tienen ID propio, así que la única forma consistente
//     de mantenerlos al día es reemplazarlos completos en cada ingesta. El
//     DELETE corre antes del dedup de liquidados para que el conteo no
//     considere filas que ya van a desaparecer.
//
//  2. Los movimientos liquidados (no provisorios) del batch se insertan con
//     el dedup de dos pasadas existente:
//
//     a. Compras en cuotas (CuotasTotales > 1): se agrupan por
//     (banco, source, fecha, descripcion, N) — sin el monto, porque el
//     banco a veces ajusta ±1 peso entre las últimas cuotas. De cada
//     grupo se inserta UNA sola fila con el monto total de la compra; si
//     ya hay alguna fila con esa llave en BD, se omite.
//
//     b. Movimientos simples (sin cuotas o CuotasTotales <= 1): dedup
//     count-based por la llave clásica (banco, source, fecha, monto,
//     descripcion). El batch puede tener M y la BD N; se inserta
//     max(0, M-N). Cubre el caso "doble café".
//
//  3. Los movimientos provisorios (unbilled) del batch se insertan directo,
//     sin dedup: tras el paso 1 la tabla no tiene provisorios, y la
//     garantía de no-duplicados para ellos la da el scraper (billed xor
//     unbilled por snapshot), no el writer.
//
// Si cualquier paso falla, se hace rollback completo.
// Retorna la cantidad de filas efectivamente insertadas.
func (w *Writer) InsertarConDedup(batch []movimientos.MovimientoBruto) (int, error) {
	fechaCarga := time.Now().UTC().Format(time.RFC3339)
	total := 0

	tx, err := w.db.Begin()
	if err != nil {
		return 0, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`DELETE FROM movimientos WHERE source LIKE '%unbilled%'`); err != nil {
		return 0, fmt.Errorf("borrando unbilled previos: %w", err)
	}

	liquidados, provisorios := separarPorProvisorio(batch)

	enCuotas, simples := separarPorTipoDeCuota(liquidados)

	n, err := w.insertarCompraEnCuotas(tx, enCuotas, fechaCarga)
	if err != nil {
		return total, err
	}
	total += n

	n, err = w.insertarSimples(tx, simples, fechaCarga)
	if err != nil {
		return total, err
	}
	total += n

	n, err = w.insertarProvisorios(tx, provisorios, fechaCarga)
	if err != nil {
		return total, err
	}
	total += n

	if err := tx.Commit(); err != nil {
		return total, fmt.Errorf("commit tx: %w", err)
	}

	return total, nil
}

// separarPorProvisorio divide el batch en movimientos liquidados
// (facturados) y provisorios (unbilled), según parserObcl.EsProvisorio.
func separarPorProvisorio(batch []movimientos.MovimientoBruto) (liquidados, provisorios []movimientos.MovimientoBruto) {
	for _, m := range batch {
		if parserObcl.EsProvisorio(m.Source) {
			provisorios = append(provisorios, m)
		} else {
			liquidados = append(liquidados, m)
		}
	}
	return
}

// insertarProvisorios inserta los movimientos unbilled del batch sin pasar
// por dedup: tras el DELETE previo la tabla no tiene provisorios, y la
// garantía de no-duplicados la da el scraper, no el writer.
func (w *Writer) insertarProvisorios(q querier, batch []movimientos.MovimientoBruto, fechaCarga string) (int, error) {
	total := 0
	for _, m := range batch {
		if err := w.insertOne(q, m, fechaCarga); err != nil {
			return total, fmt.Errorf("insert provisorio: %w", err)
		}
		total++
	}
	return total, nil
}

// separarPorTipoDeCuota divide el batch en dos: filas que pertenecen a
// una compra en N>1 cuotas y filas simples.
func separarPorTipoDeCuota(batch []movimientos.MovimientoBruto) (enCuotas, simples []movimientos.MovimientoBruto) {
	for _, m := range batch {
		if m.CuotasTotales > 1 {
			enCuotas = append(enCuotas, m)
		} else {
			simples = append(simples, m)
		}
	}
	return
}

type cuotaCompraKey struct {
	banco, fecha, descripcionNorm string
	totalCuotas                   int
}

// cuotaKeyOf agrupa por (banco, fecha, descripcion_normalizada, total_cuotas).
// No incluye source (mismas razones que keyOf) ni monto (ajustes ±1 peso del
// banco).
func cuotaKeyOf(m movimientos.MovimientoBruto) cuotaCompraKey {
	return cuotaCompraKey{
		banco:           m.Banco,
		fecha:           m.Fecha.Format("2006-01-02"),
		descripcionNorm: descripcionCanonica(m.Descripcion),
		totalCuotas:     m.CuotasTotales,
	}
}

func (w *Writer) insertarCompraEnCuotas(q querier, batch []movimientos.MovimientoBruto, fechaCarga string) (int, error) {
	grupos := map[cuotaCompraKey][]movimientos.MovimientoBruto{}
	for _, m := range batch {
		k := cuotaKeyOf(m)
		grupos[k] = append(grupos[k], m)
	}

	total := 0
	for k, group := range grupos {
		yaExiste, err := w.compraEnCuotasYaEnBD(q, k)
		if err != nil {
			return total, fmt.Errorf("chequeando compra en cuotas (%s,%s,%s,N=%d): %w",
				k.banco, k.fecha, k.descripcionNorm, k.totalCuotas, err)
		}
		if yaExiste {
			continue
		}
		representante, ok, err := construirRepresentanteCompra(group)
		if err != nil {
			return total, fmt.Errorf("construyendo representante compra en cuotas (%s,%s,%s,N=%d): %w",
				k.banco, k.fecha, k.descripcionNorm, k.totalCuotas, err)
		}
		if !ok {
			// Solo había filas "00/N" (informativas, sin facturar). No
			// insertamos nada todavía; cuando aparezca al menos una cuota
			// facturada cargaremos la compra.
			continue
		}
		if err := w.insertOne(q, representante, fechaCarga); err != nil {
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
// Convenciones distintas según el hecho canónico MontoRepresenta:
//   - cuota: cada fila tiene monto = CUOTA individual. Total = suma de cuotas
//     facturadas (o estimación promedio * N si solo conocemos algunas).
//   - total: cada movimiento ya tiene monto = TOTAL de la compra. No se
//     multiplica; se usa tal cual.
//
// Las filas con CuotaActual=0 son informativas (sin facturar) y se ignoran.
//
// Retorna (representante, true, nil) si pudo construirla, (zero, false, nil)
// si el grupo no contiene cuotas que aporten monto (todas CuotaActual=0).
func construirRepresentanteCompra(group []movimientos.MovimientoBruto) (movimientos.MovimientoBruto, bool, error) {
	for _, m := range group {
		if m.CuotaActual == 0 {
			continue
		}
		if m.MontoRepresenta == movimientos.MontoRepresentaTotal {
			base := m
			base.Cuotas = fmt.Sprintf("00/%02d", m.CuotasTotales)
			return base, true, nil
		}
	}

	var base movimientos.MovimientoBruto
	baseM := -1
	var sumCuotas float64
	cuotasFacturadas := 0
	var totalCuotas int

	for _, m := range group {
		if m.MontoRepresenta != movimientos.MontoRepresentaCuota {
			if m.CuotaActual > 0 && m.MontoRepresenta == "" {
				return movimientos.MovimientoBruto{}, false, fmt.Errorf("movimiento en cuotas sin MontoRepresenta: fecha=%s descripcion=%q cuota=%d/%d",
					m.Fecha.Format("2006-01-02"), m.Descripcion, m.CuotaActual, m.CuotasTotales)
			}
			continue
		}
		totalCuotas = m.CuotasTotales
		if m.CuotaActual == 0 {
			continue // informativa, no factura
		}
		sumCuotas += m.Monto
		cuotasFacturadas++
		if baseM == -1 || m.CuotaActual < baseM {
			baseM = m.CuotaActual
			base = m
		}
	}

	if cuotasFacturadas == 0 {
		return movimientos.MovimientoBruto{}, false, nil
	}

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
	return base, true, nil
}

func (w *Writer) compraEnCuotasYaEnBD(q querier, k cuotaCompraKey) (bool, error) {
	// Llave (banco, fecha, descripcion_normalizada, N_total). Sin source
	// (xlsx/scraper difieren) ni monto (±1 peso de ajuste). La comparación
	// de descripción se hace en Go por la limitación unicode de sqlite UPPER.
	rows, err := q.Query(`SELECT descripcion, cuotas FROM movimientos
		WHERE banco = ? AND fecha = ?`,
		k.banco, k.fecha,
	)
	if err != nil {
		return false, err
	}
	defer rows.Close()
	for rows.Next() {
		var d, c string
		if err := rows.Scan(&d, &c); err != nil {
			return false, err
		}
		if descripcionCanonica(d) != k.descripcionNorm {
			continue
		}
		if cuotaPersistidaTieneTotal(c, k.totalCuotas) {
			return true, nil
		}
	}
	return false, rows.Err()
}

func cuotaPersistidaTieneTotal(cuotas string, total int) bool {
	if cuotas == fmt.Sprintf("00/%02d", total) {
		return true
	}
	_, n, ok := parseCuotasPersistidas(cuotas)
	return ok && n == total
}

// parseCuotasPersistidas tolera formatos legacy ya guardados en sqlite. El
// writer no lo usa para interpretar movimientos nuevos del batch.
func parseCuotasPersistidas(cuotas string) (m, n int, ok bool) {
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

func (w *Writer) insertarSimples(q querier, batch []movimientos.MovimientoBruto, fechaCarga string) (int, error) {
	grouped := groupByDedupKey(batch)
	total := 0
	for _, group := range grouped {
		first := group[0]
		dbCount, err := w.countByKey(q, first)
		if err != nil {
			return total, fmt.Errorf("count para llave (%s,%s,%v,%s): %w",
				first.Banco, first.Fecha.Format("2006-01-02"), first.Monto, first.Descripcion, err)
		}
		toInsert := len(group) - dbCount
		if toInsert <= 0 {
			continue
		}
		for _, m := range group[:toInsert] {
			if err := w.insertOne(q, m, fechaCarga); err != nil {
				return total, fmt.Errorf("insert: %w", err)
			}
			total++
		}
	}
	return total, nil
}

type dedupKey struct {
	banco, fecha, descripcionNorm string
	monto                         float64
}

// keyOf construye la llave de dedup. Intencionalmente NO incluye `source`
// porque la misma compra llega a sqlite con sources distintos según
// la fuente (xlsx usa "tc_nacional", scraper usa "credit_card_billed";
// xlsx usa "cta_corriente", scraper usa "account"). La descripción se
// normaliza con TRIM+UPPER para tolerar variaciones de casing entre
// fuentes (xlsx en mayúsculas, scraper en Title Case).
func keyOf(m movimientos.MovimientoBruto) dedupKey {
	return dedupKey{
		banco:           m.Banco,
		fecha:           m.Fecha.Format("2006-01-02"),
		monto:           m.Monto,
		descripcionNorm: descripcionCanonica(m.Descripcion),
	}
}

// descripcionCanonica normaliza la descripción para la llave de dedup.
// Reemplaza caracteres no alfanuméricos por espacios y elimina palabras
// de relleno típicas que los bancos añaden cuando la transacción pasa a
// estado facturado, para evitar falsos duplicados.
// La descripción almacenada en BD mantiene su casing original.
func descripcionCanonica(s string) string {
	var b strings.Builder
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || unicode.IsSpace(r) {
			b.WriteRune(r)
		} else {
			b.WriteRune(' ')
		}
	}

	cleaned := strings.ToUpper(strings.TrimSpace(b.String()))
	fields := strings.Fields(cleaned)

	fillers := map[string]bool{
		"COMPRAS":  true,
		"SANTIAGO": true,
		"CL":       true,
		"INT":      true,
		"VI":       true,
		"APP":      true,
	}

	var out []string
	for _, f := range fields {
		if !fillers[f] {
			out = append(out, f)
		}
	}
	return strings.Join(out, " ")
}

// DescripcionCanonicaExported is exported for the cleanup script.
func DescripcionCanonicaExported(s string) string {
	return descripcionCanonica(s)
}

func groupByDedupKey(batch []movimientos.MovimientoBruto) map[dedupKey][]movimientos.MovimientoBruto {
	out := map[dedupKey][]movimientos.MovimientoBruto{}
	for _, m := range batch {
		k := keyOf(m)
		out[k] = append(out[k], m)
	}
	return out
}

func (w *Writer) countByKey(q querier, m movimientos.MovimientoBruto) (int, error) {
	// La comparación de descripción canónica se hace en Go: sqlite UPPER
	// no normaliza caracteres unicode (UPPER("Café") devuelve "CAFé") y
	// quedaríamos vulnerables a falsos negativos. Traemos las filas que
	// coinciden en (banco, fecha, monto) y comparamos en memoria.
	rows, err := q.Query(`SELECT descripcion FROM movimientos
		WHERE banco = ? AND fecha = ? AND monto = ?`,
		m.Banco, m.Fecha.Format("2006-01-02"), m.Monto,
	)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	target := descripcionCanonica(m.Descripcion)
	count := 0
	for rows.Next() {
		var d string
		if err := rows.Scan(&d); err != nil {
			return 0, err
		}
		if descripcionCanonica(d) == target {
			count++
		}
	}
	return count, rows.Err()
}

func (w *Writer) insertOne(q querier, m movimientos.MovimientoBruto, fechaCarga string) error {
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
	if m.Instrumento == movimientos.InstrumentoDesconocido {
		return fmt.Errorf("movimiento sin Instrumento: fecha=%s descripcion=%q source=%q",
			m.Fecha.Format("2006-01-02"), m.Descripcion, m.Source)
	}
	if m.Moneda == "" {
		return fmt.Errorf("movimiento sin Moneda: fecha=%s descripcion=%q source=%q",
			m.Fecha.Format("2006-01-02"), m.Descripcion, m.Source)
	}
	if m.CuotasTotales <= 0 {
		return fmt.Errorf("movimiento con CuotasTotales inválido: fecha=%s descripcion=%q source=%q cuotas_totales=%d",
			m.Fecha.Format("2006-01-02"), m.Descripcion, m.Source, m.CuotasTotales)
	}
	_, err = q.Exec(`INSERT INTO movimientos
		(banco, source, fecha, monto, descripcion, is_usd, cuotas, raw, origen, fecha_carga, instrumento, moneda, cuotas_totales)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		m.Banco, m.Source, m.Fecha.Format("2006-01-02"), m.Monto, m.Descripcion,
		isUSD, m.Cuotas, string(rawJSON), w.origen, fechaCarga, string(m.Instrumento), string(m.Moneda), m.CuotasTotales,
	)
	return err
}
