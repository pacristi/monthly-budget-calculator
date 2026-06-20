# Canonical ETL Financial Facts Implementation Plan

> **For Claude:** Use `${SUPERPOWERS_SKILLS_ROOT}/skills/collaboration/executing-plans/SKILL.md` to implement this plan task-by-task.

**Goal:** Move bank/source-specific conventions into the ETL layer so SQLite and the budget domain consume explicit canonical financial facts instead of inferring meaning from `source`, `cuotas`, or origin strings.

**Architecture:** Introduce a richer `MovimientoBruto` contract that records canonical facts: payment instrument, settlement state, currency, installment semantics, and amount semantics. Persist those facts in SQLite with a backwards-compatible migration, then update writers and adapters to use explicit columns while keeping existing data readable during migration.

**Tech Stack:** Go, SQLite via `modernc.org/sqlite`, existing `internal/cartola/ingest`, `internal/cartola/sqlite`, `internal/cartola/obchile`, and parser tests.

---

## Design Summary

Current flow:

```text
bank/parser -> MovimientoBruto{Banco, Source, Monto, IsUSD, Cuotas string}
  -> SQLite
  -> adapter infers credit/debit from source string
  -> domain receives presupuesto.Gasto
```

Target flow:

```text
bank/parser -> MovimientoBruto with canonical financial facts
  -> SQLite stores canonical columns
  -> adapter maps columns directly
  -> domain receives presupuesto.Gasto without source-string inference
```

The domain still owns budget decisions: category classification, monthly config, cut date application, monthly installment projection, `mi parte`, and ignored categories.

The ETL owns external interpretation: sign convention, instrument type, settlement state, currency, installment format, whether a row is informational, and whether an amount represents a total purchase or one installment.

Canonical vocabulary:

- `Instrumento`: `cuenta_corriente`, `tarjeta_credito`
- `Estado`: `liquidado`, `provisorio`
- `Moneda`: `CLP`, `USD`
- `MontoRepresenta`: `total`, `cuota`
- `CuotaActual`, `CuotasTotales`: integers; `0/N` means “representative total for an N-installment purchase”

---

### Task 1: Add Canonical Types to the Ingest Module

**Files:**

- Modify: `internal/cartola/ingest/bruto.go`
- Test: add focused compile-time usage in existing parser/writer tests as later tasks

**Step 1: Extend the ingest contract**

Add typed canonical fields while keeping old fields temporarily for compatibility:

```go
type Instrumento string

const (
	InstrumentoCuentaCorriente Instrumento = "cuenta_corriente"
	InstrumentoTarjetaCredito  Instrumento = "tarjeta_credito"
)

type EstadoMovimiento string

const (
	EstadoLiquidado  EstadoMovimiento = "liquidado"
	EstadoProvisorio EstadoMovimiento = "provisorio"
)

type Moneda string

const (
	MonedaCLP Moneda = "CLP"
	MonedaUSD Moneda = "USD"
)

type MontoRepresenta string

const (
	MontoRepresentaTotal MontoRepresenta = "total"
	MontoRepresentaCuota MontoRepresenta = "cuota"
)
```

Then extend `MovimientoBruto`:

```go
type MovimientoBruto struct {
	Banco       string
	Source      string
	Fecha       time.Time
	Monto       float64
	Descripcion string

	Instrumento      Instrumento
	Estado           EstadoMovimiento
	Moneda           Moneda
	MontoRepresenta  MontoRepresenta
	CuotaActual      int
	CuotasTotales    int

	// Deprecated compatibility fields. Remove after all adapters and migrations
	// use the canonical fields.
	IsUSD  bool
	Cuotas string

	Raw map[string]any
}
```

**Step 2: Add helper constructors**

Add helpers to keep parser code compact:

