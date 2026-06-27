package fuentes

import (
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"

	"presupuesto/internal/cartola/ingesta"
	sqlitepkg "presupuesto/internal/cartola/sqlite"
)

func TestOpenBankingChile_NoEntregaProvisorios(t *testing.T) {
	jsonPath := writeTempJSON(t, jsonConProvisorio)

	movs, err := NuevaOpenBankingChile(jsonPath).LeerMovimientos()
	if err != nil {
		t.Fatalf("LeerMovimientos: %v", err)
	}
	if len(movs) != 2 {
		t.Fatalf("movimientos: esperaba 2, obtuve %d", len(movs))
	}
	for _, m := range movs {
		if m.Source == "credit_card_unbilled" {
			t.Fatalf("no debió entregar provisorios: %+v", movs)
		}
	}
}

func TestOpenBankingChile_PrimeraCarga(t *testing.T) {
	jsonPath := writeTempJSON(t, jsonSintetico)
	_, db := openTempDB(t)
	repo := sqlitepkg.NewWriter(db, "obchile")

	insertados, err := ingesta.DesdeFuente(NuevaOpenBankingChile(jsonPath), repo)
	if err != nil {
		t.Fatalf("DesdeFuente: %v", err)
	}
	if insertados != 3 {
		t.Errorf("esperaba 3 insertados, obtuve %d", insertados)
	}

	var total int
	if err := db.QueryRow("SELECT COUNT(*) FROM movimientos").Scan(&total); err != nil {
		t.Fatalf("count: %v", err)
	}
	if total != 3 {
		t.Errorf("esperaba 3 filas en BD, obtuve %d", total)
	}

	var origen, raw string
	err = db.QueryRow(`SELECT origen, raw FROM movimientos WHERE descripcion = 'Starbucks'`).Scan(&origen, &raw)
	if err != nil {
		t.Fatalf("query starbucks: %v", err)
	}
	if origen != "obchile" {
		t.Errorf("origen: esperaba 'obchile', obtuve %q", origen)
	}
	var rawMap map[string]any
	if err := json.Unmarshal([]byte(raw), &rawMap); err != nil {
		t.Fatalf("unmarshal raw: %v", err)
	}
	if rawMap["installments"] != "1/3" {
		t.Errorf("raw.installments: esperaba '1/3', obtuve %v", rawMap["installments"])
	}
	if rawMap["source"] != "credit_card_billed" {
		t.Errorf("raw.source: esperaba 'credit_card_billed', obtuve %v", rawMap["source"])
	}
}

func TestOpenBankingChile_EsIdempotente(t *testing.T) {
	jsonPath := writeTempJSON(t, jsonSintetico)
	_, db := openTempDB(t)
	repo := sqlitepkg.NewWriter(db, "obchile")
	fuente := NuevaOpenBankingChile(jsonPath)

	if n, err := ingesta.DesdeFuente(fuente, repo); err != nil || n != 3 {
		t.Fatalf("primera corrida: n=%d err=%v", n, err)
	}
	if n, err := ingesta.DesdeFuente(fuente, repo); err != nil || n != 0 {
		t.Fatalf("segunda corrida: n=%d err=%v (esperaba 0)", n, err)
	}

	var total int
	db.QueryRow("SELECT COUNT(*) FROM movimientos").Scan(&total)
	if total != 3 {
		t.Errorf("total tras segunda corrida: esperaba 3, obtuve %d", total)
	}
}

func TestOpenBankingChile_NoPersisteProvisorio(t *testing.T) {
	jsonPath := writeTempJSON(t, jsonConProvisorio)
	_, db := openTempDB(t)
	repo := sqlitepkg.NewWriter(db, "obchile")

	insertados, err := ingesta.DesdeFuente(NuevaOpenBankingChile(jsonPath), repo)
	if err != nil {
		t.Fatalf("DesdeFuente: %v", err)
	}
	if insertados != 2 {
		t.Errorf("esperaba 2 insertados (unbilled excluido), obtuve %d", insertados)
	}

	var unbilled int
	if err := db.QueryRow(
		`SELECT COUNT(*) FROM movimientos WHERE source = 'credit_card_unbilled'`,
	).Scan(&unbilled); err != nil {
		t.Fatalf("count unbilled: %v", err)
	}
	if unbilled != 0 {
		t.Errorf("esperaba 0 filas unbilled en BD, obtuve %d", unbilled)
	}
}

func TestNuevaCartolaXLSX_ValidaRegistro(t *testing.T) {
	if _, err := NuevaCartolaXLSX("bchile", "tc-nacional", 0, t.TempDir()); err != nil {
		t.Fatalf("NuevaCartolaXLSX: %v", err)
	}
	if _, err := NuevaCartolaXLSX("otro", "tc-nacional", 0, t.TempDir()); err == nil {
		t.Fatal("esperaba error para banco no registrado")
	}
}

func writeTempJSON(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, "current.json")
	if err := os.WriteFile(p, []byte(content), 0644); err != nil {
		t.Fatalf("escribiendo JSON temporal: %v", err)
	}
	return p
}

func openTempDB(t *testing.T) (string, *sql.DB) {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	if err := sqlitepkg.Up(db); err != nil {
		t.Fatalf("sqlitepkg.Up: %v", err)
	}
	return dbPath, db
}

const jsonSintetico = `{
  "success": true,
  "bank": "bchile",
  "movements": [],
  "accounts": [{
    "balance": 100000,
    "movements": [
      {"date": "15-05-2026", "description": "TRASPASO A:Test", "amount": -10000, "source": "account", "installments": ""},
      {"date": "14-05-2026", "description": "SUELDO", "amount": 1500000, "source": "account", "installments": ""}
    ]
  }],
  "creditCards": [{
    "label": "Visa",
    "movements": [
      {"date": "13-05-2026", "description": "Starbucks", "amount": -3500, "source": "credit_card_billed", "installments": "1/3"}
    ]
  }]
}`

const jsonConProvisorio = `{
  "success": true,
  "bank": "bchile",
  "movements": [],
  "accounts": [{
    "balance": 100000,
    "movements": [
      {"date": "14-05-2026", "description": "SUELDO", "amount": 1500000, "source": "account", "installments": ""}
    ]
  }],
  "creditCards": [{
    "label": "Visa",
    "movements": [
      {"date": "10-05-2026", "description": "RESTORANT FACTURADO", "amount": -20000, "source": "credit_card_billed", "installments": ""},
      {"date": "13-05-2026", "description": "STARBUCKS PROVISORIO", "amount": -3500, "source": "credit_card_unbilled", "installments": ""}
    ]
  }]
}`
