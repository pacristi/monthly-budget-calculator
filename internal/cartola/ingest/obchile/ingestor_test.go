package obchile

import (
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"

	sqlitepkg "github.com/pierocristi/monthly-budget-calculator/internal/cartola/sqlite"
)

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
      {"date": "15-05-2026", "description": "TRASPASO A:Test", "amount": -10000, "source": "cta_corriente", "installments": ""},
      {"date": "14-05-2026", "description": "SUELDO", "amount": 1500000, "source": "cta_corriente", "installments": ""}
    ]
  }],
  "creditCards": [{
    "label": "Visa",
    "movements": [
      {"date": "13-05-2026", "description": "Starbucks", "amount": -3500, "source": "tarjeta_credito_visa", "installments": "1/3"}
    ]
  }]
}`

func TestIngestar_PrimeraCarga(t *testing.T) {
	jsonPath := writeTempJSON(t, jsonSintetico)
	dbPath, db := openTempDB(t)

	insertados, err := Ingestar(jsonPath, dbPath)
	if err != nil {
		t.Fatalf("Ingestar: %v", err)
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

	// Verificar que origen es "obchile" y raw está poblado para el café
	var origen, raw string
	err = db.QueryRow(`SELECT origen, raw FROM movimientos
		WHERE descripcion = 'Starbucks'`).Scan(&origen, &raw)
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
	if rawMap["source"] != "tarjeta_credito_visa" {
		t.Errorf("raw.source: esperaba 'tarjeta_credito_visa', obtuve %v", rawMap["source"])
	}
}

func TestIngestar_EsIdempotente(t *testing.T) {
	jsonPath := writeTempJSON(t, jsonSintetico)
	dbPath, db := openTempDB(t)

	if n, err := Ingestar(jsonPath, dbPath); err != nil || n != 3 {
		t.Fatalf("primera corrida: n=%d err=%v", n, err)
	}
	if n, err := Ingestar(jsonPath, dbPath); err != nil || n != 0 {
		t.Fatalf("segunda corrida: n=%d err=%v (esperaba 0)", n, err)
	}

	var total int
	db.QueryRow("SELECT COUNT(*) FROM movimientos").Scan(&total)
	if total != 3 {
		t.Errorf("total tras segunda corrida: esperaba 3, obtuve %d", total)
	}
}