```go
func (m MovimientoBruto) ConDefaultsCanonicos() MovimientoBruto {
	if m.Estado == "" {
		m.Estado = EstadoLiquidado
	}
	if m.Moneda == "" {
		if m.IsUSD {
			m.Moneda = MonedaUSD
		} else {
			m.Moneda = MonedaCLP
		}
	}
	if m.MontoRepresenta == "" {
		m.MontoRepresenta = MontoRepresentaTotal
	}
	if m.CuotasTotales == 0 {
		m.CuotasTotales = 1
	}
	return m
}
```

**Step 3: Verify compile**

Run:

```bash
go test ./...
```

Expected: tests still pass, because existing fields remain available.

**Step 4: Commit**

```bash
git add internal/cartola/ingest/bruto.go
git commit -m "refactor: add canonical financial facts to ingest"
```

---

### Task 2: Add Backwards-Compatible SQLite Migration

**Files:**

- Create: `internal/cartola/sqlite/migrations/002_canonical_financial_facts.sql`
- Modify: `internal/cartola/sqlite/migrator_test.go`

**Step 1: Write failing migration test**

Extend `migrator_test.go` to assert the new columns exist after `Up(db)`:

```go
func TestMigrator_AddsCanonicalFinancialFactColumns(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	if err := Up(db); err != nil {
		t.Fatalf("Up: %v", err)
	}

	rows, err := db.Query(`PRAGMA table_info(movimientos)`)
	if err != nil {
		t.Fatalf("PRAGMA table_info: %v", err)
	}
	defer rows.Close()

	columns := map[string]bool{}
	for rows.Next() {
		var cid int
		var name, typ string
		var notNull int
		var defaultValue any
		var pk int
		if err := rows.Scan(&cid, &name, &typ, &notNull, &defaultValue, &pk); err != nil {
			t.Fatal(err)
		}
		columns[name] = true
	}

	for _, name := range []string{
		"instrumento", "estado", "moneda", "monto_representa",
		"cuota_actual", "cuotas_totales",
	} {
		if !columns[name] {
			t.Fatalf("missing column %s", name)
		}
	}
}
```

**Step 2: Run test and confirm failure**

```bash
go test ./internal/cartola/sqlite -run TestMigrator_AddsCanonicalFinancialFactColumns
```

Expected: fails because columns do not exist.

**Step 3: Add migration**

Create:

```sql
ALTER TABLE movimientos ADD COLUMN instrumento TEXT NOT NULL DEFAULT 'cuenta_corriente';
ALTER TABLE movimientos ADD COLUMN estado TEXT NOT NULL DEFAULT 'liquidado';
ALTER TABLE movimientos ADD COLUMN moneda TEXT NOT NULL DEFAULT 'CLP';
ALTER TABLE movimientos ADD COLUMN monto_representa TEXT NOT NULL DEFAULT 'total';
ALTER TABLE movimientos ADD COLUMN cuota_actual INTEGER NOT NULL DEFAULT 1;
ALTER TABLE movimientos ADD COLUMN cuotas_totales INTEGER NOT NULL DEFAULT 1;

UPDATE movimientos
SET instrumento = CASE
    WHEN lower(source) LIKE '%credito%'
      OR lower(source) LIKE '%credit_card%'
      OR lower(source) LIKE 'tc_%'
    THEN 'tarjeta_credito'
    ELSE 'cuenta_corriente'
END,
moneda = CASE
    WHEN is_usd = 1 THEN 'USD'
    ELSE 'CLP'
END;
```

Do not try to perfectly backfill `cuota_actual` and `cuotas_totales` in SQL yet. The legacy `cuotas` field remains available as fallback during this refactor.

**Step 4: Run SQLite tests**

```bash
go test ./internal/cartola/sqlite
```

Expected: pass.

**Step 5: Commit**

```bash
git add internal/cartola/sqlite/migrations/002_canonical_financial_facts.sql internal/cartola/sqlite/migrator_test.go
git commit -m "refactor: persist canonical financial facts"
```

---

### Task 3: Teach the Writer to Store Canonical Fields

**Files:**

- Modify: `internal/cartola/sqlite/writer.go`
- Modify: `internal/cartola/sqlite/writer_test.go`

