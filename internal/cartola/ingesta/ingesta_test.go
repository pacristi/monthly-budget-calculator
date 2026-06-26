package ingesta

import (
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"

	"presupuesto/internal/cartola/ingest"
	sqlitepkg "presupuesto/internal/cartola/sqlite"
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

func TestPersistir_DelegaEnRepositorio(t *testing.T) {
	repo := &fakeRepoMovimientos{insertados: 7}
	batch := []ingest.MovimientoBruto{{Banco: "bchile", Descripcion: "Cafe"}}

	insertados, err := Persistir(batch, repo)
	if err != nil {
		t.Fatalf("Persistir: %v", err)
	}
	if insertados != 7 {
		t.Fatalf("insertados: esperaba 7, obtuve %d", insertados)
	}
	if len(repo.recibidos) != 1 || repo.recibidos[0].Descripcion != "Cafe" {
		t.Fatalf("repo recibió %+v", repo.recibidos)
	}
}

func TestPersistir_PropagaErrorDelRepositorio(t *testing.T) {
	esperado := errors.New("repo caido")
	repo := &fakeRepoMovimientos{err: esperado}

	_, err := Persistir([]ingest.MovimientoBruto{{Banco: "bchile"}}, repo)
	if !errors.Is(err, esperado) {
		t.Fatalf("error: esperaba %v, obtuve %v", esperado, err)
	}
}

func TestDesdeScraper_DelegaSoloMovimientosLiquidados(t *testing.T) {
	jsonPath := writeTempJSON(t, jsonConProvisorio)
	repo := &fakeRepoMovimientos{}

	_, err := DesdeScraper(jsonPath, repo)
	if err != nil {
		t.Fatalf("DesdeScraper: %v", err)
	}
	if len(repo.recibidos) != 2 {
		t.Fatalf("movimientos recibidos: esperaba 2, obtuve %d", len(repo.recibidos))
	}
	for _, m := range repo.recibidos {
		if m.Source == "credit_card_unbilled" {
			t.Fatalf("no debió delegar provisorios: %+v", repo.recibidos)
		}
	}
}

func TestDesdeScraper_PrimeraCarga(t *testing.T) {
	jsonPath := writeTempJSON(t, jsonSintetico)
	_, db := openTempDB(t)
	repo := sqlitepkg.NewWriter(db, "obchile")

	insertados, err := DesdeScraper(jsonPath, repo)
	if err != nil {
		t.Fatalf("DesdeScraper: %v", err)
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

type fakeRepoMovimientos struct {
	recibidos  []ingest.MovimientoBruto
	insertados int
	err        error
}

func (r *fakeRepoMovimientos) GuardarMovimientos(movs []ingest.MovimientoBruto) (int, error) {
	r.recibidos = append([]ingest.MovimientoBruto(nil), movs...)
	return r.insertados, r.err
}

func TestDesdeScraper_EsIdempotente(t *testing.T) {
	jsonPath := writeTempJSON(t, jsonSintetico)
	_, db := openTempDB(t)
	repo := sqlitepkg.NewWriter(db, "obchile")

	if n, err := DesdeScraper(jsonPath, repo); err != nil || n != 3 {
		t.Fatalf("primera corrida: n=%d err=%v", n, err)
	}
	if n, err := DesdeScraper(jsonPath, repo); err != nil || n != 0 {
		t.Fatalf("segunda corrida: n=%d err=%v (esperaba 0)", n, err)
	}

	var total int
	db.QueryRow("SELECT COUNT(*) FROM movimientos").Scan(&total)
	if total != 3 {
		t.Errorf("total tras segunda corrida: esperaba 3, obtuve %d", total)
	}
}

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

func TestDesdeScraper_NoPersisteProvisorio(t *testing.T) {
	jsonPath := writeTempJSON(t, jsonConProvisorio)
	_, db := openTempDB(t)
	repo := sqlitepkg.NewWriter(db, "obchile")

	insertados, err := DesdeScraper(jsonPath, repo)
	if err != nil {
		t.Fatalf("DesdeScraper: %v", err)
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