**Step 1: Write failing writer test**

Add a test that inserts one canonical credit-card USD movement and reads the stored canonical columns:

```go
func TestInsertarConDedup_PersistsCanonicalFinancialFacts(t *testing.T) {
	w := setupWriter(t)
	fecha := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)

	_, err := w.InsertarConDedup([]ingest.MovimientoBruto{{
		Banco:            "bchile",
		Source:           "legacy-source-for-audit",
		Fecha:            fecha,
		Monto:            -11.08,
		Descripcion:      "NETFLIX.COM",
		Instrumento:      ingest.InstrumentoTarjetaCredito,
		Estado:           ingest.EstadoLiquidado,
		Moneda:           ingest.MonedaUSD,
		MontoRepresenta:  ingest.MontoRepresentaTotal,
		CuotaActual:      1,
		CuotasTotales:    1,
	}})
	if err != nil {
		t.Fatalf("InsertarConDedup: %v", err)
	}

	var instrumento, estado, moneda, montoRepresenta string
	var cuotaActual, cuotasTotales int
	err = w.db.QueryRow(`
		SELECT instrumento, estado, moneda, monto_representa, cuota_actual, cuotas_totales
		FROM movimientos WHERE descripcion = 'NETFLIX.COM'
	`).Scan(&instrumento, &estado, &moneda, &montoRepresenta, &cuotaActual, &cuotasTotales)
	if err != nil {
		t.Fatalf("query: %v", err)
	}

	if instrumento != "tarjeta_credito" || estado != "liquidado" || moneda != "USD" ||
		montoRepresenta != "total" || cuotaActual != 1 || cuotasTotales != 1 {
		t.Fatalf("canonical fields not persisted: %s %s %s %s %d/%d",
			instrumento, estado, moneda, montoRepresenta, cuotaActual, cuotasTotales)
	}
}
```

**Step 2: Confirm failure**

```bash
go test ./internal/cartola/sqlite -run TestInsertarConDedup_PersistsCanonicalFinancialFacts
```

Expected: fails until `insertOne` writes the new columns.

**Step 3: Update `insertOne`**

Call `m = m.ConDefaultsCanonicos()` before persistence and extend the insert statement:

```go
m = m.ConDefaultsCanonicos()
```

Add columns:

```sql
instrumento, estado, moneda, monto_representa, cuota_actual, cuotas_totales
```

Add values:

```go
string(m.Instrumento), string(m.Estado), string(m.Moneda),
string(m.MontoRepresenta), m.CuotaActual, m.CuotasTotales
```

**Step 4: Preserve compatibility fields**

Keep writing `is_usd` and `cuotas` for now. Existing UI/API paths still read them in parts of the code.

**Step 5: Run writer tests**

```bash
go test ./internal/cartola/sqlite
```

Expected: pass.

**Step 6: Commit**

```bash
git add internal/cartola/sqlite/writer.go internal/cartola/sqlite/writer_test.go
git commit -m "refactor: write canonical movement facts"
```

---

### Task 4: Make SQLite Adapter Use Canonical Columns

**Files:**

- Modify: `internal/cartola/sqlite/adapter.go`
- Modify: `internal/cartola/sqlite/adapter_test.go`

**Step 1: Write failing adapter test**

Replace source-string inference with canonical `instrumento`:

```go
func TestAdapter_ObtenerGastosValidos_DetectaCreditoPorInstrumento(t *testing.T) {
	db := setupDB(t)
	insertarMovCanonico(t, db, "2026-05-10", -20000, "COMPRA TC", "weird-source",
		"tarjeta_credito", "CLP", 1, 1)

	adapter := NewAdapter(db, "", nil, "", "", nuevoResolvedorFake(900.0, 25))
	gastos, err := adapter.ObtenerGastosValidos(presupuesto.PeriodoPresupuestario{})
	if err != nil {
		t.Fatalf("ObtenerGastosValidos: %v", err)
	}
	if len(gastos) != 1 {
		t.Fatalf("esperaba 1 gasto, obtuve %d", len(gastos))
	}
	if gastos[0].PoliticaCorte.Tipo != presupuesto.Credito {
		t.Fatalf("esperaba crédito, obtuve %s", gastos[0].PoliticaCorte.Tipo)
	}
}
```

**Step 2: Confirm failure**

```bash
go test ./internal/cartola/sqlite -run TestAdapter_ObtenerGastosValidos_DetectaCreditoPorInstrumento
```

Expected: fails because adapter still reads `source`.

**Step 3: Update query and mapping**

Change the query from:

```sql
SELECT id, fecha, monto, descripcion, source, is_usd, cuotas
```

to:

```sql
SELECT id, fecha, monto, descripcion, instrumento, moneda, cuotas_totales
```

Then map:

```go
if instrumento == string(ingest.InstrumentoTarjetaCredito) {
	tipo = presupuesto.Credito
}
```

Use `cuotasTotales` directly instead of `shared.ParsearCuotas(cuotasStr)`.

**Step 4: Keep fallback for existing DBs during transition**

If migration versioning guarantees columns exist after `Up(db)`, no fallback is needed in app runtime. Tests using direct inserts must be updated to populate canonical columns.

**Step 5: Run adapter tests**

```bash
go test ./internal/cartola/sqlite
```

Expected: pass.

**Step 6: Commit**

```bash
git add internal/cartola/sqlite/adapter.go internal/cartola/sqlite/adapter_test.go
git commit -m "refactor: read budget facts from canonical columns"
```

---

### Task 5: Move Installment Representation out of Writer Origin Checks

**Files:**

- Modify: `internal/cartola/sqlite/writer.go`
- Modify: `internal/cartola/sqlite/writer_test.go`

**Step 1: Write failing tests for amount semantics**

Add two tests:

1. A cartola row where each row is an installment amount:

```go
func TestInsertarConDedup_CuotasMontoRepresentaCuota(t *testing.T) {
	w := setupWriter(t)
	f := time.Date(2026, 5, 2, 0, 0, 0, 0, time.UTC)

	batch := []ingest.MovimientoBruto{
		movCanonicoCuota(f, -22700, "VOLCOM", 1, 6, ingest.MontoRepresentaCuota),
	}
	n, err := w.InsertarConDedup(batch)
	if err != nil || n != 1 {
		t.Fatalf("InsertarConDedup: n=%d err=%v", n, err)
	}

	var monto float64
	var cuotaActual, cuotasTotales int
	err = w.db.QueryRow(`SELECT monto, cuota_actual, cuotas_totales FROM movimientos WHERE descripcion='VOLCOM'`).
		Scan(&monto, &cuotaActual, &cuotasTotales)
	if err != nil {
		t.Fatal(err)
	}
	if monto != -136200 || cuotaActual != 0 || cuotasTotales != 6 {
		t.Fatalf("monto/cuotas mal: %.0f %d/%d", monto, cuotaActual, cuotasTotales)
	}
}
```

2. A scraper row where amount already represents total:

```go
func TestInsertarConDedup_CuotasMontoRepresentaTotal(t *testing.T) {
	w := setupWriter(t)
	f := time.Date(2026, 5, 2, 0, 0, 0, 0, time.UTC)

	n, err := w.InsertarConDedup([]ingest.MovimientoBruto{
		movCanonicoCuota(f, -136200, "VOLCOM", 1, 6, ingest.MontoRepresentaTotal),
	})
	if err != nil || n != 1 {
		t.Fatalf("InsertarConDedup: n=%d err=%v", n, err)
	}

	var monto float64
	err = w.db.QueryRow(`SELECT monto FROM movimientos WHERE descripcion='VOLCOM'`).Scan(&monto)
	if err != nil {
		t.Fatal(err)
	}
	if monto != -136200 {
		t.Fatalf("monto mal: %.0f", monto)
	}
}
```

**Step 2: Confirm failures**

```bash
go test ./internal/cartola/sqlite -run 'MontoRepresenta'
```

Expected: fails until writer uses `MontoRepresenta`.

**Step 3: Replace `origen == "obchile"` logic**

Refactor `construirRepresentanteCompra` to branch on `MontoRepresenta`, not `w.origen`:

```go
func construirRepresentanteCompra(group []ingest.MovimientoBruto) (ingest.MovimientoBruto, bool) {
	group = aplicarDefaults(group)
	switch group[0].MontoRepresenta {
	case ingest.MontoRepresentaTotal:
		return representanteMontoTotal(group)
	case ingest.MontoRepresentaCuota:
		return representanteDesdeCuotas(group)
	default:
		return representanteDesdeCuotas(group)
	}
}
```

When storing representative total rows, set:

```go
base.CuotaActual = 0
base.CuotasTotales = totalCuotas
base.MontoRepresenta = ingest.MontoRepresentaTotal
base.Cuotas = fmt.Sprintf("00/%02d", totalCuotas) // temporary legacy compatibility
```

**Step 4: Run writer tests**

```bash
go test ./internal/cartola/sqlite
```

Expected: pass.

**Step 5: Commit**

```bash
git add internal/cartola/sqlite/writer.go internal/cartola/sqlite/writer_test.go
git commit -m "refactor: use explicit amount semantics for installments"
```

---

### Task 6: Update Banco de Chile XLS Parsers to Emit Canonical Facts

**Files:**

- Modify: `internal/cartola/ingest/xlsx/bchile_cta_corriente.go`
- Modify: `internal/cartola/ingest/xlsx/bchile_tc_nacional.go`
- Modify: `internal/cartola/ingest/xlsx/bchile_tc_internacional.go`
- Modify tests in same package

**Step 1: Update cuenta corriente parser tests**

For account movements assert:

```go
if movs[0].Instrumento != ingest.InstrumentoCuentaCorriente {
	t.Fatalf("instrumento: %s", movs[0].Instrumento)
}
if movs[0].Estado != ingest.EstadoLiquidado {
	t.Fatalf("estado: %s", movs[0].Estado)
}
if movs[0].Moneda != ingest.MonedaCLP {
	t.Fatalf("moneda: %s", movs[0].Moneda)
}
if movs[0].CuotasTotales != 1 {
	t.Fatalf("cuotas: %d", movs[0].CuotasTotales)
}
```

**Step 2: Update TC nacional parser tests**

Assert:

```go
mov.Instrumento == ingest.InstrumentoTarjetaCredito
mov.Estado == ingest.EstadoLiquidado
mov.Moneda == ingest.MonedaCLP
mov.MontoRepresenta == ingest.MontoRepresentaCuota
mov.CuotaActual == 1
mov.CuotasTotales == 3
```

For `00/N` informational rows, preserve current behavior for now: parser may emit them and writer decides no insert if no billed installment exists.

**Step 3: Update TC internacional parser tests**

Assert:

```go
mov.Instrumento == ingest.InstrumentoTarjetaCredito
mov.Estado == ingest.EstadoLiquidado
mov.Moneda == ingest.MonedaUSD
mov.MontoRepresenta == ingest.MontoRepresentaTotal
mov.CuotasTotales == 1
```

**Step 4: Implement parser field population**

Each parser should populate canonical fields directly. Keep `IsUSD` and `Cuotas` in sync until they are removed:

```go
Instrumento: ingest.InstrumentoTarjetaCredito,
Estado: ingest.EstadoLiquidado,
Moneda: ingest.MonedaCLP,
MontoRepresenta: ingest.MontoRepresentaCuota,
CuotaActual: cuotaActual,
CuotasTotales: cuotasTotales,
IsUSD: false,
Cuotas: f.cuotas,
```

**Step 5: Run parser tests**

```bash
go test ./internal/cartola/ingest/xlsx
```

Expected: pass.

**Step 6: Commit**

```bash
git add internal/cartola/ingest/xlsx
git commit -m "refactor: emit canonical facts from bchile parsers"
```

---

### Task 7: Update Open Banking Chile Ingestor and Provisional Adapter

**Files:**

- Modify: `internal/cartola/ingest/obchile/ingestor.go`
- Modify: `internal/cartola/ingest/obchile/ingestor_test.go`
- Modify: `internal/cartola/obchile/adapter.go`
- Modify: `internal/cartola/obchile/adapter_test.go`

**Step 1: Update ingestor tests**

For `source=credit_card_billed`, assert canonical facts:

```go
InstrumentoTarjetaCredito
EstadoLiquidado
```

For `source=account`, assert:

```go
InstrumentoCuentaCorriente
EstadoLiquidado
```

For `source=credit_card_unbilled`, assert it still is not persisted by `Ingestar`.

**Step 2: Implement source-to-facts mapping in ETL**

Add local functions:

```go
func instrumentoDesdeSource(source string) ingest.Instrumento
func estadoDesdeSource(source string) ingest.EstadoMovimiento
func monedaDesdeMonto(monto float64) ingest.Moneda
func cuotasDesdeInstallments(s string) (actual, total int)
```

This keeps Open Banking source strings inside the Open Banking ETL.

**Step 3: Update provisional adapter**

The legacy `obchile.Adapter` still directly maps `current.json` to domain for the provisional layer. Keep its behavior, but replace duplicated source heuristics with the same local Open Banking mapping helpers or a small shared adapter inside `internal/cartola/obchile`.

Do not introduce domain imports into the ETL helpers.

**Step 4: Run tests**

```bash
go test ./internal/cartola/ingest/obchile ./internal/cartola/obchile
```

Expected: pass.

**Step 5: Commit**

```bash
git add internal/cartola/ingest/obchile internal/cartola/obchile
git commit -m "refactor: normalize obchile movement facts in etl"
```

---

### Task 8: Remove Source-Based Credit Inference from SQLite Adapter

**Files:**

- Modify: `internal/cartola/sqlite/adapter.go`
- Modify: `internal/cartola/sqlite/adapter_test.go`

**Step 1: Delete or stop using `esCredito(source string)`**

The adapter should use `instrumento` only.

**Step 2: Add regression test**

Insert a movement with:

```text
source = "totally-arbitrary"
instrumento = "tarjeta_credito"
```

Expected: budget policy is credit.

Insert another with:

```text
source = "credit_card_but_account"
instrumento = "cuenta_corriente"
```

Expected: budget policy is debit.

**Step 3: Run focused test**

```bash
go test ./internal/cartola/sqlite -run 'Instrumento|Credito'
```

Expected: pass.

**Step 4: Run all tests**

```bash
go test ./...
```

Expected: pass.

**Step 5: Commit**

```bash
git add internal/cartola/sqlite/adapter.go internal/cartola/sqlite/adapter_test.go
git commit -m "refactor: remove source-based credit inference"
```

---

### Task 9: Update CLI Help and README

**Files:**

- Modify: `README.md`
- Modify: `cmd/presupuesto-cli/main.go`

**Step 1: Update wording**

Clarify that adding a bank means implementing an ETL parser that emits canonical financial facts:

```text
Cada parser bancario traduce su cartola a MovimientoBruto canónico:
cargos negativos, abonos positivos, instrumento, estado, moneda, cuotas y
semántica del monto. SQLite y el dashboard no deberían inferir estas cosas
desde strings del banco.
```

**Step 2: Update CLI help**

Keep current flags, but make `--tipo` language canonical:

```text
Tipo de instrumento/fuente: cta-corriente | tc-nacional | tc-internacional
```

**Step 3: Run docs-adjacent tests**

```bash
go test ./...
```

Expected: pass.

**Step 4: Commit**

```bash
git add README.md cmd/presupuesto-cli/main.go
git commit -m "docs: describe canonical etl contract"
```

---

### Task 10: Verify Against Real Data Without Touching Production

**Files:**

- Use existing local cartolas under `/Users/pierocristi/Documents/pers/Cartolas Banco de Chile`
- Use `.context/diagnose-cartolas/` for temporary DBs

**Step 1: Copy production DB locally**

Use the existing SSH key:

```bash
mkdir -p .context/diagnose-cartolas
scp -o BatchMode=yes -o StrictHostKeyChecking=no -i ~/.ssh/vmcasa_ed25519 \
  pierocristi@34.135.140.222:/home/pierocristi/projects/monthly-budget-calculator/data/movimientos.db \
  .context/diagnose-cartolas/refactor-probe.db
```

**Step 2: Run migration locally**

Any command that opens the DB through `sqlite.Up` should apply migrations. For explicit verification:

```bash
go run ./cmd/presupuesto-cli sqlite init --db .context/diagnose-cartolas/refactor-probe.db
```

Expected: no data loss; schema gets new columns.

**Step 3: Reingest cartolas on the local copy**

```bash
base="/Users/pierocristi/Documents/pers/Cartolas Banco de Chile"
go run ./cmd/presupuesto-cli ingestar xlsx --banco bchile --tipo cta-corriente --año 2026 --dir "$base/Cuenta Corriente/2026" --db .context/diagnose-cartolas/refactor-probe.db
go run ./cmd/presupuesto-cli ingestar xlsx --banco bchile --tipo tc-nacional --dir "$base/Tarjeta de Credito/nacional/2026" --db .context/diagnose-cartolas/refactor-probe.db
go run ./cmd/presupuesto-cli ingestar xlsx --banco bchile --tipo tc-internacional --dir "$base/Tarjeta de Credito/Internacional/2026" --db .context/diagnose-cartolas/refactor-probe.db
```

Expected: no duplicate explosion; inserted counts should be explainable.

**Step 4: Query canonical column coverage**

```bash
sqlite3 .context/diagnose-cartolas/refactor-probe.db "
SELECT instrumento, estado, moneda, monto_representa, COUNT(*)
FROM movimientos
GROUP BY instrumento, estado, moneda, monto_representa
ORDER BY 1,2,3,4;"
```

Expected:

- account movements: `cuenta_corriente/liquidado/CLP/total`
- national credit card: `tarjeta_credito/liquidado/CLP/total` after writer representative rows
- international credit card: `tarjeta_credito/liquidado/USD/total`

**Step 5: Run full test suite**

```bash
go test ./...
```

Expected: pass.

**Step 6: Commit**

No commit unless this task uncovered code/docs changes. If changes were needed:

```bash
git add <files>
git commit -m "test: verify canonical etl against real cartolas"
```

---

## Follow-Up Cleanup After Confidence

Do these only after production has run successfully with canonical columns:

1. Remove `IsUSD` and `Cuotas` from `MovimientoBruto`.
2. Stop writing legacy `is_usd` and `cuotas` columns, or keep them only as generated/compat fields.
3. Consider replacing `source` with `fuente_externa` or `external_source` to make its audit-only role explicit.
4. Move description canonicalization into a configurable dedup policy if another bank exposes incompatible description patterns.

---

## Risks

- The migration backfills old rows conservatively. Old installment rows may still rely on legacy `cuotas` until the adapter and writer are fully switched.
- Dedup remains partly description-based. This is independent from the canonical facts refactor and should be treated as a separate deepening candidate if another bank exposes unstable descriptions.
- The provisional `current.json` adapter still maps directly to domain. It should use the same Open Banking normalization helpers to avoid reintroducing source-string conventions.

---

## Acceptance Criteria

- `go test ./...` passes.
- SQLite adapter no longer decides credit/debit from `source`.
- Writer no longer decides amount semantics from `origen == "obchile"`.
- Banco de Chile parsers emit canonical facts explicitly.
- Open Banking ingestor maps `source` strings inside the ETL boundary.
- Existing real cartolas can be reingested on a DB copy without unexplained duplicate growth.
